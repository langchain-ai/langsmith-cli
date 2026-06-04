package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/spf13/cobra"
)

// Claude Code LangSmith tracing plugin coordinates. The marketplace name must
// match how the plugin is referenced (langsmith-tracing@<marketplace>), so the
// enabledPlugins key and any 'claude plugin install' use the same name.
const (
	claudeMarketplaceName = "langsmith-claude-code-plugins"
	claudeMarketplaceRepo = "langchain-ai/langsmith-claude-code-plugins"
	claudeMarketplaceURL  = "https://github.com/langchain-ai/langsmith-claude-code-plugins.git"
	claudePluginName      = "langsmith-tracing"
)

func newSetupClaudeCmd() *cobra.Command {
	var (
		project   string
		scope     string
		noInstall bool
	)
	cmd := &cobra.Command{
		Use:   "claude",
		Short: "Configure Claude Code to trace to LangSmith",
		Long: `Configure Claude Code to send full-content traces to LangSmith.

Writes the LangSmith tracing marketplace, enables the plugin, and stores your
API key and project name in Claude Code's settings.json. Every future Claude
Code session then traces to LangSmith automatically.

  langsmith setup claude                       # write ~/.claude/settings.json
  langsmith setup claude --project my-agent    # trace to a named project
  langsmith setup claude --scope project       # write ./.claude/settings.local.json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSetupClaude(cmd, project, scope, noInstall)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "LangSmith project name (default: $LANGSMITH_PROJECT, else \"claude-code\")")
	cmd.Flags().StringVar(&scope, "scope", "user", "Config scope: user (~/.claude/settings.json) or project (./.claude/settings.local.json)")
	cmd.Flags().BoolVar(&noInstall, "no-install", false, "Only write config; skip the best-effort 'claude plugin marketplace add' catalog prefetch")
	return cmd
}

func runSetupClaude(cmd *cobra.Command, project, scope string, noInstall bool) error {
	opts, err := setupClientOptions()
	if err != nil {
		return err
	}
	if project == "" {
		project = defaultTraceProject("claude-code")
	}

	settingsPath, err := claudeSettingsPath(scope)
	if err != nil {
		return err
	}

	pluginRef := claudePluginName + "@" + claudeMarketplaceName

	if err := writeClaudeSettings(settingsPath, pluginRef, opts, project); err != nil {
		return err
	}

	// The settings file is the source of truth — Claude Code resolves the
	// declared marketplace and enabled plugin on next launch. When allowed, best
	// effort prefetch the marketplace catalog so the very first session traces
	// without delay. Failure here (e.g. claude not on PATH) is non-fatal.
	prefetched := false
	var prefetchErr error
	if !noInstall {
		prefetchErr = runSetupCommand(cmd.Context(), claudeBinary(), "plugin", "marketplace", "add", claudeMarketplaceURL)
		prefetched = prefetchErr == nil
	}

	return reportClaudeSetup(cmd, settingsPath, project, opts.APIURL, scope, pluginRef, noInstall, prefetched, prefetchErr)
}

func claudeSettingsPath(scope string) (string, error) {
	switch scope {
	case "", "user":
		dir, err := claudeConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "settings.json"), nil
	case "project":
		// settings.local.json is the personal, git-ignored project override —
		// the right place for a file that embeds an API key.
		return filepath.Join(".claude", "settings.local.json"), nil
	default:
		return "", fmt.Errorf("invalid --scope %q: must be \"user\" or \"project\"", scope)
	}
}

func writeClaudeSettings(path, pluginRef string, opts client.Options, project string) error {
	return mergeJSONFile(path, func(doc map[string]any) error {
		markets := jsonObject(doc, "extraKnownMarketplaces")
		markets[claudeMarketplaceName] = map[string]any{
			"source": map[string]any{
				"source": "github",
				"repo":   claudeMarketplaceRepo,
			},
		}

		enabled := jsonObject(doc, "enabledPlugins")
		enabled[pluginRef] = true

		env := jsonObject(doc, "env")
		env[envTraceToLangSmith] = "true"
		env[envCCLangSmithAPIKey] = opts.APIKey
		env[envLangSmithAPIKey] = opts.APIKey
		env[envCCLangSmithProject] = project
		env[envLangSmithProject] = project
		if !isDefaultEndpoint(opts.APIURL) {
			env[envLangSmithEndpoint] = opts.APIURL
		}
		return nil
	})
}

func claudeBinary() string {
	if b := strings.TrimSpace(os.Getenv("LANGSMITH_CLAUDE_BIN")); b != "" {
		return b
	}
	return "claude"
}

func reportClaudeSetup(cmd *cobra.Command, settingsPath, project, apiURL, scope, pluginRef string, noInstall, prefetched bool, prefetchErr error) error {
	if GetFormat() == "pretty" {
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Configured Claude Code tracing → LangSmith project %q\n", project)
		fmt.Fprintf(out, "  settings: %s (contains your API key; mode 0600)\n", settingsPath)
		fmt.Fprintf(out, "  plugin:   %s\n", pluginRef)
		if !isDefaultEndpoint(apiURL) {
			fmt.Fprintf(out, "  endpoint: %s\n", apiURL)
		}
		if !noInstall && !prefetched {
			fmt.Fprintf(out, "\nCould not prefetch the plugin marketplace (%v).\n", prefetchErr)
			fmt.Fprintf(out, "That's fine — Claude Code installs it on next launch. To prefetch now, run:\n")
			fmt.Fprintf(out, "  claude plugin marketplace add %s\n", claudeMarketplaceURL)
		}
		fmt.Fprintf(out, "\nStart Claude Code and verify with: tail -f ~/.claude/state/hook.log\n")
		return nil
	}

	result := map[string]any{
		"status":                 "configured",
		"agent":                  "claude-code",
		"project":                project,
		"settings_path":          settingsPath,
		"scope":                  scopeOrDefault(scope),
		"plugin":                 pluginRef,
		"marketplace_prefetched": !noInstall && prefetched,
	}
	if !isDefaultEndpoint(apiURL) {
		result["endpoint"] = apiURL
	}
	if !noInstall && !prefetched && prefetchErr != nil {
		result["marketplace_prefetch_error"] = prefetchErr.Error()
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func scopeOrDefault(scope string) string {
	if scope == "" {
		return "user"
	}
	return scope
}
