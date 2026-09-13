package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	wellKnownOAuthPath    = "/.well-known/oauth-authorization-server"
	oauthDiscoveryMaxBody = 1 << 20 // 1 MiB cap on the metadata document
)

// errNoOAuthMetadata means the deployment does not serve a usable discovery
// document. It is distinct from a transport or 5xx failure, which says nothing
// about whether the document exists and so must not trigger the legacy
// fallback.
var (
	errNoOAuthMetadata      = errors.New("no authorization server metadata")
	errInvalidOAuthMetadata = errors.New("invalid authorization server metadata")
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
	Issuer                      string `json:"issuer"`
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
	TokenEndpoint               string `json:"token_endpoint"`
	RegistrationEndpoint        string `json:"registration_endpoint"`
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
// RFC 8414 metadata document. A transient failure against any candidate is
// reported in preference to errNoOAuthMetadata so callers do not mistake an
// outage for a legacy backend.
func DiscoverOAuth(ctx context.Context, apiURL string) (*OAuthMetadata, error) {
	var discoveryErr error
	for _, base := range oauthDiscoveryCandidates(apiURL) {
		meta, err := fetchOAuthMetadata(ctx, base+wellKnownOAuthPath, base)
		switch {
		case err == nil:
			return meta, nil
		case errors.Is(err, errNoOAuthMetadata):
		case discoveryErr == nil:
			discoveryErr = err
		}
	}
	if discoveryErr != nil {
		return nil, fmt.Errorf("discovering OAuth authorization server: %w", discoveryErr)
	}
	return nil, fmt.Errorf("discovering OAuth authorization server for %q: %w", apiURL, errNoOAuthMetadata)
}

// ResolveOAuth prefers discovery, falling back to <base>/oauth/* so backends
// that serve no metadata document keep working. Transient discovery failures
// are returned rather than masked, because the fallback would post credentials
// to an endpoint the deployment may not serve.
func ResolveOAuth(ctx context.Context, apiURL string) (*OAuthMetadata, error) {
	meta, err := DiscoverOAuth(ctx, apiURL)
	if err == nil {
		return meta, nil
	}
	if !errors.Is(err, errNoOAuthMetadata) {
		return nil, err
	}
	base := strings.TrimRight(NormalizeURL(apiURL), "/")
	return &OAuthMetadata{
		Issuer:                      base,
		DeviceAuthorizationEndpoint: base + "/oauth/device/code",
		TokenEndpoint:               base + "/oauth/token",
		RegistrationEndpoint:        base + "/oauth/register",
		Resource:                    base,
	}, nil
}

func fetchOAuthMetadata(ctx context.Context, metadataURL, base string) (*OAuthMetadata, error) {
	return fetchOAuthMetadataWithDelegation(ctx, metadataURL, base, true)
}

func fetchOAuthMetadataWithDelegation(ctx context.Context, metadataURL, base string, allowDelegation bool) (*OAuthMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", metadataURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if isTransientStatus(resp.StatusCode) {
			return nil, fmt.Errorf("%s: HTTP %d", metadataURL, resp.StatusCode)
		}
		return nil, fmt.Errorf("%s: HTTP %d: %w", metadataURL, resp.StatusCode, errNoOAuthMetadata)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, oauthDiscoveryMaxBody))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", metadataURL, err)
	}

	var doc oauthServerMetadata
	if err := json.Unmarshal(body, &doc); err != nil {
		// The self-hosted SPA catch-all answers unknown paths with an HTML 200.
		return nil, fmt.Errorf("%s: not authorization server metadata: %w", metadataURL, errNoOAuthMetadata)
	}
	if doc.TokenEndpoint == "" || doc.DeviceAuthorizationEndpoint == "" {
		return nil, fmt.Errorf("%s: metadata missing required endpoints: %w", metadataURL, errNoOAuthMetadata)
	}
	if strings.TrimRight(doc.Issuer, "/") != strings.TrimRight(base, "/") {
		if !allowDelegation {
			return nil, fmt.Errorf("%s: issuer %q does not match %q: %w", metadataURL, doc.Issuer, base, errInvalidOAuthMetadata)
		}
		if err := validateDelegatedOAuthMetadata(&doc); err != nil {
			return nil, fmt.Errorf("%s: %v: %w", metadataURL, err, errInvalidOAuthMetadata)
		}
		issuer := strings.TrimRight(doc.Issuer, "/")
		canonical, err := fetchOAuthMetadataWithDelegation(ctx, issuer+wellKnownOAuthPath, issuer, false)
		if err != nil {
			return nil, fmt.Errorf("validating delegated OAuth issuer %q: %v: %w", issuer, err, errInvalidOAuthMetadata)
		}
		if canonical.DeviceAuthorizationEndpoint != doc.DeviceAuthorizationEndpoint ||
			canonical.TokenEndpoint != doc.TokenEndpoint ||
			canonical.RegistrationEndpoint != doc.RegistrationEndpoint {
			return nil, fmt.Errorf("%s: delegated metadata does not match issuer metadata: %w", metadataURL, errInvalidOAuthMetadata)
		}
		return canonical, nil
	}
	if err := validateOAuthMetadata(&doc, base); err != nil {
		return nil, fmt.Errorf("%s: %w: %w", metadataURL, err, errNoOAuthMetadata)
	}

	return &OAuthMetadata{
		Issuer:                      doc.Issuer,
		DeviceAuthorizationEndpoint: doc.DeviceAuthorizationEndpoint,
		TokenEndpoint:               doc.TokenEndpoint,
		RegistrationEndpoint:        doc.RegistrationEndpoint,
		Resource:                    doc.Issuer,
	}, nil
}

