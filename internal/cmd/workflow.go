package cmd

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	langsmith "github.com/langchain-ai/langsmith-go"
)

// Workflow commands (`investigate`, `verify`) run a fixed sequence of SDK calls
// and return one compact result, so an agent makes one call and one decision
// instead of chaining list/get/stats commands itself. This file holds what the
// workflows share: time windows, window health, and grouping failed runs into
// clusters by error.

// defaultFailureSample is how many of the newest failed runs a workflow reads
// per window. Clusters are built from this sample, so counts are lower bounds
// when a window has more failures than this.
const defaultFailureSample = 200

// timeWindow is a half-open [Start, End) interval.
type timeWindow struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

func (w timeWindow) duration() time.Duration { return w.End.Sub(w.Start) }

// queryEnd is the end time sent to the API. The runs and stats queries treat
// their end bound as inclusive, so a run starting exactly at End would fall in
// this window and the next. Stopping one microsecond short, the precision runs
// are stored at, keeps adjacent windows from sharing a run.
func (w timeWindow) queryEnd() time.Time { return w.End.Add(-time.Microsecond) }

// parseWindowDuration parses a duration such as 90m, 24h, 7d or 2w. Go's
// time.ParseDuration has no day or week unit, which are the natural units for
// an incident window.
func parseWindowDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	unit := map[byte]time.Duration{'d': 24 * time.Hour, 'w': 7 * 24 * time.Hour}
	if mult, ok := unit[s[len(s)-1]]; ok {
		n, err := strconv.ParseFloat(s[:len(s)-1], 64)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid duration %q: use a positive number followed by m, h, d or w (e.g. 24h, 7d)", s)
		}
		return time.Duration(n * float64(mult)), nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("invalid duration %q: use a positive number followed by m, h, d or w (e.g. 24h, 7d)", s)
	}
	return d, nil
}

// parseWorkflowTime parses an absolute timestamp (RFC 3339, with or without a
// zone, or YYYY-MM-DD). It returns an error instead of exiting so the workflow
// commands can report it through RunE.
func parseWorkflowTime(flag, s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid %s %q: use RFC 3339 (2026-01-15T10:00:00Z) or YYYY-MM-DD", flag, s)
}

// humanDuration renders a window length the way people say it: 90m, 24h, 7d.
func humanDuration(d time.Duration) string {
	switch {
	case d >= 48*time.Hour && d%(24*time.Hour) == 0:
		return fmt.Sprintf("%dd", d/(24*time.Hour))
	case d >= time.Hour && d%time.Hour == 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	case d >= time.Minute:
		return fmt.Sprintf("%dm", int(d.Round(time.Minute)/time.Minute))
	default:
		return d.Round(time.Second).String()
	}
}

// windowHealth is the root-trace health of one window.
type windowHealth struct {
	Traces     int64   `json:"traces"`
	ErrorRate  float64 `json:"error_rate"`
	LatencyP50 float64 `json:"latency_p50_s"`
	LatencyP99 float64 `json:"latency_p99_s"`
	TotalCost  float64 `json:"total_cost"`
}

var windowHealthSelect = []langsmith.RunStatsQueryParamsSelect{
	langsmith.RunStatsQueryParamsSelectRunCount,
	langsmith.RunStatsQueryParamsSelectErrorRate,
	langsmith.RunStatsQueryParamsSelectLatencyP50,
	langsmith.RunStatsQueryParamsSelectLatencyP99,
	langsmith.RunStatsQueryParamsSelectTotalCost,
}

// fetchWindowHealth reads root-trace stats for one window.
func fetchWindowHealth(ctx context.Context, c *client.Client, sessionID string, w timeWindow, filter string) (windowHealth, error) {
	s, err := fetchStats(ctx, c, langsmith.RunStatsQueryParams{
		Session:   langsmith.F([]string{sessionID}),
		IsRoot:    langsmith.F(true),
		StartTime: langsmith.F(w.Start),
		EndTime:   langsmith.F(w.queryEnd()),
		Select:    langsmith.F(windowHealthSelect),
	}, filter)
	if err != nil {
		return windowHealth{}, err
	}
	return windowHealth{Traces: s.RunCount, ErrorRate: s.ErrorRate, LatencyP50: s.LatencyP50, LatencyP99: s.LatencyP99, TotalCost: s.TotalCost}, nil
}

