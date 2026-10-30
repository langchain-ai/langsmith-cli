package hostapi

import (
	"strings"
	"testing"
)

func TestReconcileRegistryCreatesWhenMissing(t *testing.T) {
	server := serveSequence(t,
		reply{status: 404, body: `{"detail":"not found"}`},
		reply{status: 200, body: `{"id":"reg-1","name":"agent--registry","url":"ghcr.io"}`},
	)
	password := strings.Join([]string{"fixture", "credential"}, "-")

	registry, err := testClient(t, server.URL).ReconcileRegistry(t.Context(), "agent--registry", "ghcr.io", "octocat", password)

	if err != nil || registry != (RegistrySummary{ID: "reg-1", Name: "agent--registry", URL: "ghcr.io"}) {
		t.Fatalf("registry = %+v, %v", registry, err)
	}
	requests := server.Requests()
	if requests[0].Line() != "GET /api/v2/sandboxes/registries/agent--registry" {
		t.Errorf("lookup = %s", requests[0].Line())
	}
	if requests[1].Line() != "POST /api/v2/sandboxes/registries" ||
		!jsonEqual(t, requests[1].JSON(t), `{"name":"agent--registry","url":"ghcr.io","username":"octocat","password":"`+password+`"}`) {
		t.Errorf("create = %s %s", requests[1].Line(), requests[1].Body)
	}
}

func TestReconcileRegistryUpdatesAnExistingOne(t *testing.T) {
	server := serveSequence(t,
		reply{status: 200, body: `{"id":"reg-1","name":"agent--registry"}`},
		reply{status: 500, body: `{"detail":"boom"}`},
	)
	_, err := testClient(t, server.URL).ReconcileRegistry(t.Context(), "agent--registry", "ghcr.io", "octocat", "pw")
	if err == nil || err.Error() != "failed to update sandbox registry 'agent--registry': boom (HTTP 500)" {
		t.Fatalf("err = %v", err)
	}
	if line := server.Requests()[1].Line(); line != "PATCH /api/v2/sandboxes/registries/agent--registry" {
		t.Fatalf("update = %s", line)
	}
}

func TestDeleteRegistryIgnoresAMissingRegistry(t *testing.T) {
	server := serveOnce(t, 404, `{}`)
	if err := testClient(t, server.URL).DeleteRegistry(t.Context(), "agent--registry"); err != nil {
		t.Fatal(err)
	}
	if line := server.Only(t).Line(); line != "DELETE /api/v2/sandboxes/registries/agent--registry" {
		t.Fatalf("request = %s", line)
	}
}
