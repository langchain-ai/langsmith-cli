package cmd

import (
	"encoding/json"
	"fmt"

	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
	"github.com/spf13/cobra"
)

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage saved LangSmith profiles",
	}
	cmd.AddCommand(newProfileSetWorkspaceCmd())
	return cmd
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
