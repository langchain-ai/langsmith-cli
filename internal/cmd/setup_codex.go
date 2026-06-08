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
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

// Codex LangSmith tracing plugin coordinates.
const (
	codexMarketplaceURL = "github.com/langchain-ai/langsmith-codex-plugins"
	codexPluginRef      = "tracing@langsmith-codex-plugins"
)

func newTraceSetupCodexCmd() *cobra.Command {
	var (
		project   string
		scope     string
		user      string
		email     string
		yes       bool
		noInstall bool
	)
	cmd := &cobra.Command{
		Use:   "codex [API_KEY] [API_URL] [PROJECT]",
		Short: "Configure Codex to trace to LangSmith",
		Long: `Configure Codex to send full-content traces to LangSmith.

Shows the changes to Codex's config.toml (enables the tracing plugin) and
langsmith.json (your API key + project, and your name/email as run metadata),
asks for confirmation, writes them, then fetches the plugin via 'codex plugin
marketplace add'. The API key, URL, and project may be positional args or flags.

  langsmith trace setup codex demo-key dev.smith.com shared-codex
  langsmith trace setup codex                              # key/URL from env or profile
  langsmith trace setup codex --user "Jane Doe"       # override the auto-detected name
  langsmith trace setup codex --yes                        # apply without the confirmation prompt`,
		Args: cobra.MaximumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetupCodex(cmd, args, project, scope, user, email, yes, noInstall)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "LangSmith project name (default: $LANGSMITH_PROJECT, else \"codex\")")
	cmd.Flags().StringVar(&scope, "scope", "user", "Config scope: user (~/.codex) or project (./.codex)")
	cmd.Flags().StringVar(&user, "user", "", "Name attached to every trace (default: git user.name, else OS user)")
	cmd.Flags().StringVar(&email, "email", "", "Email attached to every trace (default: git user.email)")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the confirmation prompt")
	cmd.Flags().BoolVar(&noInstall, "no-install", false, "Write config only; do not run 'codex plugin marketplace add'")
	return cmd
}

