package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestProjectConfigureFields(t *testing.T) {
	for _, mode := range []string{"read", "dry-run", "apply", "mismatch", "failure"} {
		t.Run(mode, func(t *testing.T) {
			writes := 0
			state := map[string]any{"id": workflowDataset, "name": "before", "description": nil, "trace_tier": nil, "default_dataset_id": nil, "extra": map[string]any{"private": "DO-NOT-PRINT", "thread_idle_seconds": 600}}
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.Contains(r.URL.Path, "datasets/") {
					fmt.Fprintf(w, `{"id":%q,"name":"golden"}`, workflowDataset)
					return
				}
				if r.Method == "PATCH" {
					writes++
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					if len(body) != 4 || body["extra"] != nil || body["name"] != "after" || body["description"] != "" || body["trace_tier"] != "longlived" || body["default_dataset_id"] != workflowDataset {
						t.Errorf("unexpected patch: %#v", body)
					}
					if mode == "failure" {
						w.WriteHeader(403)
						fmt.Fprint(w, `{"detail":"DO-NOT-PRINT"}`)
						return
					}
					if mode != "mismatch" {
						for key, value := range body {
							state[key] = value
						}
					}
				}
				_ = json.NewEncoder(w).Encode(state)
			})
			defer setupTestEnv(t, ts.URL)()
			flagOutputFormat = "json"
			cmd := newProjectConfigureCmd()
			args := []string{"--project-id", workflowDataset}
			if mode != "read" {
				args = append(args, "--name", "after", "--description", "", "--default-dataset", workflowDataset, "--trace-tier", "longlived")
				if mode == "dry-run" {
					args = append(args, "--dry-run")
				} else {
					args = append(args, "--apply")
				}
			}
			cmd.SetArgs(args)
			var err error
			out := captureStdout(t, func() { err = cmd.Execute() })
			if (err != nil) != (mode == "failure" || mode == "mismatch") {
				t.Fatalf("err=%v out=%s", err, out)
			}
			if strings.Contains(out, "DO-NOT-PRINT") || err != nil && strings.Contains(err.Error(), "DO-NOT-PRINT") {
				t.Fatal("leaked settings")
			}
			expected := 0
			if mode == "apply" || mode == "mismatch" || mode == "failure" {
				expected = 1
			}
			if writes != expected {
				t.Fatalf("writes=%d", writes)
			}
			if err == nil {
				var result map[string]any
				if json.Unmarshal([]byte(out), &result) != nil {
					t.Fatal(out)
				}
				settings := result["settings"].(map[string]any)
				if mode == "read" && settings["trace_tier"] != nil {
					t.Fatal("invented default")
				}
				if mode != "read" && settings["description"] != "" {
					t.Fatal("lost empty description")
				}
			}
		})
	}
}

func TestProjectConfigureInvalidEdits(t *testing.T) {
	for _, args := range [][]string{{"--apply"}, {"--name", "new"}, {"--name", "", "--dry-run"}, {"--trace-tier", "inherit", "--dry-run"}, {"--default-dataset", "", "--apply"}, {"--description", "x", "--dry-run", "--apply"}, {"--thread-idle-seconds", "119", "--dry-run"}} {
		cmd := newProjectConfigureCmd()
		cmd.SetArgs(args)
		if _, ok := cmd.Execute().(commandDiagnostic); !ok {
			t.Fatalf("expected validation diagnostic for %v", args)
		}
	}
}

func TestProjectConfigureConcurrentExtraEdit(t *testing.T) {
	reads, writes := 0, 0
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "PATCH" {
			writes++
			t.Error("must not overwrite changed extra")
		}
		reads++
		fmt.Fprintf(w, `{"id":%q,"extra":{"thread_idle_seconds":300,"revision":%d}}`, workflowDataset, reads)
	})
	defer setupTestEnv(t, ts.URL)()
	cmd := newProjectConfigureCmd()
	cmd.SetArgs([]string{"--project-id", workflowDataset, "--thread-idle-seconds", "600", "--apply"})
	err := cmd.Execute()
	d, ok := err.(commandDiagnostic)
	if !ok || d.code != "project_configuration_conflict" || writes != 0 || reads != 2 {
		t.Fatalf("err=%v reads=%d writes=%d", err, reads, writes)
	}
}
