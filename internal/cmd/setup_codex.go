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
)

func newSetupCodexCmd() *cobra.Command {
	var (
		project   string
		scope     string
		noInstall bool
	)
	cmd := &cobra.Command{
		Use:   "codex",
		Short: "Configure Codex to trace to LangSmith",
		Long: `Configure Codex to send full-content traces to LangSmith.

Enables the LangSmith tracing plugin in Codex's config.toml and stores your API
key and project name in langsmith.json. Every future Codex session then traces
to a LangSmith project automatically.

  langsmith setup codex                  # writes ~/.codex/{config.toml,langsmith.json}
  langsmith setup codex --project my-agent
  langsmith setup codex --scope project  # writes ./.codex/...`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSetupCodex(cmd, project, scope, noInstall)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "LangSmith project name (default: $LANGSMITH_PROJECT, else \"codex\")")
	cmd.Flags().StringVar(&scope, "scope", "user", "Config scope: user (~/.codex) or project (./.codex)")
	cmd.Flags().BoolVar(&noInstall, "no-install", false, "Only write config; skip running 'codex plugin marketplace add'")
	return cmd
}

func runSetupCodex(cmd *cobra.Command, project, scope string, noInstall bool) error {
	opts, err := setupClientOptions()
	if err != nil {
		return err
	}
	if project == "" {
		project = defaultTraceProject("codex")
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

	// Registering the marketplace is the one step with no declarative
	// equivalent, so shell out to codex when allowed. Non-fatal on failure.
	added := false
	var addErr error
	if !noInstall {
		addErr = runSetupCommand(cmd.Context(), codexBinary(), "plugin", "marketplace", "add", codexMarketplaceURL)
		added = addErr == nil
	}

	return reportCodexSetup(cmd, credPath, tomlPath, project, opts.APIURL, scope, noInstall, added, addErr)
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
		doc["project"] = project
		if !isDefaultEndpoint(opts.APIURL) {
			doc["api_url"] = opts.APIURL
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

func codexBinary() string {
	if b := strings.TrimSpace(os.Getenv("LANGSMITH_CODEX_BIN")); b != "" {
		return b
	}
	return "codex"
}

func reportCodexSetup(cmd *cobra.Command, credPath, tomlPath, project, apiURL, scope string, noInstall, added bool, addErr error) error {
	if GetFormat() == "pretty" {
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Configured Codex tracing → LangSmith project %q\n", project)
		fmt.Fprintf(out, "  credentials: %s (contains your API key; mode 0600)\n", credPath)
		fmt.Fprintf(out, "  config:      %s (plugin enabled)\n", tomlPath)
		if !isDefaultEndpoint(apiURL) {
			fmt.Fprintf(out, "  endpoint:    %s\n", apiURL)
		}
		if !noInstall && !added {
			fmt.Fprintf(out, "\nCould not add the Codex marketplace (%v).\n", addErr)
			fmt.Fprintf(out, "Run it yourself once Codex is installed:\n")
			fmt.Fprintf(out, "  codex plugin marketplace add %s\n", codexMarketplaceURL)
		}
		fmt.Fprintf(out, "\nStart Codex and traces will flow to LangSmith.\n")
		return nil
	}

	result := map[string]any{
		"status":            "configured",
		"agent":             "codex",
		"project":           project,
		"credentials_path":  credPath,
		"config_path":       tomlPath,
		"scope":             scopeOrDefault(scope),
		"plugin":            codexPluginRef,
		"marketplace_added": !noInstall && added,
	}
	if !isDefaultEndpoint(apiURL) {
		result["endpoint"] = apiURL
	}
	if !noInstall && !added && addErr != nil {
		result["marketplace_add_error"] = addErr.Error()
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
