package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const importDatasetID = "9970db1d-796f-4ae4-813e-25dfda3a8f74"
const importRunID = "01a0924b-ccd7-75a1-991c-d7a4263aa2a3"

func testTraceSelection() traceSelection {
	return traceSelection{Version: 1, APIURL: "https://api.smith.langchain.com", ProjectID: deleteTestProjectID, DatasetID: importDatasetID, ReferenceMode: "inputs-only", Examples: []traceExample{{RunID: importRunID, TraceID: importRunID, StartTime: time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC), Inputs: map[string]any{"question": "refund?"}}}}
}

func TestTraceSelectionSafety(t *testing.T) {
	s := testTraceSelection()
	if err := s.validate(); err != nil {
		t.Fatal(err)
	}
	id, hash := selectionIdentity(s, s.Examples[0])
	id2, hash2 := selectionIdentity(s, s.Examples[0])
	if id != id2 || hash != hash2 {
		t.Fatal("identity unstable")
	}
	s.Examples[0].Outputs = map[string]any{"answer": "corrected"}
	if s.validate() == nil {
		t.Fatal("inputs-only accepted outputs")
	}
	s.ReferenceMode = "corrected"
	if err := s.validate(); err != nil {
		t.Fatal(err)
	}
	id2, _ = selectionIdentity(s, s.Examples[0])
	if id == id2 {
		t.Fatal("different payload retained identity")
	}
	s.Examples = append(s.Examples, s.Examples[0])
	if s.validate() == nil {
		t.Fatal("duplicate run accepted")
	}
}

func TestDatasetTraceBlankSelectors(t *testing.T) {
	for _, name := range []string{"trace-id", "run-ids", "trace-ids"} {
		cmd := newDatasetSelectionPreviewCmd()
		cmd.SetArgs([]string{"--dataset", importDatasetID, "--project-id", deleteTestProjectID, "--" + name, " "})
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "blank") {
			t.Fatalf("selector %s err=%v", name, err)
		}
	}
	cmd := newDatasetSelectionApplyCmd()
	cmd.SetArgs([]string{"--dataset", importDatasetID, "--project-id", deleteTestProjectID, "--selection", " "})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "nonblank") {
		t.Fatalf("err=%v", err)
	}
}

func TestDatasetTracePartialRetry(t *testing.T) {
	const secondRun = "01a0924b-79d6-7d22-b6f7-61d385ee3716"
	stored := map[string]map[string]any{}
	writes := map[string]int{}
	denySecond := true
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/datasets/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"id": importDatasetID})
		case r.URL.Path == "/api/v1/runs/query":
			_ = json.NewEncoder(w).Encode(map[string]any{"runs": []any{map[string]any{"id": importRunID, "trace_id": importRunID}, map[string]any{"id": secondRun, "trace_id": secondRun}}, "cursors": map[string]any{}})
		case r.Method == http.MethodGet:
			id := filepath.Base(r.URL.Path)
			if ex := stored[id]; ex != nil {
				_ = json.NewEncoder(w).Encode(ex)
			} else {
				w.WriteHeader(404)
				_, _ = w.Write([]byte(`{"detail":"missing"}`))
			}
		case r.Method == http.MethodPost:
			var ex map[string]any
			_ = json.NewDecoder(r.Body).Decode(&ex)
			id := ex["id"].(string)
			writes[id]++
			if ex["source_run_id"] == secondRun && denySecond {
				w.WriteHeader(403)
				_, _ = w.Write([]byte(`{"detail":"denied"}`))
				return
			}
			stored[id] = ex
			_ = json.NewEncoder(w).Encode(ex)
		}
	})
	defer setupTestEnv(t, ts.URL)()
	s := testTraceSelection()
	s.APIURL = MustGetClient().APIURL()
	s.Examples = append(s.Examples, traceExample{RunID: secondRun, TraceID: secondRun, Inputs: map[string]any{"q": "second"}})
	path := filepath.Join(t.TempDir(), "selection.json")
	if err := writeTraceSelection(s, path); err != nil {
		t.Fatal(err)
	}
	run := func() (map[string]any, error) {
		cmd := newDatasetSelectionApplyCmd()
		cmd.SetArgs([]string{"--selection", path, "--dataset", importDatasetID, "--project-id", deleteTestProjectID})
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		var err error
		out := captureStdout(t, func() { err = cmd.Execute() })
		var result map[string]any
		if e := json.Unmarshal([]byte(out), &result); e != nil {
			t.Fatal(e)
		}
		return result, err
	}
	result, err := run()
	if err == nil || result["failed"] != float64(1) || len(stored) != 1 {
		t.Fatalf("result=%v err=%v stored=%d", result, err, len(stored))
	}
	denySecond = false
	result, err = run()
	if err != nil || result["failed"] != float64(0) || len(stored) != 2 {
		t.Fatalf("result=%v err=%v", result, err)
	}
	first, _ := selectionIdentity(s, s.Examples[0])
	if writes[first] != 1 {
		t.Fatalf("successful item re-created: %d", writes[first])
	}
}

