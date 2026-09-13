package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// Commands whose --project names a project to write into config rather than one
// to look up, so there is no session UUID to accept instead. `trace setup` and
// its per-agent variants record a project name for the tracing SDK, which
// auto-creates the project by name on first trace.
var projectIDExemptCommands = map[string]bool{
	"trace setup":        true,
	"trace setup claude": true,
	"trace setup codex":  true,
}

func walkCommands(cmd *cobra.Command, path []string, fn func(path string, c *cobra.Command)) {
	if cmd.Runnable() {
		fn(strings.Join(path, " "), cmd)
	}
	for _, sub := range cmd.Commands() {
		walkCommands(sub, append(path, sub.Name()), fn)
	}
}

// Every command that resolves a project must take the UUID as well as the name.
// The two drifted apart once already: --project-id landed on the trace commands
// only, which left `run list` and `project issues list` name-only and forced
// callers to interpolate an arbitrary user-chosen project name into a command
// line. Fail here rather than let a new command reintroduce the gap.
func TestEveryProjectCommandAcceptsProjectID(t *testing.T) {
	t.Parallel()

	root := NewRootCmd("1.0.0", "1.0.0")
	seen := 0
	walkCommands(root, []string{}, func(path string, c *cobra.Command) {
		if c.Flags().Lookup("project") == nil || projectIDExemptCommands[path] {
			return
		}
		seen++
		if c.Flags().Lookup("project-id") == nil {
			t.Errorf("%q accepts --project but not --project-id", path)
		}
	})
	if seen == 0 {
		t.Fatal("walked no commands with a --project flag; the walk is broken")
	}
}

// --project and --project-id name the same thing two ways, so passing both is a
// caller error worth catching locally instead of resolving by precedence.
func TestProjectAndProjectIDAreMutuallyExclusive(t *testing.T) {
	t.Parallel()

	root := NewRootCmd("1.0.0", "1.0.0")
	walkCommands(root, []string{}, func(path string, c *cobra.Command) {
		if c.Flags().Lookup("project-id") == nil {
			return
		}
		f := c.Flags().Lookup("project")
		if f == nil {
			return
		}
		if _, ok := f.Annotations["cobra_annotation_mutually_exclusive"]; !ok {
			t.Errorf("%q does not mark --project and --project-id mutually exclusive", path)
		}
	})
}

// A malformed --project-id should fail before any request goes out.
func TestValidateProjectIDRejectsNonUUID(t *testing.T) {
	t.Parallel()

	if _, err := validateProjectID("not-a-uuid"); err == nil {
		t.Error("expected an error for a non-UUID --project-id")
	}
	const id = "519bb9dd-079b-4488-8610-e330951ea3e4"
	got, err := validateProjectID(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != id {
		t.Errorf("expected %q returned unchanged, got %q", id, got)
	}
}
