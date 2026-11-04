package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// ==================== Command structure ====================

func TestProjectCmd_Subcommands(t *testing.T) {
	cmd := newProjectCmd()
	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	if !names["list"] {
		t.Error("project missing subcommand 'list'")
	}
	if !names["delete"] {
		t.Error("project missing subcommand 'delete'")
	}
}

func TestProjectCmd_UseField(t *testing.T) {
	cmd := newProjectCmd()
	if cmd.Use != "projects" {
		t.Errorf("expected Use=projects, got %q", cmd.Use)
	}
}

// ==================== project list flags ====================

func TestProjectListCmd_Flags(t *testing.T) {
	cmd := newProjectListCmd()
	tests := []struct {
		name   string
		defVal string
		short  string
	}{
		{"limit", "20", "n"},
		{"name-contains", "", ""},
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

func TestProjectListCmd_UseField(t *testing.T) {
	cmd := newProjectListCmd()
	if cmd.Use != "list" {
		t.Errorf("expected Use=list, got %q", cmd.Use)
	}
}

func TestProjectListCmd_HasNoArgsValidator(t *testing.T) {
	cmd := newProjectListCmd()
	// project list takes no positional args — it should not set an Args validator
	if cmd.Args != nil {
		t.Error("expected no Args validator on project list (takes no positional args)")
	}
}

// ==================== Output ====================

func TestProjectList_TotalCostOutput(t *testing.T) {
	tests := []struct {
		name    string
		apiCost string
		want    any
	}{
		{"number", `0.25`, 0.25},
		{"null", `null`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/api/v1/sessions" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				if off := r.URL.Query().Get("offset"); off != "" && off != "0" {
					_, _ = io.WriteString(w, `[]`)
					return
				}
				_, _ = io.WriteString(w, `[{"id":"019a0000-0000-7000-8000-000000000001","tenant_id":"019a0000-0000-7000-8000-000000000002","name":"p","start_time":"2026-10-26T00:00:00Z","total_cost":`+tt.apiCost+`}]`)
			})
			defer setupTestEnv(t, srv.URL)()
			flagOutputFormat = "json"

			out := captureStdout(t, func() {
				cmd := newProjectCmd()
				cmd.SetArgs([]string{"list"})
				if err := cmd.Execute(); err != nil {
					t.Fatalf("execute: %v", err)
				}
			})

			var projects []map[string]any
			if err := json.Unmarshal([]byte(out), &projects); err != nil {
				t.Fatalf("output is not a JSON array: %v\n%s", err, out)
			}
			if len(projects) != 1 {
				t.Fatalf("got %d projects, want 1:\n%s", len(projects), out)
			}
			got, ok := projects[0]["total_cost"]
			if !ok {
				t.Fatalf("total_cost missing from output:\n%s", out)
			}
			if got != tt.want {
				t.Errorf("total_cost = %#v, want %#v", got, tt.want)
			}
		})
	}
}
