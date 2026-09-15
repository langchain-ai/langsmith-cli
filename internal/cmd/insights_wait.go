package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

func waitForInsight(ctx context.Context, interval time.Duration, fetch func(context.Context) (*langsmith.SessionInsightGetJobResponse, error)) (*langsmith.SessionInsightGetJobResponse, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		detail, err := fetch(ctx)
		if err != nil {
			return nil, err
		}
		switch detail.Status {
		case "success":
			return detail, nil
		case "error":
			return detail, fmt.Errorf("Insights job failed: %s", detail.Error)
		case "queued", "running":
		default:
			return detail, fmt.Errorf("unrecognized Insights status %q; inspect the job with insights get", detail.Status)
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func newInsightsWaitCmd() *cobra.Command {
	var project, projectID, outputFile string
	var timeout, interval time.Duration
	cmd := &cobra.Command{
		Use:   "wait ID",
		Short: "Wait for an existing Insights job and return its report",
		Long:  "Poll an existing Insights job until it succeeds, fails, or the timeout expires.\nThis never creates or cancels a job. On timeout, resume with the same job ID.\nJSON output uses the same report schema as insights get.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if timeout <= 0 || interval < time.Second {
				return fmt.Errorf("--timeout must be positive and --poll-interval must be at least 1s")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			c, err := getClient()
			if err != nil {
				return err
			}
			sessionID, err := resolveSessionID(ctx, c, project, projectID, "insights wait")
			if err != nil {
				return err
			}
			detail, err := waitForInsight(ctx, interval, func(ctx context.Context) (*langsmith.SessionInsightGetJobResponse, error) {
				return c.SDK.Sessions.Insights.GetJob(ctx, sessionID, args[0])
			})
			if err != nil {
				return fmt.Errorf("waiting for Insights job %s in project %s: %w; the remote job was not cancelled", args[0], sessionID, err)
			}
			if GetFormat() == "pretty" {
				printInsightPretty(detail)
				return nil
			}
			return output.OutputJSON(buildInsightDetailJSON(detail), outputFile)
		},
	}
	addProjectFlags(cmd, &project, &projectID)
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "Maximum time to wait (does not cancel the remote job)")
	cmd.Flags().DurationVar(&interval, "poll-interval", 5*time.Second, "Time between status requests (minimum 1s)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	return cmd
}
