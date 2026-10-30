package hostapi

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
)

// clientForHost is a client for a stub host backend at hostBackendURL,
// authenticated with the API key "key" in the workspace "tenant".
func clientForHost(t *testing.T, hostBackendURL string) *Client {
	t.Helper()
	return newClient(t, apiKeyCLIClient(t, "key", "tenant"), hostBackendURL, "http://unused.invalid")
}

func TestListDeploymentsFiltersByNameSubstring(t *testing.T) {
	server := serveOnce(t, 200, `{"resources":[]}`)
	if _, err := clientForHost(t, server.URL).ListDeployments(t.Context(), "my-agent"); err != nil {
		t.Fatal(err)
	}
	if line := server.Only(t).Line(); line != "GET /v2/deployments?name_contains=my-agent" {
		t.Fatalf("request = %s", line)
	}
}

func TestListAgentDeploymentsFiltersByAgentAndEnvironment(t *testing.T) {
	server := serveOnce(t, 200, `{"resources":[]}`)
	agent := DeploymentAgent{AgentID: "agent", Environment: AgentEnvironmentStaging}
	if _, err := clientForHost(t, server.URL).ListAgentDeployments(t.Context(), agent); err != nil {
		t.Fatal(err)
	}
	if line := server.Only(t).Line(); line != "GET /v2/deployments?agent_id=agent&agent_environment=staging" {
		t.Fatalf("request = %s", line)
	}
}

func TestCreateDeploymentRequestsCallerTypeAndManagedDeepAgentFlag(t *testing.T) {
	for deploymentType, want := range map[DeploymentType]string{DeploymentTypeDev: "dev", DeploymentTypeProd: "prod"} {
		server := serveOnce(t, 200, `{"id":"deployment-1"}`)

		if _, err := clientForHost(t, server.URL).CreateDeployment(t.Context(), "agent", deploymentType, nil); err != nil {
			t.Fatal(err)
		}

		// The caller-selected deployment type is sent, and the deployment is
		// still tagged with the request-only `managed_deep_agent` flag.
		body := server.Only(t).JSON(t)
		if !jsonEqual(t, body, `{"name":"agent","source":"internal_source","source_config":{"deployment_type":"`+want+`"},"source_revision_config":{},"managed_deep_agent":true,"secrets":[]}`) {
			t.Errorf("body = %s", mustJSON(t, body))
		}
	}
}

func TestCreateAgentDeploymentSendsAgentInsteadOfName(t *testing.T) {
	server := serveOnce(t, 200, `{"id":"deployment-1","name":"generated"}`)
	agent := DeploymentAgent{AgentID: "customer-support", Environment: AgentEnvironmentStaging}

	if _, err := clientForHost(t, server.URL).CreateAgentDeployment(t.Context(), agent, DeploymentTypeDev, []Secret{{Name: "A", Value: "b"}}); err != nil {
		t.Fatal(err)
	}

	body := server.Only(t).JSON(t)
	if !jsonEqual(t, field(body, "agent"), `{"agent_id":"customer-support","environment":"staging"}`) || field(body, "name") != nil {
		t.Errorf("body = %s", mustJSON(t, body))
	}
	if !jsonEqual(t, field(body, "secrets"), `[{"name":"A","value":"b"}]`) {
		t.Errorf("secrets = %s", mustJSON(t, field(body, "secrets")))
	}
}

func TestCreateExternalDeploymentSendsImageSource(t *testing.T) {
	server := serveOnce(t, 200, `{"id":"deployment-1"}`)
	image := "registry.example/agent@sha256:" + strings.Repeat("a", 64)

	if _, err := clientForHost(t, server.URL).CreateExternalDeployment(t.Context(), "agent", image, nil); err != nil {
		t.Fatal(err)
	}

	body := server.Only(t).JSON(t)
	want := `{"name":"agent","source":"external_docker","source_config":{"resource_spec":{}},"source_revision_config":{"image_uri":"` + image + `"},"managed_deep_agent":true,"secrets":[]}`
	if !jsonEqual(t, body, want) {
		t.Errorf("body = %s", mustJSON(t, body))
	}
}

