package api

import (
	"os"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/spf13/cobra"
)

// resolveAPIKey reads the API key from cobra's inherited persistent flags → env.
// This is not a duplicate of cmd.GetAPIKey — that reads from package-level flag
// vars, while this reads from cobra's flag tree (needed because the api sub-package
// cannot import cmd due to circular dependency).
func resolveAPIKey(cmd *cobra.Command) string {
	if v, _ := cmd.Flags().GetString("api-key"); v != "" {
		return v
	}
	return os.Getenv("LANGSMITH_API_KEY")
}

// resolveAPIURL reads the API URL from cobra's inherited persistent flags → env → default.
func resolveAPIURL(cmd *cobra.Command) string {
	if v, _ := cmd.Flags().GetString("api-url"); v != "" {
		return client.NormalizeURL(v)
	}
	if v := os.Getenv("LANGSMITH_ENDPOINT"); v != "" {
		return client.NormalizeURL(v)
	}
	return "https://api.smith.langchain.com"
}

// resolveFormat reads the output format from cobra's inherited persistent flags.
func resolveFormat(cmd *cobra.Command) string {
	v, _ := cmd.Flags().GetString("format")
	if v == "" {
		return "json"
	}
	return v
}

// resolveEndpoint resolves an endpoint argument to a full URL.
//   - Full URL (http:// or https://) → returned as-is.
//   - Absolute path (starts with /) → baseURL + path.
//   - Shorthand (e.g. "sessions") → baseURL + /api/v1/ + path.
func resolveEndpoint(baseURL, path string) string {
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
