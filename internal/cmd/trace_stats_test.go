package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"
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
			"total_cost": 8.2e-6,
			"error_rate": 0.25,
			"feedback_stats": {"correctness": {"n": 2}}
		}`))
	})
	defer setupTestEnv(t, ts.URL)()

	stats, err := fetchRunStats(t.Context(), MustGetClient(), "00000000-0000-0000-0000-000000000000", "2026-01-01", "2026-01-02", 0, "")
	if err != nil {
		t.Fatalf("fetchRunStats: %v", err)
	}

	if !slices.Contains(gotSelect, "total_cost") {
		t.Errorf("select did not request total_cost: %v", gotSelect)
	}

	cost, ok := stats.TotalCost.(float64)
	if !ok {
		t.Fatalf("TotalCost is %T (%v), want float64", stats.TotalCost, stats.TotalCost)
	}
	if cost != smallExponentCost {
		t.Errorf("TotalCost = %v, want %v", cost, smallExponentCost)
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
