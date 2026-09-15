package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
)

func TestInsightsReadValidation(t *testing.T) {
	for _, args := range [][]string{
		{"invalid"},
		{deleteTestProjectID, "--cluster-id", " "},
		{deleteTestProjectID, "--limit", "0"},
		{deleteTestProjectID, "--limit", "101"},
		{deleteTestProjectID, "--offset", "-1"},
		{deleteTestProjectID, "--sort-order", "desc"},
		{deleteTestProjectID, "--sort-by", "score", "--sort-order", "invalid"},
	} {
		cmd := newInsightsRunsCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("accepted invalid arguments: %v", args)
		}
	}
}

func TestInsightsRunsPagination(t *testing.T) {
	for _, next := range []string{"null", "0", "2"} {
		t.Run(next, func(t *testing.T) {
			calls := 0
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Query().Get("limit") != "2" || r.URL.Query().Get("cluster_id") != deleteTestProjectID || r.URL.Query().Get("attribute_sort_key") != "resolved" {
					t.Error("query not forwarded")
				}
				if r.URL.Path != "/api/v1/sessions/"+deleteTestProjectID+"/insights/"+deleteTestProjectID+"/runs" {
					t.Errorf("wrong path %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{"runs":[{"id":"run","attributes":{"resolved":false,"score":0},"inputs_preview":"question"}],"offset":%s}`, next)
			})
			defer setupTestEnv(t, ts.URL)()
			flagOutputFormat = "json"
			cmd := newInsightsRunsCmd()
			cmd.SetArgs([]string{deleteTestProjectID, "--project-id", deleteTestProjectID, "--cluster-id", deleteTestProjectID, "--limit", "2", "--sort-by", "resolved"})
			var err error
			out := captureStdout(t, func() { err = cmd.Execute() })
			if err != nil {
				t.Fatal(err)
			}
			var result map[string]any
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatal(err)
			}
			p := result["pagination"].(map[string]any)
			if (p["next_offset"] != nil) != (next == "2") || result["io_mode"] != "preview" || calls != 1 {
				t.Fatalf("invalid pagination: %s", out)
			}
			attrs := result["runs"].([]any)[0].(map[string]any)["attributes"].(map[string]any)
			if attrs["resolved"] != false || attrs["score"] != float64(0) {
				t.Fatal("lost false/zero values")
			}
		})
	}
}

func TestInsightsListOnePage(t *testing.T) {
	calls := 0
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		if q.Get("limit") != "1" || q.Get("offset") != "2" || q.Get("config_id") != deleteTestProjectID {
			t.Error("incorrect pagination/config query")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"clustering_jobs":[{"id":"job","name":"demo","status":"queued"}]}`))
	})
	defer setupTestEnv(t, ts.URL)()
	flagOutputFormat = "json"
	cmd := newInsightsListCmd()
	cmd.SetArgs([]string{"--project-id", deleteTestProjectID, "--config-id", deleteTestProjectID, "--limit", "1", "--offset", "2"})
	var err error
	out := captureStdout(t, func() { err = cmd.Execute() })
	if err != nil {
		t.Fatal(err)
	}
	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(result) != 1 || result[0]["project_id"] != deleteTestProjectID {
		t.Fatal("unexpected listing")
	}
}
