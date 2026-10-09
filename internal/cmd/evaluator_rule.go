package cmd

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

func newEvaluatorRuleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rule",
		Short: "Manage the rules that attach evaluators to projects and datasets",
		Long: `Manage evaluator rules. A rule attaches one evaluator to one project or
dataset with its own sampling rate and filters. A rule's name is set when the
evaluator is attached and does not follow later evaluator renames, so rule
output shows both rule_name and evaluator_name.

Examples:
  langsmith evaluator rule list
  langsmith evaluator rule list --project my-app
  langsmith evaluator rule list --evaluator-id <evaluator-id>
  langsmith evaluator rule get <rule-id>
  langsmith evaluator rule delete <rule-id>
  langsmith evaluator rule delete accuracy --dataset my-eval-set --yes`,
	}

	cmd.AddCommand(newEvaluatorRuleListCmd())
	cmd.AddCommand(newEvaluatorRuleGetCmd())
	cmd.AddCommand(newEvaluatorRuleDeleteCmd())
	return cmd
}

func newEvaluatorRuleListCmd() *cobra.Command {
	var (
		outputFile  string
		project     string
		projectID   string
		dataset     string
		evaluatorID string
		all         bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List evaluator rules, optionally for one project, dataset, or evaluator",
		Long: `List evaluator rules, optionally for one project, dataset, or evaluator.
Webhook, add-to-dataset, and annotation-queue rules are hidden unless --all is set.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if evaluatorID != "" {
				if _, err := uuid.Parse(evaluatorID); err != nil {
					return fmt.Errorf("invalid --evaluator-id %q: must be a UUID", evaluatorID)
				}
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			ctx := context.Background()

			var target evaluatorTarget
			if project != "" || projectID != "" || dataset != "" {
				if target, err = resolveEvaluatorTarget(ctx, c, dataset, project, projectID); err != nil {
					return err
				}
			}
			params := target.ruleListParams()
			if evaluatorID != "" {
				params.EvaluatorID = langsmith.F(evaluatorID)
			}
			resp, err := c.SDK.Evaluators.List(ctx, params)
			if err != nil {
				return fmt.Errorf("listing evaluator rules: %w", err)
			}
			// The API applies only one of dataset_id, session_id, and evaluator_id,
			// so the evaluator filter is reapplied when a target is also set.
			rules := slices.DeleteFunc(*resp, func(r langsmith.Evaluator) bool {
				if evaluatorID != "" && r.EvaluatorID != evaluatorID {
					return true
				}
				return !all && !isEvaluatorRule(r)
			})

			if GetFormat() == "pretty" {
				columns := []string{"Rule", "Evaluator", "Target", "Sampling Rate", "Enabled", "Rule ID"}
				var rows [][]string
				for _, r := range rules {
					enabled := "No"
					if r.IsEnabled {
						enabled = "Yes"
					}
					rows = append(rows, []string{
						r.DisplayName, r.EvaluatorName, ruleTarget(r),
						fmt.Sprintf("%.0f%%", r.SamplingRate*100), enabled, r.ID,
					})
				}
				output.OutputTable(columns, rows, "Evaluator Rules")
				return nil
			}
			data := make([]map[string]any, 0, len(rules))
			for _, r := range rules {
				data = append(data, ruleEntry(r))
			}
			return output.OutputJSON(data, outputFile)
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	addProjectFlags(cmd, &project, &projectID)
	cmd.Flags().StringVar(&dataset, "dataset", "", "Only rules on this dataset (name or ID)")
	cmd.Flags().StringVar(&evaluatorID, "evaluator-id", "", "Only rules that use this evaluator")
	cmd.Flags().BoolVar(&all, "all", false, "Include webhook, add-to-dataset, and annotation-queue rules")
	return cmd
}

func newEvaluatorRuleGetCmd() *cobra.Command {
	var outputFile string

	cmd := &cobra.Command{
		Use:   "get RULE_ID",
		Short: "Get an evaluator rule by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := uuid.Parse(args[0]); err != nil {
				return fmt.Errorf("rule get takes a rule ID (see 'langsmith evaluator rule list')")
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			rule, err := getRule(context.Background(), c, args[0])
			if err != nil {
				return err
			}
			return output.OutputJSON(ruleDetail(*rule), outputFile)
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	return cmd
}

func newEvaluatorRuleDeleteCmd() *cobra.Command {
	var (
		project   string
		projectID string
		dataset   string
		yes       bool
	)

	cmd := &cobra.Command{
		Use:   "delete RULE_ID_OR_NAME",
		Short: "Delete one evaluator rule; the evaluator itself is kept",
		Long: `Delete one evaluator rule, detaching its evaluator from that project or
dataset. The evaluator and its other rules are kept. A rule name must be
combined with --project, --project-id, or --dataset and match exactly one rule.

Examples:
  langsmith evaluator rule delete <rule-id>
  langsmith evaluator rule delete accuracy --project my-app --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nameOrID := args[0]
			_, uuidErr := uuid.Parse(nameOrID)
			isID := uuidErr == nil
			if !isID && project == "" && projectID == "" && dataset == "" {
				return errors.New("pass a rule ID, or a rule name with --project, --project-id, or --dataset")
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			ctx := context.Background()

			var rule *langsmith.Evaluator
			if isID {
				rule, err = getRule(ctx, c, nameOrID)
			} else {
				rule, err = findRuleByName(ctx, c, nameOrID, project, projectID, dataset)
			}
			if err != nil {
				return err
			}
			if !isEvaluatorRule(*rule) {
				return fmt.Errorf("rule %s is not an evaluator rule (webhook, add-to-dataset, or annotation-queue rule); not deleted", rule.ID)
			}

			if !yes {
				identity := fmt.Sprintf("Rule: %q (id: %s, evaluator: %q, target: %s)", rule.DisplayName, rule.ID, rule.EvaluatorName, ruleTarget(*rule))
				if actions := ruleSideActions(*rule); len(actions) > 0 {
					identity += "\nThe rule's other actions are deleted with it: " + strings.Join(actions, ", ")
				}
				if err := confirmDelete(cmd, deleteConfirmation{
					target:   "the evaluator rule (the evaluator itself is kept)",
					identity: identity,
				}); err != nil {
					return err
				}
			}

			// The Go SDK has no rule delete method yet.
			if err := c.RawDelete(ctx, fmt.Sprintf("/api/v1/runs/rules/%s", rule.ID), nil); err != nil {
				return fmt.Errorf("deleting evaluator rule %s: %w", rule.ID, err)
			}
			return output.OutputJSON(map[string]any{
				"status":       "deleted",
				"rule_id":      rule.ID,
				"rule_name":    rule.DisplayName,
				"evaluator_id": nilStr(rule.EvaluatorID),
			}, "")
		},
	}

	addProjectFlags(cmd, &project, &projectID)
	cmd.Flags().StringVar(&dataset, "dataset", "", "Dataset (name or ID) the rule is attached to")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	return cmd
}

