package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestEmptyResultGuidancePreservesFields(t *testing.T) {
	for _, count := range []int{0, 1} {
		result := map[string]any{"items": []string{}, "offset": 20, "next_cursor": "next"}
		emptyResultGuidance(result, count, "No results on this page.", "Check the offset.")
		if result["offset"] != 20 || result["next_cursor"] != "next" {
			t.Fatal("pagination changed")
		}
		if (result["message"] != nil) != (count == 0) {
			t.Fatal("incorrect empty guidance")
		}
	}
}

func TestEmptyFeedbackGuidance(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	defer setupTestEnv(t, ts.URL)()
	flagOutputFormat = "json"
	cmd := newRunCmd()
	cmd.SetArgs([]string{"feedback", "list", "--run-id", presetTestID, "--offset", "20"})
	var err error
	out := captureStdout(t, func() { err = cmd.Execute() })
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if json.Unmarshal([]byte(out), &result) != nil || result["next_steps"] == nil || !strings.Contains(out, "Missing feedback is not a score") {
		t.Fatalf("invalid guidance: %s", out)
	}
	if result["offset"] != float64(20) {
		t.Fatal("offset changed")
	}
}

func TestDatasetAddMissingPlanDiagnostic(t *testing.T) {
	cmd := newDatasetAddCmd()
	cmd.SetArgs([]string{"--dataset", presetTestID})
	err := cmd.Execute()
	d, ok := err.(commandDiagnostic)
	if !ok || d.code != "selection_required" || !strings.Contains(d.next, "Preview") {
		t.Fatalf("missing actionable diagnostic: %v", err)
	}
}
