package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// The API serves total_cost as a JSON number, often in exponent form for small
// values. Decoding has to keep the flat RunStats shape rather than falling
// through to the map (group_by) variant, which would report zeros.
const smallExponentCost = 8.2e-6

func TestTraceStatsCmd_RequestsAndDecodesTotalCost(t *testing.T) {
	var gotSelect []string

	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/runs/stats" {
			http.NotFound(w, r)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("reading request body: %v", err)
		}
		var req struct {
			Select []string `json:"select"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		gotSelect = req.Select

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"run_count": 3,
			"latency_p50": 1.5,
			"latency_p99": 2.5,
			"total_tokens": 300,
			"prompt_tokens": 200,
			"completion_tokens": 100,
			"median_tokens": 90,
			"tokens_p99": 150,
			"total_cost": 8.2e-6,
			"prompt_cost": 5.2e-6,
			"completion_cost": 3.0e-6,
			"cost_p50": 2.1e-6,
			"cost_p99": 4.4e-6,
			"error_rate": 0.25,
			"feedback_stats": {"correctness": {"n": 2}}
		}`))
	})
	defer setupTestEnv(t, ts.URL)()

	stats, err := fetchRunStats(t.Context(), MustGetClient(), "00000000-0000-0000-0000-000000000000", "2026-01-01", "2026-01-02", 0, "")
	if err != nil {
		t.Fatalf("fetchRunStats: %v", err)
	}

	for _, want := range []string{"total_cost", "prompt_cost", "completion_cost", "cost_p50", "cost_p99", "median_tokens", "tokens_p99"} {
		if !slices.Contains(gotSelect, want) {
			t.Errorf("select did not request %s: %v", want, gotSelect)
		}
	}

	if stats.TotalCost != smallExponentCost {
		t.Errorf("TotalCost = %v, want %v", stats.TotalCost, smallExponentCost)
	}
	if stats.PromptCost != 5.2e-6 || stats.CompletionCost != 3.0e-6 {
		t.Errorf("cost split = (%v, %v), want (5.2e-6, 3e-6)", stats.PromptCost, stats.CompletionCost)
	}
	if stats.CostP50 != 2.1e-6 || stats.CostP99 != 4.4e-6 {
		t.Errorf("cost percentiles = (%v, %v), want (2.1e-6, 4.4e-6)", stats.CostP50, stats.CostP99)
	}
	if stats.MedianTokens != 90 || stats.TokensP99 != 150 {
		t.Errorf("token percentiles = (%d, %d), want (90, 150)", stats.MedianTokens, stats.TokensP99)
	}

	// A fall-through to the map variant zeroes every field, so assert a couple
	// of neighbours survived alongside the cost.
	if stats.RunCount != 3 {
		t.Errorf("RunCount = %d, want 3 (response decoded as the wrong union variant?)", stats.RunCount)
	}
	if len(stats.FeedbackStats) != 1 {
		t.Errorf("FeedbackStats has %d keys, want 1", len(stats.FeedbackStats))
	}
}

func TestTraceStatsSelectsKnownMetrics(t *testing.T) {
	for _, metric := range traceStatsSelect() {
		if !metric.IsKnown() {
			t.Errorf("select metric %q is not a known langsmith-go value", metric)
		}
	}
	if len(traceStatsSelect()) == 0 {
		t.Fatal("select list is empty")
	}
}

func TestPrintStatsPretty_RendersTokenAndCostMetrics(t *testing.T) {
	stats := runStats{
		RunCount: 114, TotalTokens: 611535513, MedianTokens: 53654, TokensP99: 204916,
		TotalCost: 423.98817696, PromptCost: 299.47622136, CompletionCost: 124.5119556,
		CostP50: smallExponentCost, CostP99: 0.2922659788,
	}

	out := captureStdout(t, func() { printStatsPretty(&stats, nil, false) })

	for _, want := range []string{
		"53654",    // tokens p50
		"204916",   // tokens p99
		"423.9882", // total cost
		"299.4762", // prompt cost
		"124.5120", // completion cost
		"0.000008", // sub-cent p50: extra decimals, never scientific notation
		"0.2923",   // cost p99
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q\n%s", want, out)
		}
	}
}

func TestPrintStatsPretty_ComparesCosts(t *testing.T) {
	primary := runStats{TotalTokens: 300, TokensP99: 120, TotalCost: 1.5, CostP99: 0.25}
	compare := runStats{TotalTokens: 200, TokensP99: 100, TotalCost: 1.0, CostP99: 0.30}

	out := captureStdout(t, func() { printStatsPretty(&primary, &compare, true) })

	for _, want := range []string{"+0.5000", "-0.0500", "+20"} {
		if !strings.Contains(out, want) {
			t.Errorf("comparison output is missing delta %q\n%s", want, out)
		}
	}
}

func TestFmtCost(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "-"},
		{423.98817696, "423.9882"},
		{0.01, "0.0100"},
		{smallExponentCost, "0.000008"},
		{-0.5, "-0.5000"},
	}
	for _, c := range cases {
		if got := fmtCost(c.in); got != c.want {
			t.Errorf("fmtCost(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
