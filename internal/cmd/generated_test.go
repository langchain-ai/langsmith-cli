package cmd

import (
	"strings"
	"testing"
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
	rest, profile := translateGlobalFlags([]string{
		"list", "--api-url", "http://x", "--workspace=ws", "--profile", "dev", "--format", "json",
	})
	want := []string{"list", "--base-url", "http://x", "--tenant-id=ws", "--format", "json"}
	if strings.Join(rest, " ") != strings.Join(want, " ") {
		t.Errorf("rest = %q, want %q", rest, want)
	}
	if profile != "dev" {
		t.Errorf("profile = %q, want dev", profile)
	}
}