func runSetupCodex(cmd *cobra.Command, args []string, project, scope, user, email string, yes, noInstall bool) error {
	opts, err := resolveSetupOptions(args, &project)
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
	tomlPath := filepath.Join(dir, "config.toml")
	metadata := resolveUserMetadata(user, email)

	preview := codexChangePreview(credPath, tomlPath, opts, project, metadata, noInstall)
	ok, err := confirmApply(cmd, yes, preview)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("aborted")
	}

	projectScope := scope == "project"
	if err := writeCodexCredentials(credPath, opts, project, metadata, projectScope); err != nil {
		return err
	}
	if err := enableCodexPlugin(tomlPath, projectScope); err != nil {
		return err
	}

	added := false
	var addErr error
	if !noInstall {
		addErr = installCodexPlugin(cmd.Context())
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

func writeCodexCredentials(path string, opts client.Options, project string, metadata map[string]string, projectScope bool) error {
	return mergeJSONFile(path, projectScope, func(doc map[string]any) error {
		doc["enabled"] = true
		doc["api_key"] = opts.APIKey
		doc["project"] = project
		if !isDefaultEndpoint(opts.APIURL) {
			doc["api_url"] = opts.APIURL
		} else {
			// Drop a stale self-hosted URL when re-running against the default
			// endpoint, so Codex doesn't keep tracing to the old host.
			delete(doc, "api_url")
		}
		// Attach the user's name/email as run metadata, merging into any
		// existing metadata object so manual keys are preserved.
		if len(metadata) > 0 {
			md := jsonObject(doc, "metadata")
			for k, v := range metadata {
				md[k] = v
			}
		}
		return nil
	})
}

// enableCodexPlugin sets features.plugin_hooks=true and enables the tracing
// plugin in config.toml. It preserves the user's existing content — comments and
// formatting included — by editing the file textually rather than re-marshaling
// it: if both settings are already present it doesn't touch the file, a fresh
// file gets a minimal document, and otherwise the missing entries are inserted
// or appended in place. For project scope it refuses symlinked path components.
func enableCodexPlugin(path string, projectScope bool) error {
	if projectScope {
		if err := assertSafeProjectPath(path); err != nil {
			return err
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	doc := map[string]any{}
	if len(strings.TrimSpace(string(raw))) > 0 {
		if err := toml.Unmarshal(raw, &doc); err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
	}

	featureOK := tomlBoolTrue(doc, "features", "plugin_hooks")
	pluginOK := codexPluginEnabled(doc)
	if featureOK && pluginOK {
		// Already configured — leave the file (and its comments) untouched.
		return nil
	}

	// A fresh/empty file has no comments to preserve: write a minimal document.
	if len(strings.TrimSpace(string(raw))) == 0 {
		fresh := map[string]any{
			"features": map[string]any{"plugin_hooks": true},
			"plugins":  map[string]any{codexPluginRef: map[string]any{"enabled": true}},
		}
		out, err := toml.Marshal(fresh)
		if err != nil {
			return fmt.Errorf("encoding %s: %w", path, err)
		}
		return writeConfigFile(path, out, projectScope)
	}

	// Existing content: edit in place so the user's comments/formatting survive.
	text := string(raw)
	if !featureOK {
		text = ensureTOMLTableKey(text, "features", "plugin_hooks = true")
	}
	if !pluginOK {
		text = ensureCodexPluginEnabled(text, doc)
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}

	// Validate the textual edit actually parses and sets both values. It can
	// fail when an entry is an inline table (e.g. `"tracing@..." = { enabled =
	// false }`) that the line-based edit can't locate, so the append above
	// duplicates a key. In that case fall back to a full re-marshal — correct,
	// though it drops comments — rather than emit invalid TOML.
	var check map[string]any
	if err := toml.Unmarshal([]byte(text), &check); err == nil &&
		tomlBoolTrue(check, "features", "plugin_hooks") && codexPluginEnabled(check) {
		return writeConfigFile(path, []byte(text), projectScope)
	}

	features := jsonObject(doc, "features")
	features["plugin_hooks"] = true
	plugins := jsonObject(doc, "plugins")
	jsonObject(plugins, codexPluginRef)["enabled"] = true
	out, err := toml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", path, err)
	}
	return writeConfigFile(path, out, projectScope)
}

func tomlBoolTrue(doc map[string]any, table, key string) bool {
	t, _ := doc[table].(map[string]any)
	v, _ := t[key].(bool)
	return v
}

func codexPluginEnabled(doc map[string]any) bool {
	plugins, _ := doc["plugins"].(map[string]any)
	entry, _ := plugins[codexPluginRef].(map[string]any)
	v, _ := entry["enabled"].(bool)
	return v
}

// ensureTOMLTableKey makes `[table]` contain kvLine ("key = value"), editing the
// text in place: it replaces an existing key line, inserts after the table
// header, or appends a fresh table — leaving all other lines (comments included)
// as they are. table must be a bare, unquoted name.
func ensureTOMLTableKey(text, table, kvLine string) string {
	key := strings.TrimSpace(strings.SplitN(kvLine, "=", 2)[0])
	header := "[" + table + "]"
	lines := strings.Split(text, "\n")
	hdr := -1
	for i, ln := range lines {
		if strings.TrimSpace(ln) == header {
			hdr = i
			break
		}
	}
	if hdr == -1 {
		return strings.TrimRight(text, "\n") + "\n\n" + header + "\n" + kvLine + "\n"
	}
	end := len(lines)
	for j := hdr + 1; j < len(lines); j++ {
		if strings.HasPrefix(strings.TrimSpace(lines[j]), "[") {
			end = j
			break
		}
	}
	for j := hdr + 1; j < end; j++ {
		t := strings.TrimSpace(lines[j])
		if strings.HasPrefix(t, key) && strings.HasPrefix(strings.TrimSpace(t[len(key):]), "=") {
			lines[j] = kvLine
			return strings.Join(lines, "\n")
		}
	}
	out := append([]string{}, lines[:hdr+1]...)
	out = append(out, kvLine)
	out = append(out, lines[hdr+1:]...)
	return strings.Join(out, "\n")
}

// ensureCodexPluginEnabled makes the tracing plugin's table contain
// `enabled = true`, appending the table when absent and editing it in place
// otherwise. doc is the parsed config used to tell whether the table exists.
func ensureCodexPluginEnabled(text string, doc map[string]any) string {
	header := `[plugins."` + codexPluginRef + `"]`
	plugins, _ := doc["plugins"].(map[string]any)
	if _, present := plugins[codexPluginRef]; !present {
		return strings.TrimRight(text, "\n") + "\n\n" + header + "\nenabled = true\n"
	}
	lines := strings.Split(text, "\n")
	hdr := -1
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "[plugins.") && strings.Contains(t, codexPluginRef) && strings.HasSuffix(t, "]") {
			hdr = i
			break
		}
	}
	if hdr == -1 {
		// Parsed as present but not locatable textually (unusual quoting);
		// append rather than guess.
		return strings.TrimRight(text, "\n") + "\n\n" + header + "\nenabled = true\n"
	}
	end := len(lines)
	for j := hdr + 1; j < len(lines); j++ {
		if strings.HasPrefix(strings.TrimSpace(lines[j]), "[") {
			end = j
			break
		}
	}
	for j := hdr + 1; j < end; j++ {
		t := strings.TrimSpace(lines[j])
		if strings.HasPrefix(t, "enabled") && strings.HasPrefix(strings.TrimSpace(t[len("enabled"):]), "=") {
			lines[j] = "enabled = true"
			return strings.Join(lines, "\n")
		}
	}
	out := append([]string{}, lines[:hdr+1]...)
	out = append(out, "enabled = true")
	out = append(out, lines[hdr+1:]...)
	return strings.Join(out, "\n")
}

