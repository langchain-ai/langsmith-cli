package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var (
	verifyFixTime = time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC)
	verifyNow     = verifyFixTime.Add(6 * time.Hour)
)

const rateLimitErr = "openai.RateLimitError: Error code: 429 - rate limit reached"

// newVerifyFake has ChatOpenAI failing with a rate limit 10 times in 100 runs
// before the fix. Tests set what happened after it.
func newVerifyFake(t *testing.T) *fakeWorkflowAPI {
	t.Helper()
	var before []map[string]any
	for i := range 10 {
		before = append(before, failedRun("b"+string(rune('a'+i)), "tb"+string(rune('a'+i)), "ChatOpenAI", "llm", rateLimitErr, verifyFixTime.Add(-time.Duration(i+1)*time.Hour)))
	}
	before = append(before, failedRun("bs", "tbs", "search", "tool", "ToolException('index offline')", verifyFixTime.Add(-2*time.Hour)))
	return &fakeWorkflowAPI{
		t:            t,
		split:        verifyFixTime,
		healthBefore: map[string]any{"run_count": 100, "error_rate": 0.1, "latency_p99": 12.0},
		healthAfter:  map[string]any{"run_count": 50, "error_rate": 0.02, "latency_p99": 11.0},
		failsBefore:  before,
		runCounts:    map[string][2]int64{"ChatOpenAI": {100, 50}, "search": {40, 20}},
	}
}

func verifyOpts(match string) verifyOptions {
	return verifyOptions{projectID: workflowProjectID, since: verifyFixTime.Format(time.RFC3339), baseline: "24h", match: match, sample: 200, minExpected: 3}
}

func runVerifyTest(t *testing.T, o verifyOptions) *verifyResult {
	t.Helper()
	r, err := runVerify(t.Context(), MustGetClient(), o, verifyNow)
	if err != nil {
		t.Fatalf("runVerify: %v", err)
	}
	return r
}

func TestVerify_FixedWhenEnoughRunsPassed(t *testing.T) {
	fake := newVerifyFake(t)
	fake.failsAfter = []map[string]any{
		failedRun("n1", "tn1", "summarize", "chain", "KeyError('title')", verifyFixTime.Add(time.Hour)),
	}
	defer setupTestEnv(t, fake.server().URL)()

	r := runVerifyTest(t, verifyOpts("ratelimit"))
	if r.Verdict != verdictFixed {
		t.Fatalf("verdict = %s, want fixed: %s", r.Verdict, r.Summary)
	}
	if len(r.Targets) != 1 {
		t.Fatalf("targets = %+v, want only the rate limit (--match)", r.Targets)
	}
	tg := r.Targets[0]
	if tg.Before.Fails != 10 || tg.Before.Runs != 100 || tg.After.Runs != 50 || tg.ExpectedFails != 5.0 {
		t.Errorf("target = %+v, want 10/100 before, 0/50 after, 5 expected", tg)
	}
	if len(r.NewFailures) != 1 || r.NewFailures[0].Run != "summarize" {
		t.Errorf("new failures = %+v, want summarize", r.NewFailures)
	}
	if !strings.Contains(r.Summary, "1 new failure appeared only after the fix") {
		t.Errorf("summary = %q", r.Summary)
	}
	if !r.Before.End.Equal(verifyFixTime) || !r.After.Start.Equal(verifyFixTime) || r.Before.duration() != 24*time.Hour {
		t.Errorf("windows = %+v / %+v", r.Before, r.After)
	}

	// --match keeps the run counts to the matched step: one per window.
	var names []string
	for _, s := range fake.requestsTo("/api/v1/runs/stats") {
		if f, _ := s.Body["filter"].(string); f != "" {
			names = append(names, f)
			if s.Body["is_root"] != nil {
				t.Errorf("run count read is root-only: %v", s.Body)
			}
		}
	}
	// Counted by name and run type, the way failures are grouped, so a
	// same-named step of another type does not inflate the run count.
	want := `and(eq(name, "ChatOpenAI"), eq(run_type, "llm"))`
	if len(names) != 2 || names[0] != want || names[1] != want {
		t.Errorf("run count filters = %v, want %s for each window", names, want)
	}
}

func TestVerify_StillFailing(t *testing.T) {
	fake := newVerifyFake(t)
	fake.failsAfter = []map[string]any{
		failedRun("a1", "ta1", "ChatOpenAI", "llm", "openai.RateLimitError: Error code: 429 - rate limit reached", verifyFixTime.Add(time.Hour)),
	}
	defer setupTestEnv(t, fake.server().URL)()

	r := runVerifyTest(t, verifyOpts("RateLimitError"))
	if r.Verdict != verdictStillFailing || r.Targets[0].After.Fails != 1 {
		t.Fatalf("verdict = %s, target = %+v; want still_failing with 1 failure after", r.Verdict, r.Targets[0])
	}
	if len(r.Next) == 0 || !strings.HasPrefix(r.Next[0].Command, "langsmith traces get ta1 --project-id "+workflowProjectID) {
		t.Errorf("next = %+v, want the failing trace first", r.Next)
	}
}

