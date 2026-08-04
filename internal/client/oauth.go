package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	wellKnownOAuthPath    = "/.well-known/oauth-authorization-server"
	oauthDiscoveryMaxBody = 1 << 20 // 1 MiB cap on the metadata document
)

// OAuthMetadata holds absolute authorization server endpoints. The AS lives at
// <origin>/oauth on SaaS and <origin>/api/oauth on self-hosted, so callers use
// these rather than building paths themselves.
type OAuthMetadata struct {
	Issuer                      string
	DeviceAuthorizationEndpoint string
	TokenEndpoint               string
	RegistrationEndpoint        string
	// Resource is the RFC 8707 resource indicator; it equals the issuer.
	Resource string
}

type oauthServerMetadata struct {
	Issuer                      string   `json:"issuer"`
	DeviceAuthorizationEndpoint string   `json:"device_authorization_endpoint"`
	TokenEndpoint               string   `json:"token_endpoint"`
	RegistrationEndpoint        string   `json:"registration_endpoint"`
	ProtectedResourcesSupported []string `json:"protected_resources_supported"`
}

// oauthDeploymentRoot strips a trailing "/api/v1" or "/api" mount segment,
// preserving any basePath prefix.
func oauthDeploymentRoot(apiURL string) string {
	u := strings.TrimRight(apiURL, "/")
	u = strings.TrimSuffix(u, "/api/v1")
	u = strings.TrimSuffix(u, "/api")
	return u
}

// oauthDiscoveryCandidates returns metadata base URLs to probe, most specific
// first: what the user configured, then the self-hosted and SaaS mount points.
func oauthDiscoveryCandidates(apiURL string) []string {
	given := strings.TrimRight(NormalizeURL(apiURL), "/")
	origin := strings.TrimRight(oauthDeploymentRoot(apiURL), "/")

	ordered := []string{given, origin + "/api", origin}
	seen := make(map[string]bool, len(ordered))
	out := make([]string, 0, len(ordered))
	for _, c := range ordered {
		if c == "" || c == "/api" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}

// DiscoverOAuth returns the endpoints from the first candidate serving a valid
// RFC 8414 metadata document, or an error if none does.
func DiscoverOAuth(ctx context.Context, apiURL string) (*OAuthMetadata, error) {
	var lastErr error
	for _, base := range oauthDiscoveryCandidates(apiURL) {
		meta, err := fetchOAuthMetadata(ctx, base+wellKnownOAuthPath)
		if err != nil {
			lastErr = err
			continue
		}
		return meta, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no authorization server metadata found for %q", apiURL)
	}
	return nil, fmt.Errorf("discovering OAuth authorization server: %w", lastErr)
}

// ResolveOAuth prefers discovery, falling back to <base>/oauth/* so backends
// that serve no metadata document keep working. It never returns nil.
func ResolveOAuth(ctx context.Context, apiURL string) *OAuthMetadata {
	if meta, err := DiscoverOAuth(ctx, apiURL); err == nil {
		return meta
	}
	base := strings.TrimRight(NormalizeURL(apiURL), "/")
	return &OAuthMetadata{
		Issuer:                      base,
		DeviceAuthorizationEndpoint: base + "/oauth/device/code",
		TokenEndpoint:               base + "/oauth/token",
		RegistrationEndpoint:        base + "/oauth/register",
		Resource:                    base,
	}
}

func fetchOAuthMetadata(ctx context.Context, metadataURL string) (*OAuthMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: HTTP %d", metadataURL, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, oauthDiscoveryMaxBody))
	if err != nil {
		return nil, err
	}

	var doc oauthServerMetadata
	if err := json.Unmarshal(body, &doc); err != nil {
		// The self-hosted SPA catch-all answers unknown paths with an HTML 200.
		return nil, fmt.Errorf("%s: response is not authorization server metadata", metadataURL)
	}
	if doc.TokenEndpoint == "" || doc.DeviceAuthorizationEndpoint == "" {
		return nil, fmt.Errorf("%s: metadata missing required endpoints", metadataURL)
	}

	resource := doc.Issuer
	if resource == "" && len(doc.ProtectedResourcesSupported) > 0 {
		resource = doc.ProtectedResourcesSupported[0]
	}

	return &OAuthMetadata{
		Issuer:                      doc.Issuer,
		DeviceAuthorizationEndpoint: doc.DeviceAuthorizationEndpoint,
		TokenEndpoint:               doc.TokenEndpoint,
		RegistrationEndpoint:        doc.RegistrationEndpoint,
		Resource:                    resource,
	}, nil
}
