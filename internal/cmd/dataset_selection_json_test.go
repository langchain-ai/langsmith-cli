package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectionStrictJSON(t *testing.T) {
	for _, raw := range []string{
		`{"version":1,"version":2}`, `{"unknown_field":true}`,
		`{"examples":[{"inputs":{"id":1,"id":2}}]}`, `{} {}`,
		`{"examples":` + strings.Repeat("[", 130) + `0` + strings.Repeat("]", 130) + `}`,
	} {
		var s traceSelection
		if err := decodeTraceSelection([]byte(raw), &s); err == nil {
			t.Errorf("accepted invalid selection JSON")
		}
	}
}

func TestSelectionPrecisionAndIdentity(t *testing.T) {
	first, err := preciseTraceIO(`{"inputs":{"id":9007199254740993},"outputs":{"value":0}}`)
	if err != nil {
		t.Fatal(err)
	}
	s := testTraceSelection()
	s.Examples[0].Inputs = first.Inputs
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var decoded traceSelection
	if err := decodeTraceSelection(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if stringValue := decoded.Examples[0].Inputs["id"].(json.Number).String(); stringValue != "9007199254740993" {
		t.Fatalf("rounded: %s", stringValue)
	}
	id, _ := selectionIdentity(decoded, decoded.Examples[0])
	decoded.Examples[0].Inputs["id"] = json.Number("9007199254740992")
	other, _ := selectionIdentity(decoded, decoded.Examples[0])
	if id == other {
		t.Fatal("distinct integers share an identity")
	}
	if equalTraceJSON(first.Inputs, decoded.Examples[0].Inputs) {
		t.Fatal("distinct integers compare equal")
	}
}

func TestDatasetSelectionPrecisionRoundTrip(t *testing.T) {
	const secondRun = "01a0924b-79d6-7d22-b6f7-61d385ee3716"
	var stored map[string]any
	writes := 0
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/datasets/"):
			fmt.Fprintf(w, `{"id":%q}`, importDatasetID)
		case r.URL.Path == "/api/v1/runs/query":
			fmt.Fprintf(w, `{"runs":[{"id":%q,"trace_id":%q,"inputs":{"id":9007199254740993}},{"id":%q,"trace_id":%q,"inputs":{"id":2}}]}`, importRunID, importRunID, secondRun, secondRun)
		case r.Method == "GET":
			if stored == nil {
				w.WriteHeader(404)
				fmt.Fprint(w, `{}`)
				return
			}
			_ = json.NewEncoder(w).Encode(stored)
		case r.Method == "POST":
			writes++
			d := json.NewDecoder(r.Body)
			d.UseNumber()
			if err := d.Decode(&stored); err != nil {
				t.Error(err)
			}
			_ = json.NewEncoder(w).Encode(stored)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	defer setupTestEnv(t, ts.URL)()
	flagWorkspaceID = "demo-workspace"
	path := filepath.Join(t.TempDir(), "selection.json")
	preview := newDatasetAddCmd()
	preview.SetArgs([]string{"--dataset", importDatasetID, "--project-id", deleteTestProjectID, "--thread-id", "conversation-1", "--limit", "1", "--dry-run", "--output", path})
	if err := preview.Execute(); err != nil {
		t.Fatal(err)
	}
	selection, err := readTraceSelection(path)
	if err != nil {
		t.Fatal(err)
	}
	if selection.SelectionInfo == nil || !selection.SelectionInfo.HasMore || selection.SelectionInfo.Selected != 1 || selection.SelectionInfo.Limit != 1 {
		t.Fatalf("missing truncation metadata: %+v", selection.SelectionInfo)
	}
	if len(selection.Examples) != 1 {
		t.Fatal("probe run included in import")
	}
	for _, want := range []string{"created", "skipped"} {
		cmd := newDatasetAddCmd()
		cmd.SetArgs([]string{"--dataset", importDatasetID, "--project-id", deleteTestProjectID, "--selection", path})
		var err error
		out := captureStdout(t, func() { err = cmd.Execute() })
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Counts      map[string]int
			ProjectID   string `json:"project_id"`
			WorkspaceID string `json:"workspace_id"`
		}
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatal(err)
		}
		if result.Counts[want] != 1 || result.ProjectID != deleteTestProjectID || result.WorkspaceID != "demo-workspace" {
			t.Fatalf("incorrect import summary: %s", out)
		}
	}
	if writes != 1 || stored["inputs"].(map[string]any)["id"].(json.Number).String() != "9007199254740993" {
		t.Fatal("write duplicated or input rounded")
	}
	flagWorkspaceID = "different-workspace"
	cmd := newDatasetAddCmd()
	cmd.SetArgs([]string{"--dataset", importDatasetID, "--project-id", deleteTestProjectID, "--selection", path})
	if err := cmd.Execute(); err == nil {
		t.Fatal("workspace mismatch accepted")
	}
	if writes != 1 {
		t.Fatal("workspace mismatch caused a write")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
