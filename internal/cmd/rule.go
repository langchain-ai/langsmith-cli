package cmd

import (
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

func newRuleCmd() *cobra.Command {
	group := &cobra.Command{Use: "rule", Short: "Inspect project automation rules"}
	var project, projectID string
	list := &cobra.Command{Use: "list", Short: "List automation rules for a project", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		pid, err := resolveSessionID(cmd.Context(), c, project, projectID, "rule list")
		if err != nil {
			return err
		}
		rules, err := c.SDK.Evaluators.List(cmd.Context(), langsmith.EvaluatorListParams{SessionID: langsmith.F(pid)})
		if err != nil {
			return err
		}
		return output.OutputJSON(map[string]any{"project_id": pid, "items": rules}, "")
	}}
	addProjectFlags(list, &project, &projectID)
	group.AddCommand(list)
	return group
}
