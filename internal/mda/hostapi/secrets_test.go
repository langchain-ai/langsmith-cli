package hostapi

import (
	"slices"
	"testing"
)

func TestWorkspaceSecretKeysReadsOnlyTheNames(t *testing.T) {
	server := serveOnce(t, 200, `[{"key":"OPENAI_API_KEY"},{"key":"TAVILY_API_KEY"}]`)

	keys, err := testClient(t, server.URL).WorkspaceSecretKeys(t.Context())

	if err != nil || !slices.Equal(keys, []string{"OPENAI_API_KEY", "TAVILY_API_KEY"}) {
		t.Fatalf("keys = %v, %v", keys, err)
	}
	if line := server.Only(t).Line(); line != "GET /api/v1/workspaces/current/secrets" {
		t.Fatalf("request = %s", line)
	}
}

func TestWorkspaceSecretKeysOfAnEmptyBodyAreNone(t *testing.T) {
	keys, err := testClient(t, serveOnce(t, 200, "").URL).WorkspaceSecretKeys(t.Context())
	if err != nil || len(keys) != 0 {
		t.Fatalf("keys = %v, %v", keys, err)
	}
}

func TestWorkspaceSecretKeysRejectAnEntryWithoutAKey(t *testing.T) {
	_, err := testClient(t, serveOnce(t, 200, `[{"name":"X"}]`).URL).WorkspaceSecretKeys(t.Context())
	if err == nil || err.Error() != "failed to decode the workspace secrets response: missing field `key`" {
		t.Fatalf("err = %v", err)
	}
}
