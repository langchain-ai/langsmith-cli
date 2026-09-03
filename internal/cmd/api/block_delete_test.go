package api

import (
	"strings"
	"testing"
)

func TestBlockRawDelete(t *testing.T) {
	tests := []struct {
		name   string
		apiURL string
		method string
		path   string
		want   string
	}{
		{name: "shorthand collection", apiURL: "https://api.example.com", method: "DELETE", path: "sessions", want: "langsmith project delete"},
		{name: "shorthand ID", apiURL: "https://api.example.com", method: "DELETE", path: "sessions/project-id", want: "langsmith project delete"},
		{name: "absolute API collection", apiURL: "https://api.example.com", method: "DELETE", path: "/api/v1/sessions/", want: "langsmith project delete"},
		{name: "absolute API ID", apiURL: "https://api.example.com", method: "delete", path: "/api/v1/sessions/project-id", want: "langsmith project delete"},
		{name: "bare collection", apiURL: "https://api.example.com", method: "DELETE", path: "/sessions", want: "langsmith project delete"},
		{name: "bare ID", apiURL: "https://api.example.com", method: "DELETE", path: "/sessions/project-id", want: "langsmith project delete"},
		{name: "same-host full URL", apiURL: "https://api.example.com", method: "DELETE", path: "https://api.example.com/api/v1/sessions/project-id", want: "langsmith project delete"},
		{name: "base path", apiURL: "https://api.example.com/langsmith", method: "DELETE", path: "sessions/project-id", want: "langsmith project delete"},
		{name: "other method", apiURL: "https://api.example.com", method: "GET", path: "sessions/project-id"},
		{name: "other route", apiURL: "https://api.example.com", method: "DELETE", path: "datasets/dataset-id"},
		{name: "session descendant", apiURL: "https://api.example.com", method: "DELETE", path: "sessions/project-id/runs"},
		{name: "external host", apiURL: "https://api.example.com", method: "DELETE", path: "https://other.example.com/api/v1/sessions/project-id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := blockRawDelete(tt.apiURL, tt.method, tt.path)
			if tt.want == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
				t.Fatalf("error = %v, want error containing %q", err, tt.want)
			}
		})
	}
}
