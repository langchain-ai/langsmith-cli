package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"github.com/spf13/cobra"
)

type queueAddItem struct {
	RunID     string    `json:"run_id,omitempty"`
	ThreadID  string    `json:"thread_id,omitempty"`
	StartTime time.Time `json:"start_time,omitempty"`
}
type queueAddPlan struct {
	Version     int            `json:"version"`
	APIURL      string         `json:"api_url"`
	WorkspaceID string         `json:"workspace_id"`
	ProjectID   string         `json:"project_id"`
	QueueID     string         `json:"queue_id"`
	Items       []queueAddItem `json:"items"`
}

func queueAdditionMatches(result *langsmith.AnnotationQueueItemNewResponse, queueID, projectID string, source queueAddItem) bool {
	if result == nil || len(result.Items) != 1 {
		return false
	}
	item := result.Items[0]
	if item.ID == "" || !sameResourceID(item.QueueID, queueID) || !sameResourceID(item.ProjectID, projectID) {
		return false
	}
	if source.RunID != "" {
		return item.ItemType == "RUN" && sameResourceID(item.RunID, source.RunID)
	}
	return item.ItemType == "THREAD" && item.ThreadID == source.ThreadID
}

func newQueueAddCmd() *cobra.Command {
	var project, projectID, runID, traceID, threadID, filter, planFile, out string
	var limit int
	var dryRun bool
	cmd := &cobra.Command{Use: "add NAME_OR_ID", Short: "Add runs or threads to a queue, optionally from a frozen plan", Args: cobra.ExactArgs(1),
		Long: "Select one run, one root trace, one whole thread, or a bounded root-run filter.\n--dry-run --output plan.json freezes IDs for --plan plan.json. Filter selections\nrequire a dry-run first. Apply never reruns filter discovery; limits fail closed.\nWrites are attempted once per item; inspect queue items before retrying failed writes.",
		RunE: func(cmd *cobra.Command, args []string) error {
			count := 0
			for _, v := range []string{runID, traceID, threadID, filter, planFile} {
				if v != "" {
					count++
				}
			}
			if count != 1 {
				return commandDiagnostic{"invalid_selector", "choose exactly one queue source", "Use --run-id, --trace-id, --thread-id, --filter, or --plan. Run 'langsmith queue add --help' for selection semantics."}
			}
			if limit < 1 || limit > 1000 {
				return commandDiagnostic{"invalid_queue_limit", "limit must be 1–1000", "Set --limit between 1 and 1000; narrow the filter if the selection is larger."}
			}
			if filter != "" && !dryRun {
				return commandDiagnostic{"queue_plan_required", "filter writes require a reviewed plan", "Use --filter with --dry-run --output plan.json, then apply with --plan plan.json instead of --filter."}
			}
			if out != "" && !dryRun {
				return commandDiagnostic{"invalid_queue_output", "--output requires --dry-run", "Add --dry-run to save a selection plan without writing queue items."}
			}
			for _, id := range []string{runID, traceID} {
				if id != "" {
					if err := resourceUUID(id); err != nil {
						return err
					}
				}
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			pid, err := resolveSessionID(cmd.Context(), c, project, projectID, "queue add")
			if err != nil {
				return err
			}
			qid, err := resolveQueue(cmd.Context(), c, args[0])
			if err != nil {
				return err
			}
			plan := queueAddPlan{Version: 1, APIURL: c.APIURL(), WorkspaceID: GetWorkspaceID(), ProjectID: pid, QueueID: qid, Items: []queueAddItem{}}
			if planFile != "" {
				f, err := os.Open(planFile)
				if err != nil {
					return err
				}
				b, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
				f.Close()
				if err != nil {
					return err
				}
				if len(b) > 1<<20 {
					return fmt.Errorf("queue plan exceeds 1 MiB")
				}
				var saved queueAddPlan
				if err = json.Unmarshal(b, &saved); err != nil {
					return err
				}
				if saved.Version != 1 || saved.APIURL != plan.APIURL || saved.WorkspaceID != plan.WorkspaceID || saved.ProjectID != pid || saved.QueueID != qid {
					return fmt.Errorf("plan does not match current endpoint, workspace, project and queue")
				}
				plan = saved
			} else if threadID != "" {
				plan.Items = append(plan.Items, queueAddItem{ThreadID: threadID})
			} else {
				p := langsmith.RunQueryParams{Select: langsmith.F([]langsmith.RunQueryParamsSelect{"id", "start_time"}), Limit: langsmith.F(int64(100))}
				if runID != "" {
					p.ID = langsmith.F([]string{runID})
				}
				if traceID != "" {
					p.ID = langsmith.F([]string{traceID})
					p.IsRoot = langsmith.F(true)
				}
				if filter != "" {
					p.Filter = langsmith.F(filter)
					p.IsRoot = langsmith.F(true)
				}
				runs, err := queryRuns(cmd.Context(), c, p, pid, limit+1, 0)
				if err != nil {
					return err
				}
				if len(runs) > limit {
					return commandDiagnostic{"queue_selection_too_large", "selection exceeds --limit; no items were added", "Narrow --filter or raise --limit up to 1000, then repeat --dry-run and review the saved plan."}
				}
				if len(runs) == 0 {
					return commandDiagnostic{"queue_selection_empty", "no matching runs in the selected project", "Check the project and source ID or filter with trace list/run list. --trace-id accepts only root runs; use --run-id for a child run."}
				}
				for _, r := range runs {
					plan.Items = append(plan.Items, queueAddItem{RunID: r.ID, StartTime: r.StartTime})
				}
			}
			if len(plan.Items) == 0 || len(plan.Items) > 1000 {
				return fmt.Errorf("plan must contain 1–1000 items")
			}
			seen := map[string]bool{}
			for _, item := range plan.Items {
				if (item.RunID == "") == (item.ThreadID == "") {
					return fmt.Errorf("each plan item needs exactly one run or thread")
				}
				if item.RunID != "" {
					if err := resourceUUID(item.RunID); err != nil {
						return err
					}
				}
				key := item.RunID + "\x00" + item.ThreadID
				if seen[key] {
					return fmt.Errorf("plan contains duplicate items")
				}
				seen[key] = true
			}
			if dryRun {
				if out != "" {
					f, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
					if err != nil {
						return err
					}
					err = json.NewEncoder(f).Encode(plan)
					closeErr := f.Close()
					if err != nil {
						return err
					}
					if closeErr != nil {
						return closeErr
					}
				}
				return output.OutputJSON(plan, "")
			}
			results := []map[string]any{}
			failed := 0
			for _, item := range plan.Items {
				p := langsmith.AnnotationQueueItemNewParamsItem{ProjectID: langsmith.F(pid)}
				if item.RunID != "" {
					p.ItemType = langsmith.F(langsmith.AnnotationQueueItemNewParamsItemsItemTypeRun)
					p.RunID = langsmith.F(item.RunID)
					if !item.StartTime.IsZero() {
						p.StartTime = langsmith.F(item.StartTime)
					}
				} else {
					p.ItemType = langsmith.F(langsmith.AnnotationQueueItemNewParamsItemsItemTypeThread)
					p.ThreadID = langsmith.F(item.ThreadID)
				}
				result, err := c.SDK.AnnotationQueues.Items.New(cmd.Context(), qid, langsmith.AnnotationQueueItemNewParams{Items: langsmith.F([]langsmith.AnnotationQueueItemNewParamsItem{p})}, option.WithMaxRetries(0))
				row := map[string]any{"source": item, "status": "submitted", "result": result}
				row["verification"] = "response_matched"
				if err != nil || !queueAdditionMatches(result, qid, pid, item) {
					failed++
					row["status"] = "unverified"
					row["verification"] = "unverified"
					row["error"] = "write could not be verified; inspect queue before retrying"
				}
				results = append(results, row)
			}
			if err := output.OutputJSON(map[string]any{"queue_id": qid, "project_id": pid, "results": results, "failed": failed, "atomic": false, "unverified": failed, "next_steps": []string{"Inspect queue items and the per-item results. Submitted means the response matched the requested item, not that it was reviewed or independently read back. The legacy failed count includes uncertain outcomes, not proven absence of writes.", "Read the queue before retrying unverified items; writes may already have applied. Do not blindly repeat the batch."}}, ""); err != nil {
				return err
			}
			if failed > 0 {
				return commandDiagnostic{"queue_additions_unverified", "One or more queue additions could not be verified", "Inspect the emitted per-item results and read queue items before retrying. Some writes may have applied; the batch is not atomic."}
			}
			return nil
		}}
	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return commandDiagnostic{"invalid_queue_flag", "invalid queue add flag or value", "Run 'langsmith queue add --help'. Select one item with --trace-id, --run-id, or --thread-id; use --filter and a dry-run plan for batches."}
	})
	addProjectFlags(cmd, &project, &projectID)
	cmd.Flags().StringVar(&runID, "run-id", "", "Individual run UUID")
	cmd.Flags().StringVar(&traceID, "trace-id", "", "Root trace UUID")
	cmd.Flags().StringVar(&threadID, "thread-id", "", "Whole thread ID in the selected project")
	cmd.Flags().StringVar(&filter, "filter", "", "LangSmith filter DSL selecting root runs")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum filter matches; never silently truncate")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without adding queue items")
	cmd.Flags().StringVar(&planFile, "plan", "", "Apply frozen plan file")
	cmd.Flags().StringVar(&out, "output", "", "Save dry-run plan to a new file")
	return cmd
}
