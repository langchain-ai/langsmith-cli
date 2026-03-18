package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

type agentVersionEntry struct {
	CommitSHA   string    `json:"commit_sha"`
	FirstSeenAt time.Time `json:"first_seen_at"`
}

func newAgentVersionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent-versions",
		Short: "Inspect agent version history for a tracing project",
		Long: `Inspect which agent versions (commit SHAs) have been active in a tracing project.

Agent versions are recorded automatically when traces include the LANGSMITH_AGENT_VERSION
metadata key on root runs. Each unique commit SHA seen in a project is recorded with the
timestamp of the earliest trace that carried that version.

Examples:
  langsmith project agent-versions list my-agent
  langsmith project agent-versions list my-agent --format pretty`,
	}

	cmd.AddCommand(newAgentVersionsListCmd())
	return cmd
}

func newAgentVersionsListCmd() *cobra.Command {
	var outputFile string

	cmd := &cobra.Command{
		Use:   "list <project>",
		Short: "List agent versions for a tracing project",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			projectName := args[0]

			c := mustGetClient()
			ctx := context.Background()

			sessionID, err := c.ResolveSessionID(ctx, projectName)
			if err != nil {
				exitErrorf("resolving project: %v", err)
			}

			var versions []agentVersionEntry
			if err := c.RawGet(ctx, fmt.Sprintf("/v1/platform/sessions/%s/agent-versions", sessionID), &versions); err != nil {
				exitErrorf("fetching agent versions: %v", err)
			}

			if getFormat() == "pretty" {
				columns := []string{"Commit SHA", "First Seen At"}
				var rows [][]string
				for _, v := range versions {
					rows = append(rows, []string{v.CommitSHA, formatTimeShort(v.FirstSeenAt)})
				}
				output.OutputTable(columns, rows, fmt.Sprintf("Agent Versions — %s", projectName))
			} else {
				data := make([]map[string]any, 0, len(versions))
				for _, v := range versions {
					data = append(data, map[string]any{
						"commit_sha":    v.CommitSHA,
						"first_seen_at": formatTimeISO(v.FirstSeenAt),
					})
				}
				output.OutputJSON(data, outputFile)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
}
