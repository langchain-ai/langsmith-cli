package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestDatasetAddBooleanFilters(t *testing.T) {
	for _, flag := range []string{"--error", "--no-error", "--error=false"} {
		t.Run(flag, func(t *testing.T) {
			queries := 0
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api/v1/datasets/"+importDatasetID {
					fmt.Fprintf(w, `{"id":%q}`, importDatasetID)
					return
				}
				if r.URL.Path != "/api/v1/runs/query" {
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				queries++
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if flag == "--error" && body["error"] != true {
					t.Errorf("error filter lost: %#v", body)
				}
				if flag == "--no-error" && body["error"] != false {
					t.Errorf("no-error filter lost: %#v", body)
				}
				if flag == "--error=false" {
					if _, ok := body["error"]; ok {
						t.Errorf("disabled filter sent: %#v", body)
					}
				}
				fmt.Fprint(w, `{"runs":[],"cursors":{}}`)
			})
			defer setupTestEnv(t, ts.URL)()
			cmd := newDatasetAddCmd()
			cmd.SetArgs([]string{"--dataset", importDatasetID, "--project-id", deleteTestProjectID, "--dry-run", flag})
			var err error
			out := captureStdout(t, func() { err = cmd.Execute() })
			if err != nil || queries != 1 || !json.Valid([]byte(out)) {
				t.Fatalf("err=%v queries=%d out=%s", err, queries, out)
			}
		})
	}
}

func TestDatasetAddRejectsBlankSelectors(t *testing.T) {
	for _, flag := range []string{"run-id", "thread-id", "selection"} {
		for _, value := range []string{"", " "} {
			cmd := newDatasetAddCmd()
			cmd.SetArgs([]string{"--dataset", importDatasetID, "--dry-run", "--" + flag, value})
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "must not be blank") {
				t.Fatalf("%s: %v", flag, err)
			}
		}
	}
}

func TestDatasetNestedErrorsAreSilent(t *testing.T) {
	cmd := newDatasetAddCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--dataset", importDatasetID, "--dry-run", "--error", "--no-error"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("conflicting filters accepted")
	}
	if stderr.Len() != 0 {
		t.Fatalf("nested diagnostics leaked: %s", stderr.String())
	}
}

func TestDatasetCreateReturnsErrorWithoutRetry(t *testing.T) {
	calls := 0
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		fmt.Fprint(w, `{}`)
	})
	defer setupTestEnv(t, ts.URL)()
	cmd := newDatasetCreateCmd()
	cmd.SetArgs([]string{"--name", "retry-test"})
	var err error
	out := captureStdout(t, func() { err = cmd.Execute() })
	if err == nil || calls != 1 || out != "" {
		t.Fatalf("err=%v calls=%d out=%s", err, calls, out)
	}
}

func TestDatasetAddRequiresPlan(t *testing.T) {
	c := newDatasetAddCmd()
	c.SetArgs([]string{"--dataset", importDatasetID, "--project-id", deleteTestProjectID, "--run-id", importRunID})
	if c.Execute() == nil {
		t.Fatal("write allowed without reviewed selection")
	}
}
