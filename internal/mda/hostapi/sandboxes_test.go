package hostapi

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func sandboxPage(t *testing.T, sandboxes [][2]string) string {
	t.Helper()
	entries := make([]map[string]string, 0, len(sandboxes))
	for _, sandbox := range sandboxes {
		entries = append(entries, map[string]string{"name": sandbox[0], "status": sandbox[1]})
	}
	return mustJSON(t, map[string]any{"sandboxes": entries, "offset": 0})
}

func names(sandboxes []SandboxSummary) []string {
	out := make([]string, 0, len(sandboxes))
	for _, sandbox := range sandboxes {
		out = append(out, sandbox.Name)
	}
	return out
}

func TestListsOnlySandboxesThatTrulyCarryThePrefix(t *testing.T) {
	server := serveOnce(t, 200, sandboxPage(t, [][2]string{
		{"my-agent--aaa", "ready"},
		{"my-agent--bbb", "stopped"},
		// Substring match that is not a prefix: a different deployment.
		{"staging-my-agent--ccc", "ready"},
		// A sibling deployment whose name merely starts the same way.
		{"my-agent-staging--ddd", "ready"},
	}))

	found, err := testClient(t, server.URL).ListSandboxesWithPrefix(t.Context(), "my-agent--")

	if err != nil || !slices.Equal(names(found), []string{"my-agent--aaa", "my-agent--bbb"}) {
		t.Fatalf("found = %v, %v", names(found), err)
	}
	request := server.Only(t)
	query := request.Query(t)
	if !strings.HasPrefix(request.Line(), "GET /api/v2/sandboxes/boxes?") || strings.Join(query["name_contains"], ",") != "my-agent--" ||
		strings.Join(query["limit"], ",") != "100" || strings.Join(query["offset"], ",") != "0" {
		t.Fatalf("request = %s", request.Line())
	}
}

func TestSkipsSandboxesLangSmithIsAlreadyDeleting(t *testing.T) {
	server := serveOnce(t, 200, sandboxPage(t, [][2]string{{"my-agent--aaa", "ready"}, {"my-agent--bbb", "deleting"}}))
	found, err := testClient(t, server.URL).ListSandboxesWithPrefix(t.Context(), "my-agent--")
	if err != nil || !slices.Equal(names(found), []string{"my-agent--aaa"}) {
		t.Fatalf("found = %v, %v", names(found), err)
	}
}

func TestSandboxesPaginateUntilAShortPage(t *testing.T) {
	first := make([][2]string, sandboxPageSize)
	for i := range first {
		first[i] = [2]string{fmt.Sprintf("my-agent--%04d", i), "ready"}
	}
	server := serveSequence(t, reply{status: 200, body: sandboxPage(t, first)}, reply{status: 200, body: sandboxPage(t, [][2]string{{"my-agent--last", "ready"}})})

	found, err := testClient(t, server.URL).ListSandboxesWithPrefix(t.Context(), "my-agent--")

	if err != nil || len(found) != sandboxPageSize+1 {
		t.Fatalf("got %d sandboxes, %v", len(found), err)
	}
	requests := server.Requests()
	if strings.Join(requests[0].Query(t)["offset"], ",") != "0" || strings.Join(requests[1].Query(t)["offset"], ",") != "100" {
		t.Fatalf("requests = %s, %s", requests[0].Line(), requests[1].Line())
	}
}

func TestBatchDeletePostsEveryNameInOneRequest(t *testing.T) {
	server := serveOnce(t, 200, `{"deleted":["my-agent--aaa","my-agent--bbb"],"skipped":[]}`)

	outcome, err := testClient(t, server.URL).BatchDeleteSandboxes(t.Context(), []string{"my-agent--aaa", "my-agent--bbb"})

	if err != nil || len(outcome.Deleted) != 2 {
		t.Fatalf("outcome = %+v, %v", outcome, err)
	}
	request := server.Only(t)
	if request.Line() != "POST /v2/sandboxes/boxes/batch-delete" || string(request.Body) != `{"names":["my-agent--aaa","my-agent--bbb"]}` {
		t.Fatalf("request = %s %s", request.Line(), request.Body)
	}
}

func TestBatchDeleteOfNothingMakesNoRequest(t *testing.T) {
	// No stub server: a request here would fail to connect.
	outcome, err := testClient(t, "http://unused.invalid").BatchDeleteSandboxes(t.Context(), nil)
	if err != nil || len(outcome.Deleted) != 0 {
		t.Fatalf("outcome = %+v, %v", outcome, err)
	}
}

func TestSandboxAccessProbeMakesABoundedListRequest(t *testing.T) {
	server := serveOnce(t, 200, sandboxPage(t, nil))
	if err := testClient(t, server.URL).EnsureSandboxAccess(t.Context()); err != nil {
		t.Fatal(err)
	}
	if line := server.Only(t).Line(); line != "GET /api/v2/sandboxes/boxes?limit=1&offset=0" {
		t.Fatalf("request = %s", line)
	}
}

