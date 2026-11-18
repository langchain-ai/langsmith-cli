package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDashboardRequests(t *testing.T) {
	const id = "22222222-2222-4222-8222-222222222222"
	for _, tc := range []struct {
		name                   string
		args                   []string
		method, path, response string
		check                  func(*testing.T, *http.Request, map[string]any)
	}{
		{"create", []string{"create", "--title", "Monitoring"}, "POST", "/api/v1/charts/section", `{"id":"` + id + `"}`, func(t *testing.T, _ *http.Request, b map[string]any) {
			if b["title"] != "Monitoring" {
				t.Fatal(b)
			}
		}},
		{"list", []string{"list", "--limit", "2", "--offset", "4", "--title-contains", "a & b"}, "GET", "/api/v1/charts/section", `[]`, func(t *testing.T, r *http.Request, _ map[string]any) {
			if r.URL.Query().Get("title_contains") != "a & b" || r.URL.Query().Get("offset") != "4" {
				t.Fatal(r.URL)
			}
		}},
		{"get", []string{"get", id}, "POST", "/api/v1/charts/section/" + id, `{"id":"` + id + `"}`, func(t *testing.T, _ *http.Request, b map[string]any) {
			if b["omit_data"] != true {
				t.Fatal(b)
			}
			start, err := time.Parse(time.RFC3339, b["start_time"].(string))
			if err != nil {
				t.Fatal(err)
			}
			end, err := time.Parse(time.RFC3339, b["end_time"].(string))
			if err != nil || end.Sub(start) != 24*time.Hour {
				t.Fatal(b)
			}
		}},
		{"chart", []string{"chart", "create", "--dashboard-id", id, "--config", `{"chart_type":"line","title":"Runs","series":[{"name":"Runs","metric":"run_count"}]}`}, "POST", "/api/v1/charts/create", `{"id":"` + id + `"}`, func(t *testing.T, _ *http.Request, b map[string]any) {
			if b["section_id"] != id || b["title"] != "Runs" {
				t.Fatal(b)
			}
		}},
		{"preview", []string{"chart", "preview", "--config", `{"title":"Runs","chart_type":"line","series":[{"name":"Runs","metric":"run_count"}]}`, "--query", `{"start_time":"2026-09-18T00:00:00Z","stride":{"hours":1}}`}, "POST", "/api/v1/charts/preview", `{"data":[]}`, func(t *testing.T, _ *http.Request, b map[string]any) {
			chart := b["chart"].(map[string]any)
			if chart["series"].([]any)[0].(map[string]any)["id"] != "preview-0" {
				t.Fatal(chart)
			}
			if chart["title"] != nil || chart["series"] == nil || b["bucket_info"] == nil {
				t.Fatal(b)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != tc.method || r.URL.Path != tc.path {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				var body map[string]any
				if r.Method == "POST" {
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
				}
				tc.check(t, r, body)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, tc.response)
			})
			defer setupTestEnv(t, server.URL)()
			cmd := newDashboardCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if calls != 1 || !json.Valid(out.Bytes()) {
				t.Fatalf("calls=%d output=%s", calls, out.String())
			}
		})
	}
}

func TestDashboardStructuredDiagnostics(t *testing.T) {
	for _, status := range []int{401, 403, 404, 409, 422, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				fmt.Fprint(w, `{"detail":"sensitive-response-marker"}`)
			})
			defer setupTestEnv(t, server.URL)()
			flagOutputFormat = "json"
			cmd := newDashboardCmd()
			cmd.SetArgs([]string{"create", "--title", "Test"})
			cmd.SilenceErrors, cmd.SilenceUsage = true, true
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected failure")
			}
			var out bytes.Buffer
			if _, err := fmt.Fprintln(&out, err.Error()); err != nil {
				t.Fatal(err)
			}
			if !json.Valid(out.Bytes()) || bytes.Contains(out.Bytes(), []byte("sensitive-response-marker")) || bytes.Contains(out.Bytes(), []byte(`"command_failed"`)) {
				t.Fatal(out.String())
			}
		})
	}
}

