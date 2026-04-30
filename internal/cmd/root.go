package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/cmd/api"
	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
	"github.com/spf13/cobra"
)

// Global flag values stored here; accessed by subcommands via helpers.
var (
	flagAPIKey       string
	flagAPIURL       string
	flagProfile      string
	flagOutputFormat string
)

// NewRootCmd creates the top-level `langsmith` command.
func NewRootCmd(rawVersion, displayVersion string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "langsmith",
		Short: "LangSmith CLI — query and manage LangSmith resources",
		Long: `LangSmith CLI — query and manage LangSmith resources from the command line.

Designed for AI coding agents and developers who need fast, scriptable
access to traces, runs, datasets, evaluators, experiments, and threads.
All commands output JSON by default for easy parsing.

Authentication:
  Set LANGSMITH_API_KEY, pass --api-key, or select an API-key profile.
  Optionally set LANGSMITH_ENDPOINT for self-hosted instances.
  Use --profile or LANGSMITH_PROFILE to select a saved profile.
  Set a default workspace with 'langsmith profile set-workspace <workspace-id>'.
  Set LANGSMITH_PROJECT as a default project name for trace/run queries.

Quick start:
  langsmith project list
  langsmith trace list --project my-project --limit 5
  langsmith run list --project my-project --run-type llm --limit 10
  langsmith dataset list
  langsmith evaluator list
  langsmith experiment list --dataset my-eval-dataset

Output:
  --format json    Machine-readable JSON (default). Best for agents and scripts.
  --format pretty  Human-readable tables, trees, and syntax-highlighted JSON.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       displayVersion,
	}

	rootCmd.PersistentFlags().StringVar(&flagAPIKey, "api-key", "", "LangSmith API key [env: LANGSMITH_API_KEY]")
	rootCmd.PersistentFlags().StringVar(&flagAPIURL, "api-url", "", "LangSmith API URL [env: LANGSMITH_ENDPOINT]")
	rootCmd.PersistentFlags().StringVar(&flagProfile, "profile", "", "Named profile to use [env: LANGSMITH_PROFILE]")
	rootCmd.PersistentFlags().StringVar(&flagOutputFormat, "format", "json", "Output format: json or pretty")

	// Register all subcommand groups
	rootCmd.AddCommand(newProjectCmd())
	rootCmd.AddCommand(newTraceCmd())
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newThreadCmd())
	rootCmd.AddCommand(newDatasetCmd())
	rootCmd.AddCommand(newExampleCmd())
	rootCmd.AddCommand(newEvaluatorCmd())
	rootCmd.AddCommand(newExperimentCmd())
	rootCmd.AddCommand(newSandboxCmd())
	rootCmd.AddCommand(newInsightsCmd())
	rootCmd.AddCommand(newFleetCmd())
	rootCmd.AddCommand(newPromptCmd())
	rootCmd.AddCommand(newProfileCmd())
	rootCmd.AddCommand(newWorkspaceCmd())
	rootCmd.AddCommand(newUpdateCmd(rawVersion))
	rootCmd.AddCommand(api.NewCmd())

	return rootCmd
}

// GetAPIKey resolves the API key from flag → env → profile.
func GetAPIKey() string {
	opts, _ := resolveClientOptions()
	return opts.APIKey
}

// GetAPIURL resolves the API URL from flag → env → profile → default.
func GetAPIURL() string {
	opts, _ := resolveClientOptions()
	return opts.APIURL
}

// GetWorkspaceID resolves the workspace ID from env → profile.
func GetWorkspaceID() string {
	opts, _ := resolveClientOptions()
	return opts.WorkspaceID
}

// GetFormat returns the output format.
func GetFormat() string {
	return flagOutputFormat
}

// MustGetClient creates a LangSmith client or exits with an error.
func MustGetClient() *client.Client {
	opts, err := resolveClientOptions()
	if err != nil {
		ExitError(err.Error())
	}
	if opts.APIKey == "" {
		ExitError("not authenticated; set LANGSMITH_API_KEY, pass --api-key, or select an API-key profile")
	}
	return client.NewWithOptions(opts)
}

func resolveClientOptions() (client.Options, error) {
	opts := client.Options{APIURL: lsconfig.DefaultAPIURL}

	cfg, err := lsconfig.Load()
	var cfgErr error
	if err != nil {
		cfgErr = err
		cfg = &lsconfig.Config{Profiles: make(map[string]lsconfig.Profile)}
	}

	envProfile := strings.TrimSpace(os.Getenv("LANGSMITH_PROFILE"))
	profileName, profile, hasProfile := "", lsconfig.Profile{}, false
	if flagProfile != "" || envProfile != "" || cfgErr == nil {
		if cfgErr != nil && (flagProfile != "" || envProfile != "") {
			return opts, cfgErr
		}
		profileName, profile, hasProfile = cfg.ResolveProfile(flagProfile, envProfile)
		if (flagProfile != "" || envProfile != "") && !hasProfile {
			return opts, fmt.Errorf("profile not found: %s", profileName)
		}
	}

	if hasProfile {
		if profile.APIURL != "" {
			opts.APIURL = profile.APIURL
		}
		opts.WorkspaceID = profile.WorkspaceID
	}

	if v := os.Getenv("LANGSMITH_ENDPOINT"); v != "" {
		opts.APIURL = client.NormalizeURL(v)
	}
	if flagAPIURL != "" {
		opts.APIURL = client.NormalizeURL(flagAPIURL)
	}

	if v := os.Getenv("LANGSMITH_TENANT_ID"); v != "" {
		opts.WorkspaceID = v
	}
	if v := os.Getenv("LANGSMITH_WORKSPACE_ID"); v != "" {
		opts.WorkspaceID = v
	}
	switch {
	case flagAPIKey != "":
		opts.APIKey = flagAPIKey
	case os.Getenv("LANGSMITH_API_KEY") != "":
		opts.APIKey = os.Getenv("LANGSMITH_API_KEY")
	case hasProfile && profile.APIKey != "":
		opts.APIKey = profile.APIKey
	}
	if cfgErr != nil && opts.APIKey == "" {
		return opts, cfgErr
	}

	return opts, nil
}

// ExitError prints a JSON error to stderr and exits.
func ExitError(msg string) {
	fmt.Fprintf(os.Stderr, `{"error": %q}`+"\n", msg)
	os.Exit(1)
}

// ExitErrorf prints a formatted JSON error to stderr and exits.
func ExitErrorf(format string, args ...any) {
	ExitError(fmt.Sprintf(format, args...))
}
