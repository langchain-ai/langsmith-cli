package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

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
	flagWorkspaceID  string
	flagOutputFormat string
)

// NewRootCmd creates the top-level `langsmith` command.
func NewRootCmd(rawVersion, displayVersion string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "langsmith",
		Short: "LangSmith CLI — query and manage LangSmith resources",
		Long: `LangSmith CLI — query and manage LangSmith resources from the command line.

	Designed for developers and AI coding agents who need fast access to traces,
	runs, datasets, evaluators, experiments, and threads.

Authentication:
  Run 'langsmith auth login', set LANGSMITH_API_KEY, or pass --api-key.
  Optionally set LANGSMITH_ENDPOINT for self-hosted instances.
  Use --profile or LANGSMITH_PROFILE to select a saved profile.
  Pass --workspace to target a specific workspace for one command.
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
	  --format pretty  Human-readable tables, trees, and syntax-highlighted JSON (default).
	  --format json    Machine-readable JSON for agents and scripts.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       displayVersion,
	}

	rootCmd.PersistentFlags().StringVar(&flagAPIKey, "api-key", "", "LangSmith API key [env: LANGSMITH_API_KEY]")
	rootCmd.PersistentFlags().StringVar(&flagAPIURL, "api-url", "", "LangSmith API URL [env: LANGSMITH_ENDPOINT]")
	rootCmd.PersistentFlags().StringVar(&flagProfile, "profile", "", "Named profile to use [env: LANGSMITH_PROFILE]")
	rootCmd.PersistentFlags().StringVar(&flagWorkspaceID, "workspace", "", "LangSmith workspace ID [env: LANGSMITH_WORKSPACE_ID]")
	rootCmd.PersistentFlags().StringVar(&flagWorkspaceID, "workspace-id", "", "LangSmith workspace ID [env: LANGSMITH_WORKSPACE_ID]")
	_ = rootCmd.PersistentFlags().MarkHidden("workspace-id")
	rootCmd.PersistentFlags().StringVar(&flagOutputFormat, "format", "pretty", "Output format: pretty or json")

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
	rootCmd.AddCommand(newHubCmd())
	rootCmd.AddCommand(newPromptCmd())
	rootCmd.AddCommand(authCommand.Cobra())
	rootCmd.AddCommand(newProfileCmd())
	rootCmd.AddCommand(newWorkspaceCmd())
	rootCmd.AddCommand(newUpdateCmd(rawVersion))
	rootCmd.AddCommand(api.NewCmd())

	return rootCmd
}

// GetAPIKey resolves the API key from flag → env → profile.
func GetAPIKey() string {
	opts, _ := resolveClientOptions(false)
	return opts.APIKey
}

// GetOAuthAccessToken resolves the access token from the active OAuth profile.
func GetOAuthAccessToken() string {
	opts, _ := resolveClientOptions(false)
	return opts.OAuthAccessToken
}

// GetAPIURL resolves the API URL from flag → env → profile → default.
func GetAPIURL() string {
	opts, _ := resolveClientOptions(false)
	return opts.APIURL
}

// GetWorkspaceID resolves the workspace ID from flag → env → profile.
func GetWorkspaceID() string {
	opts, _ := resolveClientOptions(false)
	return opts.WorkspaceID
}

// GetFormat returns the output format.
func GetFormat() string {
	return flagOutputFormat
}

// MustGetClient creates a LangSmith client or exits with an error.
func MustGetClient() *client.Client {
	opts, err := resolveClientOptions(true)
	if err != nil {
		ExitError(err.Error())
	}
	if opts.APIKey == "" && opts.OAuthAccessToken == "" {
		ExitError("not authenticated; run 'langsmith auth login', set LANGSMITH_API_KEY, or pass --api-key")
	}
	return client.NewWithOptions(opts)
}

func resolveClientOptions(refreshOAuth bool) (client.Options, error) {
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
	if flagWorkspaceID != "" {
		opts.WorkspaceID = flagWorkspaceID
	}
	switch {
	case flagAPIKey != "":
		opts.APIKey = flagAPIKey
	case os.Getenv("LANGSMITH_API_KEY") != "":
		if flagProfile != "" {
			fmt.Fprintln(os.Stderr, "warning: --profile was specified, but LANGSMITH_API_KEY is set and takes precedence over saved profile auth")
		}
		opts.APIKey = os.Getenv("LANGSMITH_API_KEY")
	case hasProfile && (profile.AccessToken() != "" || (refreshOAuth && profile.OAuth.RefreshToken != "")):
		if refreshOAuth && profile.OAuth.RefreshToken != "" &&
			(profile.AccessToken() == "" || profile.TokenExpiresSoon(time.Now(), time.Minute)) {
			token, err := refreshProfileToken(context.Background(), opts.APIURL, profile.OAuth.RefreshToken)
			if err != nil {
				return opts, fmt.Errorf("refreshing OAuth token for profile %q: %w; run 'langsmith auth login --profile %s' to reauthenticate", profileName, err, profileName)
			}
			applyTokenResponse(&profile, token, time.Now())
			cfg.Profiles[profileName] = profile
			if err := cfg.Save(); err != nil {
				return opts, fmt.Errorf("saving refreshed OAuth token: %w", err)
			}
		}
		opts.ProfileName = profileName
		opts.OAuthAccessToken = profile.AccessToken()
	case hasProfile && profile.APIKey != "":
		opts.APIKey = profile.APIKey
	}
	if cfgErr != nil && opts.APIKey == "" {
		return opts, cfgErr
	}

	return opts, nil
}

// ExitError prints an error to stderr and exits.
func ExitError(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}

// ExitErrorf prints a formatted error to stderr and exits.
func ExitErrorf(format string, args ...any) {
	ExitError(fmt.Sprintf(format, args...))
}
