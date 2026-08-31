package api

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

func blockRawTracingProjectDelete(apiURL, method, path string) error {
	if !isRawTracingProjectDelete(apiURL, method, path) {
		return nil
	}
	return errors.New("raw API deletion of tracing projects is blocked; use `langsmith project delete --project-id PROJECT_ID` instead")
}

func isRawTracingProjectDelete(apiURL, method, path string) bool {
	if !strings.EqualFold(method, http.MethodDelete) {
		return false
	}

	fullURL := resolveEndpoint(apiURL, path)
	if !isSameHost(fullURL, apiURL) {
		return false
	}
	u, err := url.Parse(fullURL)
	if err != nil {
		return false
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	n := len(parts)
	return (n >= 3 && parts[n-3] == "api" && parts[n-2] == "v1" && parts[n-1] == "sessions") ||
		(n >= 4 && parts[n-4] == "api" && parts[n-3] == "v1" && parts[n-2] == "sessions") ||
		(n == 1 && parts[0] == "sessions") ||
		(n == 2 && parts[0] == "sessions")
}
