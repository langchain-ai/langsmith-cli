package cmdutil

import (
	"fmt"
	"os"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/spf13/cobra"
)

// getFlagString looks up a string flag by checking Flags() (which includes
// inherited persistent flags for child commands) and then PersistentFlags()
// (which covers the root command itself where persistent flags are not yet
// merged into Flags()).
func getFlagString(cmd *cobra.Command, name string) string {
	if f := cmd.Flags().Lookup(name); f != nil {
		return f.Value.String()
	}
	if f := cmd.PersistentFlags().Lookup(name); f != nil {
		return f.Value.String()
	}
	return ""
}

// ResolveAPIKey reads the API key from cobra's flag tree → env.
func ResolveAPIKey(cmd *cobra.Command) string {
	if v := getFlagString(cmd, "api-key"); v != "" {
		return v
	}
	return os.Getenv("LANGSMITH_API_KEY")
}

// ResolveAPIURL reads the API URL from cobra's flag tree → env → default.
func ResolveAPIURL(cmd *cobra.Command) string {
	if v := getFlagString(cmd, "api-url"); v != "" {
		return client.NormalizeURL(v)
	}
	if v := os.Getenv("LANGSMITH_ENDPOINT"); v != "" {
		return client.NormalizeURL(v)
	}
	return "https://api.smith.langchain.com"
}

// ResolveFormat reads the output format from cobra's flag tree.
func ResolveFormat(cmd *cobra.Command) string {
	v := getFlagString(cmd, "format")
	if v == "" {
		return "json"
	}
	return v
}

// GetClient creates a LangSmith client from cobra flags, returning an error
// if the API key is not set.
func GetClient(cmd *cobra.Command) (*client.Client, error) {
	apiKey := ResolveAPIKey(cmd)
	if apiKey == "" {
		return nil, fmt.Errorf("LANGSMITH_API_KEY not set")
	}
	return client.New(apiKey, ResolveAPIURL(cmd)), nil
}
