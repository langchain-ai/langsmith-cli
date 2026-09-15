package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"github.com/spf13/cobra"
	"strings"
)

func resolveQueue(ctx context.Context, c *client.Client, name string) (string, error) {
	if uuid.Validate(name) == nil {
		q, err := c.SDK.AnnotationQueues.Get(ctx, name)
		if err != nil {
			return "", err
		}
		return q.ID, nil
	}
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("queue name or UUID is required")
	}
	page, err := c.SDK.AnnotationQueues.GetAnnotationQueues(ctx, langsmith.AnnotationQueueGetAnnotationQueuesParams{Name: langsmith.F(name), Limit: langsmith.F(int64(2))})
	if err != nil {
		return "", err
	}
	if len(page.Items) != 1 || page.Items[0].Name != name {
		return "", fmt.Errorf("queue name is absent or ambiguous; use a UUID")
	}
	return page.Items[0].ID, nil
}

func newQueueCmd() *cobra.Command {
	group := &cobra.Command{Use: "queue", Short: "Manage annotation queues"}
	var name, description, dataset string
	create := &cobra.Command{Use: "create", Short: "Create an annotation queue", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("--name is required")
		}
		c, err := getClient()
		if err != nil {
			return err
		}
		p := langsmith.AnnotationQueueAnnotationQueuesParams{Name: langsmith.F(name), Description: langsmith.F(description)}
		if dataset != "" {
			ds, err := resolveDataset(cmd.Context(), c, dataset)
			if err != nil {
				return err
			}
			p.DefaultDataset = langsmith.F(ds.ID)
		}
		result, err := c.SDK.AnnotationQueues.AnnotationQueues(cmd.Context(), p, option.WithMaxRetries(0))
		if err != nil {
			return err
		}
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		var response map[string]json.RawMessage
		if err := json.Unmarshal(data, &response); err != nil {
			return err
		}
		if response == nil || result.ID == "" {
			return fmt.Errorf("queue creation returned no resource ID")
		}
		response["status"] = json.RawMessage(`"created"`)
		return output.OutputJSON(response, "")
	}}
	create.Flags().StringVar(&name, "name", "", "Queue name (required)")
	create.Flags().StringVar(&description, "description", "", "Queue description")
	create.Flags().StringVar(&dataset, "dataset", "", "Default dataset name or UUID")
	var limit, offset int64
	list := &cobra.Command{Use: "list", Short: "List annotation queues", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if limit < 1 || limit > 1000 || offset < 0 || offset > (1<<63-1)-limit {
			return fmt.Errorf("limit must be 1–1000; offset must be nonnegative and leave room for the next page")
		}
		c, err := getClient()
		if err != nil {
			return err
		}
		page, err := c.SDK.AnnotationQueues.GetAnnotationQueues(cmd.Context(), langsmith.AnnotationQueueGetAnnotationQueuesParams{Limit: langsmith.F(limit), Offset: langsmith.F(offset)})
		if err != nil {
			return err
		}
		return output.OutputJSON(map[string]any{"workspace_id": resultWorkspaceID(), "items": page.Items, "limit": limit, "offset": offset, "pagination": describeOffsetPage(len(page.Items), limit, offset)}, "")
	}}
	list.Flags().Int64Var(&limit, "limit", 100, "Page size (1–1000)")
	list.Flags().Int64Var(&offset, "offset", 0, "Pagination offset")
	var yes bool
	del := &cobra.Command{Use: "delete NAME_OR_ID", Short: "Delete an annotation queue", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !yes && (GetFormat() == "json" || !inputIsTerminal(cmd.InOrStdin())) {
			return fmt.Errorf("deletion requires explicit --yes in noninteractive mode")
		}
		c, err := getClient()
		if err != nil {
			return err
		}
		id, err := resolveQueue(cmd.Context(), c, args[0])
		if err != nil {
			return err
		}
		if !yes {
			if err := confirmDelete(cmd, deleteConfirmation{target: "annotation queue", identity: id}); err != nil {
				return err
			}
		}
		_, err = c.SDK.AnnotationQueues.Delete(cmd.Context(), id, option.WithMaxRetries(0))
		if err != nil {
			return err
		}
		return output.OutputJSON(map[string]any{"queue_id": id, "status": "deleted"}, "")
	}}
	del.Flags().BoolVar(&yes, "yes", false, "Confirm queue deletion")
	var status, cursor string
	var pageSize int64
	items := &cobra.Command{Use: "items NAME_OR_ID", Short: "List a page of queue items awaiting review", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		s := langsmith.AnnotationQueueItemListParamsStatus(status)
		if !s.IsKnown() || pageSize < 1 || pageSize > 100 {
			return fmt.Errorf("invalid review status or page size (1–100)")
		}
		c, err := getClient()
		if err != nil {
			return err
		}
		id, err := resolveQueue(cmd.Context(), c, args[0])
		if err != nil {
			return err
		}
		p := langsmith.AnnotationQueueItemListParams{Status: langsmith.F(s), PageSize: langsmith.F(pageSize)}
		if cursor != "" {
			p.Cursor = langsmith.F(cursor)
		}
		page, err := c.SDK.AnnotationQueues.Items.List(cmd.Context(), id, p)
		if err != nil {
			return err
		}
		return output.OutputJSON(map[string]any{"workspace_id": resultWorkspaceID(), "queue_id": id, "items": page.Items, "next_cursor": page.NextCursor, "has_more": page.NextCursor != "", "returned": len(page.Items)}, "")
	}}
	items.Flags().StringVar(&status, "status", "needs_my_review", "needs_my_review, needs_others_review, or archived")
	items.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor")
	items.Flags().Int64Var(&pageSize, "limit", 20, "Page size (1–100)")
	group.AddCommand(create, list, del, items, newQueueAddCmd())
	return group
}
