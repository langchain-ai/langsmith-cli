package hostapi

import (
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
	options := CreateSandboxBoxOptions{
		SnapshotName:   "agent--setup-abc",
		EnvVars:        map[string]string{"A": "b"},
		TimeoutSecs:    120,
		IdleTTLSeconds: 600,
	}

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
	_, err := testClient(t, serveOnce(t, 200, `{"status":"ready"}`).URL).CreateSandboxBox(t.Context(), "builder", CreateSandboxBoxOptions{})
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
