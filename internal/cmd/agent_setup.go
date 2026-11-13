package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

// Coordinates of the LangSmith skills package and the hosted MCP server that
// `langsmith setup` installs into coding agents.
const (
	skillsRepo            = "langchain-ai/langsmith-skills"
	skillsMarketplaceURL  = "https://github.com/langchain-ai/langsmith-skills.git"
	skillsClaudePluginRef = "langsmith-skills@langsmith-skills"
	mcpServerName         = "langsmith"
	defaultSkillsRef      = "main"
)

// Step actions reported in the plan and the JSON output.
const (
	stepCreate    = "create"
	stepUpdate    = "update"
	stepUnchanged = "unchanged"
	stepSkip      = "skip"
	stepFailed    = "failed"
)

// setupLookPath finds agent binaries on PATH. Tests replace it so detection
// never depends on what the test machine has installed.
var setupLookPath = exec.LookPath

// setupStep is one change `langsmith setup` makes (or would make) to one agent.
type setupStep struct {
	Agent     string `json:"agent"`
	Component string `json:"component"`
	Target    string `json:"target"`
	Action    string `json:"action"`
	Detail    string `json:"detail,omitempty"`
	Error     string `json:"error,omitempty"`

	apply func(context.Context) error
}

func (s setupStep) changes() bool {
	return s.Action == stepCreate || s.Action == stepUpdate
}

// setupPlanEnv carries the inputs shared by every agent's planner.
type setupPlanEnv struct {
	mcpURL    string
	force     bool
	noSkills  bool
	noMCP     bool
	skillsRef string
	skills    func(context.Context) (*skillsPackage, error)
	// mcpOwned maps an agent config file to the MCP URL setup last wrote there. An entry
	// still at that URL is setup's to replace, e.g. after a profile switch.
	mcpOwned map[string]string
}

// setupAgent is one coding agent `langsmith setup` knows how to configure.
type setupAgent struct {
	id       string
	name     string
	detect   func() bool
	plan     func(context.Context, *setupPlanEnv) []setupStep
	nextStep string
}

func setupAgents() []setupAgent {
	return []setupAgent{
		{
			id:       "claude-code",
			name:     "Claude Code",
			detect:   detectClaudeCode,
			plan:     planClaudeCode,
			nextStep: "In Claude Code, run /mcp and choose langsmith to sign in.",
		},
		{
			id:       "cursor",
			name:     "Cursor",
			detect:   detectCursor,
			plan:     planCursor,
			nextStep: "In Cursor, open Settings > MCP and sign in to langsmith.",
		},
		{
			id:       "codex",
			name:     "Codex",
			detect:   detectCodex,
			plan:     planCodex,
			nextStep: "Run `codex mcp login langsmith` to sign in.",
		},
	}
}

func setupAgentIDs() []string {
	var ids []string
	for _, a := range setupAgents() {
		ids = append(ids, a.id)
	}
	return ids
}

