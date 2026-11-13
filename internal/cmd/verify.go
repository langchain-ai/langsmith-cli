package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

type verifyOptions struct {
	project     string
	projectID   string
	since       string
	before      string
	baseline    string
	match       string
	filter      string
	sample      int
	minExpected float64
	outputFile  string
}

// Verdicts, per failure and overall.
const (
	verdictFixed        = "fixed"
	verdictStillFailing = "still_failing"
	verdictInconclusive = "inconclusive"
	verdictNoTraffic    = "no_traffic"
	verdictNothing      = "no_baseline_failures"
)

func newVerifyCmd() *cobra.Command {
	var o verifyOptions
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Check that failures seen before a fix stopped after it",
		Long: `Check that the failures a fix targeted stopped happening after it shipped.

--since is when the fix went live. verify compares the window after it (until
--before, default now) with a baseline window just before it (--baseline,
default 24h):

  1. Failed runs in both windows are grouped by step and normalized error, the
     same way 'langsmith investigate' groups them. The baseline groups are the
     targets; --match narrows them to those whose step, error type or message
     contains the text (case-insensitive).
  2. For each target, the number of runs of that step in both windows sets the
     baseline failure rate and the failures to expect after the fix if nothing
     had changed.
  3. Each target gets a verdict:
       still_failing  the same error occurred after the fix
       fixed          it did not, and at least --min-expected failures were
                      expected (so its absence is unlikely to be chance)
       inconclusive   it did not, but too few runs to tell
       no_traffic     the step did not run after the fix
  4. The overall verdict is still_failing if any target is, fixed if all are,
     and inconclusive otherwise. Errors that appear only after the fix are
     listed as new_failures; they do not change the verdict.

Failure counts come from the newest --sample failed runs per window, so they
are lower bounds on a busy project; a lower baseline rate makes a "fixed"
verdict harder to reach, not easier. After the fix, each step's error count is
read exactly: a step with no errors is fixed for certain, and a step that had
errors the after-fix sample cut off is inconclusive rather than fixed.

Pass --format json for the machine-readable result.

Examples:
  langsmith verify --project my-app --since 2026-11-10T14:00:00Z
  langsmith verify --project-id <uuid> --since 2026-11-10T14:00:00Z --match RateLimitError
  langsmith verify --project my-app --since 2026-11-10T14:00:00Z --baseline 7d --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			res, err := runVerify(cmd.Context(), c, o, time.Now().UTC())
			if err != nil {
				return err
			}
			if GetFormat() == "pretty" {
				printVerifyPretty(os.Stdout, res)
				return nil
			}
			return output.OutputJSON(res, o.outputFile)
		},
	}

	addProjectFlags(cmd, &o.project, &o.projectID)
	cmd.Flags().StringVar(&o.since, "since", "", "When the fix went live, RFC 3339 or YYYY-MM-DD (required)")
	cmd.Flags().StringVar(&o.before, "before", "", "End of the after-fix window (default: now)")
	cmd.Flags().StringVar(&o.baseline, "baseline", "24h", "Length of the before-fix window that ends at --since, e.g. 24h, 7d")
	cmd.Flags().StringVar(&o.match, "match", "", "Only verify failures whose step, error type or message contains this text")
	cmd.Flags().StringVar(&o.filter, "filter", "", filterFlagUsage+". Applied to every read")
	cmd.Flags().IntVar(&o.sample, "sample", defaultFailureSample, "Newest failed runs to read per window")
	cmd.Flags().Float64Var(&o.minExpected, "min-expected", 3, "Failures that must have been expected after the fix before their absence counts as fixed")
	cmd.Flags().StringVarP(&o.outputFile, "output", "o", "", "Write JSON output to a file")
	_ = cmd.MarkFlagRequired("since")
	return cmd
}

// verifyResult is the JSON output of `langsmith verify`.
type verifyResult struct {
	Project     projectRef       `json:"project"`
	FixTime     time.Time        `json:"fix_time"`
	Before      timeWindow       `json:"before"`
	After       timeWindow       `json:"after"`
	Match       string           `json:"match,omitempty"`
	Verdict     string           `json:"verdict"`
	Summary     string           `json:"summary"`
	Health      verifyHealth     `json:"health"`
	Targets     []verifyTarget   `json:"targets"`
	NewFailures []failureCluster `json:"new_failures"`
	Coverage    verifyCoverage   `json:"coverage"`
	Next        []nextCommand    `json:"next"`
}

type verifyHealth struct {
	Before windowHealth `json:"before"`
	After  windowHealth `json:"after"`
}

type verifyTarget struct {
	Run         string     `json:"run"`
	RunType     string     `json:"run_type"`
	Error       string     `json:"error"`
	LikelyCause string     `json:"likely_cause"`
	Before      targetSide `json:"before"`
	After       targetSide `json:"after"`
	// ExpectedFails is the failures the after window would have had at the
	// baseline rate.
	ExpectedFails any      `json:"expected_fails"`
	Verdict       string   `json:"verdict"`
	SampleTraces  []string `json:"sample_traces"`
}

type targetSide struct {
	Fails       int   `json:"fails"`
	Runs        int64 `json:"runs"`
	FailureRate any   `json:"failure_rate"`
	// StepErrors counts every errored run of the step, whatever the error;
	// Fails counts only this target's error, from the failure sample.
	StepErrors int64 `json:"step_errors"`
}

type verifyCoverage struct {
	SampleLimit   int  `json:"sample_limit"`
	BeforeSampled int  `json:"before_sampled_failed_runs"`
	AfterSampled  int  `json:"after_sampled_failed_runs"`
	Truncated     bool `json:"truncated"`
	// BeforeTruncated means the before-fix sample was cut off, so a
	// new_failures entry may also have occurred before the fix.
	BeforeTruncated    bool `json:"before_truncated"`
	InterruptsExcluded int  `json:"interrupts_excluded"`
}

func runVerify(ctx context.Context, c *client.Client, o verifyOptions, now time.Time) (*verifyResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if o.sample <= 0 {
		return nil, fmt.Errorf("--sample must be positive")
	}
	if err := validateRawFilter(o.filter); err != nil {
		return nil, err
	}
	fix, err := parseWorkflowTime("--since", o.since)
	if err != nil {
		return nil, err
	}
	end := now
	if o.before != "" {
		if end, err = parseWorkflowTime("--before", o.before); err != nil {
			return nil, err
		}
	}
	if !end.After(fix) {
		return nil, fmt.Errorf("no time after the fix: --before (%s) must be later than --since (%s)", end.Format(time.RFC3339), fix.Format(time.RFC3339))
	}
	base, err := parseWindowDuration(o.baseline)
	if err != nil {
		return nil, fmt.Errorf("--baseline: %w", err)
	}
	after := timeWindow{fix, end}
	before := timeWindow{fix.Add(-base), fix}

	sessionID, err := resolveSessionID(ctx, c, o.project, o.projectID, "verify")
	if err != nil {
		return nil, err
	}

	if err := prepareParallelReads(ctx, c); err != nil {
		return nil, err
	}
	// Phase 1: health and failure samples for both windows.
	var (
		beforeHealth, afterHealth windowHealth
		beforeRuns, afterRuns     []langsmith.RunSchema
	)
	if err := runParallel(
		func() (err error) { beforeHealth, err = fetchWindowHealth(ctx, c, sessionID, before, o.filter); return },
		func() (err error) { afterHealth, err = fetchWindowHealth(ctx, c, sessionID, after, o.filter); return },
		func() (err error) {
			beforeRuns, err = fetchFailedRuns(ctx, c, sessionID, before, o.filter, o.sample)
			return
		},
		func() (err error) {
			afterRuns, err = fetchFailedRuns(ctx, c, sessionID, after, o.filter, o.sample)
			return
		},
	); err != nil {
		return nil, err
	}
	pre := clusterFailures(beforeRuns, 2)
	post := clusterFailures(afterRuns, 2)

	var targets []*failureCluster
	for _, cl := range pre.Clusters {
		if clusterMatches(cl, o.match) {
			targets = append(targets, cl)
		}
	}

	// Phase 2: how often each targeted step ran and errored in each window, one
	// read per step and window. Steps are deduplicated; several errors often
	// share one.
	runCounts := map[stepKey][2]stepCounts{}
	var mu sync.Mutex
	var reads []func() error
	for _, step := range uniqueSteps(targets) {
		for side, w := range []timeWindow{before, after} {
			reads = append(reads, func() error {
				n, err := fetchStepCounts(ctx, c, sessionID, step, w, o.filter)
				if err != nil {
					return err
				}
				mu.Lock()
				counts := runCounts[step]
				counts[side] = n
				runCounts[step] = counts
				mu.Unlock()
				return nil
			})
		}
	}
	if err := runParallel(reads...); err != nil {
		return nil, err
	}

	postByKey := map[string]*failureCluster{}
	for _, cl := range post.Clusters {
		postByKey[cl.key] = cl
	}
	targetKeys := map[string]bool{}
	res := &verifyResult{
		Project:     projectRef{ID: sessionID, Name: ResolveProject(o.project)},
		FixTime:     fix,
		Before:      before,
		After:       after,
		Match:       o.match,
		Health:      verifyHealth{Before: beforeHealth, After: afterHealth},
		Targets:     []verifyTarget{},
		NewFailures: []failureCluster{},
		Coverage: verifyCoverage{
			SampleLimit:        o.sample,
			BeforeSampled:      pre.Sampled,
			AfterSampled:       post.Sampled,
			Truncated:          pre.Sampled >= o.sample || post.Sampled >= o.sample,
			BeforeTruncated:    pre.Sampled >= o.sample,
			InterruptsExcluded: pre.Interrupts + post.Interrupts,
		},
	}
	if o.projectID != "" {
		res.Project.Name = ""
	}
	for _, cl := range targets {
		targetKeys[cl.key] = true
		counts := runCounts[stepKey{cl.Run, cl.RunType}]
		t := verifyTarget{
			Run: cl.Run, RunType: cl.RunType, Error: cl.Error, LikelyCause: cl.LikelyCause,
			Before:       targetSide{Fails: cl.Fails, Runs: counts[0].Runs, FailureRate: failureRate(cl.Fails, counts[0].Runs), StepErrors: counts[0].Errors},
			After:        targetSide{Runs: counts[1].Runs, StepErrors: counts[1].Errors},
			SampleTraces: []string{},
		}
		if p := postByKey[cl.key]; p != nil {
			t.After.Fails = p.Fails
			t.SampleTraces = p.SampleTraces
		}
		t.After.FailureRate = failureRate(t.After.Fails, t.After.Runs)
		expected := -1.0
		if counts[0].Runs > 0 {
			// The failure count can exceed the run count when a sample spans
			// runs that started just outside the stats window; cap the rate at 1.
			rate := min(float64(cl.Fails)/float64(counts[0].Runs), 1)
			expected = rate * float64(counts[1].Runs)
			t.ExpectedFails = jsonFloat(expected, 1)
		}
		// A full sample saw every failure after the fix; a cut-off one may have
		// missed this target's, unless the step had no errors at all.
		missedPossible := post.Sampled >= o.sample && t.After.StepErrors > 0
		t.Verdict = targetVerdict(t.After.Fails, t.After.Runs, missedPossible, expected, o.minExpected)
		res.Targets = append(res.Targets, t)
	}
	for _, cl := range post.Clusters {
		if targetKeys[cl.key] {
			continue
		}
		if !hasKey(pre.Clusters, cl.key) {
			res.NewFailures = append(res.NewFailures, *cl)
		}
	}
	res.Verdict = overallVerdict(res.Targets)
	res.Summary = verifySummary(res)
	res.Next = verifyNext(res, sessionID, o)
	return res, nil
}

func clusterMatches(cl *failureCluster, match string) bool {
	if match == "" {
		return true
	}
	m := strings.ToLower(match)
	for _, s := range []string{cl.Run, cl.ErrorType, cl.Error} {
		if strings.Contains(strings.ToLower(s), m) {
			return true
		}
	}
	return false
}

func uniqueSteps(cls []*failureCluster) []stepKey {
	seen := map[stepKey]bool{}
	var steps []stepKey
	for _, cl := range cls {
		k := stepKey{cl.Run, cl.RunType}
		if !seen[k] {
			seen[k] = true
			steps = append(steps, k)
		}
	}
	sort.Slice(steps, func(i, j int) bool {
		if steps[i].Name != steps[j].Name {
			return steps[i].Name < steps[j].Name
		}
		return steps[i].RunType < steps[j].RunType
	})
	return steps
}

func hasKey(cls []*failureCluster, key string) bool {
	for _, cl := range cls {
		if cl.key == key {
			return true
		}
	}
	return false
}

func failureRate(fails int, runs int64) any {
	if runs <= 0 {
		return nil
	}
	return jsonFloat(min(float64(fails)/float64(runs), 1), 4)
}

// targetVerdict decides one target. missedPossible means the after-fix sample
// was cut off while the step still errored, so a failure of this target may be
// missing from it. expected < 0 means the baseline run count was unavailable,
// so the absence of failures cannot be weighed.
func targetVerdict(afterFails int, afterRuns int64, missedPossible bool, expected, minExpected float64) string {
	switch {
	case afterFails > 0:
		return verdictStillFailing
	case afterRuns == 0:
		return verdictNoTraffic
	case missedPossible:
		return verdictInconclusive
	case expected >= minExpected:
		return verdictFixed
	default:
		return verdictInconclusive
	}
}

func overallVerdict(targets []verifyTarget) string {
	if len(targets) == 0 {
		return verdictNothing
	}
	fixed := 0
	for _, t := range targets {
		switch t.Verdict {
		case verdictStillFailing:
			return verdictStillFailing
		case verdictFixed:
			fixed++
		}
	}
	if fixed == len(targets) {
		return verdictFixed
	}
	return verdictInconclusive
}

func verifySummary(r *verifyResult) string {
	counts := map[string]int{}
	for _, t := range r.Targets {
		counts[t.Verdict]++
	}
	var s string
	switch r.Verdict {
	case verdictNothing:
		s = fmt.Sprintf("No failures in the %s before the fix", humanDuration(r.Before.duration()))
		if r.Match != "" {
			s += fmt.Sprintf(" matched %q", r.Match)
		}
		s += "; nothing to verify. Widen --baseline or check --match."
	case verdictFixed:
		s = "The targeted failure stopped after the fix."
		if len(r.Targets) > 1 {
			s = fmt.Sprintf("All %d targeted failures stopped after the fix.", len(r.Targets))
		}
	case verdictStillFailing:
		s = fmt.Sprintf("%d of %s still occur after the fix.", counts[verdictStillFailing], plural(len(r.Targets), "targeted failure"))
	default:
		s = fmt.Sprintf("No targeted failure recurred, but %d of %d lack enough runs after the fix to tell; wait for more traffic and rerun.",
			counts[verdictInconclusive]+counts[verdictNoTraffic], len(r.Targets))
	}
	if n := len(r.NewFailures); n > 0 {
		if r.Coverage.BeforeTruncated {
			s += fmt.Sprintf(" %s not in the before-fix sample appeared after it; the sample was cut off, so they may be older.", plural(n, "failure"))
		} else {
			s += fmt.Sprintf(" %s appeared only after the fix.", plural(n, "new failure"))
		}
	}
	return s
}

func verifyNext(r *verifyResult, sessionID string, o verifyOptions) []nextCommand {
	next := []nextCommand{}
	for _, t := range r.Targets {
		if t.Verdict == verdictStillFailing && len(t.SampleTraces) > 0 {
			next = append(next, nextCommand{
				Description: fmt.Sprintf("Inspect a %s failure after the fix", t.Run),
				Command:     fmt.Sprintf("langsmith traces get %s %s --include-io", t.SampleTraces[0], projectArg(sessionID)),
			})
		}
	}
	for _, f := range r.NewFailures {
		if len(f.SampleTraces) > 0 {
			next = append(next, nextCommand{
				Description: fmt.Sprintf("Inspect the new %s failure", f.Run),
				Command:     fmt.Sprintf("langsmith traces get %s %s --include-io", f.SampleTraces[0], projectArg(sessionID)),
			})
		}
	}
	if r.Verdict == verdictInconclusive {
		next = append(next, nextCommand{
			Description: "Rerun later, once the fixed steps have run more",
			Command:     fmt.Sprintf("langsmith verify %s --since %s%s", projectArg(sessionID), r.FixTime.Format(time.RFC3339), rerunArgs(o)),
		})
	}
	return next
}

// rerunArgs repeats the options that choose and weigh the targets, so a rerun
// checks the same failures the same way. --before is left out: a rerun looks
// at the window up to its own now.
func rerunArgs(o verifyOptions) string {
	args := optArg("match", o.match) + optArg("filter", o.filter)
	if o.baseline != "24h" {
		args += optArg("baseline", o.baseline)
	}
	if o.sample != defaultFailureSample {
		args += fmt.Sprintf(" --sample %d", o.sample)
	}
	if o.minExpected != 3 {
		args += " --min-expected " + strconv.FormatFloat(o.minExpected, 'g', -1, 64)
	}
	return args
}

func printVerifyPretty(w io.Writer, r *verifyResult) {
	name := r.Project.Name
	if name == "" {
		name = r.Project.ID
	}
	fmt.Fprintf(w, "%s · fix at %s UTC · %s after vs %s before\n", name, r.FixTime.Format("Jan 2 15:04"),
		humanDuration(r.After.duration()), humanDuration(r.Before.duration()))
	fmt.Fprintf(w, "Verdict: %s\n%s\n\n", strings.ToUpper(strings.ReplaceAll(r.Verdict, "_", " ")), r.Summary)

	fmt.Fprintln(w, "Health")
	ht := output.NewTable(w, []string{"", "BEFORE", "AFTER"})
	_ = ht.Append([]string{"Traces", fmt.Sprint(r.Health.Before.Traces), fmt.Sprint(r.Health.After.Traces)})
	_ = ht.Append([]string{"Error rate", fmtPct(r.Health.Before.ErrorRate), fmtPct(r.Health.After.ErrorRate)})
	_ = ht.Append([]string{"p99 latency", fmtSeconds(r.Health.Before.LatencyP99), fmtSeconds(r.Health.After.LatencyP99)})
	_ = ht.Render()

	if len(r.Targets) > 0 {
		fmt.Fprintln(w, "\nTargeted failures")
		tt := output.NewTable(w, []string{"#", "RUN", "ERROR", "BEFORE", "AFTER", "EXPECTED", "VERDICT"})
		for i, t := range r.Targets {
			exp := "-"
			if f, ok := t.ExpectedFails.(float64); ok {
				exp = fmt.Sprintf("%.1f", f)
			}
			_ = tt.Append([]string{fmt.Sprint(i + 1), truncate(t.Run, 28), truncate(t.Error, 60),
				fmt.Sprintf("%d/%d", t.Before.Fails, t.Before.Runs), fmt.Sprintf("%d/%d", t.After.Fails, t.After.Runs),
				exp, t.Verdict})
		}
		_ = tt.Render()
		fmt.Fprintln(w, "BEFORE and AFTER are failures / runs of that step.")
	}
	if len(r.NewFailures) > 0 {
		fmt.Fprintln(w, "\nNew after the fix")
		for _, f := range r.NewFailures {
			fmt.Fprintf(w, "  • %s: %s (%s)\n", f.Run, truncate(f.Error, 90), plural(f.Traces, "trace"))
		}
	}
	if r.Coverage.Truncated {
		fmt.Fprintf(w, "\nCoverage: sampled the newest %d failed runs per window; failure counts are lower bounds.\n", r.Coverage.SampleLimit)
	}
	if len(r.Next) > 0 {
		fmt.Fprintln(w, "\nNext")
		for i, n := range r.Next {
			fmt.Fprintf(w, "  %d. %s\n     $ %s\n", i+1, n.Description, n.Command)
		}
	}
}
