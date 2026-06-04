package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
	"github.com/spf13/cobra"
)

// Env-var contract consumed by the LangSmith tracing plugins for coding agents.
// The Claude Code plugin reads the CC_LANGSMITH_* names; both the Claude Code
// and Codex plugins fall back to the generic LANGSMITH_* names. Keep these in
// sync with the langsmith-claude-code-plugins / langsmith-codex-plugins repos.
const (
	envTraceToLangSmith   = "TRACE_TO_LANGSMITH"
	envCCLangSmithAPIKey  = "CC_LANGSMITH_API_KEY"
	envCCLangSmithProject = "CC_LANGSMITH_PROJECT"
	envLangSmithAPIKey    = "LANGSMITH_API_KEY"
	envLangSmithEndpoint  = "LANGSMITH_ENDPOINT"
	envLangSmithProject   = "LANGSMITH_PROJECT"
)

// runSetupCommand runs an external agent CLI (claude/codex) for plugin install.
// Indirected so tests can capture invocations without spawning real binaries.
var runSetupCommand = runSetupCommandDefault

func runSetupCommandDefault(ctx context.Context, name string, args ...string) error {
	c := exec.CommandContext(ctx, name, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Configure coding agents (Claude Code, Codex) to trace to LangSmith",
		Long: `Configure coding agents to send full-content traces to LangSmith.

These commands install the LangSmith tracing plugin for the agent and write the
credentials it needs to your local agent config, so every future session traces
to a LangSmith project automatically.

Requires a LangSmith API key (run 'langsmith profile create', set
LANGSMITH_API_KEY, or pass --api-key). The key is written to the agent config
file at owner-only (0600) permissions.`,
	}
	cmd.AddCommand(newSetupClaudeCmd())
	cmd.AddCommand(newSetupCodexCmd())
	cmd.AddCommand(newSetupAllCmd())
	return cmd
}

func newSetupAllCmd() *cobra.Command {
	var (
		project   string
		scope     string
		noInstall bool
	)
	cmd := &cobra.Command{
		Use:   "all",
		Short: "Configure both Claude Code and Codex to trace to LangSmith",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cErr := runSetupClaude(cmd, project, scope, noInstall)
			if cErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "claude setup failed: %v\n", cErr)
			}
			xErr := runSetupCodex(cmd, project, scope, noInstall)
			if xErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "codex setup failed: %v\n", xErr)
			}
			if cErr != nil && xErr != nil {
				return errors.New("both claude and codex setup failed")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "LangSmith project name (default: $LANGSMITH_PROJECT, else per-agent default)")
	cmd.Flags().StringVar(&scope, "scope", "user", "Config scope: user or project")
	cmd.Flags().BoolVar(&noInstall, "no-install", false, "Only write config; skip running the agent's plugin install")
	return cmd
}

// setupClientOptions resolves credentials and requires an API key, since the
// tracing plugins authenticate with a workspace-scoped key (OAuth won't work).
func setupClientOptions() (client.Options, error) {
	opts, err := resolveClientOptions(true)
	if err != nil {
		return opts, err
	}
	if opts.APIKey == "" {
		return opts, errors.New("tracing setup requires a LangSmith API key; run 'langsmith profile create' with an API-key profile, set LANGSMITH_API_KEY, or pass --api-key")
	}
	return opts, nil
}

// defaultTraceProject resolves the LangSmith project name from the environment,
// falling back to the per-agent default.
func defaultTraceProject(fallback string) string {
	for _, key := range []string{"LANGSMITH_AGENT_PROJECT", envLangSmithProject} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return fallback
}

func isDefaultEndpoint(apiURL string) bool {
	return apiURL == "" || apiURL == lsconfig.DefaultAPIURL
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
