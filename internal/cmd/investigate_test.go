package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

// investigateNow is the fixed "now" of every investigate test; the window is
// the 24h before it and the baseline the 7d before that.
var investigateNow = time.Date(2026, 10, 2, 23, 50, 0, 0, time.UTC)

func newInvestigateFake(t *testing.T) *fakeWorkflowAPI {
	t.Helper()
	at := func(h int) time.Time { return investigateNow.Add(-time.Duration(h) * time.Hour) }
	const fwErr = "FireworksModelNotFoundError('Model not found, inaccessible, and/or not deployed')"
	return &fakeWorkflowAPI{
		t:            t,
		split:        investigateNow.Add(-24 * time.Hour),
		healthAfter:  map[string]any{"run_count": 590, "error_rate": 0.032, "latency_p50": 3.6, "latency_p99": 29.8, "total_cost": 9.36},
		healthBefore: map[string]any{"run_count": 4513, "error_rate": 0.577, "latency_p50": 1.2, "latency_p99": 30.4, "total_cost": 53.76},
		failsAfter: []map[string]any{
			failedRun("r1", "t1", "ChatFireworks", "llm", fwErr, at(1)),
			failedRun("r2", "t1", "ChatFireworks", "llm", fwErr, at(2)),
			failedRun("r3", "t2", "ChatFireworks", "llm", fwErr, at(6)),
			failedRun("r4", "t3", "ChatOpenAI", "llm", "OpenAIModelNotFoundError('Not Found')", at(3)),
			failedRun("r5", "t4", "agent", "chain", "GraphInterrupt((Interrupt(value='approve?'),))", at(4)),
			failedRun("r6", "t5", "agent", "chain", "GraphInterrupt((Interrupt(value='approve?'),))", at(5)),
		},
		failsBefore: []map[string]any{
			failedRun("b1", "tb1", "ChatFireworks", "llm", fwErr, at(48)),
		},
		issues: []map[string]any{
			{"id": "issue-fw", "name": "Fireworks provider returns 401 UNAUTHORIZED", "description": "cron runs fail", "severity": 1, "status": "open"},
			{"id": "issue-other", "name": "Retriever returns empty context", "description": "", "severity": 2, "status": "open"},
		},
	}
}

func investigateJSON(t *testing.T, o investigateOptions) (*investigateReport, map[string]any) {
	t.Helper()
	r, err := runInvestigate(t.Context(), MustGetClient(), o, investigateNow)
	if err != nil {
		t.Fatalf("runInvestigate: %v", err)
	}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return r, m
}

func defaultInvestigateOptions() investigateOptions {
	return investigateOptions{projectID: workflowProjectID, baseline: "7d", sample: 200, maxFindings: 5, samplesPerFinding: 2}
}

