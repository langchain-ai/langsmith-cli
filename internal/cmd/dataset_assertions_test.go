package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
)

func TestDatasetAssertionsValidation(t *testing.T) {
	for _, body := range []string{`null`, `[]`, `{}`, `[{"key":"x"}]`, `[{"key":"x","comment":" "}]`, `[{"key":"x","comment":"ok"},{"key":"x","comment":"other"}]`, `[{"key":"x","comment":"ok","unknown":1}]`, `[{"key":"x","key":"y","comment":"ok"}]`} {
		t.Run(body, func(t *testing.T) {
			if _, err := readDatasetAssertions(workflowFile(t, body)); err == nil {
				t.Fatal("accepted invalid assertions")
			}
		})
	}
}

func TestDatasetAssertionsPreview(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/datasets/" + importDatasetID:
			fmt.Fprintf(w, `{"id":%q}`, importDatasetID)
		case "/api/v1/runs/query":
			fmt.Fprintf(w, `{"runs":[{"id":%q,"trace_id":%q,"start_time":"2026-09-16T00:00:00Z","inputs":{"query":"Cancel?"},"outputs":{"answer":"wrong"}}],"cursors":{}}`, importRunID, importRunID)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", 400)
		}
	})
	defer setupTestEnv(t, ts.URL)()
	file := workflowFile(t, `[{"key":"must_confirm","comment":"Ask before canceling."}]`)
	for _, selector := range []string{"--trace-id", "--run-id"} {
		t.Run(selector, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "selection.json")
			cmd := newDatasetAddCmd()
			cmd.SetArgs([]string{"--dataset", importDatasetID, "--project-id", deleteTestProjectID, selector, importRunID, "--assertions", file, "--dry-run", "--output", path})
			captureStdout(t, func() {
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
			})
			s, err := readTraceSelection(path)
			if err != nil {
				t.Fatal(err)
			}
			if s.ReferenceMode != "corrected" || len(s.Examples) != 1 {
				t.Fatalf("invalid selection: %+v", s)
			}
			want := map[string]any{"assertions": []any{map[string]any{"key": "must_confirm", "comment": "Ask before canceling."}}}
			if !equalTraceJSON(s.Examples[0].Outputs, want) || s.Examples[0].Inputs["query"] != "Cancel?" {
				t.Fatal("incorrect input/reference payload")
			}
			b, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			var copy traceSelection
			if err := decodeTraceSelection(b, &copy); err != nil {
				t.Fatal(err)
			}
			id, hash := selectionIdentity(s, s.Examples[0])
			id2, hash2 := selectionIdentity(copy, copy.Examples[0])
			if id != id2 || hash != hash2 {
				t.Fatal("replay identity changed")
			}
		})
	}
	for _, flags := range [][]string{{"--thread-id", "thread"}, {"--filter", "eq(is_root,true)"}, {"--trace-id", importRunID, "--reference-mode", "observed"}, {"--trace-id", importRunID, "--outputs-pointer", "/answer"}, {"--selection", "selection.json"}} {
		cmd := newDatasetAddCmd()
		args := append([]string{"--dataset", importDatasetID, "--project-id", deleteTestProjectID, "--assertions", file, "--dry-run"}, flags...)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("accepted conflicting flags: %v", flags)
		}
	}
}
