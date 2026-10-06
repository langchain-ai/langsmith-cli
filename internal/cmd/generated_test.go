package cmd

import (
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
)

func TestGeneratedCommandsAreRegistered(t *testing.T) {
	root := NewRootCmd("dev", "dev")
	for _, resource := range generatedResources() {
		name := resource.Name
		matches := 0
		for _, c := range root.Commands() {
			if c.Name() == name {
				matches++
			}
		}
		if matches != 1 {
			t.Errorf("command %q registered %d times, want 1", name, matches)
		}
	}
}

func TestTranslateGlobalFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantRest    []string
		wantProfile string
	}{
		{
			name:     "renames global flags",
			args:     []string{"list", "--workspace=ws", "--api-key", "k", "--format", "json"},
			wantRest: []string{"list", "--tenant-id=ws", "--api-key", "k", "--format", "json"},
		},
		{
			name:     "strips /api/v1 from --api-url",
			args:     []string{"list", "--api-url", "https://host/api/v1/"},
			wantRest: []string{"list", "--base-url=https://host"},
		},
		{
			name:     "normalizes --api-url=value",
			args:     []string{"list", "--api-url=https://host/api/v1"},
			wantRest: []string{"list", "--base-url=https://host"},
		},
		{
			name:     "renames --workspace-id",
			args:     []string{"list", "--workspace-id", "ws"},
			wantRest: []string{"list", "--tenant-id=ws"},
		},
		{
			name:        "extracts --profile",
			args:        []string{"--profile", "dev", "list"},
			wantRest:    []string{"list"},
			wantProfile: "dev",
		},
		{
			name:        "extracts --profile=value",
			args:        []string{"list", "--profile=x"},
			wantRest:    []string{"list"},
			wantProfile: "x",
		},
		{
			name:     "passes other flags through",
			args:     []string{"delete", "--webhook-id", "1", "--url=https://example.com"},
			wantRest: []string{"delete", "--webhook-id", "1", "--url=https://example.com"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rest, profile := translateGlobalFlags(tt.args)
			if strings.Join(rest, " ") != strings.Join(tt.wantRest, " ") {
				t.Errorf("rest = %q, want %q", rest, tt.wantRest)
			}
			if profile != tt.wantProfile {
				t.Errorf("profile = %q, want %q", profile, tt.wantProfile)
			}
		})
	}
}

func TestOperationName(t *testing.T) {
	resource := &cli.Command{Commands: []*cli.Command{{Name: "delete"}, {Name: "list"}}}
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"operation first", []string{"delete", "--webhook-id", "1"}, "delete"},
		{"after a global flag and its value", []string{"--api-url", "https://x", "list"}, "list"},
		{"flag value equal to an operation name", []string{"list", "--url", "delete"}, "list"},
		{"unknown first positional", []string{"other", "delete"}, ""},
		{"no positional", []string{"--help"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := operationName(resource, tt.args); got != tt.want {
				t.Errorf("operationName(%q) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}