func TestTraceSelectionPointersAndFile(t *testing.T) {
	obj := map[string]any{"a/b": map[string]any{"x": true}, "list": []any{map[string]any{"y": true}}}
	for _, p := range []string{"", "/a~1b", "/list/0"} {
		if _, err := objectAtPointer(obj, p); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{"a", "/missing", "/list/2", "/a~1b/x", "/~2"} {
		if _, err := objectAtPointer(obj, p); err == nil {
			t.Fatalf("accepted %s", p)
		}
	}
	path := filepath.Join(t.TempDir(), "selection.json")
	if err := writeTraceSelection(testTraceSelection(), path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("permissions=%v", info.Mode())
	}
	if err := writeTraceSelection(testTraceSelection(), path); err == nil {
		t.Fatal("overwrote selection")
	}
	if _, err := readTraceSelection(path); err != nil {
		t.Fatal(err)
	}
}

func TestDatasetTracePreview(t *testing.T) {
	for _, observed := range []bool{false, true} {
		t.Run(map[bool]string{false: "inputs-only", true: "observed"}[observed], func(t *testing.T) {
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.Contains(r.URL.Path, "/datasets/") {
					_ = json.NewEncoder(w).Encode(map[string]any{"id": importDatasetID})
					return
				}
				if r.URL.Path != "/api/v1/runs/query" {
					t.Errorf("unexpected path %s", r.URL.Path)
					http.Error(w, "bad path", 400)
					return
				}
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				if body["session"].([]any)[0] != deleteTestProjectID {
					t.Error("wrong scope")
				}
				if body["start_time"] != nil {
					t.Error("explicit ID unexpectedly time-bounded")
				}
				for _, field := range body["select"].([]any) {
					if !observed && field == "outputs" {
						t.Error("inputs-only fetched outputs")
					}
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"runs": []any{map[string]any{"id": importRunID, "trace_id": importRunID, "inputs": map[string]any{"q": "test"}, "outputs": map[string]any{"a": "wrong"}}}, "cursors": map[string]any{}})
			})
			defer setupTestEnv(t, ts.URL)()
			cmd := newDatasetSelectionPreviewCmd()
			args := []string{"--dataset", importDatasetID, "--project-id", deleteTestProjectID, "--trace-id", importRunID}
			if observed {
				args = append(args, "--reference-mode", "observed")
			}
			cmd.SetArgs(args)
			var err error
			out := captureStdout(t, func() { err = cmd.Execute() })
			if err != nil {
				t.Fatal(err)
			}
			var s traceSelection
			if err := json.Unmarshal([]byte(out), &s); err != nil {
				t.Fatal(err)
			}
			if len(s.Examples) != 1 || (s.Examples[0].Outputs != nil) != observed {
				t.Fatalf("bad selection %+v", s)
			}
			if s.SelectionInfo == nil || s.SelectionInfo.HasMore || s.SelectionInfo.Selected != 1 || s.SelectionInfo.Scope != "explicit_ids" {
				t.Fatalf("incorrect explicit selection metadata: %+v", s.SelectionInfo)
			}
		})
	}
}

