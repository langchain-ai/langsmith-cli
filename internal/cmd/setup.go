package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
	"github.com/spf13/cobra"
)

// runSetupCommand runs an external agent CLI (claude/codex) to install the
// plugin. Indirected so tests can capture invocations without spawning binaries.
var runSetupCommand = runSetupCommandDefault

func runSetupCommandDefault(ctx context.Context, name string, args ...string) error {
	c := exec.CommandContext(ctx, name, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// confirmApply prints a preview of the changes and asks the user to proceed.
// With --yes it proceeds without prompting; in a non-interactive shell without
// --yes it refuses rather than hang.
func confirmApply(cmd *cobra.Command, yes bool, preview string) (bool, error) {
	if GetFormat() == "pretty" {
		fmt.Fprint(cmd.OutOrStdout(), preview)
	}
	if yes {
		return true, nil
	}
	if !inputIsTerminal(os.Stdin) {
		return false, errors.New("refusing to apply without confirmation; pass --yes to apply non-interactively")
	}
	fmt.Fprint(os.Stderr, "\nApply these changes? [y/N] ")
	var confirm string
	_, _ = fmt.Scanln(&confirm)
	return strings.ToLower(strings.TrimSpace(confirm)) == "y", nil
}

// newTraceSetupCmd is `langsmith trace setup`: configure coding agents to send
// traces to LangSmith. With no subcommand it tries both Claude Code and Codex
// (best-effort — an agent that isn't installed just fails its own install step).
func newTraceSetupCmd() *cobra.Command {
	var (
		project   string
		scope     string
		user      string
		email     string
		yes       bool
		noInstall bool
	)
	cmd := &cobra.Command{
		Use:   "setup [API_KEY] [API_URL] [PROJECT]",
		Short: "Configure coding agents (Claude Code, Codex) to trace to LangSmith",
		Long: `Configure coding agents to send full-content traces to LangSmith.

With no agent subcommand it configures both Claude Code and Codex; each is
best-effort, so an agent that isn't installed simply fails its own install step.
Use the claude/codex subcommands to target one.

  langsmith trace setup                         # try both Claude Code and Codex
  langsmith trace setup claude demo-key dev.smith.com shared-claude
  langsmith trace setup codex --yes

The API key, URL, and project may be positional args or come from
LANGSMITH_API_KEY / LANGSMITH_ENDPOINT / LANGSMITH_PROJECT, the --api-url
/--project flags, or a saved profile. A non-default URL accepts a bare host (dev.smith.com →
https://dev.smith.com). An API key is required; OAuth profiles are not supported.`,
		Args: cobra.MaximumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			cErr := runSetupClaude(cmd, args, project, scope, user, email, yes, noInstall)
			if cErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "claude setup failed: %v\n", cErr)
			}
			xErr := runSetupCodex(cmd, args, project, scope, user, email, yes, noInstall)
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
	cmd.Flags().StringVar(&user, "user", "", "Name attached to every trace (default: git user.name, else OS user)")
	cmd.Flags().StringVar(&email, "email", "", "Email attached to every trace (default: git user.email)")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the confirmation prompt")
	cmd.Flags().BoolVar(&noInstall, "no-install", false, "Write config only; do not run the agent's plugin install")
	cmd.AddCommand(newTraceSetupClaudeCmd())
	cmd.AddCommand(newTraceSetupCodexCmd())
	return cmd
}

// Env-var contract consumed by the LangSmith tracing plugins for coding agents.
// Claude Code uses the CC_LANGSMITH_* namespace exclusively; a custom endpoint
// is expressed via CC_LANGSMITH_RUNS_ENDPOINTS (a JSON replica array) rather
// than a plain endpoint var. Keep these in sync with langsmith-claude-code-plugins.
const (
	envTraceToLangSmith         = "TRACE_TO_LANGSMITH"
	envCCLangSmithAPIKey        = "CC_LANGSMITH_API_KEY"
	envCCLangSmithProject       = "CC_LANGSMITH_PROJECT"
	envCCLangSmithRunsEndpoints = "CC_LANGSMITH_RUNS_ENDPOINTS"
	envCCLangSmithMetadata      = "CC_LANGSMITH_METADATA"
)

// gitConfigValue returns a trimmed `git config --get <key>`, or "" on error.
func gitConfigValue(key string) string {
	out, err := exec.Command("git", "config", "--get", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// resolveUserMetadata builds the {user_name,user_email} metadata attached to
// every trace, identifying the developer who ran setup. Explicit flags win,
// then git config, then the OS user (name, else login).
func resolveUserMetadata(userFlag, emailFlag string) map[string]string {
	name := strings.TrimSpace(userFlag)
	if name == "" {
		name = gitConfigValue("user.name")
	}
	if name == "" {
		if u, err := user.Current(); err == nil {
			if n := strings.TrimSpace(u.Name); n != "" {
				name = n
			} else {
				name = u.Username
			}
		}
	}
	email := strings.TrimSpace(emailFlag)
	if email == "" {
		email = gitConfigValue("user.email")
	}
	md := map[string]string{}
	if name != "" {
		md["user_name"] = name
	}
	if email != "" {
		md["user_email"] = email
	}
	return md
}

// applyPositionalArgs lets the setup subcommands take the API key, URL, and
// project as positional args — `langsmith <agent> setup KEY URL PROJECT` —
// instead of flags. A positional fills its slot; passing both a positional and
// the matching flag for the same slot is an error.
func applyPositionalArgs(args []string, opts *client.Options, project *string) error {
	if len(args) >= 1 && strings.TrimSpace(args[0]) != "" {
		if flagAPIKey != "" {
			return errors.New("pass the API key as a positional argument or --api-key, not both")
		}
		opts.APIKey = strings.TrimSpace(args[0])
	}
	if len(args) >= 2 && strings.TrimSpace(args[1]) != "" {
		if flagAPIURL != "" {
			return errors.New("pass the API URL as a positional argument or --api-url, not both")
		}
		opts.APIURL = normalizeTraceURL(args[1])
	}
	if len(args) >= 3 && strings.TrimSpace(args[2]) != "" {
		if *project != "" {
			return errors.New("pass the project as a positional argument or --project, not both")
		}
		*project = strings.TrimSpace(args[2])
	}
	return nil
}

// normalizeTraceURL accepts a bare host (e.g. dev.smith.com) or a full URL and
// returns a scheme-qualified URL with any trailing /api/v1 stripped.
func normalizeTraceURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "https://" + raw
	}
	return client.NormalizeURL(raw)
}

// resolveSetupOptions resolves the API key, URL, and project for a setup run —
// from positional args (KEY URL PROJECT) or flags/env/profile — and requires a key.
func resolveSetupOptions(args []string, project *string) (client.Options, error) {
	// A config-load error (e.g. a corrupt ~/.langsmith/config.json) is deferred:
	// a positional API key applied below is a valid first-class input and must
	// not be blocked by an unusable saved config.
	opts, cfgErr := resolveClientOptions(false)
	if err := applyPositionalArgs(args, &opts, project); err != nil {
		return opts, err
	}
	if opts.APIKey == "" {
		if cfgErr != nil {
			return opts, cfgErr
		}
		return opts, errors.New("tracing setup requires a LangSmith API key; pass it as the first positional argument, via --api-key, $LANGSMITH_API_KEY, or a profile")
	}
	return opts, nil
}

// defaultTraceProject resolves the LangSmith project name from the environment,
// falling back to the per-agent default.
func defaultTraceProject(fallback string) string {
	for _, key := range []string{"LANGSMITH_AGENT_PROJECT", "LANGSMITH_PROJECT"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return fallback
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

// assertSafeProjectPath rejects any symlinked component of path and ensures the
// path stays within the working directory. Used for --scope project, where the
// working directory may be an untrusted (e.g. freshly cloned) repository that
// could plant a symlink — at the file or a parent dir such as ./.claude — to
// redirect the API-key write onto an arbitrary file.
func assertSafeProjectPath(path string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolving working directory: %w", err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(cwd, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("refusing to write outside the project directory: %s", abs)
	}
	cur := cwd
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if part == "" || part == "." {
			continue
		}
		cur = filepath.Join(cur, part)
		fi, err := os.Lstat(cur)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				break // remaining components don't exist yet — nothing to follow
			}
			return fmt.Errorf("stat %s: %w", cur, err)
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to follow symlinked path component: %s", cur)
		}
		if cur != abs && !fi.IsDir() {
			return fmt.Errorf("refusing to write under non-directory: %s", cur)
		}
	}
	return nil
}

