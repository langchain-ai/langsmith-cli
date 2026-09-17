package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

func TestReadNextStepPinsContextAndQuotesNames(t *testing.T) {
	defer setupTestEnv(t, "https://example.invalid")()
	setupProjectDefaultConfig(t, "https://example.invalid")
	flagAPIKey = "never-print-this-key"
	name := "reviewer's $(touch /tmp/unsafe); #judge"
	got := readNextStep("evaluator", "get", "--session-id", defaultTestProject, "--", name)
	if !strings.HasPrefix(got, "env "+shellQuote("LANGSMITH_CONFIG_FILE="+os.Getenv("LANGSMITH_CONFIG_FILE"))+" langsmith ") {
		t.Fatalf("custom config not preserved: %s", got)
	}
	for _, want := range []string{"--profile demo", "--workspace workspace", "--api-url https://example.invalid", "--format json evaluator get", "--session-id " + defaultTestProject, "-- " + shellQuote(name)} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
	if strings.Contains(got, flagAPIKey) || strings.Contains(got, "--api-key") {
		t.Fatal("credential leaked into next step")
	}
}

func TestReadNextStepQuotesCustomConfig(t *testing.T) {
	defer setupTestEnv(t, "https://example.invalid")()
	path := "missing config's $(echo unsafe).json"
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	got := readNextStep("model", "list")
	if !strings.HasPrefix(got, "env "+shellQuote("LANGSMITH_CONFIG_FILE="+path)+" langsmith ") {
		t.Fatalf("custom config must be a single quoted argument: %s", got)
	}
	t.Setenv("LANGSMITH_CONFIG_FILE", "")
	if got := readNextStep("model", "list"); !strings.HasPrefix(got, "langsmith ") {
		t.Fatalf("unexpected environment prefix: %s", got)
	}
}
