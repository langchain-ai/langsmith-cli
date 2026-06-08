package cmd

import (
	"context"
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

// Claude Code LangSmith tracing plugin coordinates. The marketplace name must
// match how the plugin is referenced (langsmith-tracing@<marketplace>), so the
// extraKnownMarketplaces key and enabledPlugins key stay consistent.
const (
	claudeMarketplaceName = "langsmith-claude-code-plugins"
	claudeMarketplaceRepo = "langchain-ai/langsmith-claude-code-plugins"
	claudeMarketplaceURL  = "https://github.com/langchain-ai/langsmith-claude-code-plugins.git"
	claudePluginName      = "langsmith-tracing"
)

func newTraceSetupClaudeCmd() *cobra.Command {
	var (
		project   string
		scope     string
		user      string
		email     string
		yes       bool
		noInstall bool
	)
	cmd := &cobra.Command{
		Use:   "claude [API_KEY] [API_URL] [PROJECT]",
		Short: "Configure Claude Code to trace to LangSmith",
		Long: `Configure Claude Code to send full-content traces to LangSmith.

Shows the changes to Claude Code's settings.json (tracing marketplace, enabled
plugin, your API key + project, and your name/email as run metadata) and asks
for confirmation before writing them, then installs the plugin via the claude
CLI. The API key, URL, and project may be positional args or flags.

  langsmith trace setup claude demo-key dev.smith.com shared-claude
  langsmith trace setup claude                          # key/URL from env or profile
  langsmith trace setup claude --user "Jane Doe"      # override the auto-detected name
  langsmith trace setup [SAFE_TO_USE:PERSON_oxyzyjwd]                    # apply without the confirmation prompt`,
		Args: cobra.MaximumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetupClaude(cmd, args, project, scope, user, email, yes, noInstall)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "LangSmith project name (default: $LANGSMITH_PROJECT, else \"claude-code\")")
	cmd.Flags().StringVar(&scope, "scope", "user", "Config scope: user (~/.claude/settings.json) or project (./.claude/settings.local.json)")
	cmd.Flags().StringVar(&user, "user", "", "Name attached to every trace (default: git user.name, else OS user)")
	cmd.Flags().StringVar(&email, "email", "", "Email attached to every trace (default: git user.email)")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the confirmation prompt")
	cmd.Flags().BoolVar(&noInstall, "no-install", false, "Write settings only; do not run 'claude plugin install'")
	return cmd
}

func runSetupClaude(cmd *cobra.Command, args []string, project, scope, user, email string, yes, noInstall bool) error {
	opts, err := resolveSetupOptions(args, &project)
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
	metadata := resolveUserMetadata(user, email)

	preview := claudeChangePreview(settingsPath, pluginRef, opts, project, scope, metadata, noInstall)
	ok, err := confirmApply(cmd, yes, preview)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("aborted")
	}

	if err := writeClaudeSettings(settingsPath, pluginRef, opts, project, metadata, scope == "project"); err != nil {
		return err
	}

	installed := false
	var installErr error
	if !noInstall {
		installErr = installClaudePlugin(cmd.Context(), scope, pluginRef)
		installed = installErr == nil
	}

	return reportClaudeSetup(cmd, settingsPath, project, opts.APIURL, scope, pluginRef, noInstall, installed, installErr)
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

func writeClaudeSettings(path, pluginRef string, opts client.Options, project string, metadata map[string]string, projectScope bool) error {
	return mergeJSONFile(path, projectScope, func(doc map[string]any) error {
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
		env[envCCLangSmithProject] = project
		// A non-default (e.g. self-hosted) endpoint is expressed as a single
		// replica destination — the documented way to point Claude Code's
		// tracing at a custom API URL.
		if !isDefaultEndpoint(opts.APIURL) {
			replicas := []map[string]any{{
				"apiUrl":      opts.APIURL,
				"apiKey":      opts.APIKey,
				"projectName": project,
			}}
			encoded, err := json.Marshal(replicas)
			if err != nil {
				return fmt.Errorf("encoding runs endpoints: %w", err)
			}
			env[envCCLangSmithRunsEndpoints] = string(encoded)
		} else {
			delete(env, envCCLangSmithRunsEndpoints)
		}

		// Attach the user's name/email as run metadata, merging with any
		// existing CC_LANGSMITH_METADATA so manual keys are preserved.
		if len(metadata) > 0 {
			merged := map[string]any{}
			if existing, ok := env[envCCLangSmithMetadata].(string); ok && strings.TrimSpace(existing) != "" {
				_ = json.Unmarshal([]byte(existing), &merged)
			}
			for k, v := range metadata {
				merged[k] = v
			}
			encoded, err := json.Marshal(merged)
			if err != nil {
				return fmt.Errorf("encoding metadata: %w", err)
			}
			env[envCCLangSmithMetadata] = string(encoded)
		}
		return nil
	})
}

// claudeChangePreview renders the settings.json additions (API key masked) and
// the install commands, for the confirmation prompt.
func claudeChangePreview(settingsPath, pluginRef string, opts client.Options, project, scope string, metadata map[string]string, noInstall bool) string {
	env := map[string]any{
		envTraceToLangSmith:   "true",
		envCCLangSmithAPIKey:  lsconfig.MaskSecret(opts.APIKey),
		envCCLangSmithProject: project,
	}
	if !isDefaultEndpoint(opts.APIURL) {
		env[envCCLangSmithRunsEndpoints] = fmt.Sprintf("[{\"apiUrl\":%q,\"apiKey\":%q,\"projectName\":%q}]",
			opts.APIURL, lsconfig.MaskSecret(opts.APIKey), project)
	}
	if len(metadata) > 0 {
		env[envCCLangSmithMetadata] = metadata
	}
	additions := map[string]any{
		"extraKnownMarketplaces": map[string]any{
			claudeMarketplaceName: map[string]any{
				"source": map[string]any{"source": "github", "repo": claudeMarketplaceRepo},
			},
		},
		"enabledPlugins": map[string]any{pluginRef: true},
		"env":            env,
	}
	b, _ := json.MarshalIndent(additions, "", "  ")

	var sb strings.Builder
	fmt.Fprintf(&sb, "Will update %s with:\n\n%s\n", settingsPath, b)
	if !noInstall {
		fmt.Fprintf(&sb, "\nThen run:\n")
		fmt.Fprintf(&sb, "  %s plugin marketplace add %s\n", claudeBinary(), claudeMarketplaceURL)
		fmt.Fprintf(&sb, "  %s plugin install %s --scope %s\n", claudeBinary(), pluginRef, scopeOrDefault(scope))
	}
	fmt.Fprintf(&sb, "\nThe plugin runs on every Claude Code session and sends your prompts,\nresponses, and tool output to LangSmith.\n")
	return sb.String()
}

func claudeBinary() string {
	if b := strings.TrimSpace(os.Getenv("LANGSMITH_CLAUDE_BIN")); b != "" {
		return b
	}
	return "claude"
}

func installClaudePlugin(ctx context.Context, scope, pluginRef string) error {
	bin := claudeBinary()
	if err := runSetupCommand(ctx, bin, "plugin", "marketplace", "add", claudeMarketplaceURL); err != nil {
		return fmt.Errorf("%s plugin marketplace add: %w", bin, err)
	}
	if err := runSetupCommand(ctx, bin, "plugin", "install", pluginRef, "--scope", scopeOrDefault(scope)); err != nil {
		return fmt.Errorf("%s plugin install: %w", bin, err)
	}
	return nil
}

func reportClaudeSetup(cmd *cobra.Command, settingsPath, project, apiURL, scope, pluginRef string, noInstall, installed bool, installErr error) error {
	if GetFormat() == "pretty" {
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "\nConfigured Claude Code tracing → LangSmith project %q\n", project)
		fmt.Fprintf(out, "  settings: %s (contains your API key; mode 0600)\n", settingsPath)
		fmt.Fprintf(out, "  plugin:   %s\n", pluginRef)
		if !isDefaultEndpoint(apiURL) {
			fmt.Fprintf(out, "  endpoint: %s\n", apiURL)
		}
		switch {
		case noInstall:
			fmt.Fprintf(out, "\nSkipped install. Run inside Claude Code (or rerun without --no-install):\n")
			fmt.Fprintf(out, "  /plugin marketplace add %s\n", claudeMarketplaceURL)
			fmt.Fprintf(out, "  /plugin install %s\n", pluginRef)
		case installed:
			fmt.Fprintf(out, "  install:  done\n")
		default:
			fmt.Fprintf(out, "\nConfig written, but the plugin install failed (%v).\n", installErr)
			fmt.Fprintf(out, "Claude Code will still install it on next launch from settings.json, or run:\n")
			fmt.Fprintf(out, "  %s plugin marketplace add %s\n", claudeBinary(), claudeMarketplaceURL)
			fmt.Fprintf(out, "  %s plugin install %s --scope %s\n", claudeBinary(), pluginRef, scopeOrDefault(scope))
		}
		fmt.Fprintf(out, "\nVerify with: tail -f ~/.claude/state/hook.log\n")
		return nil
	}

	result := map[string]any{
		"status":        "configured",
		"agent":         "claude-code",
		"project":       project,
		"settings_path": settingsPath,
		"scope":         scopeOrDefault(scope),
		"plugin":        pluginRef,
		"installed":     !noInstall && installed,
	}
	if !isDefaultEndpoint(apiURL) {
		result["endpoint"] = apiURL
	}
	if !noInstall && !installed && installErr != nil {
		result["install_error"] = installErr.Error()
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
