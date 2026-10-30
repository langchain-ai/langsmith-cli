package hostapi

// LangSmith platform API endpoints for sandbox image registries.
//
// Private `dockerImage` pulls need a Host registry that stores credentials.
// MDA creates or updates a deployment-owned registry from the secret-safe
// declaration, then passes its id on snapshot create.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
)

const registriesPath = "/api/v2/sandboxes/registries"

// RegistrySummary is a sandbox image registry as returned by get/list.
type RegistrySummary struct {
	// ID is the registry UUID passed as `registry_id` on snapshot create.
	ID string `json:"id"`
	// Name is the author-facing name from `defineSandbox({ registry })`.
	Name string `json:"name"`
	// URL is the registry host (e.g. `ghcr.io`).
	URL string `json:"url"`
}

// UnmarshalJSON requires the id.
func (r *RegistrySummary) UnmarshalJSON(data []byte) error {
	if err := requireFields(data, "id"); err != nil {
		return err
	}
	type plain RegistrySummary
	return json.Unmarshal(data, (*plain)(r))
}

// FindRegistryByName looks up a registry by name, returning nil when it is
// already gone.
func (c *Client) FindRegistryByName(ctx context.Context, name string) (*RegistrySummary, error) {
	encoded := urlencodingPath(name)
	raw, found, err := c.sdkRequestOptional(ctx, http.MethodGet, registriesPath+"/"+encoded,
		func(ctx context.Context, opts ...option.RequestOption) error {
			_, err := c.ls.SDK.Sandboxes.Registries.Get(ctx, encoded, opts...)
			return err
		})
	if err != nil || !found {
		return nil, err
	}
	registry, err := decodeRegistry(raw)
	if err != nil {
		return nil, err
	}
	return &registry, nil
}

// ReconcileRegistry creates or updates a deployment-owned registry and returns
// it.
func (c *Client) ReconcileRegistry(ctx context.Context, name, url, username, password string) (RegistrySummary, error) {
	encoded := urlencodingPath(name)
	existing, err := c.FindRegistryByName(ctx, name)
	if err != nil {
		return RegistrySummary{}, err
	}
	var raw json.RawMessage
	if existing != nil {
		raw, err = c.sdkRequest(ctx, http.MethodPatch, registriesPath+"/"+encoded, requestOptions{},
			func(ctx context.Context, opts ...option.RequestOption) error {
				_, err := c.ls.SDK.Sandboxes.Registries.Update(ctx, encoded, langsmith.SandboxRegistryUpdateParams{
					Name:     langsmith.F(name),
					URL:      langsmith.F(url),
					Username: langsmith.F(username),
					Password: langsmith.F(password),
				}, opts...)
				return err
			})
		if err != nil {
			return RegistrySummary{}, fmt.Errorf("failed to update sandbox registry '%s': %w", name, err)
		}
	} else {
		raw, err = c.sdkRequest(ctx, http.MethodPost, registriesPath, requestOptions{},
			func(ctx context.Context, opts ...option.RequestOption) error {
				_, err := c.ls.SDK.Sandboxes.Registries.New(ctx, langsmith.SandboxRegistryNewParams{
					Name:     langsmith.F(name),
					URL:      langsmith.F(url),
					Username: langsmith.F(username),
					Password: langsmith.F(password),
				}, opts...)
				return err
			})
		if err != nil {
			return RegistrySummary{}, fmt.Errorf("failed to create sandbox registry '%s': %w", name, err)
		}
	}
	registry, err := decodeRegistry(raw)
	if err != nil {
		return RegistrySummary{}, err
	}
	if strings.TrimSpace(registry.ID) == "" {
		return RegistrySummary{}, fmt.Errorf("sandbox registry '%s' response is missing id", name)
	}
	return registry, nil
}

func decodeRegistry(raw json.RawMessage) (RegistrySummary, error) {
	var registry RegistrySummary
	if err := json.Unmarshal(nullIfEmpty(raw), &registry); err != nil {
		return RegistrySummary{}, fmt.Errorf("failed to decode sandbox registry response: %w", err)
	}
	return registry, nil
}
