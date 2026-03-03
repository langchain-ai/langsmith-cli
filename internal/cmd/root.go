package cmd

import (
	"fmt"
	"os"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/spf13/cobra"
)

// Global flag values stored here; accessed by subcommands via helpers.
var (
	flagAPIKey       string
	flagAPIURL       string
	flagOutputFormat string
)

// NewRootCmd creates the top-level `langsmith` command.
func NewRootCmd(version string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "langsmith",
		Short: "LangSmith CLI — query and manage LangSmith resources",
		Long: `LangSmith CLI — query and manage LangSmith resources from the command line.

Designed for AI coding agents and developers who need fast, scriptable
access to traces, runs, datasets, evaluators, experiments, and threads.
All commands output JSON by default for easy parsing.

Authentication:
  Set LANGSMITH_API_KEY as an environment variable, or pass --api-key.
  Optionally set LANGSMITH_ENDPOINT for self-hosted instances.
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
		Version:       version,
	}

	rootCmd.PersistentFlags().StringVar(&flagAPIKey, "api-key", "", "LangSmith API key [env: LANGSMITH_API_KEY]")
	rootCmd.PersistentFlags().StringVar(&flagAPIURL, "api-url", "", "LangSmith API URL [env: LANGSMITH_ENDPOINT]")
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

	return rootCmd
}

// getAPIKey resolves the API key from flag → env → error.
func getAPIKey() string {
	if flagAPIKey != "" {
		return flagAPIKey
	}
	if v := os.Getenv("LANGSMITH_API_KEY"); v != "" {
		return v
	}
	return ""
}

// getAPIURL resolves the API URL from flag → env → default.
func getAPIURL() string {
	if flagAPIURL != "" {
		return flagAPIURL
	}
	if v := os.Getenv("LANGSMITH_ENDPOINT"); v != "" {
		return v
	}
	return "https://api.smith.langchain.com"
}

// getFormat returns the output format.
func getFormat() string {
	return flagOutputFormat
}

// mustGetClient creates a LangSmith client or exits with an error.
func mustGetClient() *client.Client {
	apiKey := getAPIKey()
	if apiKey == "" {
		exitError("LANGSMITH_API_KEY not set")
	}
	return client.New(apiKey, getAPIURL())
}

// exitError prints a JSON error to stderr and exits.
func exitError(msg string) {
	fmt.Fprintf(os.Stderr, `{"error": %q}`+"\n", msg)
	os.Exit(1)
}

// exitErrorf prints a formatted JSON error to stderr and exits.
func exitErrorf(format string, args ...any) {
	exitError(fmt.Sprintf(format, args...))
}