func TestDashboardRejectsMalformedDefinitions(t *testing.T) {
	for _, input := range []string{
		`{"chart_type":"not-a-chart"}`,
		`{"chart_type":{},"title":"X","series":[]}`,
		`{"chart_type":"line","title":"X","series":[]}`,
		`{"chart_type":"line","title":"X","series":[{"name":"X"}]}`,
		`{"chart_type":"line","title":"X","series":[{"name":"X","metric":"feedback"}]}`,
		`{"chart_type":"text","markdown":"Hi","title":"Extra"}`,
	} {
		body, err := readDashboardObject(input)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateDashboardChart(body, false); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
	for _, input := range []string{
		`{"start_time":"yesterday"}`,
		`{"start_time":"2026-09-18T00:00:00Z","end_time":"2026-09-17T00:00:00Z"}`,
		`{"start_time":"2026-09-18T00:00:00Z","stride":{"minutes":0}}`,
		`{"omit_data":"true"}`,
	} {
		body, err := readDashboardObject(input)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateDashboardQuery(body); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}

func TestDashboardValidationAndDryRun(t *testing.T) {
	for _, args := range [][]string{
		{"create", "--title", " "},
		{"list", "--limit", "101"},
		{"get", "../other"},
		{"chart", "create", "--dashboard-id", "22222222-2222-4222-8222-222222222222", "--config", `{"chart_type":"line","chart_type":"bar"}`},
		{"chart", "create", "--dashboard-id", "22222222-2222-4222-8222-222222222222", "--config", `{"chart_type":"line","section_id":"other"}`},
		{"chart", "preview", "--config", `{"series":[]}`, "--query", `{}`},
	} {
		cmd := newDashboardCmd()
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) { t.Errorf("dry-run made request %s", r.URL) })
	defer setupTestEnv(t, server.URL)()
	cmd := newDashboardCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"create", "--title", "Preview", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"dry_run"`)) {
		t.Fatal(out.String())
	}
}

func TestDashboardCreateFailure(t *testing.T) {
	for _, status := range []int{200, 403, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				fmt.Fprint(w, `{}`)
			})
			defer setupTestEnv(t, server.URL)()
			cmd := newDashboardCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{"create", "--title", "Test"})
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			if err := cmd.Execute(); err == nil {
				t.Fatal("expected failure")
			}
			if calls != 1 || out.Len() != 0 {
				t.Fatalf("calls=%d output=%s", calls, out.String())
			}
		})
	}
}

func TestDashboardJSONForms(t *testing.T) {
	const input = `{"title":"Runs","chart_type":"line","series":[{"name":"Runs","metric":"run_count"}]}`
	path := filepath.Join(t.TempDir(), "chart.json")
	if err := os.WriteFile(path, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{input, path, "@" + path} {
		value, err := readDashboardObject(source)
		if err != nil || value["title"] != "Runs" {
			t.Fatalf("value=%v err=%v", value, err)
		}
	}
	for _, invalid := range []string{`null`, `[]`, `{}`, `{"title":"a","title":"b"}`, `{"x":1} {"x":2}`} {
		if _, err := readDashboardObject(invalid); err == nil {
			t.Fatalf("accepted %s", invalid)
		}
	}
}

func TestDashboardMutations(t *testing.T) {
	const id = "22222222-2222-4222-8222-222222222222"
	for _, tc := range []struct {
		args         []string
		method, path string
	}{
		{[]string{"clone", "--dashboard-id", id}, "POST", "/api/v1/charts/section/clone"},
		{[]string{"update", id, "--config", `{"title":"Renamed","layout":null}`}, "PATCH", "/api/v1/charts/section/" + id},
		{[]string{"chart", "update", id, "--config", `{"description":null}`}, "PATCH", "/api/v1/charts/" + id},
		{[]string{"delete", id, "--yes"}, "DELETE", "/api/v1/charts/section/" + id},
		{[]string{"chart", "delete", id, "--yes"}, "DELETE", "/api/v1/charts/" + id},
	} {
		t.Run(fmt.Sprint(tc.args), func(t *testing.T) {
			calls := 0
			server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != tc.method || r.URL.Path != tc.path {
					t.Errorf("unexpected %s %s", r.Method, r.URL)
				}
				if tc.method != "DELETE" {
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if tc.method == "POST" && body["section_id"] != id {
						t.Error(body)
					}
					if tc.method == "PATCH" && len(body) == 0 {
						t.Error("empty update")
					}
				}
				w.Header().Set("Content-Type", "application/json")
				if tc.method == "DELETE" {
					w.WriteHeader(204)
					return
				}
				fmt.Fprintf(w, `{"id":%q}`, id)
			})
			defer setupTestEnv(t, server.URL)()
			for _, dryRun := range []bool{true, false} {
				cmd := newDashboardCmd()
				args := append([]string{}, tc.args...)
				if dryRun {
					args = append(args, "--dry-run")
				}
				cmd.SetArgs(args)
				var out bytes.Buffer
				cmd.SetOut(&out)
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
				if dryRun && calls != 0 {
					t.Fatal("dry-run made a request")
				}
				if !json.Valid(out.Bytes()) {
					t.Fatal(out.String())
				}
			}
			if calls != 1 {
				t.Fatalf("calls=%d", calls)
			}
		})
	}
}

func TestDashboardMutationRejectsUnsafeInput(t *testing.T) {
	const id = "22222222-2222-4222-8222-222222222222"
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) { t.Errorf("invalid input made request %s", r.URL) })
	defer setupTestEnv(t, server.URL)()
	for _, args := range [][]string{
		{"delete", id}, {"chart", "delete", id}, {"clone", "--dashboard-id", "../x"},
		{"update", id, "--config", `{"id":"other"}`},
		{"update", id, "--config", `{"title":null}`},
		{"chart", "update", id, "--config", `{"section_id":"../x"}`},
		{"chart", "update", id, "--config", `{"layout":null}`},
	} {
		cmd := newDashboardCmd()
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
