package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
)

// Client wraps the LangSmith Go SDK and provides helpers for raw HTTP calls.
type Client struct {
	SDK    *langsmith.Client
	apiKey string
	apiURL string

	// Cached session name → ID mappings (per invocation).
	sessionCache map[string]string
}

// New creates a new Client.
func New(apiKey, apiURL string) *Client {
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	// Only set base URL if not the default (the SDK reads LANGSMITH_ENDPOINT too).
	if apiURL != "" {
		opts = append(opts, option.WithBaseURL(apiURL))
	}

	return &Client{
		SDK:          langsmith.NewClient(opts...),
		apiKey:       apiKey,
		apiURL:       strings.TrimRight(apiURL, "/"),
		sessionCache: make(map[string]string),
	}
}

// ResolveSessionID resolves a project name to its session UUID, with caching.
func (c *Client) ResolveSessionID(ctx context.Context, projectName string) (string, error) {
	if id, ok := c.sessionCache[projectName]; ok {
		return id, nil
	}
	resp, err := c.SDK.Sessions.List(ctx, langsmith.SessionListParams{
		Name:  langsmith.F(projectName),
		Limit: langsmith.F(int64(1)),
	})
	if err != nil {
		return "", fmt.Errorf("resolving project %q: %w", projectName, err)
	}
	if len(resp.Items) == 0 {
		return "", fmt.Errorf("project not found: %s", projectName)
	}
	id := resp.Items[0].ID
	c.sessionCache[projectName] = id
	return id, nil
}

// --- Raw HTTP helpers for endpoints not covered by the SDK ---

// RawGet performs a GET request to the LangSmith API.
func (c *Client) RawGet(ctx context.Context, path string, result any) error {
	return c.rawRequest(ctx, http.MethodGet, path, nil, result)
}

// RawPost performs a POST request to the LangSmith API.
func (c *Client) RawPost(ctx context.Context, path string, body any, result any) error {
	return c.rawRequest(ctx, http.MethodPost, path, body, result)
}

// RawDelete performs a DELETE request to the LangSmith API.
func (c *Client) RawDelete(ctx context.Context, path string, result any) error {
	return c.rawRequest(ctx, http.MethodDelete, path, nil, result)
}

func (c *Client) rawRequest(ctx context.Context, method, path string, body any, result any) error {
	url := c.apiURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	if wsID := os.Getenv("LANGSMITH_WORKSPACE_ID"); wsID != "" {
		req.Header.Set("x-tenant-id", wsID)
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}

	return nil
}
