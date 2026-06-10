package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
	"github.com/spf13/cobra"
)

// Env-var contract consumed by the LangSmith tracing plugins for coding agents.
// Claude Code uses the CC_LANGSMITH_* namespace exclusively; a custom endpoint
// is expressed via CC_LANGSMITH_RUNS_ENDPOINTS (a JSON replica array) rather
// than a plain endpoint var. Keep these in sync with langsmith-claude-code-plugins.
const (
	envTraceToLangSmith         = "TRACE_TO_LANGSMITH"
	envCCLangSmithAPIKey        = "CC_LANGSMITH_API_KEY"
	envCCLangSmithProject       = "CC_LANGSMITH_PROJECT"
	envCCLangSmithRunsEndpoints = "CC_LANGSMITH_RUNS_ENDPOINTS"
)

func newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Configure coding agents (Claude Code, Codex) to trace to LangSmith",
		Long: `Configure coding agents to send full-content traces to LangSmith.

These commands write the LangSmith tracing config into the agent's local config
files, so every future session traces to a LangSmith project automatically. They
do not launch Claude Code or Codex: Claude Code installs the declared plugin on
its next launch; Codex needs one manual 'codex plugin marketplace add' (printed
after setup) to fetch the plugin code.

Requires a LangSmith API key, passed as an argument (langsmith setup claude
<api-key>) or resolved from --api-key, LANGSMITH_API_KEY, or a saved profile.
The key is written to the agent config file at owner-only (0600) permissions.
The API URL defaults to https://api.smith.langchain.com; pass --api-url or set
LANGSMITH_ENDPOINT for self-hosted instances. Without --project, no project is
written and the plugin's own default applies.`,
	}
	cmd.AddCommand(newSetupClaudeCmd())
	cmd.AddCommand(newSetupCodexCmd())
	cmd.AddCommand(newSetupAllCmd())
	return cmd
}

func newSetupAllCmd() *cobra.Command {
	var (
		project string
		scope   string
	)
	cmd := &cobra.Command{
		Use:   "all [api-key]",
		Short: "Configure both Claude Code and Codex to trace to LangSmith",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiKey := positionalKey(args)
			cErr := runSetupClaude(cmd, apiKey, project, scope)
			if cErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "claude setup failed: %v\n", cErr)
			}
			xErr := runSetupCodex(cmd, apiKey, project, scope)
			if xErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "codex setup failed: %v\n", xErr)
			}
			if cErr != nil && xErr != nil {
				return errors.New("both claude and codex setup failed")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "LangSmith project name (default: $LANGSMITH_PROJECT, else each plugin's own default)")
	cmd.Flags().StringVar(&scope, "scope", "user", "Config scope: user or project")
	return cmd
}

// positionalKey extracts the optional [api-key] argument.
func positionalKey(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return strings.TrimSpace(args[0])
}

// setupClientOptions resolves credentials and requires an API key, since the
// tracing plugins authenticate with a workspace-scoped key (OAuth won't work).
// A non-empty positionalKey (the optional [api-key] argument) behaves exactly
// like --api-key, taking precedence over the environment and saved profiles.
func setupClientOptions(positionalKey string) (client.Options, error) {
	if positionalKey != "" {
		if flagAPIKey != "" && flagAPIKey != positionalKey {
			return client.Options{}, errors.New("API key passed both as an argument and via --api-key; pass it once")
		}
		flagAPIKey = positionalKey
	}
	opts, err := resolveClientOptions(true)
	if err != nil {
		return opts, err
	}
	if opts.APIKey == "" {
		return opts, errors.New("tracing setup requires a LangSmith API key; pass one as an argument (langsmith setup claude <api-key>), set LANGSMITH_API_KEY, or run 'langsmith profile create' with an API-key profile")
	}
	return opts, nil
}

// envTraceProject resolves the LangSmith project name from the environment.
// Empty means no project is written, deferring to the plugin's own default.
func envTraceProject() string {
	for _, key := range []string{"LANGSMITH_AGENT_PROJECT", "LANGSMITH_PROJECT"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

func isDefaultEndpoint(apiURL string) bool {
	return apiURL == "" || apiURL == lsconfig.DefaultAPIURL
}

func scopeOrDefault(scope string) string {
	if scope == "" {
		return "user"
	}
	return scope
}

// claudeConfigDir returns ~/.claude (or $CLAUDE_CONFIG_DIR when set).
func claudeConfigDir() (string, error) {
	if d := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR")); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".claude"), nil
}

// codexHome returns ~/.codex (or $CODEX_HOME when set).
func codexHome() (string, error) {
	if d := strings.TrimSpace(os.Getenv("CODEX_HOME")); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".codex"), nil
}

// mergeJSONFile reads path as a JSON object (empty when absent), applies mutate,
// and writes it back indented with owner-only (0600) permissions. Unknown keys
// in the existing file are preserved.
func mergeJSONFile(path string, mutate func(map[string]any) error) error {
	doc := map[string]any{}
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if len(strings.TrimSpace(string(data))) > 0 {
			if err := json.Unmarshal(data, &doc); err != nil {
				return fmt.Errorf("parsing %s: %w", path, err)
			}
		}
	case errors.Is(err, os.ErrNotExist):
		// Start from an empty object.
	default:
		return fmt.Errorf("reading %s: %w", path, err)
	}

	if err := mutate(doc); err != nil {
		return err
	}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// jsonObject returns doc[key] as a map, creating it when missing or not an object.
func jsonObject(doc map[string]any, key string) map[string]any {
	if existing, ok := doc[key].(map[string]any); ok {
		return existing
	}
	m := map[string]any{}
	doc[key] = m
	return m
}
