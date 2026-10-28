package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

type profileListItem struct {
	Name           string `json:"name"`
	Active         bool   `json:"active"`
	APIURL         string `json:"api_url,omitempty"`
	WorkspaceID    string `json:"workspace_id,omitempty"`
	Auth           string `json:"auth"`
	OAuthExpiresAt string `json:"oauth_expires_at,omitempty"`
}

type profileShowItem struct {
	Name           string `json:"name"`
	Active         bool   `json:"active"`
	APIURL         string `json:"api_url,omitempty"`
	WorkspaceID    string `json:"workspace_id,omitempty"`
	Auth           string `json:"auth"`
	APIKey         string `json:"api_key,omitempty"`
	APIKeyEnv      string `json:"api_key_env,omitempty"`
	APIKeyCmd      string `json:"api_key_cmd,omitempty"`
	OAuthExpiresAt string `json:"oauth_expires_at,omitempty"`
}

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "profiles",
		Aliases: []string{"profile"},
		Short:   "Manage saved LangSmith profiles",
	}
	cmd.AddCommand(newProfileCreateCmd())
	cmd.AddCommand(newProfileListCmd())
	cmd.AddCommand(newProfileShowCmd())
	cmd.AddCommand(newProfileDeleteCmd())
	cmd.AddCommand(newProfileUseCmd())
	cmd.AddCommand(newProfileSetWorkspaceCmd())
	return cmd
}

