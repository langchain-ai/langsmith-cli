package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
	"github.com/spf13/cobra"
)

func newGatewayCmd() *cobra.Command {
	gateway := &cobra.Command{Use: "gateway", Short: "Configure clients for LangSmith gateway inference"}
	setup := &cobra.Command{Use: "setup", Short: "Configure a gateway client"}
	setup.AddCommand(newGatewaySetupClaudeCodeCmd())
	gateway.AddCommand(setup)
	return gateway
}

func newGatewaySetupClaudeCodeCmd() *cobra.Command {
	var scope, gatewayURL string
	var yes, dryRun bool
	cmd := &cobra.Command{
		Use: "claude-code", Short: "Configure Claude Code with a LangSmith OAuth token helper",
		Long: `Configure Claude Code for LangSmith workspace/provider-funded inference, not
Claude subscription OAuth. Requires a saved OAuth profile; setup never refreshes
or validates tokens over the network. POSIX shells only (Windows unsupported).

--gateway-url is the trusted gateway root URL, optionally with a deployment path
prefix. With no model overrides, /anthropic is appended for native Anthropic
model defaults. With overrides, the bare gateway root is used instead: all four
families (Haiku, Sonnet, Opus, Fable) require provider/model slugs. Model flags
win over shell environment; saved model values are never reused. No overrides
clears all four family keys in the target settings and restores /anthropic.
Partial overrides fail without writing settings. Only the US SaaS API has an inferred gateway; other APIs require
--gateway-url.

Workspace routing defaults to the OAuth token's tenant_id. Pass --workspace to
explicitly override it; shell/profile workspace defaults are not used here.
Requires a gateway deployment supporting X-LangSmith-Auth-Mode: oauth.

Confirmation (or --yes) consents to sending OAuth credentials, prompts and model
traffic to the displayed destination and replacing the displayed settings keys.
Restart Claude Code after applying. Other settings layers and launch-time flags
can still override this configuration; review them before use.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runGatewaySetupClaudeCode(cmd, scope, gatewayURL, yes, dryRun)
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "user", "Settings scope: user or project (.claude/settings.local.json)")
	cmd.Flags().StringVar(&gatewayURL, "gateway-url", "", "Trusted gateway root URL (not the LangSmith API URL); uses /anthropic only without model overrides")
	for _, family := range gatewayModelFamilies {
		cmd.Flags().String(family+"-model", "", "Gateway provider/model slug for "+family+"; all four families required [env: "+gatewayModelEnv(family)+"]")
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Consent to the displayed destination and settings replacements without prompting")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview validated changes without writing files or refreshing tokens")
	return cmd
}

// Never interpolate a profile as a separate flag argument: a leading '-' is a
// valid saved name and must not become an option. Quote every shell argument.
func gatewayHelper(executable, profile, apiURL string) string {
	// Pin the API selection so Claude's saved/launch environment cannot redirect
	// refresh tokens for legacy profiles without a saved OAuth issuer.
	return gatewayShellQuote(executable) + " " + gatewayShellQuote("--profile="+profile) + " " + gatewayShellQuote("--api-url="+apiURL) + " --format=pretty auth token"
}

func gatewayShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func gatewayAnthropicURL(raw string) (string, error) {
	invalid := errors.New("invalid --gateway-url: use HTTPS (HTTP only for loopback), with no userinfo, query, fragment, or ambiguous path")
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.Opaque != "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(raw, "#") {
		return "", invalid
	}
	loopback := strings.EqualFold(u.Hostname(), "localhost")
	if ip := net.ParseIP(u.Hostname()); ip != nil {
		loopback = ip.IsLoopback()
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
		return "", invalid
	}
	if strings.ContainsAny(raw, "\r\n\t\\ %") || u.RawPath != "" || strings.HasSuffix(u.Host, ":") {
		return "", invalid
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", invalid
		}
	}
	prefix := strings.TrimSuffix(u.Path, "/")
	if prefix != "" && (path.Clean(prefix) != prefix || strings.Contains(prefix, "//")) {
		return "", invalid
	}
	if !strings.HasSuffix(prefix, "/anthropic") {
		prefix += "/anthropic"
	}
	for _, segment := range strings.Split(strings.TrimSuffix(prefix, "/anthropic"), "/") {
		if segment == "anthropic" {
			return "", invalid
		}
	}
	u.Path = prefix
	return u.String(), nil
}

const gatewaySetupAdvice = "Restart Claude Code. This configures workspace/provider-funded inference, not Claude subscription OAuth. The gateway must support X-LangSmith-Auth-Mode: oauth for helper credentials. Workspace defaults to the token's tenant_id; use --workspace if the token has no workspace context. Setup does not verify gateway availability or token validity. Review other settings layers and launch flags before use."
const gatewaySetupUndo = "To undo, remove or restore apiKeyHelper and env.ANTHROPIC_BASE_URL, env.CLAUDE_CODE_API_KEY_HELPER_TTL_MS, env.LANGSMITH_CONFIG_FILE; remove the X-LangSmith-Auth-Mode line and any X-Tenant-Id line added to env.ANTHROPIC_CUSTOM_HEADERS (retain unrelated headers). Remove or restore any ANTHROPIC_DEFAULT_HAIKU_MODEL, ANTHROPIC_DEFAULT_SONNET_MODEL, ANTHROPIC_DEFAULT_OPUS_MODEL, ANTHROPIC_DEFAULT_FABLE_MODEL overrides written by setup. Restore any replaced values from your own backup."

func runGatewaySetupClaudeCode(cmd *cobra.Command, scope, rawURL string, yes, dryRun bool) error {
	if runtime.GOOS == "windows" {
		return errors.New("gateway setup claude-code requires a POSIX shell; Windows command quoting is not supported")
	}
	if GetFormat() != "pretty" && GetFormat() != "json" {
		return errors.New("--format must be pretty or json")
	}
	settingsPath, err := claudeSettingsPath(scope)
	if err != nil {
		return err
	}
	settingsPath, err = filepath.Abs(settingsPath)
	if err != nil {
		return err
	}
	configPath, err := lsconfig.DefaultConfigPath()
	if err != nil {
		return err
	}
	configPath, err = filepath.Abs(configPath)
	if err != nil {
		return err
	}
	if settingsPath == configPath {
		return errors.New("Claude settings and LANGSMITH_CONFIG_FILE must be different files")
	}
	// Also reject hard links and symlink aliases to the credential store.
	if settingsInfo, err := os.Stat(settingsPath); err == nil {
		if configInfo, err := os.Stat(configPath); err == nil && os.SameFile(settingsInfo, configInfo) {
			return errors.New("Claude settings and LANGSMITH_CONFIG_FILE must be different files")
		}
	}
	// Load a snapshot only: no config transaction, lock file, refresh or discovery.
	cfg, err := lsconfig.LoadFrom(configPath)
	if err != nil {
		return err
	}
	name, profile, found := cfg.ResolveProfile(flagProfile, strings.TrimSpace(os.Getenv("LANGSMITH_PROFILE")))
	if !found || (profile.AccessToken() == "" && profile.OAuth.RefreshToken == "") {
		return errors.New("selected profile has no OAuth credentials; select an OAuth profile with --profile or run 'langsmith auth login'")
	}
	if rawURL == "" {
		// Conservatively require an explicit destination if ANY API selection is not
		// the verified US SaaS API, even when another selection overrides it.
		for _, api := range []string{profile.APIURL, flagAPIURL, os.Getenv("LANGSMITH_ENDPOINT")} {
			if api != "" && strings.TrimSuffix(api, "/") != lsconfig.DefaultAPIURL {
				return errors.New("non-default LangSmith API requires an explicit trusted --gateway-url; no gateway mapping is inferred")
			}
		}
		rawURL = "https://gateway.smith.langchain.com"
	}
	baseURL, err := gatewayAnthropicURL(rawURL)
	if err != nil {
		return err
	}
	models, err := gatewayModelOverrides(cmd)
	if err != nil {
		return err
	}
	if len(models) > 0 {
		// gatewayAnthropicURL validates and normalizes the deployment prefix;
		// remove exactly its final passthrough component for provider slugs.
		baseURL = strings.TrimSuffix(baseURL, "/anthropic")
	}
	// Resolve once from the setup process, not from Claude's saved environment.
	// Match auth token's flag > environment > profile > default precedence.
	apiURL := lsconfig.DefaultAPIURL
	for _, value := range []string{profile.APIURL, strings.TrimSpace(os.Getenv("LANGSMITH_ENDPOINT")), flagAPIURL} {
		if value != "" {
			apiURL = value
		}
	}
	apiURL = client.NormalizeURL(apiURL)
	refreshAuthority := apiURL
	if profile.OAuth.Issuer != "" {
		refreshAuthority = profile.OAuth.Issuer
	}
	// The OAuth token supplies workspace context unless explicitly overridden.
	// Do not snapshot a profile/shell workspace that may differ from the token.
	workspace := ""
	if cmd.Flags().Changed("workspace") || cmd.Flags().Changed("workspace-id") {
		workspace = flagWorkspaceID
		if validateWorkspaceID(workspace) != nil {
			return errors.New("invalid workspace ID: --workspace requires a nonempty UUID")
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable: %w", err)
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}
	if strings.ContainsRune(name, 0) || strings.ContainsRune(exe, 0) || strings.ContainsRune(apiURL, 0) {
		return errors.New("profile, executable path and API URL must not contain NUL characters")
	}
	helper := gatewayHelper(exe, name, apiURL)
	if scope == "project" {
		if err := assertSafeProjectPath(settingsPath); err != nil {
			return err
		}
	}
	original, doc, err := gatewayReadSettings(settingsPath)
	if err != nil {
		return err
	}
	env, err := gatewaySettingsEnv(doc)
	if err != nil {
		return err
	}
	if err := gatewayCredentialConflicts(env, os.Getenv); err != nil {
		return err
	}
	headers, headerNames, err := gatewayMergeHeaders(env["ANTHROPIC_CUSTOM_HEADERS"], workspace)
	if err != nil {
		return err
	}
	// Do not silently persist or forward inherited header values (which may be
	// secrets). Identical saved headers are already reviewed in the target file.
	inheritedHeaders := os.Getenv("ANTHROPIC_CUSTOM_HEADERS")
	if _, _, err := gatewayMergeHeaders(inheritedHeaders, workspace); err != nil {
		return err
	}
	if inheritedHeaders != "" && inheritedHeaders != headers {
		return errors.New("unset inherited ANTHROPIC_CUSTOM_HEADERS; explicitly review and save non-auth headers in the target settings before rerunning")
	}
	hasOAuthMode := false
	for _, name := range headerNames {
		if strings.EqualFold(name, "X-LangSmith-Auth-Mode") {
			hasOAuthMode = true
		}
	}
	if !hasOAuthMode {
		if headers != "" && !strings.HasSuffix(headers, "\n") {
			headers += "\n"
		}
		headers += "X-LangSmith-Auth-Mode: oauth"
		headerNames = append(headerNames, "X-LangSmith-Auth-Mode")
		sort.Strings(headerNames)
	}
	updates := map[string]string{
		"ANTHROPIC_BASE_URL":                baseURL,
		"CLAUDE_CODE_API_KEY_HELPER_TTL_MS": "30000",
		"LANGSMITH_CONFIG_FILE":             configPath,
	}
	if headers != "" {
		updates["ANTHROPIC_CUSTOM_HEADERS"] = headers
	}
	for key, value := range models {
		updates[key] = value
	}
	// Saved alternate config paths must be removed deliberately, not repointed.
	if v := env["LANGSMITH_CONFIG_FILE"]; v != "" && v != configPath {
		return errors.New("remove conflicting env.LANGSMITH_CONFIG_FILE from Claude settings before rerunning")
	}
	if v, present := env["CLAUDE_CONFIG_DIR"]; present && v != os.Getenv("CLAUDE_CONFIG_DIR") {
		return errors.New("remove conflicting env.CLAUDE_CONFIG_DIR from Claude settings before rerunning")
	}
	if v := os.Getenv("ANTHROPIC_BASE_URL"); v != "" && v != baseURL {
		return errors.New("unset conflicting inherited ANTHROPIC_BASE_URL before rerunning")
	}
	if err := gatewayCheckOtherSettings(settingsPath, helper, updates); err != nil {
		return err
	}
	removed := []string{}
	if len(models) == 0 {
		for _, family := range gatewayModelFamilies {
			key := gatewayModelEnv(family)
			if _, present := env[key]; present {
				delete(env, key)
				removed = append(removed, "env."+key)
			}
		}
	}
	sort.Strings(removed)
	replacements := []string{}
	if old, ok := doc["apiKeyHelper"]; ok {
		var v string
		if bytes.Equal(bytes.TrimSpace(old), []byte("null")) || json.Unmarshal(old, &v) != nil {
			return errors.New("settings apiKeyHelper must be a string")
		}
		if v != helper {
			replacements = append(replacements, "apiKeyHelper")
		}
	}
	for k, v := range updates {
		if old, ok := env[k]; ok && old != v {
			replacements = append(replacements, "env."+k)
		}
		env[k] = v
	}
	sort.Strings(replacements)
	doc["apiKeyHelper"], _ = json.Marshal(helper)
	doc["env"], _ = json.Marshal(env)
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	// Report only our new values, never the existing document/helper/header values.
	safeEnv := map[string]string{}
	for k, v := range updates {
		if k != "ANTHROPIC_CUSTOM_HEADERS" {
			safeEnv[k] = v
		}
	}
	result := map[string]any{
		"status": "dry-run", "agent": "claude-code", "scope": scope, "settings_path": settingsPath,
		"profile": name, "apiKeyHelper": helper, "env": safeEnv, "custom_header_names": headerNames,
		"replaced_keys": replacements, "removed_keys": removed, "workspace_id": workspace,
		"api_url": apiURL, "oauth_refresh_authority": refreshAuthority,
		"consent": "Trust this destination to receive LangSmith OAuth credentials, prompts and model traffic: " + baseURL + "; trust this OAuth authority for token refresh and endpoint discovery: " + refreshAuthority,
		"notes":   gatewaySetupAdvice, "undo": gatewaySetupUndo,
	}
	previewBytes, _ := json.MarshalIndent(result, "", "  ")
	preview := "Will configure Claude Code gateway access (existing custom header values are not displayed):\n" + string(previewBytes) + "\n"
	if dryRun {
		if GetFormat() == "json" {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), preview)
		return err
	}
	// confirmApply omits previews in JSON mode; keep interactive consent visible
	// on stderr without contaminating machine-readable stdout.
	if GetFormat() == "json" && !yes {
		fmt.Fprint(cmd.ErrOrStderr(), preview)
	}
	ok, err := confirmApply(cmd, yes, preview)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("aborted")
	}
	if err := gatewayWriteSettings(settingsPath, original, data, scope == "project"); err != nil {
		return err
	}
	result["status"] = "configured"
	if GetFormat() == "json" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "\nConfigured %s\n%s\n%s\n", settingsPath, gatewaySetupAdvice, gatewaySetupUndo)
	return err
}