// codexChangePreview renders the langsmith.json + config.toml changes (API key
// masked) and the install command, for the confirmation prompt.
func codexChangePreview(credPath, tomlPath string, opts client.Options, project string, metadata map[string]string, noInstall bool) string {
	cred := map[string]any{
		"enabled": true,
		"api_key": lsconfig.MaskSecret(opts.APIKey),
		"project": project,
	}
	if !isDefaultEndpoint(opts.APIURL) {
		cred["api_url"] = opts.APIURL
	}
	if len(metadata) > 0 {
		cred["metadata"] = metadata
	}
	credJSON, _ := json.MarshalIndent(cred, "", "  ")

	var sb strings.Builder
	fmt.Fprintf(&sb, "Will write %s:\n\n%s\n", credPath, credJSON)
	fmt.Fprintf(&sb, "\nWill enable the plugin in %s:\n\n", tomlPath)
	fmt.Fprintf(&sb, "  [features]\n  plugin_hooks = true\n  [plugins.\"%s\"]\n  enabled = true\n", codexPluginRef)
	if !noInstall {
		fmt.Fprintf(&sb, "\nThen run:\n  %s plugin marketplace add %s\n", codexBinary(), codexMarketplaceURL)
	}
	fmt.Fprintf(&sb, "\nThe plugin runs on every Codex session and sends your prompts,\nresponses, and tool output to LangSmith.\n")
	return sb.String()
}

func codexBinary() string {
	if b := strings.TrimSpace(os.Getenv("LANGSMITH_CODEX_BIN")); b != "" {
		return b
	}
	return "codex"
}

func installCodexPlugin(ctx context.Context) error {
	bin := codexBinary()
	if err := runSetupCommand(ctx, bin, "plugin", "marketplace", "add", codexMarketplaceURL); err != nil {
		return fmt.Errorf("%s plugin marketplace add: %w", bin, err)
	}
	return nil
}

func reportCodexSetup(cmd *cobra.Command, credPath, tomlPath, project, apiURL, scope string, noInstall, added bool, addErr error) error {
	if GetFormat() == "pretty" {
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "\nConfigured Codex tracing → LangSmith project %q\n", project)
		fmt.Fprintf(out, "  credentials: %s (contains your API key; mode 0600)\n", credPath)
		fmt.Fprintf(out, "  config:      %s (plugin enabled)\n", tomlPath)
		if !isDefaultEndpoint(apiURL) {
			fmt.Fprintf(out, "  endpoint:    %s\n", apiURL)
		}
		switch {
		case noInstall:
			fmt.Fprintf(out, "\nSkipped install. Fetch the plugin code with:\n  codex plugin marketplace add %s\n", codexMarketplaceURL)
		case added:
			fmt.Fprintf(out, "  install:     done\n")
		default:
			fmt.Fprintf(out, "\nConfig written, but 'codex plugin marketplace add' failed (%v).\n", addErr)
			fmt.Fprintf(out, "Run it once Codex is installed:\n  %s plugin marketplace add %s\n", codexBinary(), codexMarketplaceURL)
		}
		fmt.Fprintf(out, "\nStart Codex and traces will flow to LangSmith.\n")
		return nil
	}

	result := map[string]any{
		"status":           "configured",
		"agent":            "codex",
		"project":          project,
		"credentials_path": credPath,
		"config_path":      tomlPath,
		"scope":            scopeOrDefault(scope),
		"plugin":           codexPluginRef,
		"installed":        !noInstall && added,
	}
	if !isDefaultEndpoint(apiURL) {
		result["endpoint"] = apiURL
	}
	if !noInstall && !added && addErr != nil {
		result["install_error"] = addErr.Error()
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