func newAgentSetupCmd() *cobra.Command {
	var (
		agents    []string
		yes       bool
		dryRun    bool
		force     bool
		noSkills  bool
		noMCP     bool
		mcpURL    string
		skillsRef string
	)
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Install LangSmith skills and the LangSmith MCP server into your coding agents",
		Long: `Install the LangSmith skills and the hosted LangSmith MCP server into the coding
agents on this machine, so they can query traces, build datasets, and run
evaluations.

It detects Claude Code, Cursor, and Codex from their config directories and
binaries, shows what it will change, and asks before writing. Running it again
changes nothing that is already set up. It never replaces an MCP server entry or
a skill directory it didn't write unless you pass --force.

  Claude Code  skills: the langsmith-skills plugin (via the claude CLI)
               MCP:    'langsmith' server, user scope (via the claude CLI)
  Cursor       skills: ~/.cursor/skills/<skill>
               MCP:    'langsmith' in ~/.cursor/mcp.json
  Codex        skills: ~/.codex/skills/<skill>
               MCP:    [mcp_servers.langsmith] in ~/.codex/config.toml

The MCP server URL comes from the selected profile or --api-url
(https://api.smith.langchain.com/mcp by default; https://<host>/api/mcp for a
self-hosted install), or set it with --mcp-url. No API key is written; each
agent signs in to the server with OAuth.

Tracing is separate: run 'langsmith trace setup' to send your agent sessions to
LangSmith.

  langsmith setup                      # detect agents, preview, confirm
  langsmith setup --dry-run            # show the plan only
  langsmith setup --agent cursor --yes # one agent, no prompt
  langsmith setup --no-skills          # MCP server only`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedMCP, err := resolveSetupMCPURL(mcpURL)
			if err != nil {
				return err
			}
			if noSkills && noMCP {
				return errors.New("nothing to do: --no-skills and --no-mcp were both passed")
			}
			if err := validateSkillsRef(skillsRef); err != nil {
				return err
			}
			selected, err := selectSetupAgents(agents)
			if err != nil {
				return err
			}
			env := &setupPlanEnv{
				mcpURL:    resolvedMCP,
				force:     force,
				noSkills:  noSkills,
				noMCP:     noMCP,
				skillsRef: skillsRef,
				skills:    cachedSkillsLoader(skillsRef),
				mcpOwned:  loadSetupState().MCP,
			}
			return runAgentSetup(cmd, selected, env, yes, dryRun)
		},
	}
	cmd.Flags().StringSliceVar(&agents, "agent", nil, "Agent to set up, repeatable: "+strings.Join(setupAgentIDs(), ", ")+" (default: every detected agent)")
	cmd.Flags().BoolVar(&yes, "yes", false, "Apply without the confirmation prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would change without writing anything")
	cmd.Flags().BoolVar(&force, "force", false, "Replace a 'langsmith' MCP entry or a skill directory that setup didn't write")
	cmd.Flags().BoolVar(&noSkills, "no-skills", false, "Skip installing the LangSmith skills")
	cmd.Flags().BoolVar(&noMCP, "no-mcp", false, "Skip configuring the LangSmith MCP server")
	cmd.Flags().StringVar(&mcpURL, "mcp-url", "", "LangSmith MCP server URL (default: derived from the API URL)")
	cmd.Flags().StringVar(&skillsRef, "skills-ref", defaultSkillsRef, "Git ref of "+skillsRepo+" to install skills from (Cursor and Codex; Claude Code uses the published plugin)")
	return cmd
}

// resolveSetupMCPURL returns the hosted MCP endpoint: the explicit override, or
// one derived from the resolved API URL. smith-go serves MCP at /mcp on the
// LangSmith Cloud API hosts; self-hosted and BYOC installs expose it under the
// /api prefix, the same rule as smith-go's urlx.MCPResourceURLForAPIBase.
func resolveSetupMCPURL(override string) (string, error) {
	if override = strings.TrimSpace(override); override != "" {
		u, err := url.Parse(override)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return "", fmt.Errorf("invalid --mcp-url %q: must be an absolute http(s) URL", override)
		}
		if u.Scheme == "http" && !isLoopbackHost(u.Hostname()) {
			return "", fmt.Errorf("invalid --mcp-url %q: use https; plain http is only allowed for localhost", override)
		}
		return override, nil
	}
	return mcpURLForAPIBase(GetAPIURL())
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func mcpURLForAPIBase(apiURL string) (string, error) {
	base := client.NormalizeURL(strings.TrimSpace(apiURL))
	if base == "" {
		base = lsconfig.DefaultAPIURL
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("cannot derive the MCP URL from API URL %q; pass --mcp-url", apiURL)
	}
	path := strings.TrimRight(u.Path, "/")
	switch {
	case path != "":
		u.Path = path + "/mcp"
	case isCloudAPIHost(u.Hostname()):
		u.Path = "/mcp"
	default:
		u.Path = "/api/mcp"
	}
	u.RawQuery, u.Fragment = "", ""
	return u.String(), nil
}

