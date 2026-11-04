package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// ==================== Command structure ====================

func TestExperimentCmd_Subcommands(t *testing.T) {
	cmd := newExperimentCmd()
	expected := map[string]bool{"list": false, "get": false}
	for _, sub := range cmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("experiment missing subcommand %q", name)
		}
	}
}

func TestExperimentCmd_UseField(t *testing.T) {
	cmd := newExperimentCmd()
	if cmd.Use != "experiments" {
		t.Errorf("expected Use=experiments, got %q", cmd.Use)
	}
}

func TestExperimentCmd_SubcommandCount(t *testing.T) {
	cmd := newExperimentCmd()
	if got := len(cmd.Commands()); got != 2 {
		t.Errorf("expected 2 subcommands, got %d", got)
	}
}

// ==================== experiment list flags ====================

func TestExperimentListCmd_Flags(t *testing.T) {
	cmd := newExperimentListCmd()
	tests := []struct {
		name   string
		defVal string
		short  string
	}{
		{"dataset", "", ""},
		{"limit", "20", "n"},
		{"output", "", "o"},
	}
	for _, tc := range tests {
		f := cmd.Flags().Lookup(tc.name)
		if f == nil {
			t.Errorf("flag --%s not found", tc.name)
			continue
		}
		if f.DefValue != tc.defVal {
			t.Errorf("flag --%s: expected default %q, got %q", tc.name, tc.defVal, f.DefValue)
		}
		if tc.short != "" && f.Shorthand != tc.short {
			t.Errorf("flag --%s: expected shorthand %q, got %q", tc.name, tc.short, f.Shorthand)
		}
	}
}

func TestExperimentListCmd_UseField(t *testing.T) {
	cmd := newExperimentListCmd()
	if cmd.Use != "list" {
		t.Errorf("expected Use=list, got %q", cmd.Use)
	}
}

// ==================== experiment get flags ====================

func TestExperimentGetCmd_Flags(t *testing.T) {
	cmd := newExperimentGetCmd()
	f := cmd.Flags().Lookup("output")
	if f == nil {
		t.Fatal("--output flag not found")
	}
	if f.Shorthand != "o" {
		t.Errorf("expected shorthand 'o', got %q", f.Shorthand)
	}
}

func TestExperimentGetCmd_ExactArgs(t *testing.T) {
	cmd := newExperimentGetCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := cmd.Args(cmd, []string{"my-exp"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("expected error for 2 args")
	}
}

func TestExperimentGetCmd_UseField(t *testing.T) {
	cmd := newExperimentGetCmd()
	if cmd.Use != "get NAME_OR_ID" {
		t.Errorf("expected Use='get NAME_OR_ID', got %q", cmd.Use)
	}
}

// ==================== Output ====================

func TestExperimentGet_TotalCostOutput(t *testing.T) {
	const id = "019a0000-0000-7000-8000-000000000001"
	tests := []struct {
		name    string
		apiCost string
		want    any // nil means run_stats has no total_cost
	}{
		{"number", `0.25`, 0.25},
		{"null", `null`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/api/v1/sessions/"+id {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"`+id+`","tenant_id":"019a0000-0000-7000-8000-000000000002","name":"e","start_time":"2026-10-26T00:00:00Z","total_cost":`+tt.apiCost+`}`)
			})
			defer setupTestEnv(t, srv.URL)()
			flagOutputFormat = "json"

			out := captureStdout(t, func() {
				cmd := newExperimentCmd()
				cmd.SetArgs([]string{"get", id})
				if err := cmd.Execute(); err != nil {
					t.Fatalf("execute: %v", err)
				}
			})

			var experiment struct {
				RunStats map[string]any `json:"run_stats"`
			}
			if err := json.Unmarshal([]byte(out), &experiment); err != nil {
				t.Fatalf("output is not a JSON object: %v\n%s", err, out)
			}
			got, ok := experiment.RunStats["total_cost"]
			if tt.want == nil {
				if ok {
					t.Errorf("run_stats.total_cost = %#v, want it omitted", got)
				}
				return
			}
			if got != tt.want {
				t.Errorf("run_stats.total_cost = %#v, want %#v", got, tt.want)
			}
		})
	}
}
