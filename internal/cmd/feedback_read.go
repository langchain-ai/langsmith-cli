package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

func newFeedbackGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "get ID", Short: "Inspect feedback by its UUID", Args: cobra.ExactArgs(1),
		Example: "  langsmith run feedback get FEEDBACK_ID --format json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := resourceUUID(args[0]); err != nil {
				return err
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			feedback, err := c.SDK.Feedback.Get(cmd.Context(), args[0], langsmith.FeedbackGetParams{})
			if err != nil {
				return err
			}
			if GetFormat() == "pretty" {
				printFeedback([]langsmith.FeedbackSchema{*feedback})
				return nil
			}
			return output.OutputJSON(map[string]any{"workspace_id": resultWorkspaceID(), "project_id": nilStr(feedback.SessionID), "run_id": nilStr(feedback.RunID), "feedback_id": feedback.ID, "feedback": feedback}, "")
		},
	}
	return cmd
}

func printFeedback(items []langsmith.FeedbackSchema) {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		// Inspect the raw field so an absent or null score never becomes zero.
		var fields map[string]json.RawMessage
		_ = json.Unmarshal([]byte(item.JSON.RawJSON()), &fields)
		score := "N/A"
		if raw := fields["score"]; len(raw) > 0 && string(raw) != "null" {
			score = string(raw)
		}
		rows = append(rows, []string{item.ID, item.Key, score, item.Comment, item.RunID})
	}
	output.OutputTable([]string{"ID", "Key", "Score", "Comment", "Run ID"}, rows, "Run Feedback")
}

type feedbackListFilters struct {
	keys, sources        []string
	hasScore, hasComment bool
	start, end           string
}

func (f *feedbackListFilters) flags(cmd *cobra.Command) {
	cmd.Flags().StringSliceVar(&f.keys, "key", nil, "Metric keys (repeat or comma-separate)")
	cmd.Flags().StringSliceVar(&f.sources, "source", nil, "Sources: api, app, model, auto_eval (repeat or comma-separate)")
	cmd.Flags().BoolVar(&f.hasScore, "has-score", false, "Filter score presence; use --has-score=false for missing scores")
	cmd.Flags().BoolVar(&f.hasComment, "has-comment", false, "Filter comment presence; use --has-comment=false for missing comments")
	cmd.Flags().StringVar(&f.start, "start-time", "", "Minimum feedback creation timestamp (RFC3339)")
	cmd.Flags().StringVar(&f.end, "end-time", "", "Maximum feedback creation timestamp (RFC3339)")
}

func (f feedbackListFilters) params(cmd *cobra.Command) (langsmith.FeedbackListParams, error) {
	p := langsmith.FeedbackListParams{}
	for _, key := range f.keys {
		if strings.TrimSpace(key) == "" {
			return p, fmt.Errorf("--key must not contain blank metric names")
		}
	}
	if len(f.keys) > 0 {
		p.Key = langsmith.F(f.keys)
	}
	var sources []langsmith.SourceType
	for _, source := range f.sources {
		value := langsmith.SourceType(source)
		if !value.IsKnown() {
			return p, fmt.Errorf("--source must be api, app, model, or auto_eval")
		}
		sources = append(sources, value)
	}
	if len(sources) > 0 {
		p.Source = langsmith.F(sources)
	}
	if cmd.Flags().Changed("has-score") {
		p.HasScore = langsmith.F(f.hasScore)
	}
	if cmd.Flags().Changed("has-comment") {
		p.HasComment = langsmith.F(f.hasComment)
	}
	var start, end time.Time
	for _, bound := range []struct {
		name, value string
		target      *time.Time
	}{{"start-time", f.start, &start}, {"end-time", f.end, &end}} {
		if cmd.Flags().Changed(bound.name) {
			parsed, err := time.Parse(time.RFC3339Nano, bound.value)
			if err != nil {
				return p, fmt.Errorf("--%s must be an RFC3339 timestamp with timezone", bound.name)
			}
			*bound.target = parsed
		}
	}
	if !start.IsZero() && !end.IsZero() && end.Before(start) {
		return p, fmt.Errorf("--end-time must not precede --start-time")
	}
	if !start.IsZero() {
		p.MinCreatedAt = langsmith.F(start)
	}
	if !end.IsZero() {
		p.MaxCreatedAt = langsmith.F(end)
	}
	return p, nil
}