// isCloudAPIHost matches LangSmith Cloud API hosts such as
// api.smith.langchain.com and eu.api.smith.langchain.com.
func isCloudAPIHost(host string) bool {
	host = strings.ToLower(host)
	return host == "api.smith.langchain.com" || strings.HasSuffix(host, ".api.smith.langchain.com")
}

func selectSetupAgents(requested []string) ([]setupAgent, error) {
	all := setupAgents()
	if len(requested) == 0 {
		var found []setupAgent
		for _, a := range all {
			if a.detect() {
				found = append(found, a)
			}
		}
		if len(found) == 0 {
			return nil, fmt.Errorf("no supported coding agent found (looked for %s); pass --agent to choose one", strings.Join(setupAgentIDs(), ", "))
		}
		return found, nil
	}
	var selected []setupAgent
	for _, raw := range requested {
		id := strings.ToLower(strings.TrimSpace(raw))
		if id == "claude" {
			id = "claude-code"
		}
		idx := slices.IndexFunc(all, func(a setupAgent) bool { return a.id == id })
		if idx < 0 {
			return nil, fmt.Errorf("unknown --agent %q: must be one of %s", raw, strings.Join(setupAgentIDs(), ", "))
		}
		if !slices.ContainsFunc(selected, func(a setupAgent) bool { return a.id == id }) {
			selected = append(selected, all[idx])
		}
	}
	return selected, nil
}

func runAgentSetup(cmd *cobra.Command, agents []setupAgent, env *setupPlanEnv, yes, dryRun bool) error {
	ctx := cmd.Context()
	var steps []setupStep
	for _, a := range agents {
		steps = append(steps, a.plan(ctx, env)...)
	}
	pending := slices.ContainsFunc(steps, setupStep.changes)

	if dryRun || !pending {
		status := "dry_run"
		switch {
		case dryRun:
		case slices.ContainsFunc(steps, func(s setupStep) bool { return s.Action == stepFailed }):
			status = "failed"
		case slices.ContainsFunc(steps, func(s setupStep) bool { return s.Action == stepSkip }):
			status = "incomplete"
		default:
			status = "unchanged"
		}
		return reportAgentSetup(cmd, agents, env, steps, status, dryRun)
	}

	preview := renderSetupPlan(steps, env) + "\n"
	ok, err := confirmApply(cmd, yes, preview)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("aborted")
	}

	for i := range steps {
		if !steps[i].changes() || steps[i].apply == nil {
			continue
		}
		if err := steps[i].apply(ctx); err != nil {
			steps[i].Action = stepFailed
			steps[i].Error = err.Error()
			continue
		}
		if steps[i].Component == "mcp" {
			if err := recordSetupMCP(steps[i].Target, env.mcpURL); err != nil {
				steps[i].Detail = strings.TrimSpace(steps[i].Detail + "; warning: " + err.Error())
			}
		}
	}
	return reportAgentSetup(cmd, agents, env, steps, "applied", false)
}

func renderSetupPlan(steps []setupStep, env *setupPlanEnv) string {
	var sb strings.Builder
	if !env.noMCP {
		fmt.Fprintf(&sb, "LangSmith MCP server: %s\n", env.mcpURL)
	}
	if !env.noSkills {
		fmt.Fprintf(&sb, "LangSmith skills:     github.com/%s@%s\n", skillsRepo, env.skillsRef)
	}
	sb.WriteString("\n")
	tw := tabwriter.NewWriter(&sb, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "AGENT\tCOMPONENT\tACTION\tTARGET")
	for _, s := range steps {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", s.Agent, s.Component, s.Action, s.Target)
		if s.Detail != "" {
			fmt.Fprintf(tw, "\t\t\t  %s\n", s.Detail)
		}
		if s.Error != "" {
			fmt.Fprintf(tw, "\t\t\t  error: %s\n", s.Error)
		}
	}
	_ = tw.Flush()
	return sb.String()
}

