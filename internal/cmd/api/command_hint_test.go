package api

import (
	"strings"
	"testing"
)

func TestCommandHint(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		want   string
	}{
		{name: "v1 run query shorthand", method: "POST", path: "runs/query", want: "langsmith runs list"},
		{name: "v1 run query absolute", method: "post", path: "/api/v1/runs/query", want: "langsmith runs list"},
		{name: "v2 run query", method: "POST", path: "/api/v2/runs/query", want: "langsmith runs list"},
		{name: "get a run", method: "GET", path: "runs/0190b00e-0000-0000-0000-000000000000", want: "langsmith runs get"},
		{name: "list rules", method: "GET", path: "runs/rules"},
		{name: "run stats", method: "POST", path: "runs/stats"},
		{name: "sessions", method: "GET", path: "sessions"},
		{name: "external host", method: "POST", path: "https://other.example.com/api/v1/runs/query"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := commandHint("https://api.example.com", tt.method, tt.path)
			if tt.want == "" && got != "" {
				t.Fatalf("hint = %q, want none", got)
			}
			if !strings.Contains(got, tt.want) {
				t.Fatalf("hint = %q, want it to contain %q", got, tt.want)
			}
		})
	}
}
