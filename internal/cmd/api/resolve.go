package api

import (
	"strings"
)

// resolveEndpoint resolves an endpoint argument to a full URL.
//   - Full URL (http:// or https://) → returned as-is.
//   - Absolute path (starts with /) → baseURL + path.
//   - Shorthand (e.g. "sessions") → baseURL + /api/v1/ + path.
func resolveEndpoint(baseURL, path string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if strings.HasPrefix(path, "/") {
		return baseURL + path
	}
	return baseURL + "/api/v1/" + path
}

// isHTTPMethod returns true if s is an uppercase HTTP method name.
func isHTTPMethod(s string) bool {
	switch s {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return true
	}
	return false
}
