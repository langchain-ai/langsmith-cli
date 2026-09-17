package cmd

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"github.com/langchain-ai/langsmith-go/shared"
	"github.com/spf13/cobra"
)

func newFeedbackCmd() *cobra.Command {
	group := &cobra.Command{Use: "feedback", Short: "Create and inspect run feedback"}
	group.AddCommand(newFeedbackCreateCmd(), newFeedbackListCmd(), newFeedbackGetCmd())
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
		if !cmd.Flags().Changed("score") && strings.TrimSpace(comment) == "" {
			return fmt.Errorf("--comment must not be blank when no --score is supplied")
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
		pid, err := resolveSessionID(cmd.Context(), c, project, projectID, "run feedback create")
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
			if GetFormat() == "pretty" {
				fmt.Fprintf(cmd.OutOrStdout(), "Feedback %s: %s\nRun: %s\n", status, id, runID)
				return nil
			}
			return output.OutputJSON(map[string]any{"workspace_id": resultWorkspaceID(), "project_id": pid, "run_id": runID, "feedback_id": id, "status": status, "feedback": got, "next_steps": []string{readNextStep("run", "feedback", "list", "--run-id", runID)}}, "")
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
		return feedbackWriteDiagnostic(createErr, verifyErr)
	}}
	cmd.Long = "Attach a numeric score or comment to a run in a selected project. Omitted scores remain missing; zero is a valid score.\nUse --format json for structured output. Status is created (verified new write), skipped (identical ID already exists), recovered (write response failed but read-back matched), or unverified (outcome unknown, nonzero exit).\nReuse --id with the same content to reconcile retries without creating duplicate feedback."
	cmd.Example = "  langsmith run feedback create --project-id PROJECT_ID --run-id RUN_ID --key correctness --score 1\n  langsmith run feedback create --project-id PROJECT_ID --run-id RUN_ID --key review --comment 'Needs a citation'"
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
	var filters feedbackListFilters
	cmd := &cobra.Command{Use: "list", Short: "List feedback for one run", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if err := resourceUUID(runID); err != nil {
			return err
		}
		if limit < 1 || limit > 100 || offset < 0 || offset > (1<<63-1)-limit {
			return commandDiagnostic{
				"invalid_feedback_pagination",
				"--limit must be 1–100; --offset must be nonnegative and leave room for the next page",
				"Set --limit to an integer from 1 to 100 and --offset to a nonnegative integer whose sum with --limit does not exceed 9223372036854775807.",
			}
		}
		params, err := filters.params(cmd)
		if err != nil {
			return err
		}
		params.Run = langsmith.F[langsmith.FeedbackListParamsRunUnion](shared.UnionString(runID))
		params.Limit, params.Offset = langsmith.F(limit), langsmith.F(offset)
		c, err := getClient()
		if err != nil {
			return err
		}
		page, err := c.SDK.Feedback.List(cmd.Context(), params)
		if err != nil {
			return err
		}
		if GetFormat() == "pretty" {
			if len(page.Items) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No feedback returned for these filters and page. Check the run ID, filters and offset; evaluator feedback may still be pending. Missing feedback is not a passing or failing score.")
			}
			printFeedback(page.Items)
			fmt.Fprintf(cmd.ErrOrStderr(), "Returned %d feedback items (offset %d). A full page may have more; use --offset %d to continue.\n", len(page.Items), offset, offset+limit)
			return nil
		}
		return output.OutputJSON(emptyResultGuidance(map[string]any{"workspace_id": resultWorkspaceID(), "run_id": runID, "items": page.Items, "limit": limit, "offset": offset, "pagination": describeOffsetPage(len(page.Items), limit, offset)}, len(page.Items),
			"No feedback returned for these filters and page. Missing feedback is not a score.", "Check the run ID, metric filters, and offset. Evaluator feedback may still be pending; inspect evaluator execution before treating missing feedback as a result."), "")
	}}
	cmd.Long = "List one page of feedback for a run. Filters are applied by the service. Use --format json for feedback records and pagination metadata; a full page does not prove another page exists."
	cmd.Example = "  langsmith run feedback list --run-id RUN_ID --key correctness --has-score\n  langsmith run feedback list --run-id RUN_ID --source app --has-comment --limit 20"
	filters.flags(cmd)
	cmd.Flags().StringVar(&runID, "run-id", "", "Run UUID (required)")
	cmd.Flags().Int64Var(&limit, "limit", 100, "Page size (1–100)")
	cmd.Flags().Int64Var(&offset, "offset", 0, "Pagination offset")
	return cmd
}

func feedbackWriteDiagnostic(createErr, verifyErr error) error {
	for _, err := range []error{createErr, verifyErr} {
		var apiErr *langsmith.Error
		if !errors.As(err, &apiErr) {
			continue
		}
		switch apiErr.StatusCode {
		case 401, 403:
			return commandDiagnostic{"feedback_permission_denied", "feedback could not be verified because a request was unauthorized", "Check the profile/workspace and feedback read/write permissions. Inspect the emitted ID with run feedback get; reconcile using the same --id after access is restored."}
		case 400, 422:
			return commandDiagnostic{"feedback_invalid_request", "the service rejected a feedback request as invalid; the write remains unverified", "Check the run, project, metric key and score configuration. Inspect the emitted ID with run feedback get before retrying with the same --id."}
		}
	}
	return commandDiagnostic{"feedback_unverified", "feedback write could not be verified", "Use run feedback get with the emitted feedback ID; retry the same request with --id set to that ID to reconcile, not a new ID."}
}
