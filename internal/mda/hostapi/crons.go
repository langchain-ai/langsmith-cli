package hostapi

// Agent Server cron endpoints.
//
// These calls go to the deployed Agent Server URL, not the host backend. They
// authenticate with the CLI's LangSmith credential (and workspace), and
// declare the LangSmith studio auth scheme via [Client.requestAgentServer] so
// a managed-identity deployment's custom `authenticate` handler (which
// requires an end-user bearer token) does not reject these control-plane
// calls.

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

// cronSearchPageSize is the page size for cron search; a shorter page means
// the last page was reached.
const cronSearchPageSize = 100

// SearchManagedCrons searches crons owned by Managed Deep Agents on a deployed
// Agent Server.
func (c *Client) SearchManagedCrons(ctx context.Context, deploymentURL string) ([]any, error) {
	all := []any{}
	for {
		raw, err := c.requestAgentServer(ctx, strings.TrimRight(deploymentURL, "/"), http.MethodPost, "/runs/crons/search", nil, managedCronsSearchPayload(len(all)))
		if err != nil {
			return nil, err
		}
		response, err := decodeAny(raw)
		if err != nil {
			return nil, err
		}
		page, ok := response.([]any)
		if !ok {
			page, ok = field(response, "items").([]any)
		}
		if !ok {
			return nil, errors.New("unexpected cron search response shape")
		}
		complete := len(page) < cronSearchPageSize
		all = append(all, page...)
		if complete {
			return all, nil
		}
	}
}

// CreateCron creates a stateless cron, which runs on a new thread each time.
func (c *Client) CreateCron(ctx context.Context, deploymentURL string, payload map[string]any) (map[string]any, error) {
	path := "/runs/crons"
	raw, err := c.requestAgentServer(ctx, strings.TrimRight(deploymentURL, "/"), http.MethodPost, path, nil, payload)
	if err != nil {
		return nil, err
	}
	return decodeObject(raw, path)
}

// CreateThreadCron creates a cron bound to an existing/persistent thread.
func (c *Client) CreateThreadCron(ctx context.Context, deploymentURL, threadID string, payload map[string]any) (map[string]any, error) {
	path := "/threads/" + threadID + "/runs/crons"
	raw, err := c.requestAgentServer(ctx, strings.TrimRight(deploymentURL, "/"), http.MethodPost, path, nil, payload)
	if err != nil {
		return nil, err
	}
	return decodeObject(raw, path)
}

// DeleteCron deletes a cron by ID, returning whatever JSON the server
// answered with (nil for none).
func (c *Client) DeleteCron(ctx context.Context, deploymentURL, cronID string) (any, error) {
	raw, err := c.requestAgentServer(ctx, strings.TrimRight(deploymentURL, "/"), http.MethodDelete, "/runs/crons/"+cronID, nil, nil)
	if err != nil {
		return nil, err
	}
	return decodeAny(raw)
}

func managedCronsSearchPayload(offset int) map[string]any {
	return map[string]any{
		"limit":      cronSearchPageSize,
		"offset":     offset,
		"sort_by":    "created_at",
		"sort_order": "asc",
	}
}