func TestUpdateExternalDeploymentSendsImageRevisionWithoutRevisionSource(t *testing.T) {
	server := serveOnce(t, 200, `{"id":"deployment-1"}`)
	image := "registry.example/agent@sha256:" + strings.Repeat("b", 64)

	if _, err := clientForHost(t, server.URL).UpdateDeploymentExternalImage(t.Context(), "deployment-1", image, nil); err != nil {
		t.Fatal(err)
	}

	request := server.Only(t)
	if request.Line() != "PATCH /v2/deployments/deployment-1" {
		t.Errorf("request = %s", request.Line())
	}
	if !jsonEqual(t, request.JSON(t), `{"source_revision_config":{"image_uri":"`+image+`"},"managed_deep_agent":true,"secrets":[]}`) {
		t.Errorf("body = %s", request.Body)
	}
}

// The entitlement gate has to reach deploy as a typed error, not just a
// message: that is what lets deploy report it as a plan upsell.
func TestCreateDeploymentReportsAnEntitlementGateAsATypedError(t *testing.T) {
	server := serveOnce(t, 403, `{"detail":"Managed Deep Agents is not enabled for this organization"}`)

	_, err := clientForHost(t, server.URL).CreateDeployment(t.Context(), "agent", DeploymentTypeDev, nil)

	var api *APIError
	if !errors.As(err, &api) || api.Status != http.StatusForbidden || api.Detail != "Managed Deep Agents is not enabled for this organization" {
		t.Fatalf("err = %#v, want the typed entitlement error", err)
	}
	if errs.CategoryOf(err) != errs.Permission {
		t.Fatalf("category = %v", errs.CategoryOf(err))
	}
}

func TestUpdateInternalSourceSendsManagedDeepAgentFlag(t *testing.T) {
	server := serveOnce(t, 200, `{"latest_revision_id":"revision-1"}`)

	updated, err := clientForHost(t, server.URL).UpdateDeploymentInternalSource(t.Context(), "deployment-1", "source.tar.gz", "langgraph.json", nil)
	if err != nil || updated["latest_revision_id"] != "revision-1" {
		t.Fatalf("UpdateDeploymentInternalSource = %v, %v", updated, err)
	}

	// The update carries the flag but does not resend `source_config` (the
	// deployment type is fixed at create time).
	body := server.Only(t).JSON(t)
	want := `{"revision_source":"internal_source","source_revision_config":{"source_tarball_path":"source.tar.gz","langgraph_config_path":"langgraph.json"},"managed_deep_agent":true,"secrets":[]}`
	if !jsonEqual(t, body, want) {
		t.Fatalf("body = %s", mustJSON(t, body))
	}
}

func TestRevisionsAndUploadURLsComeFromTheHostBackend(t *testing.T) {
	server := serveSequence(t,
		reply{status: 200, body: `{"upload_url":"https://storage.example/u","source_tarball_path":"p"}`},
		reply{status: 200, body: `{"resources":[{"id":"r1"}]}`},
		reply{status: 200, body: `{"id":"r1","status":"DEPLOYED"}`},
	)
	c := clientForHost(t, server.URL)

	upload, err := c.RequestUploadURL(t.Context(), "d1")
	if err != nil || upload["upload_url"] != "https://storage.example/u" {
		t.Fatalf("RequestUploadURL = %v, %v", upload, err)
	}
	if _, err := c.ListRevisions(t.Context(), "d1", 1); err != nil {
		t.Fatal(err)
	}
	revision, err := c.GetRevision(t.Context(), "d1", "r1")
	if err != nil || revision["status"] != "DEPLOYED" {
		t.Fatalf("GetRevision = %v, %v", revision, err)
	}

	var lines []string
	for _, request := range server.Requests() {
		lines = append(lines, request.Line())
	}
	if strings.Join(lines, "\n") != "POST /v2/deployments/d1/upload-url\nGET /v2/deployments/d1/revisions?limit=1\nGET /v2/deployments/d1/revisions/r1" {
		t.Fatalf("requests = %v", lines)
	}
}

func TestARequestThatNeverAnswersTimesOut(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	c := clientForHost(t, "http://"+listener.Addr().String())
	c.timeout = 50 * time.Millisecond

	start := time.Now()
	_, err = c.GetDeployment(t.Context(), "d1")
	if err == nil || time.Since(start) > 5*time.Second {
		t.Fatalf("GetDeployment = %v after %s, want a timeout", err, time.Since(start))
	}
	if errs.CategoryOf(err) != errs.Network {
		t.Fatalf("category = %v, want network", errs.CategoryOf(err))
	}
}
