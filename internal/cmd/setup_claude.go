package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/spf13/cobra"
)

// Claude Code LangSmith tracing plugin coordinates. The marketplace name must
// match how the plugin is referenced (langsmith-tracing@<marketplace>), so the
// extraKnownMarketplaces key and enabledPlugins key stay consistent.
const (
	claudeMarketplaceName = "langsmith-claude-code-plugins"
	claudeMarketplaceRepo = "langchain-ai/langsmith-claude-code-plugins"
	claudeMarketplaceURL  = "https://github.com/langchain-ai/langsmith-claude-code-plugins.git"
	claudePluginName      = "langsmith-tracing"

	// Project the plugin traces to when CC_LANGSMITH_PROJECT is unset (its
	// src/config.ts). Display-only — the CLI never writes this value.
	claudeDefaultProject = "claude-code"
)

func newSetupClaudeCmd() *cobra.Command {
	var (
		project string
		scope   string
	)
	cmd := &cobra.Command{
		Use:   "claude [api-key]",
		Short: "Configure Claude Code to trace to LangSmith",
		Args:  cobra.MaximumNArgs(1),
		Long: `Configure Claude Code to send full-content traces to LangSmith.

Declares the LangSmith tracing marketplace, enables the plugin, and stores your
API key in Claude Code's settings.json. Claude Code installs and enables the
plugin on its next launch — this command does not run Claude Code.

The API URL defaults to https://api.smith.langchain.com. Without --project, no
project is written and the plugin's own default ("claude-code") applies.

  langsmith setup claude <api-key>             # write ~/.claude/settings.json
  langsmith setup claude --project my-agent    # trace to a named project
  langsmith setup claude --scope project       # write ./.claude/settings.local.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetupClaude(cmd, positionalKey(args), project, scope)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "LangSmith project name (default: $LANGSMITH_PROJECT, else the plugin's default \"claude-code\")")
	cmd.Flags().StringVar(&scope, "scope", "user", "Config scope: user (~/.claude/settings.json) or project (./.claude/settings.local.json)")
	return cmd
}

func runSetupClaude(cmd *cobra.Command, apiKey, project, scope string) error {
	opts, err := setupClientOptions(apiKey)
	if err != nil {
		return err
	}
	if project == "" {
		project = envTraceProject()
	}

	settingsPath, err := claudeSettingsPath(scope)
	if err != nil {
		return err
	}

	pluginRef := claudePluginName + "@" + claudeMarketplaceName
	if err := writeClaudeSettings(settingsPath, pluginRef, opts, project); err != nil {
		return err
	}

	return reportClaudeSetup(cmd, settingsPath, project, opts.APIURL, scope, pluginRef)
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
		// Without an explicit project the var is removed (not written empty) so
		// the plugin's own default applies — including on re-runs that drop a
		// previously configured project.
		if project != "" {
			env[envCCLangSmithProject] = project
		} else {
			delete(env, envCCLangSmithProject)
		}
		// A non-default (e.g. self-hosted) endpoint is expressed as a single
		// replica destination — the documented way to point Claude Code's
		// tracing at a custom API URL.
		if !isDefaultEndpoint(opts.APIURL) {
			replica := map[string]any{
				"apiUrl": opts.APIURL,
				"apiKey": opts.APIKey,
			}
			if project != "" {
				replica["projectName"] = project
			}
			encoded, err := json.Marshal([]map[string]any{replica})
			if err != nil {
				return fmt.Errorf("encoding runs endpoints: %w", err)
			}
			env[envCCLangSmithRunsEndpoints] = string(encoded)
		} else {
			delete(env, envCCLangSmithRunsEndpoints)
		}
		return nil
	})
}

func reportClaudeSetup(cmd *cobra.Command, settingsPath, project, apiURL, scope, pluginRef string) error {
	if GetFormat() == "pretty" {
		out := cmd.OutOrStdout()
		if project == "" {
			fmt.Fprintf(out, "Configured Claude Code tracing → LangSmith project %q (plugin default)\n", claudeDefaultProject)
		} else {
			fmt.Fprintf(out, "Configured Claude Code tracing → LangSmith project %q\n", project)
		}
		fmt.Fprintf(out, "  settings: %s (contains your API key; mode 0600)\n", settingsPath)
		fmt.Fprintf(out, "  plugin:   %s\n", pluginRef)
		if !isDefaultEndpoint(apiURL) {
			fmt.Fprintf(out, "  endpoint: %s\n", apiURL)
		}
		fmt.Fprintf(out, "\nClaude Code installs and enables the plugin on its next launch.\n")
		fmt.Fprintf(out, "If it does not appear, run inside Claude Code:\n")
		fmt.Fprintf(out, "  /plugin marketplace add %s\n", claudeMarketplaceURL)
		fmt.Fprintf(out, "  /plugin install %s\n", pluginRef)
		fmt.Fprintf(out, "Verify with: tail -f ~/.claude/state/hook.log\n")
		return nil
	}

	result := map[string]any{
		"status":        "configured",
		"agent":         "claude-code",
		"settings_path": settingsPath,
		"scope":         scopeOrDefault(scope),
		"plugin":        pluginRef,
		"installs_on":   "next claude launch",
	}
	if project != "" {
		result["project"] = project
	}
	if !isDefaultEndpoint(apiURL) {
		result["endpoint"] = apiURL
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