// stepCounts is how many runs of one step ran, and how many of them errored, in a window.
type stepCounts struct {
	Runs   int64
	Errors int64
}

// stepKey names a step the way failure clusters do: by run name and run type,
// so a tool and a chain that share a name are counted apart.
type stepKey struct {
	Name    string
	RunType string
}

// fetchStepCounts counts runs at any depth of one step in one window, and how
// many of them errored. The error count is exact, unlike counts taken from a
// failure sample.
func fetchStepCounts(ctx context.Context, c *client.Client, sessionID string, step stepKey, w timeWindow, filter string) (stepCounts, error) {
	stepFilter := fmt.Sprintf("eq(name, %q)", step.Name)
	if step.RunType != "" {
		stepFilter = joinFilters(stepFilter, fmt.Sprintf("eq(run_type, %q)", step.RunType))
	}
	s, err := fetchStats(ctx, c, langsmith.RunStatsQueryParams{
		Session:   langsmith.F([]string{sessionID}),
		StartTime: langsmith.F(w.Start),
		EndTime:   langsmith.F(w.queryEnd()),
		Select: langsmith.F([]langsmith.RunStatsQueryParamsSelect{
			langsmith.RunStatsQueryParamsSelectRunCount,
			langsmith.RunStatsQueryParamsSelectErrorRate,
		}),
	}, joinFilters(stepFilter, filter))
	if err != nil {
		return stepCounts{}, err
	}
	return stepCounts{Runs: s.RunCount, Errors: int64(math.Round(float64(s.RunCount) * s.ErrorRate))}, nil
}

func fetchStats(ctx context.Context, c *client.Client, q langsmith.RunStatsQueryParams, filter string) (runStats, error) {
	if filter != "" {
		q.Filter = langsmith.F(filter)
	}
	res, err := c.SDK.Runs.Stats(ctx, langsmith.RunStatsParams{RunStatsQueryParams: q})
	if err != nil {
		return runStats{}, withQueryHint(fmt.Errorf("fetching run stats: %w", err), false)
	}
	switch s := (*res).(type) {
	case langsmith.RunStatsResponseRunStats:
		return statsFromResponse(s), nil
	case langsmith.RunStatsResponseMap:
		for _, item := range s {
			return statsFromMapItem(item), nil
		}
		return runStats{}, nil
	default:
		return runStats{}, fmt.Errorf("unhandled stats response type: %T", *res)
	}
}

// joinFilters ANDs non-empty filter DSL expressions.
func joinFilters(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	switch len(kept) {
	case 0:
		return ""
	case 1:
		return kept[0]
	default:
		return "and(" + strings.Join(kept, ", ") + ")"
	}
}

// failureRunSelect and failureRunSelectV2 read only what clustering needs. The
// error field is the point; inputs and outputs are left out because they are
// most of a run's size and an agent asks for them per trace afterwards.
func failureRunSelect() []langsmith.RunQueryParamsSelect {
	return []langsmith.RunQueryParamsSelect{
		langsmith.RunQueryParamsSelectID,
		langsmith.RunQueryParamsSelectTraceID,
		langsmith.RunQueryParamsSelectName,
		langsmith.RunQueryParamsSelectRunType,
		langsmith.RunQueryParamsSelectStartTime,
		langsmith.RunQueryParamsSelectEndTime,
		langsmith.RunQueryParamsSelectParentRunID,
		langsmith.RunQueryParamsSelectError,
	}
}

func failureRunSelectV2() []langsmith.RunSelectField {
	return []langsmith.RunSelectField{
		langsmith.RunSelectFieldID,
		langsmith.RunSelectFieldTraceID,
		langsmith.RunSelectFieldName,
		langsmith.RunSelectFieldRunType,
		langsmith.RunSelectFieldStartTime,
		langsmith.RunSelectFieldEndTime,
		langsmith.RunSelectFieldParentRunIDs,
		langsmith.RunSelectFieldError,
	}
}

