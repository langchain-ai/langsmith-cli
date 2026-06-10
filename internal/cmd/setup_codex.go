package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

// Codex LangSmith tracing plugin coordinates.
const (
	codexMarketplaceURL = "github.com/langchain-ai/langsmith-codex-plugins"
	codexPluginRef      = "tracing@langsmith-codex-plugins"

	// Project the plugin traces to when langsmith.json has no "project" key
	// (its plugins/tracing/src/config.ts). Display-only — the CLI never
	// writes this value.
	codexDefaultProject = "codex"
)

func newSetupCodexCmd() *cobra.Command {
	var (
		project string
		scope   string
	)
	cmd := &cobra.Command{
		Use:   "codex [api-key]",
		Short: "Configure Codex to trace to LangSmith",
		Args:  cobra.MaximumNArgs(1),
		Long: `Configure Codex to send full-content traces to LangSmith.

Enables the LangSmith tracing plugin in Codex's config.toml and stores your API
key in langsmith.json. Codex has no file-only way to fetch the plugin code, so
this command prints one 'codex plugin marketplace add' command to run once — it
does not run Codex itself.

The API URL defaults to https://api.smith.langchain.com. Without --project, no
project is written and the plugin's own default ("codex") applies.

  langsmith setup codex <api-key>        # writes ~/.codex/{config.toml,langsmith.json}
  langsmith setup codex --project my-agent
  langsmith setup codex --scope project  # writes ./.codex/...`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetupCodex(cmd, positionalKey(args), project, scope)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "LangSmith project name (default: $LANGSMITH_PROJECT, else the plugin's default \"codex\")")
	cmd.Flags().StringVar(&scope, "scope", "user", "Config scope: user (~/.codex) or project (./.codex)")
	return cmd
}

func runSetupCodex(cmd *cobra.Command, apiKey, project, scope string) error {
	opts, err := setupClientOptions(apiKey)
	if err != nil {
		return err
	}
	if project == "" {
		project = envTraceProject()
	}

	dir, err := codexConfigDir(scope)
	if err != nil {
		return err
	}

	credPath := filepath.Join(dir, "langsmith.json")
	if err := writeCodexCredentials(credPath, opts, project); err != nil {
		return err
	}

	tomlPath := filepath.Join(dir, "config.toml")
	if err := enableCodexPlugin(tomlPath); err != nil {
		return err
	}

	return reportCodexSetup(cmd, credPath, tomlPath, project, opts.APIURL, scope)
}

func codexConfigDir(scope string) (string, error) {
	switch scope {
	case "", "user":
		return codexHome()
	case "project":
		return ".codex", nil
	default:
		return "", fmt.Errorf("invalid --scope %q: must be \"user\" or \"project\"", scope)
	}
}

func writeCodexCredentials(path string, opts client.Options, project string) error {
	return mergeJSONFile(path, func(doc map[string]any) error {
		doc["enabled"] = true
		doc["api_key"] = opts.APIKey
		// Without an explicit project the key is removed (not written empty —
		// the plugin treats "" as an explicit value) so its own default
		// applies, including on re-runs that drop a configured project.
		if project != "" {
			doc["project"] = project
		} else {
			delete(doc, "project")
		}
		if !isDefaultEndpoint(opts.APIURL) {
			doc["api_url"] = opts.APIURL
		} else {
			delete(doc, "api_url")
		}
		return nil
	})
}

// enableCodexPlugin sets features.plugin_hooks=true and enables the tracing
// plugin in config.toml, preserving any existing TOML keys.
func enableCodexPlugin(path string) error {
	doc := map[string]any{}
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if len(strings.TrimSpace(string(data))) > 0 {
			if err := toml.Unmarshal(data, &doc); err != nil {
				return fmt.Errorf("parsing %s: %w", path, err)
			}
		}
	case errors.Is(err, os.ErrNotExist):
		// Start from an empty document.
	default:
		return fmt.Errorf("reading %s: %w", path, err)
	}

	features := jsonObject(doc, "features")
	features["plugin_hooks"] = true

	plugins := jsonObject(doc, "plugins")
	entry := jsonObject(plugins, codexPluginRef)
	entry["enabled"] = true

	out, err := toml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

func reportCodexSetup(cmd *cobra.Command, credPath, tomlPath, project, apiURL, scope string) error {
	if GetFormat() == "pretty" {
		out := cmd.OutOrStdout()
		if project == "" {
			fmt.Fprintf(out, "Configured Codex tracing → LangSmith project %q (plugin default)\n", codexDefaultProject)
		} else {
			fmt.Fprintf(out, "Configured Codex tracing → LangSmith project %q\n", project)
		}
		fmt.Fprintf(out, "  credentials: %s (contains your API key; mode 0600)\n", credPath)
		fmt.Fprintf(out, "  config:      %s (plugin enabled)\n", tomlPath)
		if !isDefaultEndpoint(apiURL) {
			fmt.Fprintf(out, "  endpoint:    %s\n", apiURL)
		}
		fmt.Fprintf(out, "\nOne manual step — fetch the plugin code (run once):\n")
		fmt.Fprintf(out, "  codex plugin marketplace add %s\n", codexMarketplaceURL)
		fmt.Fprintf(out, "Then start Codex and traces will flow to LangSmith.\n")
		return nil
	}

	result := map[string]any{
		"status":           "configured",
		"agent":            "codex",
		"credentials_path": credPath,
		"config_path":      tomlPath,
		"scope":            scopeOrDefault(scope),
		"plugin":           codexPluginRef,
		"manual_step":      "codex plugin marketplace add " + codexMarketplaceURL,
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
