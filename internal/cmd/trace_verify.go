package cmd

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

func newTraceVerifyCmd() *cobra.Command {
	var project, projectID, traceID string
	var minutes int
	cmd := &cobra.Command{Use: "verify", Short: "Verify an actual root trace arrived in the intended project", Args: cobra.NoArgs,
		Long: "Read-only ingestion check. Pass the root trace ID from your application to verify\nthat execution, or check for any root in a bounded recent window. A recent trace\ndoes not prove it came from your current app. This command never creates a synthetic trace.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if minutes < 1 || minutes > 10080 {
				return fmt.Errorf("--last-n-minutes must be between 1 and 10080")
			}
			if cmd.Flags().Changed("trace-id") {
				if _, err := uuid.Parse(traceID); err != nil {
					return fmt.Errorf("--trace-id must be a UUID")
				}
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			id, err := resolveSessionID(cmd.Context(), c, project, projectID, "trace verify")
			if err != nil {
				return err
			}
			params := langsmith.RunQueryParams{IsRoot: langsmith.F(true), Limit: langsmith.F(int64(1)), Select: langsmith.F([]langsmith.RunQueryParamsSelect{"id", "trace_id", "start_time", "end_time", "error"})}
			if traceID != "" {
				params.ID = langsmith.F([]string{traceID})
			} else {
				params.StartTime = langsmith.F(time.Now().UTC().Add(-time.Duration(minutes) * time.Minute))
			}
			runs, err := queryRuns(cmd.Context(), c, params, id, 1, 0)
			if err != nil {
				return err
			}
			if len(runs) == 0 {
				return workflowFailure("trace_not_found", "no matching root trace was visible", "run your instrumented application and retry with its --trace-id; do not create a fake trace")
			}
			run := runs[0]
			return output.OutputJSON(map[string]any{"status": "trace_received", "project_id": id, "trace_id": run.TraceID, "run_id": run.ID, "completed": !run.EndTime.IsZero(), "has_error": run.Error != "", "matched_explicit_id": traceID != "", "next_actions": []string{"Inspect this trace with trace get --full before curating it", "Create or select a dataset, then dataset add --dry-run"}}, "")
		},
	}
	addProjectFlags(cmd, &project, &projectID)
	cmd.Flags().StringVar(&traceID, "trace-id", "", "Root trace UUID emitted by your app (bypasses recent window)")
	cmd.Flags().IntVar(&minutes, "last-n-minutes", 30, "Recent window when no trace ID is provided")
	return cmd
}
