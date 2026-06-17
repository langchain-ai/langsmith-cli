package api

import (
	"strings"
)

// resolveEndpoint resolves an endpoint argument to a full URL.
//   - Full URL (http:// or https://) → returned as-is.
//   - Absolute path (starts with /) → baseURL + path.
//   - Shorthand (e.g. "sessions") → resolved against spec when available,
//     falling back to baseURL + /api/v1/ + path. spec may be nil.
func resolveEndpoint(baseURL, path string, spec *OpenAPISpec) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if strings.HasPrefix(path, "/") {
		return baseURL + path
	}
	rel, query, hasQuery := strings.Cut(path, "?")
	resolved := ""
	if spec != nil {
		resolved = spec.resolveShorthand(rel)
	}
	if resolved == "" {
		resolved = "/api/v1/" + rel
	}
	if hasQuery {
		resolved += "?" + query
	}
	return baseURL + resolved
}

// isHTTPMethod returns true if s is an uppercase HTTP method name.
func isHTTPMethod(s string) bool {
	switch s {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return true
	}
	return false
}