func TestInvestigate_GroupsFailuresAndComparesToBaseline(t *testing.T) {
	fake := newInvestigateFake(t)
	defer setupTestEnv(t, fake.server().URL)()

	r, m := investigateJSON(t, defaultInvestigateOptions())

	if !r.Baseline.End.Equal(r.Window.Start) || r.Window.duration() != 24*time.Hour || r.Baseline.duration() != 7*24*time.Hour {
		t.Errorf("windows = %+v / %+v, want last 24h and the 7d before it", r.Window, r.Baseline)
	}

	if len(r.Findings) != 2 {
		t.Fatalf("findings = %d, want 2 (interrupts excluded): %+v", len(r.Findings), r.Findings)
	}
	fw, oa := r.Findings[0], r.Findings[1]
	if fw.Run != "ChatFireworks" || fw.Fails != 3 || fw.Traces != 2 || fw.Status != "ongoing" {
		t.Errorf("finding 1 = %s fails=%d traces=%d status=%s, want ChatFireworks 3/2 ongoing", fw.Run, fw.Fails, fw.Traces, fw.Status)
	}
	if fw.LikelyCause != "Model unavailable" || fw.Match != "exact" {
		t.Errorf("finding 1 cause = %s (%s), want Model unavailable (exact)", fw.LikelyCause, fw.Match)
	}
	if !slices.Equal(fw.SampleTraces, []string{"t1", "t2"}) {
		t.Errorf("finding 1 sample traces = %v, want one per trace [t1 t2]", fw.SampleTraces)
	}
	if oa.Run != "ChatOpenAI" || oa.Status != "new" {
		t.Errorf("finding 2 = %s %s, want ChatOpenAI new", oa.Run, oa.Status)
	}

	cov := r.Coverage
	if cov.InterruptsExcluded != 2 || cov.SampledFailedRuns != 6 || cov.EvidenceTraces != 3 || cov.Truncated {
		t.Errorf("coverage = %+v", cov)
	}

	if ch := m["health"].(map[string]any)["change"].(map[string]any); ch["error_rate_pts"] != -54.5 {
		t.Errorf("error_rate_pts = %v, want -54.5", ch["error_rate_pts"])
	}
	changed := strings.Join(r.WhatChanged, "\n")
	for _, want := range []string{"no project-wide regression", "New since baseline: ChatOpenAI, model unavailable (1 trace)"} {
		if !strings.Contains(changed, want) {
			t.Errorf("what_changed is missing %q:\n%s", want, changed)
		}
	}

	if len(r.RelatedIssues) != 1 || r.RelatedIssues[0].ID != "issue-fw" || r.RelatedIssues[0].Finding != 1 || r.RelatedIssues[0].Severity != "HIGH" {
		t.Errorf("related issues = %+v, want issue-fw linked to finding 1", r.RelatedIssues)
	}
	if r.OpenIssues == nil || *r.OpenIssues != 2 {
		t.Errorf("open_issues = %v, want 2", r.OpenIssues)
	}

	if len(r.Next) == 0 || r.Next[0].Command != "langsmith traces get t1 --project-id "+workflowProjectID+" --include-io" {
		t.Fatalf("next[0] = %+v", r.Next)
	}
	last := r.Next[len(r.Next)-1].Command
	if !strings.HasPrefix(last, "langsmith verify --project-id "+workflowProjectID) || !strings.Contains(last, "--match FireworksModelNotFoundError") {
		t.Errorf("last next command = %q, want a verify command for finding 1", last)
	}
}

// The command is a fixed sequence: two stats reads, two failure reads and one
// issues read. Failure reads ask for errors only and leave inputs and outputs
// out, which is most of a run's size.
func TestInvestigate_RequestSequence(t *testing.T) {
	fake := newInvestigateFake(t)
	defer setupTestEnv(t, fake.server().URL)()
	o := defaultInvestigateOptions()
	o.filter = `eq(run_type, "llm")`
	r, _ := investigateJSON(t, o)

	stats := fake.requestsTo("/api/v1/runs/stats")
	queries := fake.requestsTo("/api/v2/runs/query")
	issues := fake.requestsTo("/api/v1/platform/issues")
	if len(stats) != 2 || len(queries) != 2 || len(issues) != 1 {
		t.Fatalf("requests: %d stats, %d queries, %d issues; want 2, 2, 1", len(stats), len(queries), len(issues))
	}
	for _, s := range stats {
		if s.Body["is_root"] != true || s.Body["filter"] != o.filter {
			t.Errorf("stats body = %v, want root traces with the filter", s.Body)
		}
	}
	for _, q := range queries {
		if q.Body["has_error"] != true || q.Body["filter"] != o.filter {
			t.Errorf("query body = %v, want failed runs with the filter", q.Body)
		}
		ids, _ := q.Body["project_ids"].([]any)
		if len(ids) != 1 || ids[0] != workflowProjectID {
			t.Errorf("project_ids = %v", ids)
		}
		sel, _ := q.Body["selects"].([]any)
		if !slices.Contains(sel, any("ERROR")) || slices.Contains(sel, any("INPUTS")) || slices.Contains(sel, any("OUTPUTS")) {
			t.Errorf("selects = %v, want error without inputs/outputs", sel)
		}
	}
	if q := issues[0].Query; !strings.Contains(q, "session_id="+workflowProjectID) || !strings.Contains(q, "status=open") {
		t.Errorf("issues query = %q", q)
	}
	// Follow-ups keep the filter, so they look at the same runs this report did.
	quoted := shellQuote(o.filter)
	for _, n := range r.Next {
		if strings.HasPrefix(n.Command, "langsmith runs list") || strings.HasPrefix(n.Command, "langsmith verify") {
			if !strings.Contains(n.Command, "--filter "+quoted) {
				t.Errorf("follow-up drops --filter: %s", n.Command)
			}
		}
		if strings.HasPrefix(n.Command, "langsmith runs list") && !strings.Contains(n.Command, "--limit 100") {
			t.Errorf("runs list follow-up has no --limit: %s", n.Command)
		}
	}
}

