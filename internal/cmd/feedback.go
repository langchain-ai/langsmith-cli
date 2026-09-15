package cmd

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"github.com/langchain-ai/langsmith-go/shared"
	"github.com/spf13/cobra"
	"math"
	"strings"
)

func newFeedbackCmd() *cobra.Command {
	group := &cobra.Command{Use: "feedback", Short: "Create and inspect run feedback"}
	group.AddCommand(newFeedbackCreateCmd(), newFeedbackListCmd())
	return group
}

func newFeedbackCreateCmd() *cobra.Command {
	var runID, key, comment, project, projectID, id string
	var score float64
	cmd := &cobra.Command{Use: "create", Short: "Attach a score or comment to a run", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if err := resourceUUID(runID); err != nil {
			return err
		}
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("--key is required")
		}
		if !cmd.Flags().Changed("score") && !cmd.Flags().Changed("comment") {
			return fmt.Errorf("provide --score or --comment")
		}
		if math.IsNaN(score) || math.IsInf(score, 0) {
			return fmt.Errorf("score must be finite")
		}
		if id == "" {
			u, err := uuid.NewV7()
			if err != nil {
				return err
			}
			id = u.String()
		}
		if err := resourceUUID(id); err != nil {
			return err
		}
		c, err := getClient()
		if err != nil {
			return err
		}
		pid, err := resolveSessionID(cmd.Context(), c, project, projectID, "feedback create")
		if err != nil {
			return err
		}
		runs, err := queryRuns(cmd.Context(), c, langsmith.RunQueryParams{ID: langsmith.F([]string{runID}), Select: langsmith.F([]langsmith.RunQueryParamsSelect{"id", "trace_id", "start_time"})}, pid, 1, 0)
		if err != nil {
			return err
		}
		if len(runs) != 1 {
			return fmt.Errorf("run not found in selected project")
		}
		p := langsmith.FeedbackCreateSchemaParam{ID: langsmith.F(id), Key: langsmith.F(key), RunID: langsmith.F(runID), SessionID: langsmith.F(pid), TraceID: langsmith.F(runs[0].TraceID), StartTime: langsmith.F(runs[0].StartTime)}
		if cmd.Flags().Changed("score") {
			p.Score = langsmith.F[langsmith.FeedbackCreateSchemaScoreUnionParam](shared.UnionFloat(score))
		}
		if cmd.Flags().Changed("comment") {
			p.Comment = langsmith.F(comment)
		}
		matches := func(got *langsmith.FeedbackSchema) bool {
			return feedbackMatches(got, id, pid, runID, key, score, cmd.Flags().Changed("score"), comment, cmd.Flags().Changed("comment"))
		}
		emit := func(status string, got *langsmith.FeedbackSchema) error {
			return output.OutputJSON(map[string]any{"workspace_id": resultWorkspaceID(), "project_id": pid, "run_id": runID, "feedback_id": id, "status": status, "feedback": got}, "")
		}
		if cmd.Flags().Changed("id") {
			got, getErr := c.SDK.Feedback.Get(cmd.Context(), id, langsmith.FeedbackGetParams{})
			if getErr == nil {
				if matches(got) {
					return emit("skipped", got)
				}
				return commandDiagnostic{"feedback_conflict", "feedback ID already exists with different content", "Use another feedback ID or inspect the existing feedback; it was not overwritten."}
			}
			if !isHTTP404(getErr) {
				return getErr
			}
		}
		_, createErr := c.SDK.Feedback.New(cmd.Context(), langsmith.FeedbackNewParams{FeedbackCreateSchema: p}, option.WithMaxRetries(0))
		got, verifyErr := c.SDK.Feedback.Get(cmd.Context(), id, langsmith.FeedbackGetParams{})
		if verifyErr == nil && matches(got) {
			status := "created"
			if createErr != nil {
				status = "recovered"
			}
			return emit(status, got)
		}
		if err := emit("unverified", nil); err != nil {
			return err
		}
		return commandDiagnostic{"feedback_unverified", "feedback write could not be verified", "Inspect the feedback_id in stdout; retry the same request with --id set to that ID to reconcile, not a new ID."}
	}}
	addProjectFlags(cmd, &project, &projectID)
	cmd.Flags().StringVar(&runID, "run-id", "", "Run UUID (required)")
	cmd.Flags().StringVar(&key, "key", "", "Feedback metric key (required)")
	cmd.Flags().Float64Var(&score, "score", 0, "Numeric feedback score")
	cmd.Flags().StringVar(&comment, "comment", "", "Feedback comment")
	cmd.Flags().StringVar(&id, "id", "", "Optional feedback UUID for reconciliation")
	return cmd
}

func newFeedbackListCmd() *cobra.Command {
	var runID string
	var limit, offset int64
	cmd := &cobra.Command{Use: "list", Short: "List feedback for one run", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if err := resourceUUID(runID); err != nil {
			return err
		}
		if limit < 1 || limit > 1000 || offset < 0 || offset > (1<<63-1)-limit {
			return fmt.Errorf("limit must be 1–1000; offset must be nonnegative and leave room for the next page")
		}
		c, err := getClient()
		if err != nil {
			return err
		}
		page, err := c.SDK.Feedback.List(cmd.Context(), langsmith.FeedbackListParams{Run: langsmith.F[langsmith.FeedbackListParamsRunUnion](shared.UnionString(runID)), Limit: langsmith.F(limit), Offset: langsmith.F(offset)})
		if err != nil {
			return err
		}
		return output.OutputJSON(map[string]any{"workspace_id": resultWorkspaceID(), "run_id": runID, "items": page.Items, "limit": limit, "offset": offset, "pagination": describeOffsetPage(len(page.Items), limit, offset)}, "")
	}}
	cmd.Flags().StringVar(&runID, "run-id", "", "Run UUID (required)")
	cmd.Flags().Int64Var(&limit, "limit", 100, "Page size (1–1000)")
	cmd.Flags().Int64Var(&offset, "offset", 0, "Pagination offset")
	return cmd
}
