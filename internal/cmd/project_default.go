package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-cli/internal/client"
	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

func savedProjectDefault() (*lsconfig.ProjectDefault, error) {
	cfg, err := lsconfig.Load()
	if err != nil {
		return nil, err
	}
	_, profile, _ := cfg.ResolveProfile(flagProfile, strings.TrimSpace(os.Getenv("LANGSMITH_PROFILE")))
	saved := profile.ProjectDefault
	if saved == nil {
		return nil, nil
	}
	opts, err := resolveClientOptions(false)
	if err != nil {
		return nil, err
	}
	if saved.WorkspaceID != opts.WorkspaceID || client.NormalizeURL(saved.APIURL) != client.NormalizeURL(opts.APIURL) {
		return nil, commandDiagnostic{"project_default_context_mismatch", "The saved project belongs to another workspace or endpoint.", "Pass --project/--project-id, select a project with project set-default, or remove the saved selection with project clear-default."}
	}
	return saved, nil
}

func resolveSavedProject(ctx context.Context, c *client.Client) (string, error) {
	saved, err := savedProjectDefault()
	if err != nil || saved == nil {
		return "", err
	}
	if _, err := validateProjectID(saved.ID); err != nil {
		return "", err
	}
	project, err := c.SDK.Sessions.Get(ctx, saved.ID, langsmith.SessionGetParams{})
	if err != nil {
		return "", fmt.Errorf("reading saved project: %w", err)
	}
	if project == nil || project.ID != saved.ID || project.TenantID != saved.WorkspaceID {
		return "", commandDiagnostic{"project_default_unavailable", "The saved project could not be verified in this workspace.", "Use project set-default to select an accessible tracing project."}
	}
	return saved.ID, nil
}

func saveProjectDefault(id, name, workspace string) (string, error) {
	opts, err := resolveClientOptions(false)
	if err != nil {
		return "", err
	}
	if workspace == "" || workspace != opts.WorkspaceID {
		return "", commandDiagnostic{"workspace_required", "Select the project's workspace before saving a default.", "Run workspace set-default WORKSPACE_ID, then project set-default PROJECT."}
	}
	cfg, err := lsconfig.Load()
	if err != nil {
		return "", err
	}
	profileName, profile, exists := cfg.ResolveProfile(flagProfile, strings.TrimSpace(os.Getenv("LANGSMITH_PROFILE")))
	if !exists {
		return "", commandDiagnostic{"profile_required", "Saving a project default requires a saved profile.", "Run auth login or select an existing profile with --profile."}
	}
	profile.ProjectDefault = &lsconfig.ProjectDefault{ID: id, Name: name, WorkspaceID: workspace, APIURL: opts.APIURL}
	cfg.Profiles[profileName] = profile
	return profileName, cfg.Save()
}

func projectDefaultWarnings() []string {
	if os.Getenv("LANGSMITH_PROJECT") != "" {
		return []string{"LANGSMITH_PROJECT overrides this saved default. Unset it to use the saved selection."}
	}
	return []string{}
}

func newProjectSetDefaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-default PROJECT",
		Short: "Save a project name or UUID as the selected profile's default",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			id := args[0]
			if _, err := uuid.Parse(id); err != nil {
				id, err = c.ResolveSessionID(cmd.Context(), args[0])
				if err != nil {
					return err
				}
			}
			project, err := c.SDK.Sessions.Get(cmd.Context(), id, langsmith.SessionGetParams{})
			if err != nil {
				return err
			}
			if project == nil || project.ID == "" {
				return fmt.Errorf("project lookup returned no ID")
			}
			profile, err := saveProjectDefault(project.ID, project.Name, project.TenantID)
			if err != nil {
				return err
			}
			if GetFormat() == "pretty" {
				fmt.Fprintf(cmd.OutOrStdout(), "Default project: %s\nID: %s\nWorkspace: %s\nProfile: %s\n", project.Name, project.ID, project.TenantID, profile)
				for _, warning := range projectDefaultWarnings() {
					fmt.Fprintln(cmd.OutOrStdout(), warning)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Application tracing still requires its own project configuration.")
				return nil
			}
			return output.OutputJSON(map[string]any{"status": "project_set", "project_id": project.ID, "project_name": project.Name, "workspace_id": project.TenantID, "profile": profile, "warnings": projectDefaultWarnings()}, "")
		},
	}
}

func newProjectClearDefaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear-default",
		Short: "Remove the selected profile's saved project default",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := lsconfig.Load()
			if err != nil {
				return err
			}
			name, profile, exists := cfg.ResolveProfile(flagProfile, strings.TrimSpace(os.Getenv("LANGSMITH_PROFILE")))
			if !exists {
				return commandDiagnostic{"profile_required", "No saved profile is selected.", "Select an existing profile with --profile."}
			}
			profile.ProjectDefault = nil
			cfg.Profiles[name] = profile
			if err := cfg.Save(); err != nil {
				return err
			}
			if GetFormat() == "pretty" {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "Saved project default cleared. Environment variables and explicit flags still apply.")
				return err
			}
			return output.OutputJSON(map[string]any{"status": "project_default_cleared", "profile": name, "warnings": projectDefaultWarnings()}, "")
		},
	}
}
