package hostapi

// Host-backend (LangGraph Deployment) endpoints.
//
// Only the subset needed by the remote-source deploy flow is implemented:
// list/create deployments, request an upload URL, trigger an internal-source
// build, and poll revisions. All calls go to the host backend
// (`api.host.langchain.com`).

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
)

// GetDeployment fetches a deployment object from the host backend.
func (c *Client) GetDeployment(ctx context.Context, deploymentID string) (map[string]any, error) {
	return c.hostObject(ctx, http.MethodGet, "/v2/deployments/"+deploymentID, nil, nil, requestOptions{})
}

// ListDeployments lists deployments, optionally filtering by a name substring.
//
// `name_contains` is the only filter the host backend is known to support, and
// the endpoint paginates (20 per page by default). Callers must still pick the
// exact match out of the page rather than assuming a single resource comes
// back.
func (c *Client) ListDeployments(ctx context.Context, nameContains string) (map[string]any, error) {
	return c.hostObject(ctx, http.MethodGet, "/v2/deployments", []queryParam{{"name_contains", nameContains}}, nil, requestOptions{})
}

// ListAgentDeployments lists the deployments of agent.
func (c *Client) ListAgentDeployments(ctx context.Context, agent DeploymentAgent) (map[string]any, error) {
	return c.hostObject(ctx, http.MethodGet, "/v2/deployments", []queryParam{
		{"agent_id", agent.AgentID},
		{"agent_environment", agent.Environment.String()},
	}, nil, requestOptions{})
}

// CreateDeployment creates a new internal-source deployment, returning the
// created object.
//
// MDA always creates deployments as `internal_source`; the tarball +
// `langgraph.json` path are attached later via
// [Client.UpdateDeploymentInternalSource].
func (c *Client) CreateDeployment(ctx context.Context, name string, deploymentType DeploymentType, secrets []Secret) (map[string]any, error) {
	return c.postDeployment(ctx, "name", name, deploymentType, secrets)
}

// CreateAgentDeployment is [Client.CreateDeployment] for an agent deployment.
func (c *Client) CreateAgentDeployment(ctx context.Context, agent DeploymentAgent, deploymentType DeploymentType, secrets []Secret) (map[string]any, error) {
	return c.postDeployment(ctx, "agent", agent, deploymentType, secrets)
}

func (c *Client) postDeployment(ctx context.Context, identityKey string, identity any, deploymentType DeploymentType, secrets []Secret) (map[string]any, error) {
	payload := map[string]any{
		"source": "internal_source",
		// MDA deployments carry the caller-selected `deployment_type` plus the
		// request-only `managed_deep_agent` flag; the Host Backend gates the
		// flag on the org's Managed Deep Agents entitlement and persists the
		// marker used for billing and the UI tag.
		"source_config":          map[string]any{"deployment_type": deploymentType.String()},
		"source_revision_config": map[string]any{},
		"managed_deep_agent":     true,
	}
	payload[identityKey] = identity
	// The host backend requires `secrets` to be present (an empty list is
	// valid), so always include it.
	payload["secrets"] = secretList(secrets)
	return c.hostObject(ctx, http.MethodPost, "/v2/deployments", nil, payload, requestOptions{timeout: CreateDeploymentTimeout})
}

// CreateExternalDeployment creates a deployment from an image in a
// customer-managed registry.
func (c *Client) CreateExternalDeployment(ctx context.Context, name, imageURI string, secrets []Secret) (map[string]any, error) {
	return c.postExternalDeployment(ctx, "name", name, imageURI, secrets)
}

// CreateExternalAgentDeployment creates an agent deployment from an image in
// a customer-managed registry.
func (c *Client) CreateExternalAgentDeployment(ctx context.Context, agent DeploymentAgent, imageURI string, secrets []Secret) (map[string]any, error) {
	return c.postExternalDeployment(ctx, "agent", agent, imageURI, secrets)
}

func (c *Client) postExternalDeployment(ctx context.Context, identityKey string, identity any, imageURI string, secrets []Secret) (map[string]any, error) {
	payload := map[string]any{
		"source":                 "external_docker",
		"source_config":          map[string]any{"resource_spec": map[string]any{}},
		"source_revision_config": map[string]any{"image_uri": imageURI},
		"managed_deep_agent":     true,
	}
	payload[identityKey] = identity
	payload["secrets"] = secretList(secrets)
	return c.hostObject(ctx, http.MethodPost, "/v2/deployments", nil, payload, requestOptions{timeout: CreateDeploymentTimeout})
}

// RequestUploadURL requests a signed upload URL for the source tarball used
// by remote builds.
func (c *Client) RequestUploadURL(ctx context.Context, deploymentID string) (map[string]any, error) {
	return c.hostObject(ctx, http.MethodPost, "/v2/deployments/"+deploymentID+"/upload-url", nil, nil, requestOptions{})
}