// writeConfigFile writes data to path at owner-only (0600) permissions.
//
// For project scope (untrusted working directory) it rejects symlinked path
// components — at the file or any parent dir — and writes atomically via a temp
// file + rename so a planted symlink is never followed. For user scope it writes
// in place, following any symlink the user set up (e.g. a dotfile manager), and
// enforces 0600.
func writeConfigFile(path string, data []byte, projectScope bool) error {
	if projectScope {
		if err := assertSafeProjectPath(path); err != nil {
			return err
		}
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	if projectScope {
		if err := assertSafeProjectPath(path); err != nil {
			return err
		}
		return atomicWriteFile(dir, path, data)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	// WriteFile's mode only applies on create; tighten an existing file too.
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("securing %s: %w", path, err)
	}
	return nil
}

// atomicWriteFile writes data to a fresh 0600 temp file in dir and renames it
// onto path. Rename replaces a symlink (or file) at path rather than writing
// through it, neutralizing a planted leaf symlink even under TOCTOU.
func atomicWriteFile(dir, path string, data []byte) error {
	tmp, err := os.CreateTemp(dir, ".langsmith-setup-*")
	if err != nil {
		return fmt.Errorf("creating temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("securing %s: %w", tmpName, err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// mergeJSONFile reads path as a JSON object (empty when absent), applies mutate,
// and writes it back indented at owner-only (0600) permissions. Unknown keys in
// the existing file are preserved. For project scope it refuses symlinked path
// components before reading or writing.
func mergeJSONFile(path string, projectScope bool, mutate func(map[string]any) error) error {
	if projectScope {
		if err := assertSafeProjectPath(path); err != nil {
			return err
		}
	}
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
	return writeConfigFile(path, append(out, '\n'), projectScope)
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