// Without --match every failure before the fix is a target, so one that
// persists makes the whole fix "still failing".
func TestVerify_WithoutMatchTargetsEveryBaselineFailure(t *testing.T) {
	fake := newVerifyFake(t)
	fake.failsAfter = []map[string]any{
		failedRun("a1", "ta1", "search", "tool", "ToolException('index offline')", verifyFixTime.Add(time.Hour)),
	}
	defer setupTestEnv(t, fake.server().URL)()

	r := runVerifyTest(t, verifyOpts(""))
	if len(r.Targets) != 2 || r.Verdict != verdictStillFailing {
		t.Fatalf("targets = %d, verdict = %s; want 2 and still_failing", len(r.Targets), r.Verdict)
	}
	byRun := map[string]string{}
	for _, tg := range r.Targets {
		byRun[tg.Run] = tg.Verdict
	}
	if byRun["ChatOpenAI"] != verdictFixed || byRun["search"] != verdictStillFailing {
		t.Errorf("verdicts = %v", byRun)
	}
	if len(r.NewFailures) != 0 {
		t.Errorf("a failure seen before the fix was reported as new: %+v", r.NewFailures)
	}
}

func TestVerify_InconclusiveOnLowTraffic(t *testing.T) {
	fake := newVerifyFake(t)
	fake.runCounts["ChatOpenAI"] = [2]int64{100, 10} // 1 failure expected
	defer setupTestEnv(t, fake.server().URL)()

	r := runVerifyTest(t, verifyOpts("ChatOpenAI"))
	if r.Verdict != verdictInconclusive || r.Targets[0].Verdict != verdictInconclusive {
		t.Fatalf("verdict = %s / %s, want inconclusive", r.Verdict, r.Targets[0].Verdict)
	}
	if len(r.Next) != 1 || !strings.Contains(r.Next[0].Command, "langsmith verify --project-id "+workflowProjectID+" --since 2026-11-10T14:00:00Z --match ChatOpenAI") {
		t.Errorf("next = %+v, want a rerun command", r.Next)
	}
}

// A rerun checks the same targets the same way, so it keeps the options that
// choose and weigh them.
func TestVerify_RerunKeepsOptions(t *testing.T) {
	fake := newVerifyFake(t)
	fake.runCounts["ChatOpenAI"] = [2]int64{100, 10}
	defer setupTestEnv(t, fake.server().URL)()
	o := verifyOpts("ChatOpenAI")
	o.filter = `eq(run_type, "llm")`
	o.baseline = "7d"
	r := runVerifyTest(t, o)
	if len(r.Next) != 1 {
		t.Fatalf("next = %+v", r.Next)
	}
	want := "langsmith verify --project-id " + workflowProjectID + " --since 2026-11-10T14:00:00Z --match ChatOpenAI --filter " + shellQuote(o.filter) + " --baseline 7d"
	if r.Next[0].Command != want {
		t.Errorf("rerun = %q\nwant    %q", r.Next[0].Command, want)
	}
}

// Windows meet at the fix time; a run starting exactly then belongs to the
// after window only, so the before window's reads stop just short of it.
func TestVerify_WindowsDoNotOverlap(t *testing.T) {
	fake := newVerifyFake(t)
	defer setupTestEnv(t, fake.server().URL)()
	runVerifyTest(t, verifyOpts(""))
	for _, path := range []string{"/api/v2/runs/query", "/api/v1/runs/stats"} {
		key := map[string][2]string{"/api/v2/runs/query": {"min_start_time", "max_start_time"}, "/api/v1/runs/stats": {"start_time", "end_time"}}[path]
		for _, req := range fake.requestsTo(path) {
			start, _ := time.Parse(time.RFC3339Nano, req.Body[key[0]].(string))
			end, _ := time.Parse(time.RFC3339Nano, req.Body[key[1]].(string))
			if start.Before(verifyFixTime) && !end.Before(verifyFixTime) {
				t.Errorf("%s before-window read ends at %s, at or after the fix time", path, end)
			}
		}
	}
}

func TestVerify_NoBaselineFailures(t *testing.T) {
	fake := newVerifyFake(t)
	defer setupTestEnv(t, fake.server().URL)()
	r := runVerifyTest(t, verifyOpts("nothing-matches-this"))
	if r.Verdict != verdictNothing || len(r.Targets) != 0 {
		t.Fatalf("verdict = %s, targets = %d", r.Verdict, len(r.Targets))
	}
	if n := len(fake.requestsTo("/api/v1/runs/stats")); n != 2 {
		t.Errorf("stats requests = %d, want only the 2 health reads", n)
	}
}

func TestTargetVerdict(t *testing.T) {
	cases := []struct {
		afterFails int
		afterRuns  int64
		expected   float64
		want       string
	}{
		{1, 100, 10, verdictStillFailing},
		{0, 0, 0, verdictNoTraffic},
		{0, 50, 5, verdictFixed},
		{0, 50, 3, verdictFixed},
		{0, 10, 1, verdictInconclusive},
		{0, 10, -1, verdictInconclusive}, // baseline run count unknown
	}
	for _, c := range cases {
		if got := targetVerdict(c.afterFails, c.afterRuns, false, c.expected, 3); got != c.want {
			t.Errorf("targetVerdict(%d, %d, %v) = %s, want %s", c.afterFails, c.afterRuns, c.expected, got, c.want)
		}
	}
}

