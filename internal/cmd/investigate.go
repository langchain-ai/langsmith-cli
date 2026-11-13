package cmd

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

type investigateOptions struct {
	project           string
	projectID         string
	since             string
	before            string
	lastNMinutes      int
	baseline          string
	filter            string
	sample            int
	maxFindings       int
	samplesPerFinding int
	noIssues          bool
	showAllIssues     bool
	showCommands      bool
	outputFile        string
}

func newInvestigateCmd() *cobra.Command {
	var o investigateOptions
	cmd := &cobra.Command{
		Use:   "investigate",
		Short: "Diagnose a project: health vs baseline, failures grouped by error, evidence traces",
		Long: `Diagnose what is going wrong in a tracing project in one command.

investigate runs a fixed set of reads and returns one compact report:

  1. Health: root-trace count, error rate, p50/p99 latency and cost for the
     window, against a baseline window just before it.
  2. Findings: the newest failed runs (any depth) in the window, grouped by
     step and normalized error, with a likely cause, how many traces each hit,
     when it was first seen, and whether it is new or already failing in the
     baseline (possibly_new when the baseline sample was cut off before it).
     LangGraph interrupts are excluded; they are control flow.
  3. Related issues: open Issues Board issues that mention a finding's step or
     error.
  4. Next: the exact follow-up commands, e.g. 'traces get' for an evidence
     trace (also in .next in JSON).

The window defaults to the last 24 hours and the baseline to the 7 days before
it. Findings come from the newest --sample failed runs, so their counts are
lower bounds when the window has more failures than that; the coverage line
says when that happened.

Pass --format json for the machine-readable report.

Examples:
  langsmith investigate --project my-app
  langsmith investigate --project my-app --last-n-minutes 60 --baseline 24h
  langsmith investigate --project-id <uuid> --since 2026-11-10T14:00:00Z --format json
  langsmith investigate --project my-app --filter 'eq(run_type, "llm")'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			report, err := runInvestigate(cmd.Context(), c, o, time.Now().UTC())
			if err != nil {
				return err
			}
			if GetFormat() == "pretty" {
				printInvestigatePretty(os.Stdout, report, o.showCommands)
				return nil
			}
			return output.OutputJSON(report, o.outputFile)
		},
	}

	addProjectFlags(cmd, &o.project, &o.projectID)
	cmd.Flags().StringVar(&o.since, "since", "", "Window start, RFC 3339 or YYYY-MM-DD (default: 24h before --before)")
	cmd.Flags().StringVar(&o.before, "before", "", "Window end, RFC 3339 or YYYY-MM-DD (default: now)")
	cmd.Flags().IntVar(&o.lastNMinutes, "last-n-minutes", 0, "Window = the last N minutes (overrides --since)")
	cmd.Flags().StringVar(&o.baseline, "baseline", "7d", "Length of the baseline window that ends where the window starts, e.g. 24h, 7d")
	cmd.Flags().StringVar(&o.filter, "filter", "", filterFlagUsage+". Applied to every read")
	cmd.Flags().IntVar(&o.sample, "sample", defaultFailureSample, "Newest failed runs to read per window")
	cmd.Flags().IntVar(&o.maxFindings, "max-findings", 5, "Findings to return, largest first")
	cmd.Flags().IntVar(&o.samplesPerFinding, "samples-per-finding", 2, "Evidence traces to return per finding")
	cmd.Flags().BoolVar(&o.noIssues, "no-issues", false, "Skip the Issues Board lookup")
	cmd.Flags().BoolVar(&o.showAllIssues, "show-all-issues", false, "List every open issue, not only those related to a finding")
	cmd.Flags().BoolVar(&o.showCommands, "show-commands", false, "Print every suggested next command (pretty output shows two)")
	cmd.Flags().StringVarP(&o.outputFile, "output", "o", "", "Write JSON output to a file")
	cmd.MarkFlagsMutuallyExclusive("since", "last-n-minutes")
	return cmd
}

// investigateReport is the JSON output of `langsmith investigate`.
type investigateReport struct {
	Project         projectRef           `json:"project"`
	Window          timeWindow           `json:"window"`
	Baseline        timeWindow           `json:"baseline"`
	Coverage        investigateCoverage  `json:"coverage"`
	Health          healthComparison     `json:"health"`
	WhatChanged     []string             `json:"what_changed"`
	Findings        []investigateFinding `json:"findings"`
	OmittedFindings int                  `json:"omitted_findings"`
	RelatedIssues   []relatedIssue       `json:"related_issues"`
	OpenIssues      *int                 `json:"open_issues"`
	// OpenIssuesIncomplete means the board may have more open issues than were
	// read: the read stopped at maxIssuesRead and either found more or could not
	// tell. open_issues and related_issues cover only the issues read.
	OpenIssuesIncomplete bool           `json:"open_issues_incomplete,omitempty"`
	AllIssues            []relatedIssue `json:"all_issues,omitempty"`
	Next                 []nextCommand  `json:"next"`
}

type projectRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type investigateCoverage struct {
	SampledFailedRuns  int    `json:"sampled_failed_runs"`
	SampleLimit        int    `json:"sample_limit"`
	Truncated          bool   `json:"truncated"`
	InterruptsExcluded int    `json:"interrupts_excluded"`
	Clusters           int    `json:"clusters"`
	EvidenceTraces     int    `json:"evidence_traces"`
	Reduction          any    `json:"reduction"`
	Issues             string `json:"issues"`
}

type healthComparison struct {
	Now      windowHealth `json:"now"`
	Baseline windowHealth `json:"baseline"`
	// Trace and cost changes are per unit of time, so windows of different
	// lengths compare; rates and latencies compare directly.
	Change map[string]any `json:"change"`
}

type investigateFinding struct {
	failureCluster
	// Status is "ongoing" (also in the baseline sample), "new" (not in a
	// complete baseline sample), or "possibly_new" (not in a baseline sample
	// that was cut off, so it may have occurred earlier in the baseline).
	Status string `json:"status"`
}

type relatedIssue struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Severity string `json:"severity"`
	Status   string `json:"status"`
	Finding  int    `json:"finding,omitempty"` // 1-based index into findings
}

// investigateWindows resolves the window and the baseline that ends where it starts.
func investigateWindows(o investigateOptions, now time.Time) (timeWindow, timeWindow, error) {
	end := now
	if o.before != "" {
		t, err := parseWorkflowTime("--before", o.before)
		if err != nil {
			return timeWindow{}, timeWindow{}, err
		}
		end = t
	}
	start := end.Add(-24 * time.Hour)
	switch {
	case o.lastNMinutes > 0:
		start = end.Add(-time.Duration(o.lastNMinutes) * time.Minute)
	case o.since != "":
		t, err := parseWorkflowTime("--since", o.since)
		if err != nil {
			return timeWindow{}, timeWindow{}, err
		}
		start = t
	}
	if !end.After(start) {
		return timeWindow{}, timeWindow{}, fmt.Errorf("the window is empty: --before (%s) must be later than its start (%s)", end.Format(time.RFC3339), start.Format(time.RFC3339))
	}
	base, err := parseWindowDuration(o.baseline)
	if err != nil {
		return timeWindow{}, timeWindow{}, fmt.Errorf("--baseline: %w", err)
	}
	return timeWindow{start, end}, timeWindow{start.Add(-base), start}, nil
}

func runInvestigate(ctx context.Context, c *client.Client, o investigateOptions, now time.Time) (*investigateReport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if o.sample <= 0 || o.maxFindings <= 0 || o.samplesPerFinding <= 0 {
		return nil, fmt.Errorf("--sample, --max-findings and --samples-per-finding must be positive")
	}
	if err := validateRawFilter(o.filter); err != nil {
		return nil, err
	}
	window, baseline, err := investigateWindows(o, now)
	if err != nil {
		return nil, err
	}
	sessionID, err := resolveSessionID(ctx, c, o.project, o.projectID, "investigate")
	if err != nil {
		return nil, err
	}

	var (
		nowHealth, baseHealth windowHealth
		nowFails, baseFails   []langsmith.RunSchema
		issues                []langsmith.Issue
		issuesIncomplete      bool
		issuesErr             error
	)
	reads := []func() error{
		func() (err error) { nowHealth, err = fetchWindowHealth(ctx, c, sessionID, window, o.filter); return },
		func() (err error) { baseHealth, err = fetchWindowHealth(ctx, c, sessionID, baseline, o.filter); return },
		func() (err error) {
			nowFails, err = fetchFailedRuns(ctx, c, sessionID, window, o.filter, o.sample)
			return
		},
		func() (err error) {
			baseFails, err = fetchFailedRuns(ctx, c, sessionID, baseline, o.filter, o.sample)
			return
		},
	}
	if !o.noIssues {
		// The Issues Board is optional context; a deployment without it still
		// gets the rest of the report.
		reads = append(reads, func() error { issues, issuesIncomplete, issuesErr = fetchOpenIssues(ctx, c, sessionID); return nil })
	}
	if err := prepareParallelReads(ctx, c); err != nil {
		return nil, err
	}
	if err := runParallel(reads...); err != nil {
		return nil, err
	}

	current := clusterFailures(nowFails, o.samplesPerFinding)
	before := clusterFailures(baseFails, 0)
	seenBefore := make(map[string]bool, len(before.Clusters))
	for _, cl := range before.Clusters {
		seenBefore[cl.key] = true
	}

	r := &investigateReport{
		Project:       projectRef{ID: sessionID, Name: ResolveProject(o.project)},
		Window:        window,
		Baseline:      baseline,
		Health:        compareHealth(nowHealth, baseHealth, window, baseline),
		Findings:      []investigateFinding{},
		RelatedIssues: []relatedIssue{},
		Next:          []nextCommand{},
	}
	if o.projectID != "" {
		r.Project.Name = ""
	}
	for i, cl := range current.Clusters {
		if i >= o.maxFindings {
			r.OmittedFindings = len(current.Clusters) - o.maxFindings
			break
		}
		status := "ongoing"
		if !seenBefore[cl.key] {
			status = "new"
			if before.Sampled >= o.sample {
				status = "possibly_new"
			}
		}
		r.Findings = append(r.Findings, investigateFinding{failureCluster: *cl, Status: status})
	}

	evidence := 0
	for _, f := range r.Findings {
		evidence += len(f.SampleTraces)
	}
	r.Coverage = investigateCoverage{
		SampledFailedRuns:  current.Sampled,
		SampleLimit:        o.sample,
		Truncated:          current.Sampled >= o.sample,
		InterruptsExcluded: current.Interrupts,
		Clusters:           len(current.Clusters),
		EvidenceTraces:     evidence,
		Reduction:          nil,
		Issues:             "ok",
	}
	if current.Sampled > 0 {
		r.Coverage.Reduction = jsonFloat(1-float64(evidence)/float64(current.Sampled), 2)
	}

	switch {
	case o.noIssues:
		r.Coverage.Issues = "skipped"
	case issuesErr != nil:
		r.Coverage.Issues = "unavailable: " + truncate(issuesErr.Error(), 120)
	default:
		n := len(issues)
		r.OpenIssues = &n
		r.OpenIssuesIncomplete = issuesIncomplete
		r.RelatedIssues = relateIssues(issues, r.Findings)
		if o.showAllIssues {
			r.AllIssues = make([]relatedIssue, 0, len(issues))
			for _, is := range issues {
				r.AllIssues = append(r.AllIssues, toRelatedIssue(is, 0))
			}
		}
	}

	r.WhatChanged = describeChanges(r)
	r.Next = investigateNext(r, sessionID, o.filter)
	return r, nil
}

// issuesPageSize and maxIssuesRead bound the Issues Board read. A board past
// maxIssuesRead open issues is reported as truncated rather than read whole.
const (
	issuesPageSize = 100
	maxIssuesRead  = 500
)

// fetchOpenIssues pages through the project's open issues, up to
// maxIssuesRead. incomplete is true when the board may have more: a read past
// the cap found another issue, or failed so that it is unknown.
func fetchOpenIssues(ctx context.Context, c *client.Client, sessionID string) (issues []langsmith.Issue, incomplete bool, err error) {
	for offset := 0; offset < maxIssuesRead; offset += issuesPageSize {
		page, err := c.SDK.Issues.List(ctx, langsmith.IssueListParams{
			SessionID: langsmith.F(sessionID),
			Status:    langsmith.F(langsmith.IssueListParamsStatusOpen),
			Limit:     langsmith.F(int64(issuesPageSize)),
			Offset:    langsmith.F(int64(offset)),
		})
		if err != nil {
			return nil, false, err
		}
		issues = append(issues, page.Items...)
		if len(page.Items) < issuesPageSize {
			return issues, false, nil
		}
	}
	// A full last page only shows the cap was reached; ask for one more issue
	// to know whether the board actually has more.
	more, err := c.SDK.Issues.List(ctx, langsmith.IssueListParams{
		SessionID: langsmith.F(sessionID),
		Status:    langsmith.F(langsmith.IssueListParamsStatusOpen),
		Limit:     langsmith.F(int64(1)),
		Offset:    langsmith.F(int64(maxIssuesRead)),
	})
	if err != nil {
		// The issues already read stand; only whether more exist is unknown.
		return issues, true, nil
	}
	return issues, len(more.Items) > 0, nil
}

func compareHealth(now, base windowHealth, w, b timeWindow) healthComparison {
	perHour := func(v float64, win timeWindow) float64 { return v / win.duration().Hours() }
	return healthComparison{
		Now:      now,
		Baseline: base,
		Change: map[string]any{
			"traces_per_hour_pct": jsonFloat(pctChange(perHour(float64(now.Traces), w), perHour(float64(base.Traces), b))*100, 1),
			"error_rate_pts":      jsonFloat((now.ErrorRate-base.ErrorRate)*100, 1),
			"latency_p50_pct":     jsonFloat(pctChange(now.LatencyP50, base.LatencyP50)*100, 1),
			"latency_p99_pct":     jsonFloat(pctChange(now.LatencyP99, base.LatencyP99)*100, 1),
			"cost_per_hour_pct":   jsonFloat(pctChange(perHour(now.TotalCost, w), perHour(base.TotalCost, b))*100, 1),
		},
	}
}

// describeChanges writes the short "what changed" lines an agent reads first.
func describeChanges(r *investigateReport) []string {
	var out []string
	now, base := r.Health.Now, r.Health.Baseline
	pts := (now.ErrorRate - base.ErrorRate) * 100
	switch {
	case now.Traces == 0:
		out = append(out, "No traces in the window.")
	case base.Traces == 0:
		out = append(out, fmt.Sprintf("Error rate %s; the baseline has no traces to compare with.", fmtPct(now.ErrorRate)))
	case pts >= 2 && now.ErrorRate >= 1.5*base.ErrorRate:
		out = append(out, fmt.Sprintf("Error rate %s vs %s baseline (+%.1f points): project-wide regression.", fmtPct(now.ErrorRate), fmtPct(base.ErrorRate), pts))
	default:
		out = append(out, fmt.Sprintf("Error rate %s vs %s baseline (%+.1f points): no project-wide regression.", fmtPct(now.ErrorRate), fmtPct(base.ErrorRate), pts))
	}
	if ch := pctChange(now.LatencyP99, base.LatencyP99); !math.IsNaN(ch) && ch >= 0.5 && now.Traces > 0 {
		out = append(out, fmt.Sprintf("p99 latency %s vs %s baseline (+%.0f%%).", fmtSeconds(now.LatencyP99), fmtSeconds(base.LatencyP99), ch*100))
	}
	for _, f := range r.Findings {
		switch f.Status {
		case "new":
			out = append(out, fmt.Sprintf("New since baseline: %s, %s (%s).", f.Run, strings.ToLower(f.LikelyCause), plural(f.Traces, "trace")))
		case "possibly_new":
			out = append(out, fmt.Sprintf("Not in the baseline sample (cut off, so it may be older): %s, %s (%s).", f.Run, strings.ToLower(f.LikelyCause), plural(f.Traces, "trace")))
		}
	}
	if r.Coverage.SampledFailedRuns == 0 {
		out = append(out, "No failed runs in the window.")
	}
	return out
}

// followUpRunLimit is the page size of the suggested `runs list` follow-ups.
const followUpRunLimit = 100

func investigateNext(r *investigateReport, sessionID, filter string) []nextCommand {
	var next []nextCommand
	for _, f := range r.Findings {
		if len(f.SampleTraces) == 0 {
			continue
		}
		next = append(next, nextCommand{
			Description: fmt.Sprintf("Inspect a sample %s failure", f.Run),
			Command:     fmt.Sprintf("langsmith traces get %s %s --include-io", f.SampleTraces[0], projectArg(sessionID)),
		})
	}
	since := r.Window.Start.Format(time.RFC3339)
	before := r.Window.End.Format(time.RFC3339)
	for _, f := range r.Findings {
		next = append(next, nextCommand{
			Description: fmt.Sprintf("List the newest %d failed %s runs in the window", followUpRunLimit, f.Run),
			Command: fmt.Sprintf("langsmith runs list %s --name %s --error --since %s --before %s%s --limit %d --include-io --format json",
				projectArg(sessionID), shellArg(f.Run), since, before, optArg("filter", filter), followUpRunLimit),
		})
	}
	for _, is := range r.RelatedIssues {
		next = append(next, nextCommand{
			Description: fmt.Sprintf("Read related issue %q", truncate(is.Name, 60)),
			Command:     "langsmith projects issues get " + is.ID,
		})
	}
	if len(r.Findings) > 0 {
		f := r.Findings[0]
		match := f.ErrorType
		if match == "" {
			match = f.Run
		}
		next = append(next, nextCommand{
			Description: "After shipping a fix, check that this failure stopped (replace FIX_TIME)",
			Command:     fmt.Sprintf("langsmith verify %s --since FIX_TIME --match %s%s", projectArg(sessionID), shellArg(match), optArg("filter", filter)),
		})
	}
	return next
}

// relateIssues links open issues to findings by the words they share: the
// step's name and error type, split on case, without generic words. It is a
// hint ("possibly related"), not a match the Issues Board made.
func relateIssues(issues []langsmith.Issue, findings []investigateFinding) []relatedIssue {
	out := []relatedIssue{}
	for _, is := range issues {
		text := strings.ToLower(is.Name + " " + is.Description)
		for i, f := range findings {
			if mentionsAny(text, findingKeywords(f.failureCluster)) {
				out = append(out, toRelatedIssue(is, i+1))
				break
			}
		}
	}
	return out
}

func toRelatedIssue(is langsmith.Issue, finding int) relatedIssue {
	sev := severityLabels[int(is.Severity)]
	if sev == "" {
		sev = fmt.Sprintf("%d", is.Severity)
	}
	return relatedIssue{ID: is.ID, Name: is.Name, Severity: sev, Status: string(is.Status), Finding: finding}
}

var (
	camelRe        = regexp.MustCompile(`[A-Z]+[a-z0-9]*|[a-z0-9]+`)
	genericKeyword = map[string]bool{
		"chat": true, "error": true, "errors": true, "exception": true, "model": true, "models": true,
		"not": true, "found": true, "run": true, "runnable": true, "sequence": true, "lambda": true,
		"tool": true, "tools": true, "agent": true, "node": true, "call": true, "llm": true, "api": true,
		"invalid": true, "value": true, "type": true, "runtime": true, "request": true, "response": true,
		"internal": true, "server": true, "base": true, "client": true, "graph": true,
	}
)

func findingKeywords(cl failureCluster) []string {
	var kws []string
	for _, src := range []string{cl.Run, cl.ErrorType} {
		for _, w := range camelRe.FindAllString(src, -1) {
			w = strings.ToLower(w)
			if len(w) >= 4 && !genericKeyword[w] {
				kws = append(kws, w)
			}
		}
	}
	if cl.ErrorType != "" {
		kws = append(kws, strings.ToLower(cl.ErrorType))
	}
	return kws
}

func mentionsAny(text string, words []string) bool {
	for _, w := range words {
		if strings.Contains(text, w) {
			return true
		}
	}
	return false
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func fmtSeconds(s float64) string {
	if s == 0 || math.IsNaN(s) {
		return "-"
	}
	return fmt.Sprintf("%.1fs", s)
}

// fmtChange renders a percent change with an arrow, or "-" when there is none.
func fmtChange(v any, unit string) string {
	f, ok := v.(float64)
	if !ok {
		return "-"
	}
	switch {
	case f > 0:
		return fmt.Sprintf("▲ %.1f%s", f, unit)
	case f < 0:
		return fmt.Sprintf("▼ %.1f%s", -f, unit)
	}
	return "0" + unit
}

func printInvestigatePretty(w io.Writer, r *investigateReport, showCommands bool) {
	name := r.Project.Name
	if name == "" {
		name = r.Project.ID
	}
	fmt.Fprintf(w, "%s · last %s vs previous %s  (%s → %s UTC)\n", name,
		humanDuration(r.Window.duration()), humanDuration(r.Baseline.duration()),
		r.Window.Start.Format("Jan 2 15:04"), r.Window.End.Format("Jan 2 15:04"))
	cov := r.Coverage
	sampled := fmt.Sprintf("all %d failed runs", cov.SampledFailedRuns)
	if cov.Truncated {
		sampled = fmt.Sprintf("sampled, newest %d failed runs", cov.SampledFailedRuns)
	}
	reduction := ""
	if f, ok := cov.Reduction.(float64); ok {
		reduction = fmt.Sprintf(" (%.0f%% reduction)", f*100)
	}
	fmt.Fprintf(w, "Coverage: %s · %d interrupts excluded · %s, %s returned%s\n",
		sampled, cov.InterruptsExcluded, plural(cov.Clusters, "cluster"), plural(cov.EvidenceTraces, "evidence trace"), reduction)
	fmt.Fprintf(w, "Baseline: %s · %d traces\n\n", humanDuration(r.Baseline.duration()), r.Health.Baseline.Traces)

	now, base, ch := r.Health.Now, r.Health.Baseline, r.Health.Change
	fmt.Fprintln(w, "Health")
	tbl := output.NewTable(w, []string{"", "NOW", "BASELINE", "CHANGE"})
	_ = tbl.Append([]string{"Traces", fmt.Sprint(now.Traces), fmt.Sprint(base.Traces), fmtChange(ch["traces_per_hour_pct"], "%")})
	_ = tbl.Append([]string{"Error rate", fmtPct(now.ErrorRate), fmtPct(base.ErrorRate), fmtChange(ch["error_rate_pts"], " pts")})
	_ = tbl.Append([]string{"p50 latency", fmtSeconds(now.LatencyP50), fmtSeconds(base.LatencyP50), fmtChange(ch["latency_p50_pct"], "%")})
	_ = tbl.Append([]string{"p99 latency", fmtSeconds(now.LatencyP99), fmtSeconds(base.LatencyP99), fmtChange(ch["latency_p99_pct"], "%")})
	_ = tbl.Append([]string{"Cost", fmtDollars(now.TotalCost), fmtDollars(base.TotalCost), fmtChange(ch["cost_per_hour_pct"], "%")})
	_ = tbl.Render()
	fmt.Fprintln(w, "Trace and cost changes are per unit of time.")

	fmt.Fprintln(w, "\nWhat changed")
	for _, line := range r.WhatChanged {
		fmt.Fprintf(w, "  • %s\n", line)
	}

	if len(r.Findings) > 0 {
		fmt.Fprintf(w, "\nFindings · %s\n", plural(r.Coverage.Clusters, "cluster"))
		ft := output.NewTable(w, []string{"#", "RUN", "TYPE", "FAILS", "TRACES", "LIKELY CAUSE", "MATCH", "STATUS", "FIRST SEEN"})
		et := [][]string{}
		for i, f := range r.Findings {
			status := f.Status
			switch status {
			case "new":
				status = "NEW"
			case "possibly_new":
				status = "NEW?"
			}
			_ = ft.Append([]string{fmt.Sprint(i + 1), truncate(f.Run, 30), f.RunType, fmt.Sprint(f.Fails), fmt.Sprint(f.Traces),
				f.LikelyCause, strings.ToUpper(f.Match), status, f.FirstSeen.UTC().Format("Jan 2 15:04 UTC")})
			sample := "-"
			if len(f.SampleTraces) > 0 {
				sample = f.SampleTraces[0]
			}
			et = append(et, []string{fmt.Sprint(i + 1), truncate(f.Error, 90), sample})
		}
		_ = ft.Render()
		fmt.Fprintln(w)
		etbl := output.NewTable(w, []string{"#", "ERROR", "SAMPLE TRACE"})
		for _, row := range et {
			_ = etbl.Append(row)
		}
		_ = etbl.Render()
		if r.OmittedFindings > 0 {
			fmt.Fprintf(w, "%d smaller clusters omitted; raise --max-findings to see them.\n", r.OmittedFindings)
		}
	}

	if r.OpenIssues != nil {
		total := fmt.Sprint(*r.OpenIssues)
		if r.OpenIssuesIncomplete {
			total = "at least " + total
		}
		fmt.Fprintf(w, "\nRelated issues · %d of %s open\n", len(r.RelatedIssues), total)
		for _, is := range r.RelatedIssues {
			fmt.Fprintf(w, "  POSSIBLY RELATED  %-7s %s → finding %d · %s\n", is.Severity, truncate(is.Name, 70), is.Finding, is.ID)
		}
		for _, is := range r.AllIssues {
			fmt.Fprintf(w, "  OPEN  %-7s %s · %s\n", is.Severity, truncate(is.Name, 80), is.ID)
		}
		if len(r.AllIssues) == 0 && *r.OpenIssues > len(r.RelatedIssues) {
			fmt.Fprintf(w, "  %s open issues in total; --show-all-issues lists them.\n", total)
		}
	} else if r.Coverage.Issues != "skipped" {
		fmt.Fprintf(w, "\nRelated issues · %s\n", r.Coverage.Issues)
	}

	if len(r.Next) > 0 {
		fmt.Fprintln(w, "\nNext")
		shown := r.Next
		if !showCommands && len(shown) > 2 {
			shown = shown[:2]
		}
		for i, n := range shown {
			fmt.Fprintf(w, "  %d. %s\n     $ %s\n", i+1, n.Description, n.Command)
		}
		if rest := len(r.Next) - len(shown); rest > 0 {
			fmt.Fprintf(w, "  %d more commands (run lists, issues, verify): add --show-commands, or see .next in JSON.\n", rest)
		}
	}
}

func fmtDollars(f float64) string {
	if f == 0 || math.IsNaN(f) {
		return "-"
	}
	return fmt.Sprintf("$%.2f", f)
}