func TestSandboxAccessProbePreservesEntitlementError(t *testing.T) {
	server := serveOnce(t, 403, `{"detail":{"error":"FeatureDisabled","message":"Sandbox feature is not enabled for this organization"}}`)
	err := testClient(t, server.URL).EnsureSandboxAccess(t.Context())
	if err == nil || err.Error() != "Sandbox feature is not enabled for this organization (HTTP 403)" {
		t.Fatalf("err = %v", err)
	}
}

func TestCreateSandboxBoxWaitsForReady(t *testing.T) {
	server := serveOnce(t, 200, `{"name":"builder","status":"ready","dataplane_url":"https://dp.example","id":"b-1"}`)
	options := DefaultCreateSandboxBoxOptions()
	options.SnapshotName = "agent--setup-abc"
	options.EnvVars = map[string]string{"A": "b"}

	created, err := testClient(t, server.URL).CreateSandboxBox(t.Context(), "builder", options)

	if err != nil || created != (SandboxSummary{Name: "builder", Status: "ready", DataplaneURL: "https://dp.example", ID: "b-1"}) {
		t.Fatalf("created = %+v, %v", created, err)
	}
	request := server.Only(t)
	want := `{"name":"builder","wait_for_ready":true,"timeout":120,"idle_ttl_seconds":600,"snapshot_name":"agent--setup-abc","env_vars":{"A":"b"}}`
	if request.Line() != "POST /api/v2/sandboxes/boxes" || !jsonEqual(t, request.JSON(t), want) {
		t.Fatalf("request = %s %s", request.Line(), request.Body)
	}
}

func TestCreateSandboxBoxRequiresAName(t *testing.T) {
	_, err := testClient(t, serveOnce(t, 200, `{"status":"ready"}`).URL).CreateSandboxBox(t.Context(), "builder", DefaultCreateSandboxBoxOptions())
	if err == nil || err.Error() != "failed to decode create sandbox response: missing field `name`" {
		t.Fatalf("err = %v", err)
	}
}

func TestDeleteSandboxBoxIgnoresAMissingBox(t *testing.T) {
	server := serveOnce(t, 404, `{"detail":"gone"}`)
	if err := testClient(t, server.URL).DeleteSandboxBox(t.Context(), "builder"); err != nil {
		t.Fatal(err)
	}
	if line := server.Only(t).Line(); line != "DELETE /api/v2/sandboxes/boxes/builder" {
		t.Fatalf("request = %s", line)
	}
}

func TestExecuteSandboxCommandRunsOnTheDataplane(t *testing.T) {
	server := serveSequence(t, reply{status: 200, body: `{"stdout":"ok\n","stderr":""}`}, reply{status: 500, body: `{"detail":"boom"}`})
	c := testClient(t, "http://unused.invalid")

	result, err := c.ExecuteSandboxCommand(t.Context(), server.URL+"/", "echo ok", 30)
	if err != nil || result != (SandboxCommandResult{Stdout: "ok\n", ExitCode: -1}) {
		t.Fatalf("result = %+v, %v", result, err)
	}
	_, err = c.ExecuteSandboxCommand(t.Context(), server.URL, "echo ok", 30)
	if err == nil || err.Error() != "boom (HTTP 500)" {
		t.Fatalf("err = %v", err)
	}

	request := server.Requests()[0]
	if request.Line() != "POST /execute" || request.Header.Get("X-Api-Key") != "key" || request.Header.Get("X-Tenant-Id") != "tenant" ||
		!jsonEqual(t, request.JSON(t), `{"command":"echo ok","timeout":30,"shell":"/bin/bash"}`) {
		t.Fatalf("request = %s %v %s", request.Line(), request.Header, request.Body)
	}
}

func TestDecodesBatchDeleteResults(t *testing.T) {
	outcome, err := decodeBatchDelete([]byte(`{"deleted":["agent--abc"],"skipped":[{"name":"agent--def","reason":"not found"}]}`))
	if err != nil || !slices.Equal(outcome.Deleted, []string{"agent--abc"}) || len(outcome.Skipped) != 1 ||
		outcome.Skipped[0] != (SandboxSkipped{Name: "agent--def", Reason: "not found"}) {
		t.Fatalf("outcome = %+v, %v", outcome, err)
	}
}

func TestDecodesBatchDeleteWithoutSkipped(t *testing.T) {
	outcome, err := decodeBatchDelete([]byte(`{"deleted":[]}`))
	if err != nil || len(outcome.Deleted) != 0 || len(outcome.Skipped) != 0 {
		t.Fatalf("outcome = %+v, %v", outcome, err)
	}
}