// A failure sample cut off after the fix can miss the target's failures, so no
// sampled failure is not proof of a fix while the step still errors. With no
// step errors at all it is.
func TestVerify_TruncatedAfterSample(t *testing.T) {
	newFake := func(stepErrorRateAfter float64) *fakeWorkflowAPI {
		fake := newVerifyFake(t)
		for i := range 11 {
			id := string(rune('a' + i))
			fake.failsAfter = append(fake.failsAfter, failedRun("s"+id, "ts"+id, "summarize", "chain", "KeyError('title')", verifyNow.Add(-time.Duration(i+1)*time.Minute)))
		}
		// Older than every summarize failure, so an 11-run sample drops it.
		fake.failsAfter = append(fake.failsAfter, failedRun("late", "tlate", "ChatOpenAI", "llm", rateLimitErr, verifyFixTime.Add(time.Minute)))
		fake.stepErrorRates = map[string][2]float64{"ChatOpenAI": {0.1, stepErrorRateAfter}}
		return fake
	}
	o := verifyOpts("ratelimit")
	o.sample = 11

	fake := newFake(0.02)
	cleanup := setupTestEnv(t, fake.server().URL)
	r := runVerifyTest(t, o)
	cleanup()
	if tg := r.Targets[0]; tg.After.Fails != 0 || tg.After.StepErrors != 1 || tg.Verdict != verdictInconclusive {
		t.Errorf("target = %+v, want 0 sampled fails, 1 step error, inconclusive", tg)
	}
	if r.Verdict != verdictInconclusive || !r.Coverage.Truncated {
		t.Errorf("verdict = %s, truncated = %v; want inconclusive and true", r.Verdict, r.Coverage.Truncated)
	}
	// The before sample was cut off too, so summarize may predate the fix.
	if !r.Coverage.BeforeTruncated || !strings.Contains(r.Summary, "may be older") {
		t.Errorf("before_truncated = %v, summary = %q; want a hedged new-failure note", r.Coverage.BeforeTruncated, r.Summary)
	}

	fake = newFake(0)
	defer setupTestEnv(t, fake.server().URL)()
	if r := runVerifyTest(t, o); r.Targets[0].Verdict != verdictFixed {
		t.Errorf("verdict = %s, want fixed when the step had no errors after the fix", r.Targets[0].Verdict)
	}
}

func TestVerify_RejectsBadInputBeforeAnyRequest(t *testing.T) {
	fake := newVerifyFake(t)
	defer setupTestEnv(t, fake.server().URL)()
	for name, mutate := range map[string]func(*verifyOptions){
		"bad since":     func(o *verifyOptions) { o.since = "yesterday" },
		"fix in future": func(o *verifyOptions) { o.before = verifyFixTime.Add(-time.Hour).Format(time.RFC3339) },
		"bad baseline":  func(o *verifyOptions) { o.baseline = "0d" },
		"nonpositive n": func(o *verifyOptions) { o.sample = 0 },
	} {
		o := verifyOpts("")
		mutate(&o)
		if _, err := runVerify(t.Context(), MustGetClient(), o, verifyNow); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if len(fake.requests) != 0 {
		t.Errorf("sent %d requests for invalid input", len(fake.requests))
	}
}

func TestVerify_PrettyOutput(t *testing.T) {
	fake := newVerifyFake(t)
	defer setupTestEnv(t, fake.server().URL)()
	r := runVerifyTest(t, verifyOpts("ratelimit"))

	var buf bytes.Buffer
	printVerifyPretty(&buf, r)
	for _, want := range []string{"fix at Nov 10 14:00 UTC", "6h after vs 24h before", "Verdict: FIXED", "10/100", "0/50", "5.0"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("pretty output is missing %q\n%s", want, buf.String())
		}
	}
}

func TestVerifyCmd_RequiresSinceAndPrintsJSON(t *testing.T) {
	fake := newVerifyFake(t)
	ts := fake.server()
	if _, err := executeCommand(t, "verify", "--api-url", ts.URL, "--api-key", "k", "--project-id", workflowProjectID); err == nil || !strings.Contains(err.Error(), "since") {
		t.Fatalf("verify without --since: err = %v, want a required-flag error", err)
	}
	out := captureStdout(t, func() {
		if _, err := executeCommand(t, "verify", "--api-url", ts.URL, "--api-key", "k", "--project-id", workflowProjectID,
			"--since", verifyFixTime.Format(time.RFC3339), "--before", verifyNow.Format(time.RFC3339), "--format", "json"); err != nil {
			t.Fatalf("verify: %v", err)
		}
	})
	var r verifyResult
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	// ChatOpenAI is fixed; search expects 0.5 failures after the fix, too few to tell.
	if r.Verdict != verdictInconclusive || len(r.Targets) != 2 {
		t.Errorf("verdict = %s, targets = %d; want inconclusive with 2 targets", r.Verdict, len(r.Targets))
	}
}
