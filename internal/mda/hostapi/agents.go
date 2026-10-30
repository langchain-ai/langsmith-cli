package hostapi

// Agent Platform endpoints used by deploy and dev.

import (
	"context"
	"errors"
	"net/http"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
)

// DeploymentAgentTagValueID finds the Agent resource tag of the MDA agent
// bound to deploymentID.
//
// Returns false when the server has no Agent API, no agent is bound, or the
// agent has no tag. Looks up by deployment, not name, because the host
// backend can derive or suffix the agent ID.
func (c *Client) DeploymentAgentTagValueID(ctx context.Context, deploymentID string) (string, bool, error) {
	raw, found, err := c.sdkRequestOptional(ctx, http.MethodGet, "/api/v1/agents",
		func(ctx context.Context, opts ...option.RequestOption) error {
			// The SDK's list parameters have no runtime binding filter.
			opts = append(opts, option.WithQuery("runtime_binding_source_id", deploymentID))
			_, err := c.ls.SDK.Agents.List(ctx, langsmith.AgentListParams{
				SourceType: langsmith.F(langsmith.AgentListParamsSourceTypeMda),
			}, opts...)
			return err
		})
	if err != nil || !found {
		return "", false, err
	}
	page, err := decodeAny(raw)
	if err != nil {
		return "", false, err
	}
	items, ok := field(page, "items").([]any)
	if !ok {
		return "", false, errors.New("Agent lookup response has no items array")
	}
	if len(items) == 0 {
		return "", false, nil
	}
	tag := field(items[0], "tag_value_id")
	if tag == nil {
		return "", false, nil
	}
	text, ok := tag.(string)
	if !ok || text == "" {
		return "", false, errors.New("Agent lookup response has an invalid tag_value_id")
	}
	return text, true, nil
}

// LocalEnvironmentProject is the tracing project of agentID's LOCAL
// environment, creating the agent or the environment when missing.
func (c *Client) LocalEnvironmentProject(ctx context.Context, agentID string) (string, error) {
	base := c.PlatformBase()
	path := "/api/v1/agents/" + agentID + "/environments"
	local := map[string]any{"environment": "LOCAL"}
	environment, found, err := c.requestOptional(ctx, base, http.MethodPost, path, nil, local)
	if err != nil {
		return "", err
	}
	if !found {
		if _, err := c.sdkRequest(ctx, http.MethodPost, "/api/v1/agents", requestOptions{},
			func(ctx context.Context, opts ...option.RequestOption) error {
				_, err := c.ls.SDK.Agents.New(ctx, langsmith.AgentNewParams{
					ID:         langsmith.F(agentID),
					SourceType: langsmith.F(langsmith.AgentNewParamsSourceTypeMda),
				}, opts...)
				return err
			}); err != nil {
			return "", err
		}
		if environment, err = c.request(ctx, base, http.MethodPost, path, nil, local); err != nil {
			return "", err
		}
	}
	value, err := decodeAny(environment)
	if err != nil {
		return "", err
	}
	project, ok := field(value, "tracer_session_name").(string)
	if !ok {
		return "", errors.New("Agent environment response has no tracer_session_name")
	}
	return project, nil
}
