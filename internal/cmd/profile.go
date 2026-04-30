package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

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

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage saved LangSmith profiles",
	}
	cmd.AddCommand(newProfileListCmd())
	cmd.AddCommand(newProfileSetWorkspaceCmd())
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

func runProfileList(cmd *cobra.Command) error {
	cfg, err := lsconfig.Load()
	if err != nil {
		return err
	}

	activeName := cfg.ResolveProfileName(flagProfile, os.Getenv("LANGSMITH_PROFILE"))
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
	case profile.AccessToken() != "":
		return "oauth"
	case profile.APIKey != "":
		return "api_key"
	default:
		return "none"
	}
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
