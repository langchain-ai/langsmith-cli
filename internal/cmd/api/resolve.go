package api

import (
	"fmt"
	"os"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/spf13/cobra"
)

// resolveAPIKey resolves the API key from cobra persistent flags → env → empty.
func resolveAPIKey(cmd *cobra.Command) string {
	if v, _ := cmd.Flags().GetString("api-key"); v != "" {
		return v
	}
	return os.Getenv("LANGSMITH_API_KEY")
}

// resolveAPIURL resolves the API URL from cobra persistent flags → env → default.
func resolveAPIURL(cmd *cobra.Command) string {
	if v, _ := cmd.Flags().GetString("api-url"); v != "" {
		return client.NormalizeURL(v)
	}
	if v := os.Getenv("LANGSMITH_ENDPOINT"); v != "" {
		return client.NormalizeURL(v)
	}
	return "https://api.smith.langchain.com"
}

// resolveFormat resolves the output format from cobra persistent flags.
func resolveFormat(cmd *cobra.Command) string {
	v, _ := cmd.Flags().GetString("format")
	if v == "" {
		return "json"
	}
	return v
}

// mustClient creates a client or exits with an error.
func mustClient(cmd *cobra.Command) *client.Client {
	apiKey := resolveAPIKey(cmd)
	if apiKey == "" {
		exitError("LANGSMITH_API_KEY not set")
	}
	return client.New(apiKey, resolveAPIURL(cmd))
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

// exitError prints a JSON error to stderr and exits.
func exitError(msg string) {
	fmt.Fprintf(os.Stderr, `{"error": %q}`+"\n", msg)
	os.Exit(1)
}

// exitErrorf prints a formatted JSON error to stderr and exits.
func exitErrorf(format string, args ...any) {
	exitError(fmt.Sprintf(format, args...))
}
