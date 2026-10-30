package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/langchain-ai/langsmith-go/option"
)

// The headers that carry a LangSmith credential or workspace.
const (
	apiKeyHeader        = "X-Api-Key"
	authorizationHeader = "Authorization"
	userIDHeader        = "X-User-Id"
	tenantHeader        = "X-Tenant-Id"
)

var credentialHeaders = []string{apiKeyHeader, authorizationHeader, userIDHeader}

// withoutAmbientCredential drops the credential the SDK's defaults attached
// (the process LANGSMITH_API_KEY, the current profile's API key), so the
// options that follow decide alone.
func withoutAmbientCredential() []option.RequestOption {
	opts := make([]option.RequestOption, 0, len(credentialHeaders))
	for _, name := range credentialHeaders {
		opts = append(opts, option.WithHeaderDel(name))
	}
	return opts
}

// stripCredentials is the middleware of an isolated client without a
// credential. The current profile's OAuth token is attached by middleware of
// the SDK's defaults, which runs before this one.
func stripCredentials(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
	for _, name := range credentialHeaders {
		req.Header.Del(name)
	}
	return next(req)
}

// Origin is u's scheme and authority, lowercased and without the scheme's
// default port: two URLs with the same origin reach the same server.
func Origin(u *url.URL) string {
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
		port = ""
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port != "" {
		host += ":" + port
	}
	return scheme + "://" + host
}

// SameOrigin reports whether a and b are absolute http(s) URLs with the same
// [Origin].
func SameOrigin(a, b string) bool {
	ua, okA := parseHTTPURL(a)
	ub, okB := parseHTTPURL(b)
	return okA && okB && Origin(ua) == Origin(ub)
}

func parseHTTPURL(raw string) (*url.URL, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, false
	}
	return u, true
}

// CredentialTransportAllowed reports whether a credential may be sent to u:
// over `https`, or over plain `http` only to this machine (a local LangSmith
// or a test server), where it never crosses the network.
func CredentialTransportAllowed(u *url.URL) bool {
	switch u.Scheme {
	case "https":
		return true
	case "http":
		host := u.Hostname()
		if host == "localhost" {
			return true
		}
		ip := net.ParseIP(host)
		return ip != nil && ip.IsLoopback()
	default:
		return false
	}
}

// CheckCredentialTransport fails unless credentials may be sent to rawURL, an
// absolute http(s) URL ([CredentialTransportAllowed]).
func CheckCredentialTransport(rawURL string) error {
	u, ok := parseHTTPURL(rawURL)
	if !ok {
		return fmt.Errorf("request URL %q must be an absolute http(s) URL", rawURL)
	}
	if !CredentialTransportAllowed(u) {
		return fmt.Errorf("refusing to send LangSmith credentials over plain HTTP to %s; use https", Origin(u))
	}
	return nil
}

// maxRedirects is how many redirects a request follows, as net/http's
// default.
const maxRedirects = 10

// sameOriginRedirects follows a redirect only within the origin of the
// request: the credential, workspace and custom headers go with the request,
// and net/http strips only Authorization when it leaves the origin.
var sameOriginRedirects = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("stopped after %d redirects", maxRedirects)
		}
		if from, to := Origin(via[0].URL), Origin(req.URL); from != to {
			return fmt.Errorf("refusing to follow a redirect from %s to another origin, %s", from, to)
		}
		return nil
	},
}

// customHeaderNames are the names of the headers the SDK adds from
// LANGCHAIN_CUSTOM_HEADERS, parsed as the SDK does.
func customHeaderNames() []string {
	value, ok := os.LookupEnv("LANGCHAIN_CUSTOM_HEADERS")
	if !ok {
		return nil
	}
	var names []string
	for _, line := range strings.Split(value, "\n") {
		if name, _, found := strings.Cut(line, ":"); found {
			names = append(names, strings.TrimSpace(name))
		}
	}
	return names
}

// Response is the status, headers and body of the response to
// [Client.DoAbsolute].
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// errNoCredential is DoAbsolute's answer when the client has no credential
// of its own.
var errNoCredential = errors.New("the client has no LangSmith credential to send")

// DoAbsolute sends one request to absoluteURL, on any host (a LangSmith
// service other than the API URL, a deployed Agent Server, a sandbox
// dataplane), with this client's credentials. It goes through the SDK like
// every other call: the SDK attaches the API key, or the profile's OAuth token
// refreshed under the shared lock, and the workspace header. The body is sent
// as JSON. Like RawDo it makes one attempt and returns a 4xx or 5xx as a
// Response, not an error. opts apply after the defaults (a 30-second timeout),
// e.g. option.WithRequestTimeout, or option.WithHTTPClient with a client that
// does not follow redirects.
//
// The credential only leaves on the client's terms: the client must carry
// one itself (the SDK would otherwise pick one up from the environment or
// the current profile), absoluteURL must be `https` (or `http` to this
// machine), and a redirect to another origin is an error. The headers of
// LANGCHAIN_CUSTOM_HEADERS go only to the origin of the API URL.
func (c *Client) DoAbsolute(ctx context.Context, method, absoluteURL string, body io.Reader, headers http.Header, opts ...option.RequestOption) (*Response, error) {
	u, ok := parseHTTPURL(absoluteURL)
	if !ok {
		return nil, fmt.Errorf("request URL %q must be an absolute http(s) URL", absoluteURL)
	}
	origin := Origin(u)
	if c.apiKey == "" && c.oauthAccessToken == "" && c.profileName == "" {
		return nil, fmt.Errorf("refusing to send a request to %s: %w", origin, errNoCredential)
	}
	if err := CheckCredentialTransport(absoluteURL); err != nil {
		return nil, err
	}
	reqOpts := []option.RequestOption{option.WithHTTPClient(sameOriginRedirects)}
	if !SameOrigin(u.String(), c.apiURL) {
		for _, name := range customHeaderNames() {
			if headers.Get(name) == "" && !isCredentialHeader(name) {
				reqOpts = append(reqOpts, option.WithHeaderDel(name))
			}
		}
	}
	reqOpts = append(reqOpts, opts...)
	resp, err := c.doAuthenticated(ctx, method, origin+u.EscapedPath(), u.String(), body, headers, false, reqOpts...)
	if err != nil {
		return nil, err
	}
	return &Response{StatusCode: resp.statusCode, Header: resp.headers, Body: resp.body}, nil
}

// isCredentialHeader reports whether name is one of the headers the client's
// own options set.
func isCredentialHeader(name string) bool {
	switch http.CanonicalHeaderKey(name) {
	case apiKeyHeader, authorizationHeader, userIDHeader, tenantHeader:
		return true
	}
	return false
}
