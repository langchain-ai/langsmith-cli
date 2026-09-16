package cmd

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"github.com/langchain-ai/langsmith-go/shared"
	"github.com/spf13/cobra"
)

type queueRubricFlags struct {
	file         string
	instructions string
}

type queueRubricItem struct {
	FeedbackKey       string             `json:"feedback_key"`
	Description       *string            `json:"description,omitempty"`
	IsRequired        *bool              `json:"is_required,omitempty"`
	IsAssertion       *bool              `json:"is_assertion,omitempty"`
	ScoreDescriptions *map[string]string `json:"score_descriptions,omitempty"`
	ValueDescriptions *map[string]string `json:"value_descriptions,omitempty"`
}

func invalidQueueConfiguration(message string) error {
	return commandDiagnostic{"invalid_queue_configuration", message, "Use queue configure --help. Rubric files must contain an array with unique, nonblank feedback_key values; omitted settings remain unchanged."}
}

func (f *queueRubricFlags) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.file, "rubric", "", "JSON file containing the complete rubric array; [] clears it")
	cmd.Flags().StringVar(&f.instructions, "instructions", "", "Reviewer instructions; an explicit empty string clears them")
}

func (f *queueRubricFlags) params(cmd *cobra.Command) (langsmith.AnnotationQueueUpdateParams, error) {
	p := langsmith.AnnotationQueueUpdateParams{}
	if cmd.Flags().Changed("instructions") {
		p.RubricInstructions = langsmith.F(f.instructions)
	}
	if !cmd.Flags().Changed("rubric") {
		return p, nil
	}
	var items []queueRubricItem
	if err := readDatasetEditFile(f.file, &items); err != nil {
		return p, invalidQueueConfiguration("rubric file must be readable, at most 8 MiB, and valid JSON without unknown fields, duplicate keys, or excessive nesting")
	}
	if items == nil {
		return p, invalidQueueConfiguration("rubric must be an array, not null; use [] to clear it")
	}
	rubric := make([]langsmith.AnnotationQueueRubricItemSchemaParam, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		if item.IsAssertion != nil && *item.IsAssertion {
			return p, invalidQueueConfiguration("queue-wide assertion rubric items are not supported by the current UI; use dataset add --assertions to curate reference criteria")
		}
		key := strings.TrimSpace(item.FeedbackKey)
		if key == "" || key != item.FeedbackKey || seen[key] {
			return p, invalidQueueConfiguration("rubric feedback keys must be unique, nonblank, and have no surrounding whitespace")
		}
		seen[key] = true
		entry := langsmith.AnnotationQueueRubricItemSchemaParam{FeedbackKey: langsmith.F(key)}
		if item.Description != nil {
			entry.Description = langsmith.F(*item.Description)
		}
		if item.IsRequired != nil {
			entry.IsRequired = langsmith.F(*item.IsRequired)
		}
		if item.IsAssertion != nil {
			entry.IsAssertion = langsmith.F(*item.IsAssertion)
		}
		if item.ScoreDescriptions != nil {
			entry.ScoreDescriptions = langsmith.F(*item.ScoreDescriptions)
		}
		if item.ValueDescriptions != nil {
			entry.ValueDescriptions = langsmith.F(*item.ValueDescriptions)
		}
		rubric = append(rubric, entry)
	}
	p.RubricItems = langsmith.F(rubric)
	return p, nil
}

func newQueueGetCmd() *cobra.Command {
	return &cobra.Command{Use: "get NAME_OR_ID", Short: "Read annotation queue settings, instructions, and rubric", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		id, err := resolveQueue(cmd.Context(), c, args[0])
		if err != nil {
			return err
		}
		queue, err := c.SDK.AnnotationQueues.Get(cmd.Context(), id)
		if err != nil {
			return err
		}
		if queue == nil || !sameResourceID(queue.ID, id) {
			return invalidQueueConfiguration("service returned no matching queue")
		}
		return output.OutputJSON(map[string]any{"queue_id": id, "workspace_id": resultWorkspaceID(), "queue": json.RawMessage(queue.JSON.RawJSON())}, "")
	}}
}

