package cmd

import (
	"os"
	"strings"

	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
)

// readNextStep pins context without copying credentials into shell history.
// These are read commands, never automatic retries of a potentially applied write.
func readNextStep(args ...string) string {
	parts := []string{"langsmith"}
	if path := os.Getenv("LANGSMITH_CONFIG_FILE"); path != "" {
		parts = append([]string{"env", "LANGSMITH_CONFIG_FILE=" + path}, parts...)
	}
	if cfg, err := lsconfig.Load(); err == nil {
		if name, _, ok := cfg.ResolveProfile(flagProfile, strings.TrimSpace(os.Getenv("LANGSMITH_PROFILE"))); ok {
			parts = append(parts, "--profile", name)
		}
	}
	if opts, err := resolveClientOptions(false); err == nil {
		if opts.WorkspaceID != "" {
			parts = append(parts, "--workspace", opts.WorkspaceID)
		}
		if opts.APIURL != "" && opts.APIURL != lsconfig.DefaultAPIURL {
			parts = append(parts, "--api-url", opts.APIURL)
		}
	}
	parts = append(parts, "--format", "json")
	parts = append(parts, args...)
	for i, part := range parts {
		if part == "" || strings.IndexFunc(part, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("_./:-", r))
		}) >= 0 {
			parts[i] = shellQuote(part)
		}
	}
	return strings.Join(parts, " ")
}

// Add guidance only to successful empty reads. Callers retain their original
// resource fields and pagination; an empty page does not imply an empty account.
func emptyResultGuidance(result map[string]any, count int, message string, next ...string) map[string]any {
	if count == 0 {
		result["message"] = message
		result["next_steps"] = next
	}
	return result
}
