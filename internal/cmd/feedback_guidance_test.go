package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestEmptyFeedbackGuidance(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	defer setupTestEnv(t, ts.URL)()
	flagOutputFormat = "json"
	cmd := newRunCmd()
	cmd.SetArgs([]string{"feedback", "list", "--run-id", workflowDataset, "--offset", "20"})
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