func TestDatasetTraceImportRetryAndFailures(t *testing.T) {
	for _, mode := range []string{"success", "ambiguous", "denied", "conflict", "wrong-source"} {
		t.Run(mode, func(t *testing.T) {
			stored := map[string]any(nil)
			writes := 0
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Header.Get("x-tenant-id") != "demo-workspace" {
					t.Error("workspace not preserved")
				}
				switch {
				case strings.Contains(r.URL.Path, "/datasets/"):
					_ = json.NewEncoder(w).Encode(map[string]any{"id": importDatasetID})
				case r.URL.Path == "/api/v1/runs/query":
					trace := importRunID
					if mode == "wrong-source" {
						trace = "different"
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"runs": []any{map[string]any{"id": importRunID, "trace_id": trace, "inputs": map[string]any{"question": "refund?"}}}, "cursors": map[string]any{}})
				case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/examples/"):
					if mode == "conflict" {
						_ = json.NewEncoder(w).Encode(map[string]any{"id": "existing", "dataset_id": importDatasetID, "inputs": map[string]any{"changed": true}})
						return
					}
					if stored == nil {
						w.WriteHeader(404)
						_, _ = w.Write([]byte(`{"detail":"not found"}`))
						return
					}
					_ = json.NewEncoder(w).Encode(stored)
				case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/examples"):
					writes++
					if mode == "denied" {
						w.WriteHeader(403)
						_, _ = w.Write([]byte(`{"detail":"denied"}`))
						return
					}
					_ = json.NewDecoder(r.Body).Decode(&stored)
					if stored["outputs"] != nil {
						t.Error("unexpected output copied")
					}
					if stored["use_source_run_io"] != false {
						t.Error("source copy not disabled")
					}
					if stored["source_run_id"] != importRunID || stored["source_session_id"] != deleteTestProjectID {
						t.Error("native provenance missing")
					}
					if mode == "ambiguous" {
						w.WriteHeader(500)
						_, _ = w.Write([]byte(`{"detail":"response lost"}`))
						return
					}
					_ = json.NewEncoder(w).Encode(stored)
				default:
					t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
					http.Error(w, "unexpected", 400)
				}
			})
			defer setupTestEnv(t, ts.URL)()
			flagWorkspaceID = "demo-workspace"
			s := testTraceSelection()
			s.APIURL = MustGetClient().APIURL()
			path := filepath.Join(t.TempDir(), "selection.json")
			if err := writeTraceSelection(s, path); err != nil {
				t.Fatal(err)
			}
			run := func() (string, error) {
				cmd := newDatasetSelectionApplyCmd()
				cmd.SetArgs([]string{"--selection", path, "--dataset", importDatasetID, "--project-id", deleteTestProjectID})
				cmd.SetErr(io.Discard)
				cmd.SetOut(io.Discard)
				var err error
				out := captureStdout(t, func() { err = cmd.Execute() })
				return out, err
			}
			out, err := run()
			if mode == "success" || mode == "ambiguous" {
				if err != nil || writes != 1 {
					t.Fatalf("err=%v writes=%d out=%s", err, writes, out)
				}
				out, err = run()
				if err != nil || writes != 1 || !strings.Contains(out, `"skipped"`) {
					t.Fatalf("retry err=%v writes=%d out=%s", err, writes, out)
				}
			} else {
				if err == nil {
					t.Fatal("expected failure")
				}
				if mode != "denied" && writes != 0 {
					t.Fatal("unexpected mutation")
				}
			}
		})
	}
}
