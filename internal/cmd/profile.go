package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
	"github.com/olekukonko/tablewriter"
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
	OAuthExpiresAt string `json:"oauth_expires_at,omitempty"`
}

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage saved LangSmith profiles",
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
		workspaceID string
		setCurrent  bool
	)
	cmd := &cobra.Command{
		Use:   "create NAME",
		Short: "Create an API-key profile",
		Long: `Create an API-key profile.

To create or update a profile that uses OAuth, run:
  langsmith auth login --profile NAME`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileCreate(cmd, args[0], workspaceID, setCurrent)
		},
	}
	cmd.Flags().StringVar(&workspaceID, "workspace-id", "", "Default workspace ID to save in the profile")
	cmd.Flags().BoolVar(&setCurrent, "set-current", false, "Set the new profile as the current profile")
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

func runProfileCreate(cmd *cobra.Command, profileName, workspaceID string, setCurrent bool) error {
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

	apiKey := profileCreateAPIKey()
	if apiKey == "" {
		return fmt.Errorf("api key required; pass --api-key or set LANGSMITH_API_KEY")
	}
	apiURL := profileCreateAPIURL()

	cfg.Profiles[profileName] = lsconfig.Profile{
		APIKey:      apiKey,
		APIURL:      apiURL,
		WorkspaceID: workspaceID,
	}
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
	return envAPIKey()
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

	activeName := activeProfileName(cfg)
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
	}

	if GetFormat() == "pretty" {
		renderProfileShowTable(cmd, item)
		printProfileAuthNote(cmd)
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

	activeName := activeProfileName(cfg)
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
		printProfileAuthNote(cmd)
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
	case profile.APIKey != "":
		return "api_key"
	default:
		return "none"
	}
}

func activeProfileName(cfg *lsconfig.Config) string {
	if flagAPIKey != "" || envAPIKey() != "" {
		return ""
	}
	return cfg.ResolveProfileName(flagProfile, profileEnvName())
}

func profileAuthNote() string {
	if envAPIKey() != "" {
		return "LANGSMITH_API_KEY is set and takes precedence over saved profiles."
	}
	return ""
}

func printProfileAuthNote(cmd *cobra.Command) {
	if note := profileAuthNote(); note != "" {
		fmt.Fprintln(cmd.OutOrStdout(), note)
	}
}

func profileEnvName() string {
	return strings.TrimSpace(os.Getenv("LANGSMITH_PROFILE"))
}

func renderProfileTable(cmd *cobra.Command, profiles []profileListItem) {
	table := tablewriter.NewWriter(cmd.OutOrStdout())
	table.SetHeader([]string{"Active", "Name", "API URL", "Workspace ID", "Auth", "Expires At"})
	table.SetBorder(false)
	table.SetColumnSeparator("  ")
	table.SetHeaderLine(true)
	table.SetAutoWrapText(false)
	for _, profile := range profiles {
		active := ""
		if profile.Active {
			active = "*"
		}
		table.Append([]string{
			active,
			profile.Name,
			profile.APIURL,
			profile.WorkspaceID,
			profile.Auth,
			profile.OAuthExpiresAt,
		})
	}
	table.Render()
}

func renderProfileShowTable(cmd *cobra.Command, profile profileShowItem) {
	table := tablewriter.NewWriter(cmd.OutOrStdout())
	table.SetHeader([]string{"Active", "Name", "API URL", "Workspace ID", "Auth", "API Key", "Expires At"})
	table.SetBorder(false)
	table.SetColumnSeparator("  ")
	table.SetHeaderLine(true)
	table.SetAutoWrapText(false)
	active := ""
	if profile.Active {
		active = "*"
	}
	table.Append([]string{
		active,
		profile.Name,
		profile.APIURL,
		profile.WorkspaceID,
		profile.Auth,
		profile.APIKey,
		profile.OAuthExpiresAt,
	})
	table.Render()
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
