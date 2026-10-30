package hostapi

// LangSmith platform API endpoints for managed sandboxes, used by deploy's
// sandbox access check and recipe bake.
//
// Calls go to the platform API (`api.smith.langchain.com`), not the host
// backend.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
)

// SandboxSummary is a sandbox as LangSmith reports it.
type SandboxSummary struct {
	// Name is the display name, which is also the delete key.
	Name string `json:"name"`
	// Status is `provisioning`, `ready`, `failed`, `stopped`, or `deleting`.
	Status string `json:"status"`
	// DataplaneURL is the dataplane URL used for execute during bake.
	DataplaneURL string `json:"dataplane_url"`
	// ID is the sandbox UUID.
	ID string `json:"id"`
}

// UnmarshalJSON requires the name.
func (s *SandboxSummary) UnmarshalJSON(data []byte) error {
	if err := requireFields(data, "name"); err != nil {
		return err
	}
	type plain SandboxSummary
	return json.Unmarshal(data, (*plain)(s))
}

// EnsureSandboxAccess verifies that the current organization may use managed
// sandboxes.
//
// A sandbox declaration without a recipe does not otherwise touch the sandbox
// API during deploy, so an entitlement failure would only surface later when
// the agent first uses a filesystem or execution tool.
func (c *Client) EnsureSandboxAccess(ctx context.Context) error {
	_, err := c.sdkRequest(ctx, http.MethodGet, "/api/v2/sandboxes/boxes", requestOptions{},
		func(ctx context.Context, opts ...option.RequestOption) error {
			_, err := c.ls.SDK.Sandboxes.Boxes.List(ctx, langsmith.SandboxBoxListParams{
				Limit:  langsmith.F(int64(1)),
				Offset: langsmith.F(int64(0)),
			}, opts...)
			return err
		})
	return err
}

// CreateSandboxBox creates a sandbox box (bake builder) and waits until it is
// ready.
func (c *Client) CreateSandboxBox(ctx context.Context, name string, options CreateSandboxBoxOptions) (SandboxSummary, error) {
	params := langsmith.SandboxBoxNewParams{
		Name:           langsmith.F(name),
		IdleTtlSeconds: langsmith.F(int64(options.IdleTTLSeconds)),
	}
	if options.SnapshotName != "" {
		params.SnapshotName = langsmith.F(options.SnapshotName)
	}
	if options.SnapshotID != "" {
		params.SnapshotID = langsmith.F(options.SnapshotID)
	}
	if len(options.EnvVars) > 0 {
		params.EnvVars = langsmith.F(options.EnvVars)
	}
	timeout := time.Duration(options.TimeoutSecs+30) * time.Second
	raw, err := c.sdkRequest(ctx, http.MethodPost, "/api/v2/sandboxes/boxes", requestOptions{timeout: timeout},
		func(ctx context.Context, opts ...option.RequestOption) error {
			// The SDK's create parameters have neither field.
			opts = append(opts,
				option.WithJSONSet("wait_for_ready", true),
				option.WithJSONSet("timeout", options.TimeoutSecs),
			)
			_, err := c.ls.SDK.Sandboxes.Boxes.New(ctx, params, opts...)
			return err
		})
	if err != nil {
		return SandboxSummary{}, err
	}
	var created SandboxSummary
	if err := json.Unmarshal(nullIfEmpty(raw), &created); err != nil {
		return SandboxSummary{}, fmt.Errorf("failed to decode create sandbox response: %w", err)
	}
	return created, nil
}

// DeleteSandboxBox deletes one sandbox by name (best-effort; 404 is ok).
func (c *Client) DeleteSandboxBox(ctx context.Context, name string) error {
	_, _, err := c.sdkRequestOptional(ctx, http.MethodDelete, "/api/v2/sandboxes/boxes/"+name,
		func(ctx context.Context, opts ...option.RequestOption) error {
			return c.ls.SDK.Sandboxes.Boxes.Delete(ctx, name, opts...)
		})
	return err
}

// ExecuteSandboxCommand runs a command on a sandbox via the dataplane HTTP
// execute endpoint.
func (c *Client) ExecuteSandboxCommand(ctx context.Context, dataplaneURL, command string, timeoutSecs uint64) (SandboxCommandResult, error) {
	payload := map[string]any{
		"command": command,
		"timeout": timeoutSecs,
		"shell":   "/bin/bash",
	}
	status, text, err := c.send(ctx, strings.TrimRight(dataplaneURL, "/")+"/execute", http.MethodPost, nil, payload,
		requestOptions{timeout: time.Duration(timeoutSecs+30) * time.Second})
	if err != nil {
		return SandboxCommandResult{}, fmt.Errorf("sandbox execute request failed: %w", err)
	}
	if !isSuccess(status) {
		return SandboxCommandResult{}, apiErrorFromResponse(status, text)
	}
	result := SandboxCommandResult{ExitCode: -1}
	if err := json.Unmarshal(text, &result); err != nil {
		return SandboxCommandResult{}, fmt.Errorf("failed to decode execute response: %w", err)
	}
	return result, nil
}

// CreateSandboxBoxOptions are the options of [Client.CreateSandboxBox].
type CreateSandboxBoxOptions struct {
	SnapshotName   string
	SnapshotID     string
	EnvVars        map[string]string
	TimeoutSecs    uint64
	IdleTTLSeconds uint64
}

// SandboxCommandResult is the result of a blocking dataplane `/execute` call.
// A response without an exit code reads as -1.
type SandboxCommandResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int32  `json:"exit_code"`
}

// nullIfEmpty is raw, or JSON null for an empty body, so a struct decode of
// an empty response fails like serde's does.
func nullIfEmpty(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return json.RawMessage("null")
	}
	return raw
}
