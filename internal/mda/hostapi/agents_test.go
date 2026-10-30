package hostapi

import (
	"strings"
	"testing"
)

func TestLocalEnvironmentProjectCreatesAMissingAgent(t *testing.T) {
	server := serveSequence(t,
		reply{status: 404, body: `{"detail":"not found"}`},
		reply{status: 201, body: `{"id":"research"}`},
		reply{status: 200, body: `{"environment":"LOCAL","tracer_session_name":"research-local"}`},
	)

	project, err := testClient(t, server.URL).LocalEnvironmentProject(t.Context(), "research")

	if err != nil || project != "research-local" {
		t.Fatalf("LocalEnvironmentProject = %q, %v", project, err)
	}
	requests := server.Requests()
	if requests[0].Line() != "POST /api/v1/agents/research/environments" || !jsonEqual(t, requests[0].JSON(t), `{"environment":"LOCAL"}`) {
		t.Errorf("first request = %s %s", requests[0].Line(), requests[0].Body)
	}
	if requests[1].Line() != "POST /api/v1/agents" || !jsonEqual(t, requests[1].JSON(t), `{"id":"research","source_type":"MDA"}`) {
		t.Errorf("second request = %s %s", requests[1].Line(), requests[1].Body)
	}
	if requests[2].Line() != "POST /api/v1/agents/research/environments" {
		t.Errorf("third request = %s", requests[2].Line())
	}
}

func TestDeploymentAgentTagUsesTheRegisteredAgentNotTheDeploymentName(t *testing.T) {
	server := serveOnce(t, 200, `{"items":[{"id":"actual-agent","tag_value_id":"agent-tag"}],"next_cursor":null}`)

	tag, ok, err := testClient(t, server.URL).DeploymentAgentTagValueID(t.Context(), "deployment-id")

	if err != nil || !ok || tag != "agent-tag" {
		t.Fatalf("DeploymentAgentTagValueID = %q, %v, %v", tag, ok, err)
	}
	request := server.Only(t)
	query := request.Query(t)
	if !strings.HasPrefix(request.Line(), "GET /api/v1/agents?") ||
		strings.Join(query["runtime_binding_source_id"], ",") != "deployment-id" ||
		strings.Join(query["source_type"], ",") != "MDA" ||
		request.Header.Get("X-Tenant-Id") != "tenant" {
		t.Fatalf("request = %s, tenant %q", request.Line(), request.Header.Get("X-Tenant-Id"))
	}
}

func TestDeploymentAgentTagSkipsWorkspacesWithoutTheRollout(t *testing.T) {
	for _, answer := range []reply{
		{status: 200, body: `{"items":[],"next_cursor":null}`},
		{status: 200, body: `{"items":[{"id":"actual-agent","tag_value_id":null}],"next_cursor":null}`},
		{status: 200, body: `{"items":[{"id":"actual-agent"}],"next_cursor":null}`},
		{status: 404, body: `{"detail":"not found"}`},
	} {
		tag, ok, err := testClient(t, serveSequence(t, answer).URL).DeploymentAgentTagValueID(t.Context(), "deployment-id")
		if err != nil || ok || tag != "" {
			t.Errorf("%s: got %q, %v, %v; want no tag", answer.body, tag, ok, err)
		}
	}
}

func TestDeploymentAgentTagReportsLookupErrors(t *testing.T) {
	for _, answer := range []reply{
		{status: 403, body: `{"detail":"lookup failed"}`},
		{status: 500, body: `{"detail":"lookup failed"}`},
		{status: 200, body: `{"items":[{"tag_value_id":7}]}`},
		{status: 200, body: `{"agents":[]}`},
	} {
		if _, _, err := testClient(t, serveSequence(t, answer).URL).DeploymentAgentTagValueID(t.Context(), "deployment-id"); err == nil {
			t.Errorf("%d %s: no error", answer.status, answer.body)
		}
	}
}