func reportAgentSetup(cmd *cobra.Command, agents []setupAgent, env *setupPlanEnv, steps []setupStep, status string, dryRun bool) error {
	failed := slices.ContainsFunc(steps, func(s setupStep) bool { return s.Action == stepFailed })
	if GetFormat() == "pretty" {
		out := cmd.OutOrStdout()
		if status == "applied" {
			fmt.Fprint(out, "\nResult:\n")
		}
		fmt.Fprint(out, renderSetupPlan(steps, env))
		switch {
		case dryRun:
			fmt.Fprintln(out, "\nDry run: nothing was written. Rerun without --dry-run to apply.")
		case failed:
			fmt.Fprintln(out, "\nSome steps failed; see the errors above.")
		case slices.ContainsFunc(steps, func(s setupStep) bool { return s.Action == stepSkip }):
			fmt.Fprintln(out, "\nSome steps were skipped; see the details above.")
		case status == "unchanged":
			fmt.Fprintln(out, "\nEverything is already set up.")
		}
		if !dryRun && !env.noMCP {
			fmt.Fprintln(out, "\nNext steps:")
			for _, a := range agents {
				fmt.Fprintf(out, "  %s: %s\n", a.name, a.nextStep)
			}
		}
		fmt.Fprintln(out, "\nTo trace your agent sessions to LangSmith, run: langsmith trace setup")
	} else {
		result := map[string]any{
			"status": status,
			"steps":  steps,
		}
		if !env.noMCP {
			result["mcp_url"] = env.mcpURL
		}
		if !env.noSkills {
			result["skills_source"] = "github.com/" + skillsRepo + "@" + env.skillsRef
		}
		if err := output.WriteJSON(cmd.OutOrStdout(), result); err != nil {
			return err
		}
	}
	if failed {
		return ReportedError{Err: errors.New("langsmith setup: one or more steps failed")}
	}
	return nil
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func onPath(bin string) bool {
	_, err := setupLookPath(bin)
	return err == nil
}

// --- Claude Code -----------------------------------------------------------

func detectClaudeCode() bool {
	if dir, err := claudeConfigDir(); err == nil && pathExists(dir) {
		return true
	}
	return onPath(claudeBinary())
}

// claudeUserConfigPath is the file holding Claude Code's user-scope MCP
// servers: $CLAUDE_CONFIG_DIR/.claude.json when that is set, else ~/.claude.json.
func claudeUserConfigPath() (string, error) {
	if d := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR")); d != "" {
		return filepath.Join(d, ".claude.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".claude.json"), nil
}

// planClaudeCode goes through the claude CLI for both components. Claude Code
// rewrites ~/.claude.json while it runs, so setup only reads that file and
// lets `claude mcp` make the change.
func planClaudeCode(_ context.Context, env *setupPlanEnv) []setupStep {
	const agent = "claude-code"
	bin := claudeBinary()
	hasBin := onPath(bin)
	missingBin := fmt.Sprintf("%s CLI not found on PATH; install Claude Code and rerun", bin)
	var steps []setupStep

	if !env.noSkills {
		step := setupStep{Agent: agent, Component: "skills", Target: "plugin " + skillsClaudePluginRef}
		dir, _ := claudeConfigDir()
		switch {
		case claudePluginInstalled(dir, skillsClaudePluginRef):
			step.Action = stepUnchanged
		case dir != "" && pathExists(filepath.Join(dir, "skills", "langsmith-trace", "SKILL.md")):
			step.Action = stepUnchanged
			step.Detail = "LangSmith skills already installed in " + filepath.Join(dir, "skills")
		case env.skillsRef != defaultSkillsRef:
			step.Action = stepSkip
			step.Detail = "--skills-ref applies to Cursor and Codex; Claude Code installs the plugin's published version, so rerun without --skills-ref for Claude Code"
		case !hasBin:
			step.Action = stepSkip
			step.Detail = missingBin
		default:
			step.Action = stepCreate
			step.Detail = fmt.Sprintf("%s plugin marketplace add %s && %s plugin install %s --scope user", bin, skillsMarketplaceURL, bin, skillsClaudePluginRef)
			step.apply = func(ctx context.Context) error {
				if err := runSetupCommand(ctx, bin, "plugin", "marketplace", "add", skillsMarketplaceURL); err != nil {
					return fmt.Errorf("%s plugin marketplace add: %w", bin, err)
				}
				if err := runSetupCommand(ctx, bin, "plugin", "install", skillsClaudePluginRef, "--scope", "user"); err != nil {
					return fmt.Errorf("%s plugin install: %w", bin, err)
				}
				return nil
			}
		}
		steps = append(steps, step)
	}

	if !env.noMCP {
		cfgPath, err := claudeUserConfigPath()
		step := setupStep{Agent: agent, Component: "mcp", Target: cfgPath}
		existing, pristine, readErr := "", false, error(nil)
		if err == nil {
			existing, pristine, readErr = claudeMCPServerURL(cfgPath)
		}
		addArgs := []string{"mcp", "add", "--transport", "http", "--scope", "user", mcpServerName, env.mcpURL}
		add := func(ctx context.Context) error {
			if err := runSetupCommand(ctx, bin, addArgs...); err != nil {
				return fmt.Errorf("%s mcp add: %w", bin, err)
			}
			return nil
		}
		switch {
		case err != nil:
			step.Action, step.Error = stepFailed, err.Error()
		case readErr != nil:
			step.Action, step.Error = stepFailed, readErr.Error()
		case existing == env.mcpURL:
			step.Action = stepUnchanged
		case existing != "" && !env.canReplaceMCP(cfgPath, existing, pristine):
			step.Action = stepSkip
			step.Detail = mcpConflictDetail(existing)
		case !hasBin:
			step.Action = stepSkip
			step.Detail = missingBin
		case existing != "":
			step.Action = stepUpdate
			step.Detail = fmt.Sprintf("%s → %s", existing, env.mcpURL)
			step.apply = func(ctx context.Context) error {
				if err := runSetupCommand(ctx, bin, "mcp", "remove", "--scope", "user", mcpServerName); err != nil {
					return fmt.Errorf("%s mcp remove: %w", bin, err)
				}
				return add(ctx)
			}
		default:
			step.Action = stepCreate
			step.Detail = bin + " " + strings.Join(addArgs, " ")
			step.apply = add
		}
		steps = append(steps, step)
	}
	return steps
}

// claudePluginInstalled reports whether Claude Code's plugin registry lists ref.
func claudePluginInstalled(claudeDir, ref string) bool {
	if claudeDir == "" {
		return false
	}
	data, err := os.ReadFile(filepath.Join(claudeDir, "plugins", "installed_plugins.json"))
	if err != nil {
		return false
	}
	var doc struct {
		Plugins map[string]json.RawMessage `json:"plugins"`
	}
	if json.Unmarshal(data, &doc) != nil {
		return false
	}
	raw, ok := doc.Plugins[ref]
	if !ok {
		return false
	}
	// Newer registries list one entry per install, each with its scope; only a
	// user-scope install makes the skills available in every project.
	var installs []struct {
		Scope string `json:"scope"`
	}
	if json.Unmarshal(raw, &installs) != nil {
		return true // older registry: one entry per plugin, installed for the user
	}
	return slices.ContainsFunc(installs, func(i struct {
		Scope string `json:"scope"`
	}) bool {
		return i.Scope == "" || i.Scope == "user"
	})
}

// claudeMCPServerURL returns the URL of the "langsmith" MCP server in a Claude
// Code or Cursor config, or "" when there is none. pristine reports whether the
// entry holds nothing beyond what setup writes (url, and Claude Code's type).
func claudeMCPServerURL(path string) (existing string, pristine bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("reading %s: %w", path, err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return "", false, nil
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return "", false, fmt.Errorf("parsing %s: %w", path, err)
	}
	servers, _ := doc["mcpServers"].(map[string]any)
	existing, pristine = mcpEntryURL(servers[mcpServerName], servers != nil && servers[mcpServerName] != nil)
	return existing, pristine, nil
}

// mcpEntryURL describes an MCP server entry by its URL. A stdio entry is
// described by its command so it still counts as an existing, different entry.
// pristine is true only for an entry with no settings beyond url and type.
func mcpEntryURL(raw any, present bool) (string, bool) {
	if !present {
		return "", false
	}
	entry, ok := raw.(map[string]any)
	if !ok {
		return "(unrecognized entry)", false
	}
	pristine := true
	for k := range entry {
		if k != "url" && k != "type" {
			pristine = false
		}
	}
	if u, _ := entry["url"].(string); u != "" {
		return u, pristine
	}
	if c, _ := entry["command"].(string); c != "" {
		return "command " + c, false
	}
	return "(entry without a url)", false
}

// --- Cursor ----------------------------------------------------------------

func cursorHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".cursor"), nil
}

