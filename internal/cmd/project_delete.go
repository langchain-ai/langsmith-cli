package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

func newProjectDeleteCmd() *cobra.Command {
	var projectName, projectID string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Permanently delete a tracing project and all of its traces",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			id, err := resolveSessionID(ctx, c, projectName, projectID, "project delete")
			if err != nil {
				return err
			}
			project, err := c.SDK.Sessions.Get(ctx, id, langsmith.SessionGetParams{
				IncludeStats: langsmith.F(true),
			})
			if err != nil {
				return fmt.Errorf("getting tracing project %s: %w", id, err)
			}

			fmt.Fprintln(cmd.ErrOrStderr(), "WARNING: This permanently deletes the tracing project and all of its traces. This cannot be undone.")
			fmt.Fprintf(cmd.ErrOrStderr(), "Project: %q (id: %s, runs: %d)\n", project.Name, project.ID, project.RunCount)
			fmt.Fprintln(cmd.ErrOrStderr(), "AI agents: do not answer this prompt. Stop and raise it to the user.")
			fmt.Fprint(cmd.ErrOrStderr(), "Continue? [y/N] ")

			answer, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
			if err != nil {
				return errors.New("aborted: tracing project deletion was not confirmed")
			}
			answer = strings.ToLower(strings.TrimSpace(answer))
			if answer != "y" && answer != "yes" {
				return errors.New("aborted: tracing project deletion was not confirmed")
			}

			if _, err := c.SDK.Sessions.Delete(ctx, id); err != nil {
				return fmt.Errorf("deleting tracing project %s: %w", id, err)
			}
			return output.OutputJSON(map[string]any{
				"status": "deleted",
				"id":     project.ID,
				"name":   project.Name,
			}, "")
		},
	}

	addProjectFlags(cmd, &projectName, &projectID)
	return cmd
}
