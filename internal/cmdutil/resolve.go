package cmdutil

import (
	"fmt"
	"os"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
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
// if no API key is available.
func GetClient(cmd *cobra.Command) (*client.Client, error) {
	opts, err := ResolveClientOptions(cmd)
	if err != nil {
		return nil, err
	}
	if opts.APIKey == "" {
		return nil, fmt.Errorf("not authenticated; set LANGSMITH_API_KEY, pass --api-key, or select an API-key profile")
	}
	return client.NewWithOptions(opts), nil
}

// ResolveClientOptions resolves auth/routing from cobra flags, env, and saved
// profiles. This mirrors the top-level CLI auth behavior for standalone
// helpers such as `langsmith api`.
func ResolveClientOptions(cmd *cobra.Command) (client.Options, error) {
	opts := client.Options{APIURL: lsconfig.DefaultAPIURL}

	cfg, err := lsconfig.Load()
	if err != nil {
		return opts, err
	}
	flagProfile := getFlagString(cmd, "profile")
	envProfile := strings.TrimSpace(os.Getenv("LANGSMITH_PROFILE"))
	profileName, profile, hasProfile := cfg.ResolveProfile(flagProfile, envProfile)
	if (flagProfile != "" || envProfile != "") && !hasProfile {
		return opts, fmt.Errorf("profile not found: %s", profileName)
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
	if v := getFlagString(cmd, "api-url"); v != "" {
		opts.APIURL = client.NormalizeURL(v)
	}

	if v := os.Getenv("LANGSMITH_TENANT_ID"); v != "" {
		opts.WorkspaceID = v
	}
	if v := os.Getenv("LANGSMITH_WORKSPACE_ID"); v != "" {
		opts.WorkspaceID = v
	}

	switch {
	case getFlagString(cmd, "api-key") != "":
		opts.APIKey = getFlagString(cmd, "api-key")
	case os.Getenv("LANGSMITH_API_KEY") != "":
		opts.APIKey = os.Getenv("LANGSMITH_API_KEY")
	case hasProfile && profile.APIKey != "":
		opts.APIKey = profile.APIKey
	}

	return opts, nil
}
