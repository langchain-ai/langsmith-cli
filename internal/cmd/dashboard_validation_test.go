package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

// Exercise commands, not just helpers: invalid definitions must never write,
// while valid modern and legacy payloads must survive dry-run unchanged.
func TestDashboardVisualizationCompatibility(t *testing.T) {
	const id = "22222222-2222-4222-8222-222222222222"
	const count = `{"name":"Calls","metric_definition":{"type":"count"},"filter_definition":{"source_type":"tracing_project","project_ids":["` + id + `"]}}`
	const grouped = `{"name":"Calls","metric_definition":{"type":"count"},"filter_definition":{"source_type":"tracing_project","project_ids":["` + id + `"]},"group_by_definitions":[{"attribute":"name"}]}`
	const latency = `{"name":"Latency","metric_definition":{"type":"percentile","field":"latency_seconds","params":{"p":0.5}},"filter_definition":{"source_type":"tracing_project","project_ids":["` + id + `"]}}`
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) { t.Errorf("validation made request %s", r.URL) })
	defer setupTestEnv(t, server.URL)()
	for _, tc := range []struct {
		name, kind, series string
		valid              bool
	}{
		{"legacy line", "line", `{"name":"Calls","metric":"run_count"}`, true},
		{"modern ranked bar", "top-k", grouped, true},
		{"modern donut", "pie", grouped, true},
		{"modern KPI", "kpi", count, true},
		{"modern count table", "table", count, true},
		{"multi metric bars", "bar", count + "," + count, true},
		{"legacy ranked bar", "top-k", `{"name":"Calls","metric":"run_count"}`, false},
		{"multi metric donut", "pie", count + "," + count, false},
		{"ungrouped donut", "pie", count, false},
		{"latency table", "table", latency, false},
		{"grouped KPI", "kpi", grouped, false},
		{"grouped multi metric", "line", grouped + "," + count, false},
		{"display series ID", "line", `{"name":"Calls","metric":"run_count","id":"` + id + `:group"}`, false},
		{"malformed source", "line", `{"name":"Calls","metric_definition":{"type":"count"},"filter_definition":[]}`, false},
		{"malformed metric", "line", `{"name":"Calls","metric":"run_count","metric_definition":[]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := `{"title":"Chart","chart_type":"` + tc.kind + `","series":[` + tc.series + `]}`
			for _, update := range []bool{false, true} {
				args := []string{"chart", "create", "--dashboard-id", id, "--config", config, "--dry-run"}
				if update {
					args = []string{"chart", "update", id, "--config", config, "--dry-run"}
				}
				cmd := newDashboardCmd()
				cmd.SetArgs(args)
				cmd.SilenceUsage, cmd.SilenceErrors = true, true
				var out bytes.Buffer
				cmd.SetOut(&out)
				err := cmd.Execute()
				if (err == nil) != tc.valid {
					t.Fatalf("update=%v err=%v", update, err)
				}
				if tc.valid {
					var result struct {
						Request map[string]any `json:"request"`
					}
					if err := json.Unmarshal(out.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					if result.Request["chart_type"] != tc.kind {
						t.Fatal(result)
					}
				} else if out.Len() != 0 {
					t.Fatalf("invalid command emitted success: %s", out.String())
				}
			}
		})
	}
}