// The Issues Board is context, not a requirement: without it the report still comes back.
func TestInvestigate_IssuesUnavailable(t *testing.T) {
	fake := newInvestigateFake(t)
	fake.issuesStatus = 404
	defer setupTestEnv(t, fake.server().URL)()

	r, m := investigateJSON(t, defaultInvestigateOptions())
	if !strings.HasPrefix(r.Coverage.Issues, "unavailable") {
		t.Errorf("coverage.issues = %q, want unavailable", r.Coverage.Issues)
	}
	if m["open_issues"] != nil || len(r.Findings) != 2 {
		t.Errorf("open_issues = %v, findings = %d; want null and 2", m["open_issues"], len(r.Findings))
	}
}

func TestInvestigate_NoIssuesSkipsTheRead(t *testing.T) {
	fake := newInvestigateFake(t)
	defer setupTestEnv(t, fake.server().URL)()
	o := defaultInvestigateOptions()
	o.noIssues = true
	r, _ := investigateJSON(t, o)
	if n := len(fake.requestsTo("/api/v1/platform/issues")); n != 0 || r.Coverage.Issues != "skipped" {
		t.Errorf("issues requests = %d, coverage = %q; want 0 and skipped", n, r.Coverage.Issues)
	}
}

// A failure missing from a baseline sample that was cut off may still have
// occurred earlier in the baseline, so it is possibly_new, not new.
func TestInvestigate_TruncatedBaselineHedgesNew(t *testing.T) {
	fake := newInvestigateFake(t)
	for i := range 6 {
		fake.failsBefore = append(fake.failsBefore, failedRun("o"+string(rune('a'+i)), "to", "retriever", "retriever", "KeyError('doc')", investigateNow.Add(-30*time.Hour)))
	}
	defer setupTestEnv(t, fake.server().URL)()
	o := defaultInvestigateOptions()
	o.sample = 6 // the window's 6 failures fit; the baseline's 7 do not
	r, _ := investigateJSON(t, o)
	status := map[string]string{}
	for _, f := range r.Findings {
		status[f.Run] = f.Status
	}
	if status["ChatFireworks"] != "ongoing" || status["ChatOpenAI"] != "possibly_new" {
		t.Errorf("statuses = %v, want ChatFireworks ongoing and ChatOpenAI possibly_new", status)
	}
	if !strings.Contains(strings.Join(r.WhatChanged, "\n"), "may be older") {
		t.Errorf("what_changed does not hedge: %v", r.WhatChanged)
	}
}

// Open issues are paged, so a board past one page is read whole, and one past
// the cap says so.
func TestInvestigate_PagesOpenIssues(t *testing.T) {
	for _, c := range []struct {
		total, wantRead, wantRequests int
		wantIncomplete                bool
	}{
		{250, 250, 3, false},
		{maxIssuesRead, maxIssuesRead, maxIssuesRead/issuesPageSize + 1, false}, // exactly at the cap: read whole
		{600, maxIssuesRead, maxIssuesRead/issuesPageSize + 1, true},
	} {
		fake := newInvestigateFake(t)
		fake.issues = nil
		for i := range c.total {
			fake.issues = append(fake.issues, map[string]any{"id": fmt.Sprintf("issue-%d", i), "name": "Retriever returns empty context", "severity": 2, "status": "open"})
		}
		cleanup := setupTestEnv(t, fake.server().URL)
		r, _ := investigateJSON(t, defaultInvestigateOptions())
		cleanup()
		if *r.OpenIssues != c.wantRead || r.OpenIssuesIncomplete != c.wantIncomplete {
			t.Errorf("%d issues: open_issues = %d, incomplete = %v; want %d, %v", c.total, *r.OpenIssues, r.OpenIssuesIncomplete, c.wantRead, c.wantIncomplete)
		}
		if n := len(fake.requestsTo("/api/v1/platform/issues")); n != c.wantRequests {
			t.Errorf("%d issues: %d issue requests, want %d", c.total, n, c.wantRequests)
		}
	}
}

