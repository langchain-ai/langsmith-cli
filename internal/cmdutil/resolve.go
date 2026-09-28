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

func isFlagChanged(cmd *cobra.Command, name string) bool {
	if f := cmd.Flags().Lookup(name); f != nil {
		return f.Changed
	}
	if f := cmd.PersistentFlags().Lookup(name); f != nil {
		return f.Changed
	}
	return false
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
		return "pretty"
	}
	return v
}

// ResolveJQ reads the jq filter from cobra's flag tree.
func ResolveJQ(cmd *cobra.Command) string {
	return getFlagString(cmd, "jq")
}

// GetClient creates a LangSmith client from cobra flags, returning an error
// if no API key or OAuth access token is available.
func GetClient(cmd *cobra.Command) (*client.Client, error) {
	opts, err := ResolveClientOptions(cmd)
	if err != nil {
		return nil, err
	}
	if !opts.HasAuth() {
		return nil, fmt.Errorf("not authenticated; run 'langsmith auth login', set LANGSMITH_API_KEY, or pass --api-key")
	}
	return client.NewWithOptions(opts), nil
}

// ResolveClientOptions resolves auth/routing from cobra flags, env, and saved
// profiles. This mirrors the top-level CLI auth behavior for standalone
// helpers such as `langsmith api`.
func ResolveClientOptions(cmd *cobra.Command) (client.Options, error) {
	opts := client.Options{APIURL: lsconfig.DefaultAPIURL}

	cfg, err := lsconfig.Load()
	var cfgErr error
	if err != nil {
		cfgErr = err
		cfg = &lsconfig.Config{Profiles: make(map[string]lsconfig.Profile)}
	}

	flagProfile := getFlagString(cmd, "profile")
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
		if flagProfile != "" && hasProfile && profile.APIURL != "" {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: ignoring LANGSMITH_ENDPOINT because profile %q was selected with --profile\n", profileName)
		} else {
			opts.APIURL = client.NormalizeURL(v)
		}
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
	if v := getFlagString(cmd, "workspace"); v != "" {
		opts.WorkspaceID = v
	} else if v := getFlagString(cmd, "workspace-id"); v != "" {
		opts.WorkspaceID = v
	}

	switch {
	case getFlagString(cmd, "api-key") != "":
		opts.APIKey = getFlagString(cmd, "api-key")
	case os.Getenv("LANGSMITH_API_KEY") != "":
		if isFlagChanged(cmd, "profile") {
			fmt.Fprintln(cmd.ErrOrStderr(), "warning: --profile was specified, but LANGSMITH_API_KEY is set and takes precedence over saved profile auth")
		}
		opts.APIKey = os.Getenv("LANGSMITH_API_KEY")
	case hasProfile && (profile.AccessToken() != "" || profile.OAuth.RefreshToken != ""):
		// The SDK refreshes the profile's OAuth token on each request, under a
		// lock shared by every process using this config.
		opts.ProfileName = profileName
	case hasProfile && profile.APIKey != "":
		opts.APIKey = profile.APIKey
		// Route the resolved profile through the SDK (WithProfile) so an explicit
		// selection replaces the config's current_profile instead of inheriting
		// its tenant/base URL. APIKey is kept for the raw-HTTP helpers.
		opts.ProfileName = profileName
	}
	if cfgErr != nil && opts.APIKey == "" {
		return opts, cfgErr
	}

	return opts, nil
}