// fetchFailedRuns returns up to limit of the newest failed runs, at any depth,
// in the window.
func fetchFailedRuns(ctx context.Context, c *client.Client, sessionID string, w timeWindow, filter string, limit int) ([]langsmith.RunSchema, error) {
	params := langsmith.RunQueryParams{
		StartTime: langsmith.F(w.Start),
		EndTime:   langsmith.F(w.queryEnd()),
		Error:     langsmith.F(true),
		Order:     langsmith.F(langsmith.RunQueryParamsOrderDesc),
		Select:    langsmith.F(failureRunSelect()),
	}
	if filter != "" {
		params.Filter = langsmith.F(filter)
	}
	return queryRunsAuto(ctx, c, params, failureRunSelectV2(), sessionID, limit, 0)
}

// failureCluster is one group of failed runs that share a step and an error.
type failureCluster struct {
	Run          string    `json:"run"`
	RunType      string    `json:"run_type"`
	Error        string    `json:"error"`
	ErrorType    string    `json:"error_type,omitempty"`
	LikelyCause  string    `json:"likely_cause"`
	Match        string    `json:"match"`
	Fails        int       `json:"fails"`
	Traces       int       `json:"traces"`
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
	SampleTraces []string  `json:"sample_traces"`
	SampleRuns   []string  `json:"sample_runs"`

	key      string
	traceIDs map[string]bool
}

// clusterResult is the grouping of one window's failure sample.
type clusterResult struct {
	Clusters   []*failureCluster
	Sampled    int // failed runs read
	Interrupts int // failed runs dropped as LangGraph interrupts
}

// clusterFailures groups failed runs by (run name, run type, normalized error).
// LangGraph interrupts are reported as errors but are control flow, not
// failures, so they are counted and dropped. Clusters are ordered by the number
// of distinct traces they hit, then by failures, then newest first.
func clusterFailures(runs []langsmith.RunSchema, samplesPerCluster int) clusterResult {
	res := clusterResult{Sampled: len(runs)}
	byKey := map[string]*failureCluster{}
	for _, r := range runs {
		sig := parseErrorSignature(r.Error)
		if sig.interrupt {
			res.Interrupts++
			continue
		}
		key := strings.Join([]string{r.Name, string(r.RunType), sig.key}, "\x00")
		cl, ok := byKey[key]
		if !ok {
			cause, match := classifyFailure(sig, string(r.RunType))
			cl = &failureCluster{
				Run: r.Name, RunType: string(r.RunType), Error: sig.display, ErrorType: sig.errType,
				LikelyCause: cause, Match: match, FirstSeen: r.StartTime, LastSeen: r.StartTime,
				SampleTraces: []string{}, SampleRuns: []string{},
				key: key, traceIDs: map[string]bool{},
			}
			byKey[key] = cl
			res.Clusters = append(res.Clusters, cl)
		}
		cl.Fails++
		if r.StartTime.Before(cl.FirstSeen) {
			cl.FirstSeen = r.StartTime
		}
		if r.StartTime.After(cl.LastSeen) {
			cl.LastSeen = r.StartTime
		}
		if !cl.traceIDs[r.TraceID] {
			cl.traceIDs[r.TraceID] = true
			cl.Traces++
			if len(cl.SampleTraces) < samplesPerCluster {
				cl.SampleTraces = append(cl.SampleTraces, r.TraceID)
				cl.SampleRuns = append(cl.SampleRuns, r.ID)
			}
		}
	}
	sort.SliceStable(res.Clusters, func(i, j int) bool {
		a, b := res.Clusters[i], res.Clusters[j]
		if a.Traces != b.Traces {
			return a.Traces > b.Traces
		}
		if a.Fails != b.Fails {
			return a.Fails > b.Fails
		}
		return a.LastSeen.After(b.LastSeen)
	})
	return res
}

