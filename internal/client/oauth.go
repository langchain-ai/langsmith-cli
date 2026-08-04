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

// OAuthMetadata holds the OAuth authorization server endpoints resolved via
// RFC 8414 discovery. Endpoints are the absolute URLs advertised by the server,
// so callers never construct OAuth paths themselves — this is what lets the same
// code path serve SaaS (AS at <origin>/oauth/*) and self-hosted (AS at
// <origin>/api/oauth/*) without branching.
type OAuthMetadata struct {
	Issuer                      string
	DeviceAuthorizationEndpoint string
	TokenEndpoint               string
	RegistrationEndpoint        string
	// Resource is the RFC 8707 resource indicator to request (the API the token
	// is minted for). It equals the authorization server's issuer.
	Resource string
}

type oauthServerMetadata struct {
	Issuer                      string   `json:"issuer"`
	DeviceAuthorizationEndpoint string   `json:"device_authorization_endpoint"`
	TokenEndpoint               string   `json:"token_endpoint"`
	RegistrationEndpoint        string   `json:"registration_endpoint"`
	ProtectedResourcesSupported []string `json:"protected_resources_supported"`
}

// oauthDeploymentRoot reduces apiURL to the deployment root by stripping a
// trailing "/api/v1" or "/api" mount segment, mirroring how the SDK derives its
// REST base URL. A basePath prefix is preserved.
func oauthDeploymentRoot(apiURL string) string {
	u := strings.TrimRight(apiURL, "/")
	u = strings.TrimSuffix(u, "/api/v1")
	u = strings.TrimSuffix(u, "/api")
	return u
}

// oauthDiscoveryCandidates returns the base URLs to probe for the RFC 8414
// metadata document, most specific first. The user-provided path (minus a
// trailing /api/v1) is tried first, so basePath deployments and an explicit
// "<origin>/api" resolve directly; the "<origin>/api" self-hosted default and
// the bare "<origin>" (SaaS) follow so a user who passes only the origin still
// resolves.
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

// DiscoverOAuth resolves the OAuth authorization server endpoints for apiURL via
// RFC 8414 discovery. It probes candidate .well-known locations and returns the
// first that yields a valid metadata document (one that parses as JSON and
// advertises token and device-authorization endpoints). It returns an error if
// no candidate yields valid metadata, so callers can fall back to legacy path
// construction against older backends that do not serve the document.
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

// ResolveOAuth returns the OAuth endpoints for apiURL. It prefers RFC 8414
// discovery and falls back to legacy path construction (<base>/oauth/*) when the
// backend serves no metadata document, where <base> is apiURL with a trailing
// /api/v1 stripped. The fallback reproduces the pre-discovery behavior, so an
// older self-hosted backend keeps working when apiURL points at the API root
// (e.g. "<origin>/api"). It never returns nil.
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
		// A self-hosted SPA catch-all answers unknown paths with an HTML 200;
		// reject anything that is not a valid metadata JSON document.
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
