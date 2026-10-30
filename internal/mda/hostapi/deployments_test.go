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

func TestArtifactReportSendsRevisionCredentialWithoutChangingOtherRequests(t *testing.T) {
	server := serveSequence(t, reply{status: 204}, reply{status: 200, body: `{"id":"deployment"}`})
	c := clientForHost(t, server.URL)

	response, err := c.PutManagedDeepAgentArtifacts(t.Context(), "deployment", "revision", map[string]any{"version": 1, "assistant_id": "agent"}, "test-build-token")
	if err != nil || response != nil {
		t.Fatalf("PutManagedDeepAgentArtifacts = %v, %v; want nothing", response, err)
	}
	if _, err := c.GetDeployment(t.Context(), "deployment"); err != nil {
		t.Fatal(err)
	}

	requests := server.Requests()
	report := requests[0]
	if report.Line() != "PUT /v2/managed-deep-agents/deployment/revisions/revision/build-artifacts" {
		t.Errorf("request = %s", report.Line())
	}
	for name, want := range map[string]string{"X-Mda-Build-Token": "test-build-token", "X-Api-Key": "key", "X-Tenant-Id": "tenant"} {
		if got := report.Header.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if !jsonEqual(t, report.JSON(t), `{"version":1,"assistant_id":"agent"}`) {
		t.Errorf("body = %s", report.Body)
	}
	if got := requests[1].Header.Get("X-Mda-Build-Token"); got != "" {
		t.Errorf("a later request carried the build token %q", got)
	}
}

func TestArtifactReportPreservesForbiddenError(t *testing.T) {
	server := serveOnce(t, 403, `{"detail":"A revision build credential is required"}`)

	_, err := clientForHost(t, server.URL).PutManagedDeepAgentArtifacts(t.Context(), "deployment", "revision", map[string]any{}, "rejected-token")

	var api *APIError
	if !errors.As(err, &api) || api.Status != http.StatusForbidden {
		t.Fatalf("err = %v, want a 403 API error", err)
	}
	if strings.Contains(err.Error(), "rejected-token") {
		t.Fatalf("err leaks the token: %v", err)
	}
}

func TestArtifactReportDoesNotFollowRedirects(t *testing.T) {
	for _, target := range []string{"/other-endpoint", "https://other.example/artifacts"} {
		server := serveSequence(t, reply{status: http.StatusTemporaryRedirect, header: map[string]string{"Location": target}})

		_, err := clientForHost(t, server.URL).PutManagedDeepAgentArtifacts(t.Context(), "deployment", "revision", map[string]any{}, "test-build-token")

		var api *APIError
		if !errors.As(err, &api) || api.Status != http.StatusTemporaryRedirect {
			t.Fatalf("%s: err = %v, want a 307 API error", target, err)
		}
		if line := server.Only(t).Line(); !strings.HasPrefix(line, "PUT /v2/managed-deep-agents/") {
			t.Fatalf("request = %s", line)
		}
	}
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

func TestDeleteDeploymentRequestsTracingProjectRemoval(t *testing.T) {
	server := serveOnce(t, 200, "{}")
	deleted, err := clientForHost(t, server.URL).DeleteDeployment(t.Context(), "deployment-1", true)
	if err != nil || !deleted {
		t.Fatalf("DeleteDeployment = %v, %v", deleted, err)
	}
	if line := server.Only(t).Line(); line != "DELETE /v2/deployments/deployment-1?delete_tracing_project=true" {
		t.Fatalf("request = %s", line)
	}
}

func TestDeleteDeploymentCanKeepTheTracingProject(t *testing.T) {
	server := serveOnce(t, 200, "{}")
	if _, err := clientForHost(t, server.URL).DeleteDeployment(t.Context(), "deployment-1", false); err != nil {
		t.Fatal(err)
	}
	if line := server.Only(t).Line(); line != "DELETE /v2/deployments/deployment-1?delete_tracing_project=false" {
		t.Fatalf("request = %s", line)
	}
}

func TestDeleteDeploymentTreatsMissingDeploymentAsAlreadyGone(t *testing.T) {
	deleted, err := clientForHost(t, serveOnce(t, 404, `{"detail":"not found"}`).URL).DeleteDeployment(t.Context(), "deployment-1", true)
	if err != nil || deleted {
		t.Fatalf("DeleteDeployment = %v, %v; want already gone", deleted, err)
	}
}

func TestDeleteDeploymentSurfacesOtherErrors(t *testing.T) {
	_, err := clientForHost(t, serveOnce(t, 403, `{"detail":"missing permission"}`).URL).DeleteDeployment(t.Context(), "deployment-1", true)
	if err == nil || err.Error() != "missing permission (HTTP 403)" {
		t.Fatalf("err = %v", err)
	}
}

func TestPutManagedDeepAgentArtifactsTargetsTheRevisionRoute(t *testing.T) {
	server := serveOnce(t, 200, "{}")
	if _, err := clientForHost(t, server.URL).PutManagedDeepAgentArtifacts(t.Context(), "deployment-1", "revision-1", map[string]any{"version": 1}, "test-build-token"); err != nil {
		t.Fatal(err)
	}
	request := server.Only(t)
	if request.Line() != "PUT /v2/managed-deep-agents/deployment-1/revisions/revision-1/build-artifacts" || field(request.JSON(t), "version") != float64(1) {
		t.Fatalf("request = %s %s", request.Line(), request.Body)
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
	c := clientForHost(t, "http://"+listener.Addr().String()).WithTimeout(50 * time.Millisecond)

	start := time.Now()
	_, err = c.GetDeployment(t.Context(), "d1")
	if err == nil || time.Since(start) > 5*time.Second {
		t.Fatalf("GetDeployment = %v after %s, want a timeout", err, time.Since(start))
	}
	if errs.CategoryOf(err) != errs.Network {
		t.Fatalf("category = %v, want network", errs.CategoryOf(err))
	}
}
