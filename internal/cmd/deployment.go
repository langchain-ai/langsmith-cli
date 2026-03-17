package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

type deploymentEntry struct {
	CommitSHA   string    `json:"commit_sha"`
	FirstSeenAt time.Time `json:"first_seen_at"`
}

func newDeploymentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deployment",
		Short: "Inspect agent deployment history for a tracing project",
		Long: `Inspect which agent versions (commit SHAs) have been active in a tracing project.

Deployments are recorded automatically when traces include the LANGSMITH_AGENT_VERSION
metadata key on root runs. Each unique commit SHA seen in a project is recorded as a
deployment with the timestamp of the earliest trace that carried that version.

Examples:
  langsmith deployment list --project my-agent
  langsmith deployment list --project my-agent --format pretty`,
	}

	cmd.AddCommand(newDeploymentListCmd())
	return cmd
}

func newDeploymentListCmd() *cobra.Command {
	var (
		projectName string
		outputFile  string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List agent deployments for a tracing project",
		Run: func(cmd *cobra.Command, args []string) {
			if projectName == "" {
				exitError("--project is required")
			}

			c := mustGetClient()
			ctx := context.Background()

			sessionID, err := c.ResolveSessionID(ctx, projectName)
			if err != nil {
				exitErrorf("resolving project: %v", err)
			}

			var deployments []deploymentEntry
			if err := c.RawGet(ctx, fmt.Sprintf("/api/v1/sessions/%s/deployments", sessionID), &deployments); err != nil {
				exitErrorf("fetching deployments: %v", err)
			}

			if getFormat() == "pretty" {
				columns := []string{"Commit SHA", "First Seen At"}
				var rows [][]string
				for _, d := range deployments {
					rows = append(rows, []string{d.CommitSHA, formatTimeShort(d.FirstSeenAt)})
				}
				output.OutputTable(columns, rows, fmt.Sprintf("Agent Deployments — %s", projectName))
			} else {
				var data []map[string]any
				for _, d := range deployments {
					data = append(data, map[string]any{
						"commit_sha":    d.CommitSHA,
						"first_seen_at": formatTimeISO(d.FirstSeenAt),
					})
				}
				output.OutputJSON(data, outputFile)
			}
		},
	}

	cmd.Flags().StringVarP(&projectName, "project", "p", "", "Tracing project name (required)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
}
