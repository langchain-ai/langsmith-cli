//go:build integration

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/structured"
	"github.com/stretchr/testify/require"
)

// Live smoke test for generated commands against a real workspace.
//
//	LANGSMITH_API_KEY, LANGSMITH_ENDPOINT   required
//	LANGSMITH_SMOKE_QUEUE_ID                optional: exercises cursor pagination
//	LANGSMITH_SMOKE_PROJECT                 optional: with LANGSMITH_SMOKE_WRITES=1,
//	                                        creates and deletes a disabled rule there
//
// Run with `make test-integration` or
// `go test -tags=integration -run GeneratedSmokeIntegration ./internal/cmd/`.

func smokeJSON(t *testing.T, args ...string) any {
	t.Helper()
	out, err := executeCommand(t, append(args, "--format", "json")...)
	require.NoError(t, err, "langsmith %s\n%s", strings.Join(args, " "), out)
	var v any
	require.NoError(t, json.Unmarshal([]byte(out), &v), "non-JSON output from langsmith %s:\n%s", strings.Join(args, " "), out)
	return v
}

func smokeErr(t *testing.T, args ...string) *structured.Error {
	t.Helper()
	_, err := executeCommand(t, append(args, "--format", "json")...)
	require.Error(t, err, "langsmith %s should fail", strings.Join(args, " "))
	return structured.Classify(err)
}

func TestGeneratedSmokeIntegrationReads(t *testing.T) {
	requireIntegrationEnv(t)

	rules, ok := smokeJSON(t, "automation-rule", "list", "--name-contains", "zz-no-such-rule-").([]any)
	require.True(t, ok, "automation-rule list should return a JSON array")
	require.Empty(t, rules)

	e := smokeErr(t, "automation-rule", "get-last-applied", "--rule-id", "00000000-0000-0000-0000-000000000000")
	require.Equal(t, structured.CategoryNotFound, e.Category)
	require.Equal(t, 5, e.ExitCode())

	e = smokeErr(t, "automation-rule", "list", "--project", "zz-no-such-project-"+randomHandle("smoke"))
	require.Equal(t, structured.CategoryNotFound, e.Category)
}

func TestGeneratedSmokeIntegrationPagination(t *testing.T) {
	requireIntegrationEnv(t)
	queueID := os.Getenv("LANGSMITH_SMOKE_QUEUE_ID")
	if queueID == "" {
		t.Skip("LANGSMITH_SMOKE_QUEUE_ID not set")
	}

	page := smokeJSON(t, "annotation-queue", "item", "list", "--queue-id", queueID, "--status", "archived", "--limit", "1").(map[string]any)
	require.Contains(t, page, "items")
	require.Contains(t, page, "has_more")
	if page["has_more"] != true {
		t.Skip("queue has one page or fewer; cannot follow a cursor")
	}
	next := smokeJSON(t, "annotation-queue", "item", "list", "--queue-id", queueID, "--status", "archived", "--limit", "1",
		"--cursor", page["next_cursor"].(string)).(map[string]any)
	first := page["items"].([]any)[0].(map[string]any)["id"]
	second := next["items"].([]any)[0].(map[string]any)["id"]
	require.NotEqual(t, first, second, "the cursor should advance to a different item")
}

func TestGeneratedSmokeIntegrationRuleLifecycle(t *testing.T) {
	requireIntegrationEnv(t)
	project := os.Getenv("LANGSMITH_SMOKE_PROJECT")
	if project == "" || os.Getenv("LANGSMITH_SMOKE_WRITES") != "1" {
		t.Skip("set LANGSMITH_SMOKE_PROJECT and LANGSMITH_SMOKE_WRITES=1 to run writes")
	}
	name := fmt.Sprintf("cli-smoke-%s-%d", randomHandle("rule"), time.Now().Unix())

	created := smokeJSON(t, "automation-rule", "create", "--project", project,
		"--display-name", name, "--sampling-rate", "0.01", "--is-enabled=false").(map[string]any)
	id, _ := created["id"].(string)
	require.NotEmpty(t, id)
	deleted := false
	t.Cleanup(func() {
		if !deleted {
			_, _ = executeCommand(t, "automation-rule", "delete", "--rule-id", id, "--yes", "--format", "json")
		}
	})

	found := smokeJSON(t, "automation-rule", "list", "--id", id).([]any)
	require.Len(t, found, 1)
	require.Equal(t, false, found[0].(map[string]any)["is_enabled"])

	updated := smokeJSON(t, "automation-rule", "update", "--rule-id", id, "--project", project,
		"--display-name", name+"-edited", "--sampling-rate", "0.02", "--is-enabled=false").(map[string]any)
	require.Equal(t, name+"-edited", updated["display_name"])

	logs := smokeJSON(t, "automation-rule", "list-logs", "--rule-id", id, "--limit", "1").(map[string]any)
	require.Contains(t, logs, "next_cursor")

	e := smokeErr(t, "automation-rule", "delete", "--rule-id", id)
	require.Equal(t, structured.CategoryInvalidInput, e.Category, "delete without --yes must be refused")

	smokeJSON(t, "automation-rule", "delete", "--rule-id", id, "--yes")
	deleted = true
	require.Empty(t, smokeJSON(t, "automation-rule", "list", "--id", id).([]any))
}