// UpdateDeploymentInternalSource triggers a remote build revision from an
// uploaded source tarball.
func (c *Client) UpdateDeploymentInternalSource(ctx context.Context, deploymentID, sourceTarballPath, configPath string, secrets []Secret) (map[string]any, error) {
	payload := map[string]any{
		"revision_source": "internal_source",
		"source_revision_config": map[string]any{
			"source_tarball_path":   sourceTarballPath,
			"langgraph_config_path": configPath,
		},
		// Re-affirm the MDA marker on update: this keeps the flag on the
		// "existing deployment" path (where CreateDeployment is skipped) so the
		// Host Backend re-checks the entitlement here too. The deployment type
		// is set at create and is not resent.
		"managed_deep_agent": true,
		"secrets":            secretList(secrets),
	}
	return c.hostObject(ctx, http.MethodPatch, "/v2/deployments/"+deploymentID, nil, payload, requestOptions{})
}

// UpdateDeploymentExternalImage creates a new revision using a
// customer-managed image.
func (c *Client) UpdateDeploymentExternalImage(ctx context.Context, deploymentID, imageURI string, secrets []Secret) (map[string]any, error) {
	payload := map[string]any{
		"source_revision_config": map[string]any{"image_uri": imageURI},
		"managed_deep_agent":     true,
		"secrets":                secretList(secrets),
	}
	return c.hostObject(ctx, http.MethodPatch, "/v2/deployments/"+deploymentID, nil, payload, requestOptions{})
}

// DeleteDeployment deletes a deployment, optionally removing its tracing
// project too.
//
// Returns false when the deployment is already gone (HTTP 404), so teardown
// can report "nothing to do" instead of failing.
//
// The tracing project LangSmith auto-creates alongside a deployment is a
// separate resource that outlives the deployment. Leaving it behind makes the
// next deploy of the same name fail with a conflict, so callers tearing a
// deployment down should pass deleteTracingProject.
func (c *Client) DeleteDeployment(ctx context.Context, deploymentID string, deleteTracingProject bool) (bool, error) {
	_, deleted, err := c.requestOptional(ctx, c.hostBackendURL, http.MethodDelete, "/v2/deployments/"+deploymentID,
		[]queryParam{{"delete_tracing_project", strconv.FormatBool(deleteTracingProject)}}, nil)
	return deleted, err
}

// ListRevisions lists a deployment's revisions, newest first.
func (c *Client) ListRevisions(ctx context.Context, deploymentID string, limit uint32) (map[string]any, error) {
	return c.hostObject(ctx, http.MethodGet, "/v2/deployments/"+deploymentID+"/revisions",
		[]queryParam{{"limit", strconv.FormatUint(uint64(limit), 10)}}, nil, requestOptions{})
}

// PutManagedDeepAgentArtifacts stores the build artifacts `mda prepare`
// produced for a revision.
//
// Host-backend accepts this only while the revision is building; the deploy
// step reads the snapshot reference and the reconcile step reads the cron and
// channel payloads.
func (c *Client) PutManagedDeepAgentArtifacts(ctx context.Context, deploymentID, revisionID string, artifacts any, buildToken string) (map[string]any, error) {
	// This credential must not enter shared client state or follow redirects.
	path := fmt.Sprintf("/v2/managed-deep-agents/%s/revisions/%s/build-artifacts", deploymentID, revisionID)
	status, text, err := c.send(ctx, c.hostBackendURL+path, http.MethodPut, nil, artifacts, requestOptions{
		noRedirects: true,
		header:      http.Header{"X-Mda-Build-Token": {buildToken}},
	})
	if err != nil {
		return nil, fmt.Errorf("build artifact report request failed: %w", err)
	}
	raw, err := successResult(status, text, path)
	if err != nil {
		return nil, err
	}
	return decodeObject(raw, path)
}

// GetRevision fetches a single revision (expected to carry a `status`).
func (c *Client) GetRevision(ctx context.Context, deploymentID, revisionID string) (map[string]any, error) {
	return c.hostObject(ctx, http.MethodGet, "/v2/deployments/"+deploymentID+"/revisions/"+revisionID, nil, nil, requestOptions{})
}

// hostObject is a host backend request answered with a JSON object.
func (c *Client) hostObject(ctx context.Context, method, path string, params []queryParam, payload any, options requestOptions) (map[string]any, error) {
	raw, err := c.requestInner(ctx, c.hostBackendURL, method, path, params, payload, options)
	if err != nil {
		return nil, err
	}
	return decodeObject(raw, path)
}

// secretList is secrets as the host backend expects them: always a list.
func secretList(secrets []Secret) []Secret {
	if secrets == nil {
		return []Secret{}
	}
	return secrets
}
