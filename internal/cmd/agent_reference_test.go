package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// agentReferencePath is the committed output of `langsmith docs agents`.
var agentReferencePath = filepath.Join("..", "..", "docs", "agent-reference.md")

func TestAgentReferenceIsCurrent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the committed reference describes the non-Windows build; some commands describe themselves differently on Windows")
	}
	var got bytes.Buffer
	require.NoError(t, writeAgentReference(&got, NewRootCmd("dev", "dev")))

	want, err := os.ReadFile(agentReferencePath)
	require.NoError(t, err)
	if !bytes.Equal(got.Bytes(), want) {
		gotLines, wantLines := strings.Split(got.String(), "\n"), strings.Split(string(want), "\n")
		line := 0
		for line < len(gotLines) && line < len(wantLines) && gotLines[line] == wantLines[line] {
			line++
		}
		t.Fatalf("docs/agent-reference.md is out of date with the command tree from line %d; run `make agent-docs` in the CLI module and commit the result", line+1)
	}
}

func TestAgentReferenceChecksExamples(t *testing.T) {
	root := NewRootCmd("dev", "dev")
	for _, tc := range []struct {
		example string
		valid   bool
	}{
		{"langsmith traces list --project p --limit 5 --format json", true},
		{"langsmith traces list -n 5 --jq '.[].id'", true},
		{"langsmith feedback retrieve --feedback-id f", true},
		{"langsmith datasets delete my-dataset --yes", true},
		{"langsmith feedback delete --feedback-id f --yes", true},
		{"langsmith traces list --no-such-flag", false},
		{"langsmith traces fetch abc", false},
		{"langsmith traces", false},
		{"langsmith feedback retrieve --run-id r", false},
		{"langsmith traces list --api-key k", false},
		{"langsmith api sessions?limit=5", true},
		{"langsmith traces setup claude --yes", true},
		{"langsmith projects delete my-project --yes", false},
		{"langsmith sandboxes delete my-box --yes", false},
	} {
		err := checkExample(root, tc.example)
		if tc.valid {
			require.NoError(t, err, tc.example)
		} else {
			require.Error(t, err, tc.example)
		}
	}
}
