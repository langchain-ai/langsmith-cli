package deployment

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHostBackendClientListDeployments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/deployments" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "GET" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("X-Api-Key") != "test-key" {
			t.Errorf("unexpected API key: %s", r.Header.Get("X-Api-Key"))
		}

		json.NewEncoder(w).Encode(map[string]any{
			"resources": []map[string]any{
				{"id": "dep-1", "name": "test-dep"},
			},
		})
	}))
	defer server.Close()

	client := NewHostBackendClient(server.URL, "test-key", "")
	result, err := client.ListDeployments("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resources, ok := result["resources"].([]any)
	if !ok || len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %v", result["resources"])
	}
}

func TestHostBackendClientDeleteDeployment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/deployments/dep-123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "DELETE" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewHostBackendClient(server.URL, "test-key", "")
	err := client.DeleteDeployment("dep-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHostBackendClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer server.Close()

	client := NewHostBackendClient(server.URL, "test-key", "")
	_, err := client.ListDeployments("")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}

	hbErr, ok := err.(*HostBackendError)
	if !ok {
		t.Fatal("expected HostBackendError")
	}
	if hbErr.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", hbErr.StatusCode)
	}
}

func TestHostBackendClientTenantID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Tenant-ID") != "tenant-123" {
			t.Errorf("expected X-Tenant-ID=tenant-123, got %s", r.Header.Get("X-Tenant-ID"))
		}
		json.NewEncoder(w).Encode(map[string]any{"resources": []any{}})
	}))
	defer server.Close()

	client := NewHostBackendClient(server.URL, "test-key", "tenant-123")
	_, err := client.ListDeployments("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHostBackendClientCreateDeployment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v2/deployments" {
			t.Errorf("expected /v2/deployments, got %s", r.URL.Path)
		}

		var payload map[string]any
		json.NewDecoder(r.Body).Decode(&payload)
		if payload["name"] != "my-dep" {
			t.Errorf("expected name=my-dep, got %v", payload["name"])
		}

		json.NewEncoder(w).Encode(map[string]any{"id": "new-dep-1", "name": "my-dep"})
	}))
	defer server.Close()

	client := NewHostBackendClient(server.URL, "test-key", "")
	result, err := client.CreateDeployment(map[string]any{"name": "my-dep"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["id"] != "new-dep-1" {
		t.Errorf("expected id=new-dep-1, got %v", result["id"])
	}
}

func TestHostBackendClientUpdateDeployment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		if r.URL.Path != "/v2/deployments/dep-1" {
			t.Errorf("expected /v2/deployments/dep-1, got %s", r.URL.Path)
		}

		var payload map[string]any
		json.NewDecoder(r.Body).Decode(&payload)
		srcConfig, _ := payload["source_revision_config"].(map[string]any)
		if srcConfig["image_uri"] != "my-image:latest" {
			t.Errorf("expected image_uri=my-image:latest, got %v", srcConfig["image_uri"])
		}

		json.NewEncoder(w).Encode(map[string]any{"id": "dep-1"})
	}))
	defer server.Close()

	client := NewHostBackendClient(server.URL, "test-key", "")
	_, err := client.UpdateDeployment("dep-1", "my-image:latest", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