func detectCursor() bool {
	if dir, err := cursorHome(); err == nil && pathExists(dir) {
		return true
	}
	return onPath("cursor") || onPath("cursor-agent")
}

func planCursor(ctx context.Context, env *setupPlanEnv) []setupStep {
	const agent = "cursor"
	dir, err := cursorHome()
	if err != nil {
		return []setupStep{{Agent: agent, Component: "config", Action: stepFailed, Error: err.Error()}}
	}
	var steps []setupStep
	if !env.noSkills {
		steps = append(steps, planSkillsDir(ctx, env, agent, filepath.Join(dir, "skills")))
	}
	if !env.noMCP {
		steps = append(steps, planJSONMCP(env, agent, filepath.Join(dir, "mcp.json")))
	}
	return steps
}

// planJSONMCP sets mcpServers.langsmith = {"url": ...} in a JSON config,
// keeping every other key.
func planJSONMCP(env *setupPlanEnv, agent, path string) setupStep {
	step := setupStep{Agent: agent, Component: "mcp", Target: path}
	existing, pristine, err := claudeMCPServerURL(path)
	switch {
	case err != nil:
		step.Action, step.Error = stepFailed, err.Error()
		return step
	case existing == env.mcpURL:
		step.Action = stepUnchanged
		return step
	case existing != "" && !env.canReplaceMCP(path, existing, pristine):
		step.Action = stepSkip
		step.Detail = mcpConflictDetail(existing)
		return step
	case existing != "":
		step.Action = stepUpdate
		step.Detail = fmt.Sprintf("%s → %s", existing, env.mcpURL)
	default:
		step.Action = stepCreate
		step.Detail = fmt.Sprintf(`add mcpServers.%s = {"url": %q}`, mcpServerName, env.mcpURL)
	}
	step.apply = func(context.Context) error {
		return mergeJSONFile(path, false, func(doc map[string]any) error {
			jsonObject(doc, "mcpServers")[mcpServerName] = map[string]any{"url": env.mcpURL}
			return nil
		})
	}
	return step
}