func newProfileCreateCmd() *cobra.Command {
	var (
		setCurrent bool
		keyEnv     string
		keyCmd     string
	)
	cmd := &cobra.Command{
		Use:   "create NAME",
		Short: "Create an API-key profile",
		Long: `Create an API-key profile.

By default the key from --api-key or LANGSMITH_API_KEY is saved in the config
file. To keep it out of the file, name where it lives instead: --api-key-env
reads it from an environment variable, and --api-key-cmd runs a command that
prints it, such as a password manager lookup. The variable is read, or the
command run, each time the profile is used.`,
		Example: `  langsmith profile create prod --api-key-env PROD_LANGSMITH_API_KEY
  langsmith profile create prod --api-key-cmd 'op read "op://Private/LangSmith/credential"'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileCreate(cmd, args[0], flagWorkspaceID, setCurrent, keyEnv, keyCmd)
		},
	}
	cmd.Flags().BoolVar(&setCurrent, "set-current", false, "Set the new profile as the current profile")
	cmd.Flags().StringVar(&keyEnv, "api-key-env", "", "Read the API key from this environment variable instead of saving it")
	cmd.Flags().StringVar(&keyCmd, "api-key-cmd", "", "Run this command to get the API key instead of saving it")
	cmd.MarkFlagsMutuallyExclusive("api-key-env", "api-key-cmd")
	return cmd
}

func newProfileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List saved profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileList(cmd)
		},
	}
}

func newProfileShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show NAME",
		Short: "Show a saved profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileShow(cmd, args[0])
		},
	}
}

func newProfileDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete a saved profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileDelete(cmd, args[0])
		},
	}
}

func newProfileUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use NAME",
		Short: "Set the current profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileUse(cmd, args[0])
		},
	}
}

func newProfileSetWorkspaceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-workspace WORKSPACE_ID",
		Short: "Set the default workspace for a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileSetWorkspace(cmd, args[0])
		},
	}
}

func runProfileCreate(cmd *cobra.Command, profileName, workspaceID string, setCurrent bool, keyEnv, keyCmd string) error {
	if err := validateProfileName(profileName); err != nil {
		return err
	}
	if workspaceID != "" {
		if err := validateWorkspaceID(workspaceID); err != nil {
			return err
		}
	}

	cfg, err := lsconfig.Load()
	if err != nil {
		return err
	}
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]lsconfig.Profile)
	}
	if _, exists := cfg.Profiles[profileName]; exists {
		return fmt.Errorf("profile %q already exists", profileName)
	}

	profile := lsconfig.Profile{
		APIKeyEnv:   strings.TrimSpace(keyEnv),
		APIKeyCmd:   strings.TrimSpace(keyCmd),
		APIURL:      profileCreateAPIURL(),
		WorkspaceID: workspaceID,
	}
	switch {
	case profile.APIKeyEnv != "" || profile.APIKeyCmd != "":
		if flagAPIKey != "" {
			return fmt.Errorf("pass only one of --api-key, --api-key-env, or --api-key-cmd")
		}
	default:
		profile.APIKey = profileCreateAPIKey()
		if profile.APIKey == "" {
			return fmt.Errorf("api key required; pass --api-key, --api-key-env, or --api-key-cmd, or set LANGSMITH_API_KEY")
		}
	}
	apiURL := profile.APIURL

	cfg.Profiles[profileName] = profile
	if cfg.CurrentProfile == "" || setCurrent {
		cfg.CurrentProfile = profileName
	}
	if err := cfg.Save(); err != nil {
		return err
	}

	if GetFormat() == "pretty" {
		if cfg.CurrentProfile == profileName {
			fmt.Fprintf(cmd.OutOrStdout(), "Created and selected profile %q\n", profileName)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Created profile %q\n", profileName)
		}
		return nil
	}

	result := map[string]any{
		"status":  "created",
		"profile": profileName,
		"api_url": apiURL,
		"auth":    "api_key",
		"active":  cfg.CurrentProfile == profileName,
	}
	if workspaceID != "" {
		result["workspace_id"] = workspaceID
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func profileCreateAPIKey() string {
	if flagAPIKey != "" {
		return flagAPIKey
	}
	return os.Getenv("LANGSMITH_API_KEY")
}

func profileCreateAPIURL() string {
	apiURL := lsconfig.DefaultAPIURL
	if envURL := os.Getenv("LANGSMITH_ENDPOINT"); envURL != "" {
		apiURL = envURL
	}
	if flagAPIURL != "" {
		apiURL = flagAPIURL
	}
	return client.NormalizeURL(apiURL)
}

func runProfileShow(cmd *cobra.Command, profileName string) error {
	cfg, err := lsconfig.Load()
	if err != nil {
		return err
	}
	profile, ok := cfg.Profiles[profileName]
	if !ok {
		return fmt.Errorf("profile %q not found", profileName)
	}

	activeName := cfg.ResolveProfileName(flagProfile, profileEnvName())
	item := profileShowItem{
		Name:           profileName,
		Active:         profileName == activeName,
		APIURL:         profile.APIURL,
		WorkspaceID:    profile.WorkspaceID,
		Auth:           profileAuthType(profile),
		OAuthExpiresAt: profile.OAuth.ExpiresAt,
	}
	if profile.APIKey != "" {
		item.APIKey = lsconfig.MaskSecret(profile.APIKey)
	} else {
		item.APIKeyEnv = profile.APIKeyEnv
		item.APIKeyCmd = profile.APIKeyCmd
	}

	if GetFormat() == "pretty" {
		renderProfileShowTable(cmd, item)
		return nil
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(item)
}

func runProfileDelete(cmd *cobra.Command, profileName string) error {
	cfg, err := lsconfig.Load()
	if err != nil {
		return err
	}
	if _, ok := cfg.Profiles[profileName]; !ok {
		return fmt.Errorf("profile %q not found", profileName)
	}
	delete(cfg.Profiles, profileName)
	if cfg.CurrentProfile == profileName {
		cfg.CurrentProfile = ""
	}
	if err := cfg.Save(); err != nil {
		return err
	}

	if GetFormat() == "pretty" {
		fmt.Fprintf(cmd.OutOrStdout(), "Deleted profile %q\n", profileName)
		return nil
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]string{
		"status":  "deleted",
		"profile": profileName,
	})
}

func runProfileUse(cmd *cobra.Command, profileName string) error {
	cfg, err := lsconfig.Load()
	if err != nil {
		return err
	}
	if _, ok := cfg.Profiles[profileName]; !ok {
		return fmt.Errorf("profile %q not found", profileName)
	}
	cfg.CurrentProfile = profileName
	if err := cfg.Save(); err != nil {
		return err
	}

	if GetFormat() == "pretty" {
		fmt.Fprintf(cmd.OutOrStdout(), "Using profile %q\n", profileName)
		return nil
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]string{
		"status":  "switched",
		"profile": profileName,
	})
}

func runProfileList(cmd *cobra.Command) error {
	cfg, err := lsconfig.Load()
	if err != nil {
		return err
	}

	activeName := cfg.ResolveProfileName(flagProfile, profileEnvName())
	items := make([]profileListItem, 0, len(cfg.Profiles))
	for name, profile := range cfg.Profiles {
		items = append(items, profileListItem{
			Name:           name,
			Active:         name == activeName,
			APIURL:         profile.APIURL,
			WorkspaceID:    profile.WorkspaceID,
			Auth:           profileAuthType(profile),
			OAuthExpiresAt: profile.OAuth.ExpiresAt,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Active != items[j].Active {
			return items[i].Active
		}
		return items[i].Name < items[j].Name
	})

	if GetFormat() == "pretty" {
		renderProfileTable(cmd, items)
		return nil
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(items)
}

func profileAuthType(profile lsconfig.Profile) string {
	switch {
	case profile.AccessToken() != "" || profile.OAuth.RefreshToken != "":
		return "oauth"
	case profile.HasAPIKey():
		return "api_key"
	default:
		return "none"
	}
}

func profileEnvName() string {
	return strings.TrimSpace(os.Getenv("LANGSMITH_PROFILE"))
}

func renderProfileTable(cmd *cobra.Command, profiles []profileListItem) {
	table := output.NewTable(cmd.OutOrStdout(), []string{"Active", "Name", "API URL", "Workspace ID", "Auth", "Expires At"})
	for _, profile := range profiles {
		active := ""
		if profile.Active {
			active = "*"
		}
		_ = table.Append([]string{
			active,
			profile.Name,
			profile.APIURL,
			profile.WorkspaceID,
			profile.Auth,
			profile.OAuthExpiresAt,
		})
	}
	_ = table.Render()
}

func renderProfileShowTable(cmd *cobra.Command, profile profileShowItem) {
	table := output.NewTable(cmd.OutOrStdout(), []string{"Active", "Name", "API URL", "Workspace ID", "Auth", "API Key", "Expires At"})
	active := ""
	if profile.Active {
		active = "*"
	}
	_ = table.Append([]string{
		active,
		profile.Name,
		profile.APIURL,
		profile.WorkspaceID,
		profile.Auth,
		profileShowAPIKey(profile),
		profile.OAuthExpiresAt,
	})
	_ = table.Render()
}

func runProfileSetWorkspace(cmd *cobra.Command, workspaceID string) error {
	if err := validateWorkspaceID(workspaceID); err != nil {
		return err
	}
	cfg, err := lsconfig.Load()
	if err != nil {
		return err
	}
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]lsconfig.Profile)
	}

	profileName := loginProfileName(cfg)
	if err := validateProfileName(profileName); err != nil {
		return err
	}
	profile := cfg.Profiles[profileName]
	profile.WorkspaceID = workspaceID
	cfg.Profiles[profileName] = profile
	if cfg.CurrentProfile == "" {
		cfg.CurrentProfile = profileName
	}
	if err := cfg.Save(); err != nil {
		return err
	}

	if GetFormat() == "pretty" {
		fmt.Fprintf(cmd.OutOrStdout(), "Set default workspace for profile %q\n", profileName)
		return nil
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]string{
		"status":       "workspace_set",
		"profile":      profileName,
		"workspace_id": workspaceID,
	})
}

// profileShowAPIKey is the API Key column: the masked key, or where the key is
// read from when the profile does not store it.
func profileShowAPIKey(profile profileShowItem) string {
	switch {
	case profile.APIKey != "":
		return profile.APIKey
	case profile.APIKeyEnv != "":
		return "$" + profile.APIKeyEnv
	case profile.APIKeyCmd != "":
		return "$(" + profile.APIKeyCmd + ")"
	default:
		return ""
	}
}
