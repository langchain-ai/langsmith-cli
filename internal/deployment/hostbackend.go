package deployment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HostBackendError represents an error from the host backend API.
type HostBackendError struct {
	Message    string
	StatusCode int
}

func (e *HostBackendError) Error() string {
	return e.Message
}

// HostBackendClient is an HTTP client for the LangGraph host backend deployment service.
type HostBackendClient struct {
	baseURL    string
	httpClient *http.Client
	headers    map[string]string
}

// NewHostBackendClient creates a new HostBackendClient.
func NewHostBackendClient(baseURL, apiKey string, tenantID string) *HostBackendClient {
	if baseURL == "" {
		baseURL = "https://api.smith.langchain.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	headers := map[string]string{
		"X-Api-Key": apiKey,
		"Accept":    "application/json",
	}
	if tenantID != "" {
		headers["X-Tenant-ID"] = tenantID
	}

	return &HostBackendClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		headers: headers,
	}
}

func (c *HostBackendClient) request(method, path string, payload map[string]interface{}, params map[string]string) (map[string]interface{}, error) {
	url := c.baseURL + path

	// Add query params
	if len(params) > 0 {
		parts := make([]string, 0, len(params))
		for k, v := range params {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
		url += "?" + strings.Join(parts, "&")
	}

	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, &HostBackendError{Message: fmt.Sprintf("marshaling request: %v", err)}
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, &HostBackendError{Message: fmt.Sprintf("creating request: %v", err)}
	}

	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &HostBackendError{Message: fmt.Sprintf("request failed: %v", err)}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &HostBackendError{Message: fmt.Sprintf("reading response: %v", err)}
	}

	if resp.StatusCode >= 400 {
		detail := string(respBody)
		if detail == "" {
			detail = fmt.Sprintf("%d", resp.StatusCode)
		}
		return nil, &HostBackendError{
			Message:    fmt.Sprintf("%s %s failed with status %d: %s", method, path, resp.StatusCode, detail),
			StatusCode: resp.StatusCode,
		}
	}

	if len(respBody) == 0 {
		return nil, nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, &HostBackendError{
			Message: fmt.Sprintf("failed to decode response from %s: %v", path, err),
		}
	}
	return result, nil
}

// CreateDeployment creates a new deployment.
func (c *HostBackendClient) CreateDeployment(payload map[string]interface{}) (map[string]interface{}, error) {
	return c.request("POST", "/v2/deployments", payload, nil)
}

// ListDeployments lists deployments, optionally filtered by name.
func (c *HostBackendClient) ListDeployments(nameContains string) (map[string]interface{}, error) {
	params := map[string]string{}
	if nameContains != "" {
		params["name_contains"] = nameContains
	}
	return c.request("GET", "/v2/deployments", nil, params)
}

// GetDeployment gets a deployment by ID.
func (c *HostBackendClient) GetDeployment(deploymentID string) (map[string]interface{}, error) {
	return c.request("GET", fmt.Sprintf("/v2/deployments/%s", deploymentID), nil, nil)
}

// DeleteDeployment deletes a deployment.
func (c *HostBackendClient) DeleteDeployment(deploymentID string) error {
	_, err := c.request("DELETE", fmt.Sprintf("/v2/deployments/%s", deploymentID), nil, nil)
	return err
}

// RequestPushToken requests a push token for a deployment.
func (c *HostBackendClient) RequestPushToken(deploymentID string) (map[string]interface{}, error) {
	return c.request("POST", fmt.Sprintf("/v2/deployments/%s/push-token", deploymentID), nil, nil)
}

// UpdateDeployment updates a deployment with a new image.
func (c *HostBackendClient) UpdateDeployment(deploymentID, imageURI string, secrets []map[string]string) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"source_revision_config": map[string]interface{}{
			"image_uri": imageURI,
		},
	}
	if secrets != nil {
		payload["secrets"] = secrets
	}
	return c.request("PATCH", fmt.Sprintf("/v2/deployments/%s", deploymentID), payload, nil)
}

// ListRevisions lists revisions for a deployment.
func (c *HostBackendClient) ListRevisions(deploymentID string, limit int) (map[string]interface{}, error) {
	params := map[string]string{
		"limit": fmt.Sprintf("%d", limit),
	}
	return c.request("GET", fmt.Sprintf("/v2/deployments/%s/revisions", deploymentID), nil, params)
}

// GetRevision gets a specific revision.
func (c *HostBackendClient) GetRevision(deploymentID, revisionID string) (map[string]interface{}, error) {
	return c.request("GET", fmt.Sprintf("/v2/deployments/%s/revisions/%s", deploymentID, revisionID), nil, nil)
}

// GetBuildLogs fetches build logs for a revision.
func (c *HostBackendClient) GetBuildLogs(projectID, revisionID string, payload map[string]interface{}) (map[string]interface{}, error) {
	return c.request("POST", fmt.Sprintf("/v1/projects/%s/revisions/%s/build_logs", projectID, revisionID), payload, nil)
}

// GetDeployLogs fetches deploy logs.
func (c *HostBackendClient) GetDeployLogs(projectID string, revisionID string, payload map[string]interface{}) (map[string]interface{}, error) {
	var path string
	if revisionID != "" {
		path = fmt.Sprintf("/v1/projects/%s/revisions/%s/deploy_logs", projectID, revisionID)
	} else {
		path = fmt.Sprintf("/v1/projects/%s/deploy_logs", projectID)
	}
	return c.request("POST", path, payload, nil)
}