func getRule(ctx context.Context, c *client.Client, id string) (*langsmith.Evaluator, error) {
	rules, err := c.SDK.Evaluators.List(ctx, langsmith.EvaluatorListParams{ID: langsmith.F([]string{id})})
	if err != nil {
		return nil, fmt.Errorf("fetching evaluator rule %s: %w", id, err)
	}
	for i := range *rules {
		if (*rules)[i].ID == id {
			return &(*rules)[i], nil
		}
	}
	return nil, fmt.Errorf("evaluator rule %s not found", id)
}

func findRuleByName(ctx context.Context, c *client.Client, name, project, projectID, dataset string) (*langsmith.Evaluator, error) {
	target, err := resolveEvaluatorTarget(ctx, c, dataset, project, projectID)
	if err != nil {
		return nil, err
	}
	rules, err := c.SDK.Evaluators.List(ctx, target.ruleListParams())
	if err != nil {
		return nil, fmt.Errorf("listing evaluator rules: %w", err)
	}
	matches := slices.DeleteFunc(findEvaluators(*rules, name, target.datasetID, target.projectID), func(r langsmith.Evaluator) bool {
		return !isEvaluatorRule(r)
	})
	switch len(matches) {
	case 1:
		return &matches[0], nil
	case 0:
		return nil, fmt.Errorf("no evaluator rule named %q on that target", name)
	default:
		ids := make([]string, 0, len(matches))
		for _, r := range matches {
			ids = append(ids, r.ID)
		}
		return nil, fmt.Errorf("%d rules are named %q on that target; pass a rule ID instead: %s", len(matches), name, strings.Join(ids, ", "))
	}
}

