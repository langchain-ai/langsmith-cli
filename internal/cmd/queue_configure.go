package cmd

import (
	"encoding/json"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
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
