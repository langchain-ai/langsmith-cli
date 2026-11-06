package deployment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
)

// hostClient is a client for a stub host backend at hostURL, authenticated
// with the API key "test-key" in workspace.
func hostClient(t *testing.T, hostURL, workspace string) *hostapi.Client {
	t.Helper()
	isolateEnv(t)
	endpoints := LangSmithEndpoints{HostURL: hostURL, PlatformAPIURL: hostURL}
	c, err := endpoints.withAuth(client.Options{APIKey: "test-key"}, workspace).Client()
	require.NoError(t, err)
	return c
}

func TestAgentTargetUsesTheNameAsAgentIDAndSuffixesNonProductionResources(t *testing.T) {
	name, agent, err := AgentTarget("support", ptr(hostapi.AgentEnvironmentStaging))
	require.NoError(t, err)
	assert.Equal(t, "support-staging", name)
	assert.Equal(t, &hostapi.DeploymentAgent{AgentID: "support", Environment: hostapi.AgentEnvironmentStaging}, agent)

	name, _, err = AgentTarget("support", ptr(hostapi.AgentEnvironmentProduction))
	require.NoError(t, err)
	assert.Equal(t, "support", name)

	name, agent, err = AgentTarget("support", nil)
	require.NoError(t, err)
	assert.Equal(t, "support", name)
	assert.Nil(t, agent)
}

func TestAgentTargetRejectsAnInvalidAgentID(t *testing.T) {
	_, _, err := AgentTarget("2support", ptr(hostapi.AgentEnvironmentStaging))
	assert.EqualError(t, err, `agent ID "2support" must be 1-63 lowercase letters, digits, or hyphens, start with a letter, and end with a letter or digit`)
}

func TestPostCommandDeploymentLookupMatchesExactNameOrAgent(t *testing.T) {
	for _, agentMode := range []bool{false, true} {
		server := serve(t, `{"resources":[{"name":"agent-preview","id":"preview","is_preview":true},{"name":"agent","id":"deployment"}]}`)
		c := hostClient(t, server.URL, "workspace")
		var agent *hostapi.DeploymentAgent
		wantQuery := "name_contains=agent"
		if agentMode {
			agent = &hostapi.DeploymentAgent{AgentID: "agent", Environment: hostapi.AgentEnvironmentStaging}
			wantQuery = "agent_id=agent&agent_environment=staging"
		}

		deployment, err := FindTargetDeployment(t.Context(), c, "agent", agent)

		require.NoError(t, err)
		assert.Equal(t, "deployment", deployment["id"])
		assert.Equal(t, "/v2/deployments?"+wantQuery, server.Requests()[0].URI)
	}

	server := serve(t, `{"resources":[]}`)
	deployment, err := FindTargetDeployment(t.Context(), hostClient(t, server.URL, ""), "deleted", nil)
	require.NoError(t, err)
	assert.Nil(t, deployment)
}

func TestFetchDeploymentWithMarkerRereadsATrimmedListing(t *testing.T) {
	server := serve(t, `{"id":"dep-1","name":"agent","is_managed_deep_agent":true}`)
	c := hostClient(t, server.URL, "")

	deployment := FetchDeploymentWithMarker(t.Context(), c, map[string]any{"id": "dep-1", "name": "agent"})

	assert.Equal(t, map[string]any{"id": "dep-1", "name": "agent", "is_managed_deep_agent": true}, deployment)
	assert.Equal(t, "/v2/deployments/dep-1", server.Requests()[0].URI)
}

func TestFetchDeploymentWithMarkerKeepsTheListingWhenTheRereadIsUnmarked(t *testing.T) {
	server := serve(t, `{"id":"dep-1","name":"agent"}`)
	listed := map[string]any{"id": "dep-1", "name": "agent", "source": "listing"}

	deployment := FetchDeploymentWithMarker(t.Context(), hostClient(t, server.URL, ""), listed)

	assert.Equal(t, listed, deployment)
}
