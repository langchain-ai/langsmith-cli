package cmd

import (
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

Examples:
  langsmith --profile demo --workspace <workspace-id> project create --name my-app
  langsmith project create --name my-app --description "Application traces" --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("--name must not be blank")
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
				return fmt.Errorf("creating tracing project: %w", err)
			}
			return output.OutputJSON(map[string]any{
				"status": "created",
				"id":     project.ID,
				"name":   project.Name,
			}, "")
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Name of the new tracing project (required)")
	cmd.Flags().StringVar(&description, "description", "", "Description of the tracing project")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}