// When only the read past the issue cap fails, the issues already read stay
// in the report, flagged as possibly incomplete.
func TestInvestigate_IssueProbeFailureKeepsIssues(t *testing.T) {
	fake := newInvestigateFake(t)
	fake.issues = nil
	for i := range maxIssuesRead {
		fake.issues = append(fake.issues, map[string]any{"id": fmt.Sprintf("issue-%d", i), "name": "Retriever returns empty context", "severity": 2, "status": "open"})
	}
	fake.issuesProbeStatus = 503
	defer setupTestEnv(t, fake.server().URL)()
	r, _ := investigateJSON(t, defaultInvestigateOptions())
	if r.Coverage.Issues != "ok" || r.OpenIssues == nil || *r.OpenIssues != maxIssuesRead || !r.OpenIssuesIncomplete {
		t.Errorf("coverage.issues = %q, open_issues = %v, incomplete = %v; want ok, %d, true", r.Coverage.Issues, r.OpenIssues, r.OpenIssuesIncomplete, maxIssuesRead)
	}
}

func TestInvestigate_MaxFindingsReportsOmitted(t *testing.T) {
	fake := newInvestigateFake(t)
	defer setupTestEnv(t, fake.server().URL)()
	o := defaultInvestigateOptions()
	o.maxFindings = 1
	r, _ := investigateJSON(t, o)
	if len(r.Findings) != 1 || r.OmittedFindings != 1 || r.Coverage.Clusters != 2 {
		t.Errorf("findings = %d, omitted = %d, clusters = %d; want 1, 1, 2", len(r.Findings), r.OmittedFindings, r.Coverage.Clusters)
	}
}

func TestInvestigate_RejectsBadWindowsBeforeAnyRequest(t *testing.T) {
	fake := newInvestigateFake(t)
	defer setupTestEnv(t, fake.server().URL)()
	for name, o := range map[string]investigateOptions{
		"bad baseline": {projectID: workflowProjectID, baseline: "a week", sample: 1, maxFindings: 1, samplesPerFinding: 1},
		"empty window": {projectID: workflowProjectID, baseline: "7d", since: "2026-10-03", before: "2026-10-02", sample: 1, maxFindings: 1, samplesPerFinding: 1},
		"nested path":  {projectID: workflowProjectID, baseline: "7d", filter: `eq(metadata.user, "x")`, sample: 1, maxFindings: 1, samplesPerFinding: 1},
	} {
		if _, err := runInvestigate(t.Context(), MustGetClient(), o, investigateNow); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if len(fake.requests) != 0 {
		t.Errorf("sent %d requests for invalid input", len(fake.requests))
	}
}

func TestInvestigate_PrettyOutput(t *testing.T) {
	fake := newInvestigateFake(t)
	defer setupTestEnv(t, fake.server().URL)()
	r, _ := investigateJSON(t, defaultInvestigateOptions())

	var buf bytes.Buffer
	printInvestigatePretty(&buf, r, false)
	out := buf.String()
	for _, want := range []string{
		"last 24h vs previous 7d",
		"2 interrupts excluded",
		"Health", "Error rate", "3.2%", "57.7%",
		"Findings · 2 clusters", "ChatFireworks", "Model unavailable", "EXACT", "NEW",
		"FireworksModelNotFoundError: Model not found",
		"Related issues · 1 of 2 open", "POSSIBLY RELATED  HIGH",
		"$ langsmith traces get t1 --project-id " + workflowProjectID,
		"more commands",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("pretty output is missing %q\n%s", want, out)
		}
	}

	buf.Reset()
	printInvestigatePretty(&buf, r, true)
	if strings.Contains(buf.String(), "more commands") || !strings.Contains(buf.String(), "langsmith verify") {
		t.Errorf("--show-commands did not print every command:\n%s", buf.String())
	}
}

// End to end through the root command: flags parse and --format json prints the report.
func TestInvestigateCmd_JSON(t *testing.T) {
	fake := newInvestigateFake(t)
	ts := fake.server()
	out := captureStdout(t, func() {
		if _, err := executeCommand(t, "investigate", "--api-url", ts.URL, "--api-key", "k",
			"--project-id", workflowProjectID, "--before", investigateNow.Format(time.RFC3339), "--format", "json"); err != nil {
			t.Fatalf("investigate: %v", err)
		}
	})
	var r investigateReport
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if len(r.Findings) != 2 || r.Project.ID != workflowProjectID {
		t.Errorf("findings = %d, project = %+v", len(r.Findings), r.Project)
	}
}
