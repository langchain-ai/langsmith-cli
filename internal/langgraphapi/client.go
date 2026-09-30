// Package langgraphapi is a typed client for the LangSmith Deployment control plane,
// whose API is not part of the generated langsmith-go SDK.
package langgraphapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// MaxPageSize is the largest page the control plane serves.
const MaxPageSize = 100

// Auth carries the LangSmith credentials forwarded to the control plane.
type Auth struct {
	APIKey      string
	BearerToken string
	TenantID    string
}

type Client struct {
	endpoints Endpoints
	auth      Auth
	http      *http.Client
}

func New(endpoints Endpoints, auth Auth) *Client {
	return &Client{
		endpoints: endpoints,
		auth:      auth,
		http: &http.Client{
			Timeout: 30 * time.Second,
			// Redirects would carry X-Api-Key to whatever host they name.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (c *Client) Endpoints() Endpoints { return c.endpoints }

// Error is a non-2xx control-plane response.
type Error struct {
	Method     string
	Path       string
	StatusCode int
	Detail     string
	Body       string
}

func (e *Error) Error() string {
	reason := e.Detail
	if reason == "" {
		reason = strings.TrimSpace(e.Body)
	}
	if reason == "" {
		reason = strconv.Itoa(e.StatusCode)
	}
	return fmt.Sprintf("%s %s failed with status %d: %s", e.Method, e.Path, e.StatusCode, reason)
}

// StatusCode returns the HTTP status of a control-plane error, or 0.
func StatusCode(err error) int {
	if e, ok := errors.AsType[*Error](err); ok {
		return e.StatusCode
	}
	return 0
}

// raw keeps the server payload so JSON output is lossless.
type raw struct {
	payload json.RawMessage
}

func (r raw) MarshalJSON() ([]byte, error) {
	if r.payload == nil {
		return []byte("null"), nil
	}
	return r.payload, nil
}

type Deployment struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Source           string `json:"source"`
	LatestRevisionID string `json:"latest_revision_id"`
	TenantID         string `json:"tenant_id"`
	IsPreview        bool   `json:"is_preview"`
	SourceConfig     struct {
		CustomURL string `json:"custom_url"`
	} `json:"source_config"`
	raw
}

func (d *Deployment) UnmarshalJSON(data []byte) error {
	type plain Deployment
	if err := json.Unmarshal(data, (*plain)(d)); err != nil {
		return err
	}
	d.payload = bytes.Clone(data)
	return nil
}

func (d Deployment) MarshalJSON() ([]byte, error) { return d.raw.MarshalJSON() }

type Revision struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	raw
}

func (r *Revision) UnmarshalJSON(data []byte) error {
	type plain Revision
	if err := json.Unmarshal(data, (*plain)(r)); err != nil {
		return err
	}
	r.payload = bytes.Clone(data)
	return nil
}

func (r Revision) MarshalJSON() ([]byte, error) { return r.raw.MarshalJSON() }

type Listener struct {
	ID            string `json:"id"`
	ComputeID     string `json:"compute_id"`
	ComputeConfig struct {
		K8sNamespaces []string `json:"k8s_namespaces"`
	} `json:"compute_config"`
}

type Secret struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Agent selects a deployment by logical agent and environment instead of by name.
type Agent struct {
	AgentID     string `json:"agent_id"`
	Environment string `json:"environment"`
}

type DeploymentCreate struct {
	Name                 string         `json:"name,omitempty"`
	Agent                *Agent         `json:"agent,omitempty"`
	Source               string         `json:"source"`
	SourceConfig         map[string]any `json:"source_config"`
	SourceRevisionConfig map[string]any `json:"source_revision_config"`
	Secrets              []Secret       `json:"secrets"`
}

type DeploymentUpdate struct {
	RevisionSource       string         `json:"revision_source,omitempty"`
	SourceConfig         map[string]any `json:"source_config,omitempty"`
	SourceRevisionConfig map[string]any `json:"source_revision_config"`
	Secrets              []Secret       `json:"secrets"`
}

type DeploymentFilter struct {
	Name             string
	NameContains     string
	AgentID          string
	AgentEnvironment string
	Limit            int
	Offset           int
}

type PushToken struct {
	Token       string `json:"token"`
	RegistryURL string `json:"registry_url"`
}

type UploadURL struct {
	UploadURL  string `json:"upload_url"`
	ObjectPath string `json:"object_path"`
}

type LogsRequest struct {
	Limit     int    `json:"limit,omitempty"`
	Order     string `json:"order,omitempty"`
	Offset    string `json:"offset,omitempty"`
	Level     string `json:"level,omitempty"`
	Query     string `json:"query,omitempty"`
	StartTime string `json:"start_time,omitempty"`
	EndTime   string `json:"end_time,omitempty"`
}

// LogEntry timestamps arrive as epoch milliseconds or as strings.
type LogEntry struct {
	ID        string `json:"id,omitempty"`
	Timestamp any    `json:"timestamp,omitempty"`
	Level     string `json:"level,omitempty"`
	Message   string `json:"message"`
}

type LogsResponse struct {
	Logs       []LogEntry `json:"logs"`
	NextOffset string     `json:"next_offset"`
}

type page[T any] struct {
	Resources []T `json:"resources"`
}

func (c *Client) CreateDeployment(ctx context.Context, body DeploymentCreate) (*Deployment, error) {
	var out Deployment
	if err := c.do(ctx, call{method: http.MethodPost, path: "/v2/deployments", body: body}, &out); err != nil {
		return nil, err
	}
	if out.ID == "" {
		return nil, errors.New("POST /v2/deployments succeeded but the response has no deployment id")
	}
	return &out, nil
}

func (c *Client) ListDeployments(ctx context.Context, f DeploymentFilter) ([]Deployment, error) {
	q := url.Values{}
	setQuery(q, "name", f.Name)
	setQuery(q, "name_contains", f.NameContains)
	setQuery(q, "agent_id", f.AgentID)
	setQuery(q, "agent_environment", f.AgentEnvironment)
	if f.Limit > 0 {
		q.Set("limit", strconv.Itoa(f.Limit))
	}
	if f.Offset > 0 {
		q.Set("offset", strconv.Itoa(f.Offset))
	}
	var out page[Deployment]
	if err := c.do(ctx, call{method: http.MethodGet, path: "/v2/deployments", query: q}, &out); err != nil {
		return nil, err
	}
	return out.Resources, nil
}

func (c *Client) GetDeployment(ctx context.Context, id string) (*Deployment, error) {
	var out Deployment
	if err := c.do(ctx, call{method: http.MethodGet, path: "/v2/deployments/" + url.PathEscape(id)}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateDeployment(ctx context.Context, id string, body DeploymentUpdate) (*Deployment, error) {
	var out Deployment
	if err := c.do(ctx, call{method: http.MethodPatch, path: "/v2/deployments/" + url.PathEscape(id), body: body}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteDeployment(ctx context.Context, id string) error {
	return c.do(ctx, call{method: http.MethodDelete, path: "/v2/deployments/" + url.PathEscape(id)}, nil)
}

func (c *Client) RequestPushToken(ctx context.Context, id string) (*PushToken, error) {
	var out PushToken
	if err := c.do(ctx, call{method: http.MethodPost, path: "/v2/deployments/" + url.PathEscape(id) + "/push-token"}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) RequestUploadURL(ctx context.Context, id string) (*UploadURL, error) {
	var out UploadURL
	if err := c.do(ctx, call{method: http.MethodPost, path: "/v2/deployments/" + url.PathEscape(id) + "/upload-url"}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListRevisions(ctx context.Context, deploymentID string, limit int) ([]Revision, error) {
	q := url.Values{"limit": {strconv.Itoa(limit)}}
	var out page[Revision]
	if err := c.do(ctx, call{method: http.MethodGet, path: "/v2/deployments/" + url.PathEscape(deploymentID) + "/revisions", query: q}, &out); err != nil {
		return nil, err
	}
	return out.Resources, nil
}

func (c *Client) GetRevision(ctx context.Context, deploymentID, revisionID string) (*Revision, error) {
	var out Revision
	path := "/v2/deployments/" + url.PathEscape(deploymentID) + "/revisions/" + url.PathEscape(revisionID)
	if err := c.do(ctx, call{method: http.MethodGet, path: path}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetListener(ctx context.Context, id string) (*Listener, error) {
	var out Listener
	if err := c.do(ctx, call{method: http.MethodGet, path: "/v2/listeners/" + url.PathEscape(id)}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListListeners(ctx context.Context) ([]Listener, error) {
	q := url.Values{"limit": {strconv.Itoa(MaxPageSize)}}
	var out page[Listener]
	if err := c.do(ctx, call{method: http.MethodGet, path: "/v2/listeners", query: q}, &out); err != nil {
		return nil, err
	}
	return out.Resources, nil
}

func (c *Client) BuildLogs(ctx context.Context, deploymentID, revisionID string, req LogsRequest) (*LogsResponse, error) {
	path := "/v1/projects/" + url.PathEscape(deploymentID) + "/revisions/" + url.PathEscape(revisionID) + "/build_logs"
	var out LogsResponse
	if err := c.do(ctx, call{method: http.MethodPost, path: path, body: req}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeployLogs reads runtime logs for the deployment, or for one revision when revisionID is set.
func (c *Client) DeployLogs(ctx context.Context, deploymentID, revisionID string, req LogsRequest) (*LogsResponse, error) {
	path := "/v1/projects/" + url.PathEscape(deploymentID) + "/deploy_logs"
	if revisionID != "" {
		path = "/v1/projects/" + url.PathEscape(deploymentID) + "/revisions/" + url.PathEscape(revisionID) + "/deploy_logs"
	}
	var out LogsResponse
	if err := c.do(ctx, call{method: http.MethodPost, path: path, body: req}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func setQuery(q url.Values, key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}

type call struct {
	method string
	path   string
	query  url.Values
	body   any
}

func (c *Client) do(ctx context.Context, cl call, out any) error {
	method, path := cl.method, cl.path
	var reader io.Reader
	if cl.body != nil {
		data, err := json.Marshal(cl.body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	target := c.endpoints.ControlPlaneURL + path
	if len(cl.query) > 0 {
		target += "?" + cl.query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if cl.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.auth.APIKey != "" {
		req.Header.Set("X-Api-Key", c.auth.APIKey)
	}
	if c.auth.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.auth.BearerToken)
	}
	if c.auth.TenantID != "" {
		req.Header.Set("X-Tenant-ID", c.auth.TenantID)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading %s %s response: %w", method, path, err)
	}
	if resp.StatusCode >= 300 {
		return c.translate(&Error{
			Method:     method,
			Path:       path,
			StatusCode: resp.StatusCode,
			Detail:     errorDetail(data),
			Body:       string(data),
		})
	}
	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decoding %s %s response: %w", method, path, err)
	}
	return nil
}

func (c *Client) translate(err *Error) error {
	if err.StatusCode != http.StatusForbidden {
		return err
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "requires workspace specification"):
		return fmt.Errorf("the API key is org-scoped and requires a workspace: pass --workspace <workspace-id> or set LANGSMITH_WORKSPACE_ID (find it in LangSmith under Settings > Workspaces): %w", err)
	case strings.Contains(strings.ToLower(message), "not enabled"):
		return fmt.Errorf("LangSmith Deployment is not enabled for this organization; enable it at %s/host/deployments (make sure it is the organization your credentials belong to): %w", c.endpoints.DashboardURL, err)
	}
	return err
}

func errorDetail(body []byte) string {
	var parsed struct {
		Detail any `json:"detail"`
	}
	if json.Unmarshal(body, &parsed) != nil {
		return ""
	}
	detail, _ := parsed.Detail.(string)
	return detail
}