// errorSignature is the part of a run's error that identifies the failure.
type errorSignature struct {
	errType   string // exception class, e.g. RateLimitError; "" if none found
	message   string
	display   string // "Type: message", truncated for output
	key       string // normalized for grouping
	interrupt bool
}

var (
	// "openai.RateLimitError('...')", "RateLimitError: ...", or a bare type.
	errHeadRe   = regexp.MustCompile(`^([A-Za-z_][\w.]*)(?:\((.*)\)\s*$|:\s*(.*))?$`)
	errTypeRe   = regexp.MustCompile(`(Error|Exception|Interrupt|Timeout|Exit|Exceeded|Failure|Cancelled)$`)
	uuidRe      = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	hexRe       = regexp.MustCompile(`(?i)\b[0-9a-f]{12,}\b`)
	numRe       = regexp.MustCompile(`\d+(\.\d+)?`)
	spaceRe     = regexp.MustCompile(`\s+`)
	interruptRe = regexp.MustCompile(`^(GraphInterrupt|NodeInterrupt|Interrupt)$`)
)

const maxErrorDisplay = 160

// parseErrorSignature extracts the exception type and message from a run's
// error. LangSmith stores Python-style errors, sometimes followed or preceded
// by a traceback; the exception line is the first line, or the last line of a
// traceback that comes first.
func parseErrorSignature(raw string) errorSignature {
	line := errorHeadline(raw)
	sig := errorSignature{message: line}
	if m := errHeadRe.FindStringSubmatch(line); m != nil {
		full := m[1]
		short := full[strings.LastIndex(full, ".")+1:]
		if errTypeRe.MatchString(short) {
			sig.errType = short
			sig.message = strings.Trim(strings.TrimSpace(m[2]+m[3]), `'"`)
		}
	}
	sig.interrupt = interruptRe.MatchString(sig.errType)
	switch {
	case sig.errType != "" && sig.message != "":
		sig.display = sig.errType + ": " + sig.message
	case sig.errType != "":
		sig.display = sig.errType
	case line != "":
		sig.display = line
	default:
		sig.display = "(no error message)"
	}
	sig.display = truncate(sig.display, maxErrorDisplay)

	norm := strings.ToLower(sig.message)
	norm = uuidRe.ReplaceAllString(norm, "<id>")
	norm = hexRe.ReplaceAllString(norm, "<hex>")
	norm = numRe.ReplaceAllString(norm, "<n>")
	norm = spaceRe.ReplaceAllString(norm, " ")
	if len(norm) > 120 {
		norm = norm[:120]
	}
	sig.key = sig.errType + "|" + norm
	return sig
}

func errorHeadline(raw string) string {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	var nonEmpty []string
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			nonEmpty = append(nonEmpty, l)
		}
	}
	if len(nonEmpty) == 0 {
		return ""
	}
	if strings.HasPrefix(nonEmpty[0], "Traceback (most recent call last)") {
		return nonEmpty[len(nonEmpty)-1]
	}
	return nonEmpty[0]
}

// causeRule maps an error to a likely cause. typeRe is checked against the
// exception class (an "exact" match); msgRe against the message text (a
// "pattern" match, weaker because messages are free text).
type causeRule struct {
	cause  string
	typeRe *regexp.Regexp
	msgRe  *regexp.Regexp
}