func validateDelegatedOAuthMetadata(doc *oauthServerMetadata) error {
	issuer, err := validateOAuthEndpointOrigins(doc)
	if err != nil {
		return err
	}
	if issuer.Scheme == "https" {
		return nil
	}
	ip := net.ParseIP(issuer.Hostname())
	if issuer.Scheme == "http" && (issuer.Hostname() == "localhost" || ip != nil && ip.IsLoopback()) {
		return nil
	}
	return fmt.Errorf("delegated issuer %q must use HTTPS", doc.Issuer)
}

func isTransientStatus(code int) bool {
	return code >= 500 || code == http.StatusRequestTimeout || code == http.StatusTooManyRequests
}

// validateOAuthMetadata enforces that the document describes the deployment we
// probed: RFC 8414 requires the issuer to match the URL the well-known path was
// built from, and every endpoint must share the issuer's origin. The CLI posts
// refresh tokens and device codes to these URLs, so an unvalidated document
// could redirect long-lived credentials to another host.
func validateOAuthMetadata(doc *oauthServerMetadata, base string) error {
	if strings.TrimRight(doc.Issuer, "/") != strings.TrimRight(base, "/") {
		return fmt.Errorf("issuer %q does not match %q", doc.Issuer, base)
	}
	_, err := validateOAuthEndpointOrigins(doc)
	return err
}

func validateOAuthEndpointOrigins(doc *oauthServerMetadata) (*url.URL, error) {
	issuer, err := url.Parse(doc.Issuer)
	if err != nil || issuer.Scheme == "" || issuer.Host == "" || issuer.User != nil || issuer.Fragment != "" || issuer.RawQuery != "" {
		return nil, fmt.Errorf("unparseable issuer %q", doc.Issuer)
	}
	for _, ep := range []string{doc.DeviceAuthorizationEndpoint, doc.TokenEndpoint, doc.RegistrationEndpoint} {
		if ep == "" {
			continue
		}
		u, err := url.Parse(ep)
		if err != nil {
			return nil, fmt.Errorf("unparseable endpoint %q", ep)
		}
		if u.Scheme != issuer.Scheme || u.Host != issuer.Host {
			return nil, fmt.Errorf("endpoint %q is not on issuer origin %s://%s", ep, issuer.Scheme, issuer.Host)
		}
	}
	return issuer, nil
}
