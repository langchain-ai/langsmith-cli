package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"github.com/spf13/cobra"
)

func newProjectCreateCmd() *cobra.Command {
	var name, description string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a tracing project in the selected workspace",
		Long: `Create an empty tracing project in the selected workspace.

This does not instrument your application, enable tracing, or create evaluators.
Configure your application's LangSmith tracing credentials and LANGSMITH_PROJECT
separately, run it, then verify ingestion with 'langsmith trace list --project-id <id>'.

Use --format json for status, id, name, and workspace_id. Otherwise a creation
confirmation is printed. Creation is never automatically retried. After an
uncertain failure, use 'project list --name-contains NAME' in the same workspace
and check exact names before retrying; the list is paginated and substring-based.

Examples:
  langsmith --profile demo --workspace <workspace-id> project create --name my-app
  langsmith project create --name my-app --description "Application traces" --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(name) == "" {
				return commandDiagnostic{"invalid_project_name", "--name must not be blank", "Provide a nonblank tracing project name with --name."}
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			params := langsmith.SessionNewParams{Name: langsmith.F(name)}
			if cmd.Flags().Changed("description") {
				params.Description = langsmith.F(description)
			}
			project, err := c.SDK.Sessions.New(cmd.Context(), params, option.WithMaxRetries(0))
			if err != nil {
				return projectCreateDiagnostic(err)
			}
			if project == nil || project.ID == "" {
				return projectCreateDiagnostic(nil)
			}
			workspaceID := resultWorkspaceID()
			if project.TenantID != "" {
				workspaceID = &project.TenantID
			}
			if GetFormat() == "pretty" {
				workspace := "unknown"
				if workspaceID != nil {
					workspace = *workspaceID
				}
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "Created tracing project %q\nID: %s\nWorkspace: %s\nNext: configure tracing in your application and run it; project creation alone does not enable tracing. Verify with langsmith trace list --project-id %s in this workspace.\n", project.Name, project.ID, workspace, project.ID)
				return err
			}
			return output.OutputJSON(map[string]any{
				"status":       "created",
				"id":           project.ID,
				"name":         project.Name,
				"workspace_id": workspaceID,
				"next_steps":   []string{"Configure tracing credentials and LANGSMITH_PROJECT in your application, then run it. Creating a project does not instrument the application or create evaluators.", "Verify ingestion with trace list --project-id using the returned id in the same workspace."},
			}, "")
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Name of the new tracing project (required)")
	cmd.Flags().StringVar(&description, "description", "", "Description of the tracing project")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func projectCreateDiagnostic(err error) error {
	var apiErr *langsmith.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case 409:
			return commandDiagnostic{"project_conflict", "creating tracing project: the project conflicts with existing state", "Use project list --name-contains with the requested name in the same workspace. Check exact names before choosing a different name."}
		case 401, 403:
			return commandDiagnostic{"project_permission_denied", "creating tracing project: authentication or permission was denied", "Verify the selected profile and workspace, and permission to create tracing projects."}
		case 400, 422:
			return commandDiagnostic{"invalid_project_request", "creating tracing project: the service rejected the request", "Check the project name and description, then review project create --help."}
		}
	}
	return commandDiagnostic{"project_creation_unverified", "creating tracing project: the outcome could not be confirmed", "Do not blindly repeat creation. Use project list --name-contains with the requested name in the same workspace, check exact names and additional pages, then retry only if no project was created."}
}
