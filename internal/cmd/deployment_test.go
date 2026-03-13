package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestDeploymentCmdExists(t *testing.T) {
	cmd := NewRootCmd("test")
	// Check that "deployment" subcommand exists
	for _, c := range cmd.Commands() {
		if c.Name() == "deployment" {
			return
		}
	}
	t.Error("expected 'deployment' subcommand")
}

func TestDeploymentSubcommands(t *testing.T) {
	cmd := NewRootCmd("test")
	var depCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "deployment" {
			depCmd = c
			break
		}
	}
	if depCmd == nil {
		t.Fatal("'deployment' command not found")
	}

	expected := []string{"up", "build", "dev", "deploy", "dockerfile", "new"}
	for _, name := range expected {
		found := false
		for _, c := range depCmd.Commands() {
			if c.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand %q", name)
		}
	}
}

func TestDeploymentDeploySubcommands(t *testing.T) {
	cmd := NewRootCmd("test")
	var depCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "deployment" {
			depCmd = c
			break
		}
	}
	if depCmd == nil {
		t.Fatal("'deployment' command not found")
	}

	var deployCmd *cobra.Command
	for _, c := range depCmd.Commands() {
		if c.Name() == "deploy" {
			deployCmd = c
			break
		}
	}
	if deployCmd == nil {
		t.Fatal("'deploy' subcommand not found")
	}

	expected := []string{"list", "delete", "logs"}
	for _, name := range expected {
		found := false
		for _, c := range deployCmd.Commands() {
			if c.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected deploy subcommand %q", name)
		}
	}
}

func TestDeploymentUpFlags(t *testing.T) {
	cmd := newDeploymentUpCmd()
	flags := []string{
		"config", "docker-compose", "port", "recreate", "pull",
		"watch", "wait", "verbose", "debugger-port", "debugger-base-url",
		"postgres-uri", "api-version", "engine-runtime-mode", "image", "base-image",
	}
	for _, name := range flags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not found", name)
		}
	}
}

func TestDeploymentBuildFlags(t *testing.T) {
	cmd := newDeploymentBuildCmd()
	flags := []string{"config", "tag", "pull", "base-image", "api-version", "engine-runtime-mode"}
	for _, name := range flags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not found", name)
		}
	}
}

func TestDeploymentDevFlags(t *testing.T) {
	cmd := newDeploymentDevCmd()
	flags := []string{"host", "port", "no-reload", "config", "n-jobs-per-worker", "no-browser", "debug-port", "studio-url", "allow-blocking", "server-log-level"}
	for _, name := range flags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not found", name)
		}
	}
}

func TestDeploymentDeployFlags(t *testing.T) {
	cmd := newDeploymentDeployCmd()
	flags := []string{"config", "tag", "api-key", "host-url", "name", "wait", "verbose"}
	for _, name := range flags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not found", name)
		}
	}
}

func TestDeploymentDockerfileFlags(t *testing.T) {
	cmd := newDeploymentDockerfileCmd()
	flags := []string{"config", "add-docker-compose", "base-image", "api-version", "engine-runtime-mode"}
	for _, name := range flags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not found", name)
		}
	}
}

func TestDeploymentNewFlags(t *testing.T) {
	cmd := newDeploymentNewCmd()
	if cmd.Flags().Lookup("template") == nil {
		t.Error("flag --template not found")
	}
}

func TestDeploymentDeployListFlags(t *testing.T) {
	cmd := newDeploymentDeployListCmd()
	flags := []string{"api-key", "host-url", "name-contains"}
	for _, name := range flags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not found", name)
		}
	}
}

func TestDeploymentDeployDeleteFlags(t *testing.T) {
	cmd := newDeploymentDeployDeleteCmd()
	flags := []string{"api-key", "host-url", "force"}
	for _, name := range flags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not found", name)
		}
	}
}

func TestDeploymentDeployLogsFlags(t *testing.T) {
	cmd := newDeploymentDeployLogsCmd()
	flags := []string{"api-key", "host-url", "name", "deployment-id", "type", "revision-id", "level", "limit", "query", "start-time", "end-time", "follow"}
	for _, name := range flags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not found", name)
		}
	}
}

func TestDeploymentDeployListEndToEnd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/deployments" {
			json.NewEncoder(w).Encode(map[string]any{
				"resources": []map[string]any{
					{
						"id":   "dep-123",
						"name": "alpha",
						"source_config": map[string]any{
							"custom_url": "https://alpha.example.com",
						},
					},
				},
			})
		}
	}))
	defer server.Close()

	out, err := executeCommand(t,
		"deployment", "deploy", "list",
		"--api-key", "test-key",
		"--host-url", server.URL,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v, output: %s", err, out)
	}
	if !strings.Contains(out, "dep-123") {
		t.Errorf("expected dep-123 in output, got: %s", out)
	}
	if !strings.Contains(out, "alpha") {
		t.Errorf("expected alpha in output, got: %s", out)
	}
}

func TestDeploymentDeployListEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"resources": []any{}})
	}))
	defer server.Close()

	out, err := executeCommand(t,
		"deployment", "deploy", "list",
		"--api-key", "test-key",
		"--host-url", server.URL,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v, output: %s", err, out)
	}
	if !strings.Contains(out, "No deployments found") {
		t.Errorf("expected 'No deployments found' in output, got: %s", out)
	}
}

func TestDeploymentHelpOutput(t *testing.T) {
	out, err := executeCommand(t, "deployment", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "up") {
		t.Error("expected 'up' in help output")
	}
	if !strings.Contains(out, "build") {
		t.Error("expected 'build' in help output")
	}
	if !strings.Contains(out, "dev") {
		t.Error("expected 'dev' in help output")
	}
	if !strings.Contains(out, "deploy") {
		t.Error("expected 'deploy' in help output")
	}
	if !strings.Contains(out, "dockerfile") {
		t.Error("expected 'dockerfile' in help output")
	}
	if !strings.Contains(out, "new") {
		t.Error("expected 'new' in help output")
	}
}
