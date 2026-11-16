package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/url"
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

// newRulesAliasCmd is a hidden top-level "rules" for agents that look for
// rules outside "evaluator".
func newRulesAliasCmd() *cobra.Command {
	cmd := newEvaluatorRuleCmd()
	cmd.Use, cmd.Aliases, cmd.Hidden = "rules", []string{"rule"}, true
	return cmd
}

func newEvaluatorRuleListCmd() *cobra.Command {
	var (
		outputFile   string
		project      string
		projectID    string
		dataset      string
		evaluatorID  string
		nameContains string
		targetType   string
		limit        int
		cursor       string
		all          bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List evaluator rules, optionally for one project, dataset, or evaluator",
		Long: `List evaluator rules, newest change first, optionally for one project,
dataset, or evaluator. Filters combine. Webhook, add-to-dataset, and
annotation-queue rules are hidden unless --all is set.
Each rule's actions field lists what it does: evaluator, webhook, add_to_dataset,
add_to_annotation_queue, or alert. 'rule get' shows the details.

Examples:
  langsmith evaluator rule list --project my-app
  langsmith evaluator rule list --type dataset --name-contains accuracy
  langsmith evaluator rule list --limit 20 --cursor ""`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if evaluatorID != "" {
				if _, err := uuid.Parse(evaluatorID); err != nil {
					return fmt.Errorf("invalid --evaluator-id %q: must be a UUID", evaluatorID)
				}
			}
			var sourceType langsmith.EvaluatorListV2ParamsType
			switch strings.ToLower(targetType) {
			case "":
			case "project":
				sourceType = langsmith.EvaluatorListV2ParamsTypeProject
			case "dataset":
				sourceType = langsmith.EvaluatorListV2ParamsTypeDataset
			default:
				return fmt.Errorf("invalid --type %q: must be project or dataset", targetType)
			}
			if limit < 0 {
				return fmt.Errorf("--limit must not be negative")
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
			if nameContains != "" {
				params.NameContains = langsmith.F(nameContains)
			}
			if sourceType != "" {
				params.Type = langsmith.F(sourceType)
			}
			keep := func(r langsmith.Rule) bool { return all || isEvaluatorRule(r) }
			rules, next, err := listRulesPage(ctx, c, params, cursor, limit, keep)
			if err != nil {
				return fmt.Errorf("listing evaluator rules: %w", err)
			}

			if GetFormat() == "pretty" {
				columns := []string{"Rule", "Evaluator", "Target", "Actions", "Sampling Rate", "Enabled", "Rule ID"}
				var rows [][]string
				for _, r := range rules {
					enabled := "No"
					if r.IsEnabled {
						enabled = "Yes"
					}
					rows = append(rows, []string{
						r.DisplayName, r.EvaluatorName, ruleTarget(r), strings.Join(ruleActions(r), ", "),
						fmt.Sprintf("%.0f%%", r.SamplingRate*100), enabled, r.ID,
					})
				}
				output.OutputTable(columns, rows, "Evaluator Rules")
				printNextCursorHint(next)
				return nil
			}
			data := make([]map[string]any, 0, len(rules))
			for _, r := range rules {
				data = append(data, ruleEntry(r))
			}
			var out any = data
			if cmd.Flags().Changed("cursor") {
				out = map[string]any{"rules": data, "cursors": cursorsField(next)}
			}
			return output.OutputJSON(out, outputFile)
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	addProjectFlags(cmd, &project, &projectID)
	cmd.Flags().StringVar(&dataset, "dataset", "", "Only rules on this dataset (name or ID)")
	cmd.Flags().StringVar(&evaluatorID, "evaluator-id", "", "Only rules that use this evaluator")
	cmd.Flags().StringVar(&nameContains, "name-contains", "", "Only rules whose name contains this text (case-insensitive; _ matches any single character and % any sequence)")
	cmd.Flags().StringVar(&targetType, "type", "", "Only rules on a project or a dataset: project or dataset")
	cmd.Flags().IntVarP(&limit, "limit", "n", 0, "Maximum number of rules to return (0 for all)")
	addCursorFlag(cmd, &cursor)
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

			var rule *langsmith.Rule
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

func getRule(ctx context.Context, c *client.Client, id string) (*langsmith.Rule, error) {
	rules, err := listRules(ctx, c, langsmith.EvaluatorListV2Params{ID: langsmith.F([]string{id})})
	if err != nil {
		return nil, fmt.Errorf("fetching evaluator rule %s: %w", id, err)
	}
	for i := range rules {
		if rules[i].ID == id {
			return &rules[i], nil
		}
	}
	return nil, fmt.Errorf("evaluator rule %s not found", id)
}

func findRuleByName(ctx context.Context, c *client.Client, name, project, projectID, dataset string) (*langsmith.Rule, error) {
	target, err := resolveEvaluatorTarget(ctx, c, dataset, project, projectID)
	if err != nil {
		return nil, err
	}
	rules, err := listRules(ctx, c, target.ruleListParams())
	if err != nil {
		return nil, fmt.Errorf("listing evaluator rules: %w", err)
	}
	matches := slices.DeleteFunc(findEvaluators(rules, name, target.datasetID, target.projectID), func(r langsmith.Rule) bool {
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

// maxRulePageSize is the largest page the rule list accepts.
const maxRulePageSize = 100

// listRules returns every rule matching params.
func listRules(ctx context.Context, c *client.Client, params langsmith.EvaluatorListV2Params) ([]langsmith.Rule, error) {
	rules, _, err := listRulesPage(ctx, c, params, "", 0, nil)
	return rules, err
}

// listRulesPage returns up to limit rules (all when limit is 0) matching
// params, starting at cursor, with the cursor for the next page. keep, when
// non-nil, drops rules client-side.
func listRulesPage(ctx context.Context, c *client.Client, params langsmith.EvaluatorListV2Params, cursor string, limit int, keep func(langsmith.Rule) bool) ([]langsmith.Rule, string, error) {
	fetch := func(cursor string, pageSize int) ([]langsmith.Rule, string, error) {
		page := params
		page.PageSize = langsmith.F(int64(pageSize))
		if cursor != "" {
			page.Cursor = langsmith.F(cursor)
		}
		res, err := c.SDK.Evaluators.ListV2(ctx, page)
		if err != nil {
			return nil, "", err
		}
		return res.Items, res.NextCursor, nil
	}
	return collectPages(cursor, limit, maxRulePageSize, fetch, keep)
}

// isEvaluatorRule reports whether a rule runs an evaluator. The rules API also
// holds webhook, add-to-dataset, and annotation-queue rules, and older rules
// embed their evaluator inline instead of setting evaluator_id.
func isEvaluatorRule(r langsmith.Rule) bool {
	return r.EvaluatorID != "" || len(r.Evaluators) > 0 || len(r.CodeEvaluators) > 0
}

// ruleSideActions describes what a rule does besides running its evaluator.
// Deleting the rule deletes these too.
func ruleSideActions(r langsmith.Rule) []string {
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

// ruleActions names what a rule does to each sampled run: evaluator, webhook,
// add_to_dataset, add_to_annotation_queue, and alert.
func ruleActions(r langsmith.Rule) []string {
	actions := []string{}
	if isEvaluatorRule(r) {
		actions = append(actions, "evaluator")
	}
	if len(r.Webhooks) > 0 {
		actions = append(actions, "webhook")
	}
	if r.AddToDatasetID != "" {
		actions = append(actions, "add_to_dataset")
	}
	if r.AddToAnnotationQueueID != "" {
		actions = append(actions, "add_to_annotation_queue")
	}
	if len(r.Alerts) > 0 {
		actions = append(actions, "alert")
	}
	return actions
}

// maskWebhookURL keeps only the scheme and host of a webhook URL. Webhook URLs
// often carry secrets in their userinfo, query string, or path (for example
// chat-app incoming webhooks), so the rest is never printed.
func maskWebhookURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "(hidden)"
	}
	masked := u.Scheme + "://" + u.Host
	if (u.Path != "" && u.Path != "/") || u.RawQuery != "" {
		masked += "/..."
	}
	return masked
}

func ruleEntry(r langsmith.Rule) map[string]any {
	return map[string]any{
		"rule_id":        r.ID,
		"rule_name":      r.DisplayName,
		"evaluator_id":   nilStr(r.EvaluatorID),
		"evaluator_name": nilStr(r.EvaluatorName),
		"session_id":     nilStr(r.ProjectID),
		"project":        nilStr(r.ProjectName),
		"dataset_id":     nilStr(r.DatasetID),
		"dataset":        nilStr(r.DatasetName),
		"sampling_rate":  r.SamplingRate,
		"is_enabled":     r.IsEnabled,
		"actions":        ruleActions(r),
	}
}

// ruleDetail adds a rule's settings to its list entry. The rules API returns
// enum values in upper case; they are printed in lower case.
func ruleDetail(r langsmith.Rule) map[string]any {
	entry := ruleEntry(r)
	for key, value := range map[string]string{
		"filter":       r.Filter,
		"trace_filter": r.TraceFilter,
		"tree_filter":  r.TreeFilter,
		"group_by":     strings.ToLower(string(r.GroupBy)),
	} {
		if value != "" {
			entry[key] = value
		}
	}
	if len(r.Webhooks) > 0 {
		// Headers usually hold credentials, so only the masked URL is shown.
		webhooks := make([]map[string]any, 0, len(r.Webhooks))
		for _, w := range r.Webhooks {
			webhooks = append(webhooks, map[string]any{"url": maskWebhookURL(w.URL)})
		}
		entry["webhooks"] = webhooks
	}
	if r.AddToDatasetID != "" {
		entry["add_to_dataset"] = map[string]any{
			"id":                r.AddToDatasetID,
			"name":              nilStr(r.AddToDatasetName),
			"prefer_correction": r.AddToDatasetPreferCorrection,
		}
	}
	if r.AddToAnnotationQueueID != "" {
		entry["add_to_annotation_queue"] = map[string]any{
			"id":   r.AddToAnnotationQueueID,
			"name": nilStr(r.AddToAnnotationQueueName),
		}
	}
	if len(r.Alerts) > 0 {
		// The routing key is a credential, so it is left out.
		alerts := make([]map[string]any, 0, len(r.Alerts))
		for _, a := range r.Alerts {
			alerts = append(alerts, map[string]any{
				"type":     nilStr(strings.ToLower(string(a.Type))),
				"severity": nilStr(strings.ToLower(string(a.Severity))),
				"summary":  nilStr(a.Summary),
			})
		}
		entry["alerts"] = alerts
	}
	if r.SpendLimit.LimitUsd != "" {
		entry["spend_limit"] = map[string]any{
			"limit_usd": r.SpendLimit.LimitUsd,
			"window":    strings.ToLower(string(r.SpendLimit.Window)),
		}
	}
	if len(r.CodeEvaluators) > 0 {
		entry["type"] = "code"
		entry["language"] = strings.ToLower(string(r.CodeEvaluators[0].Language))
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

func ruleTarget(r langsmith.Rule) string {
	if label := targetLabel(r.ProjectName, r.ProjectID, r.DatasetName, r.DatasetID); label != "" {
		return label
	}
	return "all runs"
}