// --- Codex -----------------------------------------------------------------

func detectCodex() bool {
	if dir, err := codexHome(); err == nil && pathExists(dir) {
		return true
	}
	return onPath(codexBinary())
}

func planCodex(ctx context.Context, env *setupPlanEnv) []setupStep {
	const agent = "codex"
	dir, err := codexHome()
	if err != nil {
		return []setupStep{{Agent: agent, Component: "config", Action: stepFailed, Error: err.Error()}}
	}
	var steps []setupStep
	if !env.noSkills {
		steps = append(steps, planSkillsDir(ctx, env, agent, filepath.Join(dir, "skills")))
	}
	if !env.noMCP {
		steps = append(steps, planCodexMCP(env, filepath.Join(dir, "config.toml")))
	}
	return steps
}

func planCodexMCP(env *setupPlanEnv, path string) setupStep {
	step := setupStep{Agent: "codex", Component: "mcp", Target: path}
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		step.Action, step.Error = stepFailed, fmt.Sprintf("reading %s: %v", path, err)
		return step
	}
	doc := map[string]any{}
	if len(strings.TrimSpace(string(raw))) > 0 {
		if err := toml.Unmarshal(raw, &doc); err != nil {
			step.Action, step.Error = stepFailed, fmt.Sprintf("parsing %s: %v", path, err)
			return step
		}
	}
	servers, _ := doc["mcp_servers"].(map[string]any)
	entryRaw, present := servers[mcpServerName]
	existing, pristine := mcpEntryURL(entryRaw, present)

	urlLine := fmt.Sprintf("url = %s", tomlString(env.mcpURL))
	switch {
	case existing == env.mcpURL:
		step.Action = stepUnchanged
		return step
	case present && !env.canReplaceMCP(path, existing, pristine):
		step.Action = stepSkip
		step.Detail = fmt.Sprintf("[mcp_servers.%s], which setup didn't write, already points to %s; pass --force to replace it", mcpServerName, existing)
		return step
	case present:
		step.Action = stepUpdate
		step.Detail = fmt.Sprintf("%s → %s", existing, env.mcpURL)
	default:
		step.Action = stepCreate
		step.Detail = fmt.Sprintf("add [mcp_servers.%s] %s", mcpServerName, urlLine)
	}
	step.apply = func(context.Context) error {
		table := "mcp_servers." + mcpServerName
		var text string
		if present {
			// Replace the entry's settings, including a stdio server's
			// command/args/env, keeping comments and the rest of the file.
			text = replaceTOMLTable(string(raw), []string{"mcp_servers", mcpServerName}, urlLine)
		} else {
			text = strings.TrimRight(string(raw), "\n")
			if text != "" {
				text += "\n\n"
			}
			text += "[" + table + "]\n" + urlLine + "\n"
		}
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		var check map[string]any
		if err := toml.Unmarshal([]byte(text), &check); err != nil {
			return fmt.Errorf("could not update %s in place (%v); set [%s] url = %s by hand", path, err, table, tomlString(env.mcpURL))
		}
		checkServers, _ := check["mcp_servers"].(map[string]any)
		checkEntry, _ := checkServers[mcpServerName].(map[string]any)
		if got, _ := checkEntry["url"].(string); got != env.mcpURL || len(checkEntry) != 1 {
			return fmt.Errorf("could not update %s in place; set [%s] url = %s by hand", path, table, tomlString(env.mcpURL))
		}
		// Everything outside the langsmith entry must read back exactly as before.
		if !reflect.DeepEqual(withoutMCPEntry(doc), withoutMCPEntry(check)) {
			return fmt.Errorf("could not update %s without changing other settings; set [%s] url = %s by hand", path, table, tomlString(env.mcpURL))
		}
		return writeConfigFile(path, []byte(text), false)
	}
	return step
}