// isEvaluatorRule reports whether a rule runs an evaluator. The rules API also
// holds webhook, add-to-dataset, and annotation-queue rules, and older rules
// embed their evaluator inline instead of setting evaluator_id.
func isEvaluatorRule(r langsmith.Evaluator) bool {
	return r.EvaluatorID != "" || len(r.Evaluators) > 0 || len(r.CodeEvaluators) > 0
}

// ruleSideActions describes what a rule does besides running its evaluator.
// Deleting the rule deletes these too.
func ruleSideActions(r langsmith.Evaluator) []string {
	var actions []string
	if n := len(r.Webhooks); n > 0 {
		actions = append(actions, fmt.Sprintf("%d webhook(s)", n))
	}
	if r.AddToDatasetID != "" {
		actions = append(actions, "add to dataset "+nameOrID(r.AddToDatasetName, r.AddToDatasetID))
	}
	if r.AddToAnnotationQueueID != "" {
		actions = append(actions, "add to annotation queue "+nameOrID(r.AddToAnnotationQueueName, r.AddToAnnotationQueueID))
	}
	if n := len(r.Alerts); n > 0 {
		actions = append(actions, fmt.Sprintf("%d alert(s)", n))
	}
	return actions
}

func nameOrID(name, id string) string {
	if name != "" {
		return fmt.Sprintf("%q", name)
	}
	return id
}

func ruleEntry(r langsmith.Evaluator) map[string]any {
	return map[string]any{
		"rule_id":        r.ID,
		"rule_name":      r.DisplayName,
		"evaluator_id":   nilStr(r.EvaluatorID),
		"evaluator_name": nilStr(r.EvaluatorName),
		"session_id":     nilStr(r.SessionID),
		"project":        nilStr(r.SessionName),
		"dataset_id":     nilStr(r.DatasetID),
		"dataset":        nilStr(r.DatasetName),
		"sampling_rate":  r.SamplingRate,
		"is_enabled":     r.IsEnabled,
	}
}

func ruleDetail(r langsmith.Evaluator) map[string]any {
	entry := ruleEntry(r)
	if r.Filter != "" {
		entry["filter"] = r.Filter
	}
	if len(r.CodeEvaluators) > 0 {
		entry["type"] = "code"
		entry["language"] = string(r.CodeEvaluators[0].Language)
		entry["code"] = r.CodeEvaluators[0].Code
	} else if len(r.Evaluators) > 0 {
		entry["type"] = "llm"
		entry["hub_ref"] = r.Evaluators[0].Structured.HubRef
		if len(r.Evaluators[0].Structured.VariableMapping) > 0 {
			entry["variable_mapping"] = r.Evaluators[0].Structured.VariableMapping
		}
	}
	return entry
}

func ruleTarget(r langsmith.Evaluator) string {
	if label := targetLabel(r.SessionName, r.SessionID, r.DatasetName, r.DatasetID); label != "" {
		return label
	}
	return "all runs"
}