func newQueueConfigureCmd() *cobra.Command {
	var flags queueRubricFlags
	var dryRun, apply bool
	cmd := &cobra.Command{
		Use: "configure NAME_OR_ID", Short: "Preview or update reviewer instructions and rubric", Args: cobra.ExactArgs(1),
		Long: `Update reviewer instructions and rubric using existing queue APIs.
Supply --instructions and/or --rubric, and exactly one of --dry-run or --apply.
Omitted fields stay unchanged. The rubric replaces all current criteria; [] clears
it. An explicit empty instruction string clears instructions. Preview is not a
frozen plan or authorization check; concurrent edits can overwrite one another.
These settings guide human reviewers, not an automated evaluator. Existing feedback
is not rescored, and score descriptions do not create workspace feedback schemas.`,
		Example: "  langsmith queue get QUEUE_ID\n  langsmith queue configure QUEUE_ID --rubric rubric.json --instructions 'Check correctness' --dry-run\n  langsmith queue configure QUEUE_ID --rubric rubric.json --instructions 'Check correctness' --apply",
	}
	flags.addFlags(cmd)
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview supplied settings without changing the queue")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply supplied settings to the queue")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRun == apply || !(cmd.Flags().Changed("rubric") || cmd.Flags().Changed("instructions")) {
			return invalidQueueConfiguration("supply edit flags and exactly one of --dry-run or --apply; use queue get to read")
		}
		params, err := flags.params(cmd)
		if err != nil {
			return err
		}
		c, err := getClient()
		if err != nil {
			return err
		}
		id, err := resolveQueue(cmd.Context(), c, args[0])
		if err != nil {
			return err
		}
		result := map[string]any{"queue_id": id, "workspace_id": resultWorkspaceID(), "changes": params, "status": "dry_run"}
		if err := validateQueueFeedbackConfigs(cmd.Context(), c, params); err != nil {
			return err
		}
		current, err := c.SDK.AnnotationQueues.Get(cmd.Context(), id)
		if err != nil {
			return err
		}
		preserved, err := preserveQueueReviewSettings(current, &params)
		if err != nil {
			return err
		}
		result["preserved_settings"] = preserved
		result["warnings"] = []string{"Reviewer settings are read and resent because the API resets omitted values. Concurrent changes between this read and update can be overwritten."}
		if dryRun {
			result["next_steps"] = []string{"Review the replacement rubric and instructions. Repeat with --apply instead of --dry-run; the file and queue can change between commands."}
			return output.OutputJSON(result, "")
		}
		if _, err := c.SDK.AnnotationQueues.Update(cmd.Context(), id, params, option.WithMaxRetries(0)); err != nil {
			return err
		}
		result["status"] = "updated"
		result["verification"] = "acknowledged_not_read_back"
		result["next_steps"] = []string{"Use queue get with the returned queue_id to verify saved settings before retrying a write."}
		return output.OutputJSON(result, "")
	}
	return cmd
}

func validateQueueFeedbackConfigs(ctx context.Context, c *client.Client, params langsmith.AnnotationQueueUpdateParams) error {
	if len(params.RubricItems.Value) == 0 {
		return nil
	}
	var configs []struct {
		Key    string `json:"feedback_key"`
		Config *struct {
			Type string `json:"type"`
		} `json:"feedback_config"`
	}
	// The pinned SDK exposes deletion but not listing of feedback configurations.
	if err := c.RawGet(ctx, "/api/v1/feedback-configs", &configs); err != nil {
		return commandDiagnostic{"queue_feedback_configs_unavailable", "could not verify workspace feedback configurations; no queue write attempted", "Check the profile, workspace permissions, and deployment support for GET /api/v1/feedback-configs before retrying."}
	}
	keys := map[string]bool{}
	for _, config := range configs {
		if config.Config != nil && config.Config.Type != "" {
			keys[config.Key] = true
		}
	}
	for _, item := range params.RubricItems.Value {
		if !keys[item.FeedbackKey.Value] {
			return commandDiagnostic{"queue_feedback_config_missing", "one or more rubric feedback keys lack a workspace configuration; no queue write attempted", "Inspect langsmith api feedback-configs. Use configured keys, or create the missing feedback configuration in workspace settings before retrying. A rubric alone does not create score controls."}
		}
	}
	return nil
}

// The update API assigns defaults to these omitted fields instead of preserving
// their stored values. Resend the snapshot, retaining nulls rather than zeros.
func preserveQueueReviewSettings(current *langsmith.AnnotationQueueGetResponse, params *langsmith.AnnotationQueueUpdateParams) (map[string]json.RawMessage, error) {
	if current == nil {
		return nil, invalidQueueConfiguration("cannot preserve reviewer settings without a queue response")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(current.JSON.RawJSON()), &raw); err != nil {
		return nil, invalidQueueConfiguration("cannot read stored reviewer settings")
	}
	preserved := map[string]json.RawMessage{}
	for _, key := range []string{"num_reviewers_per_item", "enable_reservations", "reservation_minutes"} {
		if raw[key] == nil {
			return nil, invalidQueueConfiguration("service omitted reviewer settings; refusing an update that could reset them")
		}
		preserved[key] = raw[key]
	}
	var state struct {
		Reviewers *int64 `json:"num_reviewers_per_item"`
		Enabled   *bool  `json:"enable_reservations"`
		Minutes   *int64 `json:"reservation_minutes"`
	}
	if err := json.Unmarshal([]byte(current.JSON.RawJSON()), &state); err != nil || state.Enabled == nil {
		return nil, invalidQueueConfiguration("stored reviewer settings cannot be safely preserved")
	}
	params.EnableReservations = langsmith.F(*state.Enabled)
	params.NumReviewersPerItem = langsmith.Null[langsmith.AnnotationQueueUpdateParamsNumReviewersPerItemUnion]()
	if state.Reviewers != nil {
		params.NumReviewersPerItem = langsmith.F[langsmith.AnnotationQueueUpdateParamsNumReviewersPerItemUnion](shared.UnionInt(*state.Reviewers))
	}
	params.ReservationMinutes = langsmith.Null[int64]()
	if state.Minutes != nil {
		params.ReservationMinutes = langsmith.F(*state.Minutes)
	}
	return preserved, nil
}