var causeRules = []causeRule{
	{"Model unavailable", regexp.MustCompile(`ModelNotFound`), regexp.MustCompile(`(?i)model.*(not found|does not exist|unavailable|not deployed|inaccessible)|(no such|unknown) model`)},
	{"Rate limited", regexp.MustCompile(`RateLimit|TooManyRequests|ResourceExhausted`), regexp.MustCompile(`(?i)rate.?limit|\b429\b|too many requests|quota`)},
	{"Auth failure", regexp.MustCompile(`Authentication|PermissionDenied|Unauthorized|Forbidden`), regexp.MustCompile(`(?i)\b40[13]\b|unauthori[sz]ed|invalid.{0,10}api.?key|forbidden|permission denied`)},
	{"Context length exceeded", regexp.MustCompile(`ContextWindow|ContextLength|TokenLimit`), regexp.MustCompile(`(?i)context.?(length|window)|maximum context|too many tokens|prompt is too long`)},
	{"Timeout", regexp.MustCompile(`Timeout|DeadlineExceeded`), regexp.MustCompile(`(?i)timed? ?out|deadline exceeded`)},
	{"Recursion limit", regexp.MustCompile(`GraphRecursion|RecursionError`), regexp.MustCompile(`(?i)recursion limit`)},
	{"Provider overloaded", regexp.MustCompile(`InternalServer|ServiceUnavailable|Overloaded|BadGateway`), regexp.MustCompile(`(?i)overloaded|\b50[0234]\b|service unavailable|bad gateway`)},
	{"Connection error", regexp.MustCompile(`Connect`), regexp.MustCompile(`(?i)connection (refused|reset|error|aborted)|name resolution|ssl`)},
	{"Output parsing", regexp.MustCompile(`OutputParser|JSONDecode|ValidationError|ParseError`), regexp.MustCompile(`(?i)failed to parse|invalid json|validation error|could not parse`)},
	{"Cancelled", regexp.MustCompile(`Cancel`), regexp.MustCompile(`(?i)\bcancell?ed\b`)},
	{"Tool error", regexp.MustCompile(`ToolException|ToolError`), nil},
}

// classifyFailure returns the likely cause and how it was matched: "exact" on
// the exception type, "pattern" on the message, "none" when nothing matched.
func classifyFailure(sig errorSignature, runType string) (cause, match string) {
	if sig.errType != "" {
		for _, r := range causeRules {
			if r.typeRe.MatchString(sig.errType) {
				return r.cause, "exact"
			}
		}
	}
	for _, r := range causeRules {
		if r.msgRe != nil && r.msgRe.MatchString(sig.message) {
			return r.cause, "pattern"
		}
	}
	if runType == "tool" {
		return "Tool error", "pattern"
	}
	return "Unclassified", "none"
}

// maxParallelReads caps how many reads a workflow has in flight. verify reads
// two counts per failing step, which can be hundreds without --match.
const maxParallelReads = 8

// runParallel runs fns, at most maxParallelReads at a time, and returns the
// first error. Callers resolve the deployment version first (see
// prepareParallelReads): the client caches it without a lock.
func runParallel(fns ...func() error) error {
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxParallelReads)
	errs := make([]error, len(fns))
	for i, fn := range fns {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			errs[i] = fn()
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// prepareParallelReads fetches and caches the deployment version, which every
// runs query reads to pick v1 or v2, before reads run concurrently.
func prepareParallelReads(ctx context.Context, c *client.Client) error {
	_, err := c.UseV2API(ctx)
	return err
}

// pctChange is (now-base)/base, or NaN when there is no base.
func pctChange(now, base float64) float64 {
	if base == 0 {
		return math.NaN()
	}
	return (now - base) / base
}

// jsonFloat returns f rounded to digits, or nil for NaN/Inf, which JSON cannot carry.
func jsonFloat(f float64, digits int) any {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return nil
	}
	p := math.Pow(10, float64(digits))
	return math.Round(f*p) / p
}

// nextCommand is a follow-up command a workflow suggests, with why.
type nextCommand struct {
	Description string `json:"description"`
	Command     string `json:"command"`
}

// projectArg is the project selector for a suggested command. It always uses
// the UUID so a generated command line never has to quote a project name.
func projectArg(sessionID string) string {
	return "--project-id " + sessionID
}

var shellSafeRe = regexp.MustCompile(`^[A-Za-z0-9_./:@%+=,-]+$`)

// optArg renders " --flag value" for a suggested command, or "" when value is
// empty, so a follow-up command keeps the options the current one ran with.
func optArg(flag, value string) string {
	if value == "" {
		return ""
	}
	return " --" + flag + " " + shellArg(value)
}

// shellArg quotes s for a suggested command line when it needs quoting.
func shellArg(s string) string {
	if shellSafeRe.MatchString(s) {
		return s
	}
	return shellQuote(s)
}
