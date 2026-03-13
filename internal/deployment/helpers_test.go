package deployment

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFormatTimestamp(t *testing.T) {
	// Epoch milliseconds
	result := FormatTimestamp(float64(1700000000000))
	if result == "" {
		t.Error("expected non-empty result for epoch ms")
	}

	// String passthrough
	result = FormatTimestamp("2023-01-01 00:00:00")
	if result != "2023-01-01 00:00:00" {
		t.Errorf("expected passthrough, got %s", result)
	}

	// Nil
	result = FormatTimestamp(nil)
	if result != "" {
		t.Errorf("expected empty for nil, got %s", result)
	}
}

func TestFormatLogEntry(t *testing.T) {
	entry := map[string]any{
		"timestamp": "2023-01-01 00:00:00",
		"level":     "ERROR",
		"message":   "Something went wrong",
	}
	result := FormatLogEntry(entry)
	if !strings.Contains(result, "ERROR") {
		t.Error("expected ERROR in output")
	}
	if !strings.Contains(result, "Something went wrong") {
		t.Error("expected message in output")
	}
}

func TestLevelColor(t *testing.T) {
	if LevelColor("ERROR") == "" {
		t.Error("expected color for ERROR")
	}
	if LevelColor("WARNING") == "" {
		t.Error("expected color for WARNING")
	}
	if LevelColor("INFO") != "" {
		t.Error("expected no color for INFO")
	}
}

func TestFormatDeploymentsTable(t *testing.T) {
	deployments := []map[string]any{
		{
			"id":   "dep-123",
			"name": "alpha",
			"source_config": map[string]any{
				"custom_url": "https://alpha.example.com",
			},
		},
		{
			"id":   "dep-456",
			"name": "beta",
			"source_config": map[string]any{
				"custom_url": "https://beta.example.com",
			},
		},
	}

	result := FormatDeploymentsTable(deployments)
	if !strings.Contains(result, "Deployment ID") {
		t.Error("expected header")
	}
	if !strings.Contains(result, "dep-123") {
		t.Error("expected dep-123")
	}
	if !strings.Contains(result, "alpha") {
		t.Error("expected alpha")
	}
	if !strings.Contains(result, "https://beta.example.com") {
		t.Error("expected beta URL")
	}
}

func TestCleanEmptyLines(t *testing.T) {
	input := "line1\n\nline2\n\n\nline3\n"
	result := CleanEmptyLines(input)
	if strings.Contains(result, "\n\n") {
		t.Error("expected no double newlines")
	}
	if !strings.Contains(result, "line1") || !strings.Contains(result, "line2") || !strings.Contains(result, "line3") {
		t.Error("expected all lines present")
	}
}

func TestResolveDeploymentID(t *testing.T) {
	// Direct ID
	id, err := ResolveDeploymentID(nil, "dep-123", "")
	if err != nil || id != "dep-123" {
		t.Errorf("expected dep-123, got %s, err: %v", id, err)
	}

	// No ID or name
	_, err = ResolveDeploymentID(nil, "", "")
	if err == nil {
		t.Error("expected error with no ID or name")
	}

	// By name with mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"resources": []map[string]any{
				{"id": "found-id", "name": "my-dep"},
			},
		})
	}))
	defer server.Close()

	client := NewHostBackendClient(server.URL, "test-key", "")
	id, err = ResolveDeploymentID(client, "", "my-dep")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "found-id" {
		t.Errorf("expected found-id, got %s", id)
	}
}