// replaceTOMLTable replaces every key of [table] and drops its sub-tables
// ([table.*]), leaving body as the table's only setting. Comment and blank lines
// in the table stay. A table defined some other way (dotted keys, an inline
// table) is left untouched for the caller's validation to catch.
func replaceTOMLTable(text string, table []string, body string) string {
	lines := strings.Split(text, "\n")
	var out []string
	inTable, inSub, wrote := false, false, false
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "[") {
			keys := tomlHeaderKeys(t)
			inTable = slices.Equal(keys, table)
			inSub = len(keys) > len(table) && slices.Equal(keys[:len(table)], table)
			if inSub {
				continue
			}
			out = append(out, ln)
			if inTable && !wrote {
				out = append(out, body)
				wrote = true
			}
			continue
		}
		if inSub || (inTable && t != "" && !strings.HasPrefix(t, "#")) {
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

// tomlHeaderKeys returns the key segments of a [table] header line, so
// `[ mcp_servers."langsmith" ] # note` reads as [mcp_servers langsmith] and
// `[mcp_servers."langsmith.foo"]` as [mcp_servers langsmith.foo]. It returns
// nil for anything else, including [[array]] headers.
func tomlHeaderKeys(line string) []string {
	if !strings.HasPrefix(line, "[") || strings.HasPrefix(line, "[[") {
		return nil
	}
	var keys []string
	// Whitespace outside quotes is skipped below, so cur holds only key text and
	// a quoted key keeps its spaces.
	var cur strings.Builder
	i := 1
	for i < len(line) {
		c := line[i]
		switch {
		case c == '"' || c == '\'':
			j := i + 1
			for j < len(line) && line[j] != c {
				if c == '"' && line[j] == '\\' {
					j++
				}
				j++
			}
			if j >= len(line) {
				return nil
			}
			part := line[i+1 : j]
			if c == '"' {
				unq, err := strconv.Unquote(`"` + part + `"`)
				if err != nil {
					return nil
				}
				part = unq
			}
			cur.WriteString(part)
			i = j + 1
		case c == '.':
			keys = append(keys, cur.String())
			cur.Reset()
			i++
		case c == ']':
			keys = append(keys, cur.String())
			if rest := strings.TrimSpace(line[i+1:]); rest != "" && !strings.HasPrefix(rest, "#") {
				return nil
			}
			return keys
		case c == ' ' || c == '\t':
			i++
		default:
			cur.WriteByte(c)
			i++
		}
	}
	return nil
}

// withoutMCPEntry returns doc without mcp_servers.langsmith, leaving doc as is.
func withoutMCPEntry(doc map[string]any) map[string]any {
	out := make(map[string]any, len(doc))
	for k, v := range doc {
		out[k] = v
	}
	if servers, ok := doc["mcp_servers"].(map[string]any); ok {
		rest := make(map[string]any, len(servers))
		for k, v := range servers {
			if k != mcpServerName {
				rest[k] = v
			}
		}
		if len(rest) == 0 {
			delete(out, "mcp_servers")
		} else {
			out["mcp_servers"] = rest
		}
	}
	return out
}

// tomlString quotes s as a TOML basic string. JSON string escapes are a
// subset of TOML's, so the JSON encoding is valid TOML.
func tomlString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// canReplaceMCP reports whether setup may replace the "langsmith" MCP entry in
// the config file at path: with --force, or when the entry is still exactly
// what setup last wrote to that same file (same URL, no settings added since).
func (e *setupPlanEnv) canReplaceMCP(path, existing string, pristine bool) bool {
	return e.force || (pristine && existing != "" && e.mcpOwned[mcpOwnershipKey(path)] == existing)
}

// mcpOwnershipKey identifies an agent config file in the setup state, so an
// entry setup wrote to one file never makes the same entry in another file
// (after CODEX_HOME or CLAUDE_CONFIG_DIR changes) count as setup's.
func mcpOwnershipKey(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return filepath.Clean(path)
}

func mcpConflictDetail(existing string) string {
	return fmt.Sprintf("a %q MCP server that setup didn't write already points to %s; pass --force to replace it", mcpServerName, existing)
}

// setupState is what `langsmith setup` remembers between runs, stored next to
// the CLI config: the MCP URL it last wrote into each agent config file, keyed
// by the file's absolute path.
type setupState struct {
	MCP map[string]string `json:"mcp_by_config_file,omitempty"`
}

func setupStatePath() (string, error) {
	cfg, err := lsconfig.DefaultConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(cfg), "setup-state.json"), nil
}

// loadSetupState returns the saved state, or an empty one when it is missing
// or unreadable; without it setup just treats existing entries as the user's.
func loadSetupState() setupState {
	st := setupState{MCP: map[string]string{}}
	path, err := setupStatePath()
	if err != nil {
		return st
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return st
	}
	_ = json.Unmarshal(data, &st)
	if st.MCP == nil {
		st.MCP = map[string]string{}
	}
	return st
}

func recordSetupMCP(configPath, mcpURL string) error {
	path, err := setupStatePath()
	if err != nil {
		return err
	}
	return mergeJSONFile(path, false, func(doc map[string]any) error {
		jsonObject(doc, "mcp_by_config_file")[mcpOwnershipKey(configPath)] = mcpURL
		return nil
	})
}
