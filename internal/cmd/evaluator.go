package cmd

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newEvaluatorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "evaluator",
		Short: "Manage online and offline evaluator rules",
		Long: `Manage online and offline evaluator rules.

Evaluators are Python functions uploaded to LangSmith that automatically
score runs. They can target a specific dataset (offline/experiment
evaluators) or a project (online evaluators that score production runs).

Examples:
  langsmith evaluator list
  langsmith evaluator upload eval.py --name accuracy --function check_accuracy --dataset my-eval-set
  langsmith evaluator delete accuracy --yes`,
	}

	cmd.AddCommand(newEvaluatorListCmd())
	cmd.AddCommand(newEvaluatorUploadCmd())
	cmd.AddCommand(newEvaluatorDeleteCmd())
	return cmd
}

// evaluatorRule matches the JSON from GET /runs/rules.
type evaluatorRule struct {
	ID               string   `json:"id"`
	DisplayName      string   `json:"display_name"`
	SamplingRate     float64  `json:"sampling_rate"`
	IsEnabled        bool     `json:"is_enabled"`
	DatasetID        string   `json:"dataset_id"`
	SessionID        string   `json:"session_id"`
	TargetDatasetIDs []string `json:"target_dataset_ids"`
	TargetProjectIDs []string `json:"target_project_ids"`
}

func newEvaluatorListCmd() *cobra.Command {
	var outputFile string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all evaluator rules in the workspace",
		Run: func(cmd *cobra.Command, args []string) {
			c := mustGetClient()
			ctx := context.Background()

			var rules []evaluatorRule
			if err := c.RawGet(ctx, "/runs/rules", &rules); err != nil {
				exitErrorf("listing evaluators: %v", err)
			}

			fmt_ := getFormat()

			if fmt_ == "pretty" {
				columns := []string{"Name", "Sampling Rate", "Target", "Enabled"}
				var rows [][]string
				for _, rule := range rules {
					rate := fmt.Sprintf("%.0f%%", rule.SamplingRate*100)
					target := "All runs"
					if rule.DatasetID != "" || len(rule.TargetDatasetIDs) > 0 {
						target = "dataset"
					} else if rule.SessionID != "" || len(rule.TargetProjectIDs) > 0 {
						target = "project"
					}
					enabled := "No"
					if rule.IsEnabled {
						enabled = "Yes"
					}
					rows = append(rows, []string{rule.DisplayName, rate, target, enabled})
				}
				output.OutputTable(columns, rows, "Evaluator Rules")
			} else {
				var data []map[string]any
				for _, rule := range rules {
					data = append(data, map[string]any{
						"id":            rule.ID,
						"name":          rule.DisplayName,
						"sampling_rate": rule.SamplingRate,
						"is_enabled":    rule.IsEnabled,
						"dataset_id":    nilStr(rule.DatasetID),
						"session_id":    nilStr(rule.SessionID),
					})
				}
				output.OutputJSON(data, outputFile)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	return cmd
}

func newEvaluatorUploadCmd() *cobra.Command {
	var (
		name          string
		funcName      string
		targetDataset string
		targetProject string
		samplingRate  float64
		replace       bool
		yes           bool
	)

	cmd := &cobra.Command{
		Use:   "upload EVALUATOR_FILE",
		Short: "Upload a Python evaluator function to LangSmith",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			evaluatorFile := args[0]

			if targetDataset == "" && targetProject == "" {
				output.OutputJSON(map[string]any{
					"error": "Must specify --dataset or --project (global evaluators not supported)",
				}, "")
				return
			}

			c := mustGetClient()
			ctx := context.Background()

			// Resolve targets
			var datasetID, projectID string

			if targetDataset != "" {
				ds, err := resolveDataset(ctx, c, targetDataset)
				if err != nil {
					exitErrorf("%v", err)
				}
				datasetID = ds.ID
			}

			if targetProject != "" {
				sid, err := c.ResolveSessionID(ctx, targetProject)
				if err != nil {
					exitErrorf("%v", err)
				}
				projectID = sid
			}

			// Check for existing evaluator
			var rules []evaluatorRule
			if err := c.RawGet(ctx, "/runs/rules", &rules); err != nil {
				exitErrorf("checking existing evaluators: %v", err)
			}

			existing := findEvaluator(rules, name, datasetID, projectID)
			if existing != nil {
				if !replace {
					output.OutputJSON(map[string]any{
						"error": fmt.Sprintf("Evaluator '%s' already exists (use --replace to overwrite)", name),
						"id":    existing.ID,
					}, "")
					return
				}
				if !yes {
					fmt.Fprintf(os.Stderr, "Replace existing evaluator '%s'? [y/N] ", name)
					var confirm string
					_, _ = fmt.Scanln(&confirm)
					if strings.ToLower(confirm) != "y" {
						exitError("aborted")
					}
				}
				if err := c.RawDelete(ctx, fmt.Sprintf("/runs/rules/%s", existing.ID), nil); err != nil {
					exitErrorf("deleting existing evaluator: %v", err)
				}
			}

			// Read and prepare function source
			source, err := os.ReadFile(evaluatorFile)
			if err != nil {
				exitErrorf("reading evaluator file: %v", err)
			}

			sourceStr := string(source)

			// Rename function to perform_eval
			re := regexp.MustCompile(`\bdef\s+` + regexp.QuoteMeta(funcName) + `\s*\(`)
			sourceStr = re.ReplaceAllString(sourceStr, "def perform_eval(")

			// Build payload
			payload := map[string]any{
				"display_name":           name,
				"sampling_rate":          samplingRate,
				"is_enabled":             true,
				"include_extended_stats": false,
				"code_evaluators": []map[string]any{
					{"code": sourceStr, "language": "python"},
				},
			}

			if datasetID != "" {
				payload["dataset_id"] = datasetID
			}
			if projectID != "" {
				payload["session_id"] = projectID
			}

			var result map[string]any
			if err := c.RawPost(ctx, "/runs/rules", payload, &result); err != nil {
				exitErrorf("uploading evaluator: %v", err)
			}

			target := "project"
			if datasetID != "" {
				target = "dataset"
			}

			output.OutputJSON(map[string]any{
				"status": "uploaded",
				"id":     result["id"],
				"name":   name,
				"target": target,
			}, "")
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Display name for the evaluator (required)")
	cmd.Flags().StringVar(&funcName, "function", "", "Name of the Python function to upload (required)")
	cmd.Flags().StringVar(&targetDataset, "dataset", "", "Target dataset name (offline evaluator)")
	cmd.Flags().StringVar(&targetProject, "project", "", "Target project name (online evaluator)")
	cmd.Flags().Float64Var(&samplingRate, "sampling-rate", 1.0, "Fraction of runs to evaluate (0.0-1.0)")
	cmd.Flags().BoolVar(&replace, "replace", false, "Replace existing evaluator with same name")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt when replacing")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("function")

	return cmd
}

func newEvaluatorDeleteCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete an evaluator rule by its display name",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]

			c := mustGetClient()
			ctx := context.Background()

			var rules []evaluatorRule
			if err := c.RawGet(ctx, "/runs/rules", &rules); err != nil {
				exitErrorf("listing evaluators: %v", err)
			}

			var matching []evaluatorRule
			for _, r := range rules {
				if r.DisplayName == name {
					matching = append(matching, r)
				}
			}

			if len(matching) == 0 {
				output.OutputJSON(map[string]any{"error": fmt.Sprintf("Evaluator '%s' not found", name)}, "")
				return
			}

			if !yes {
				fmt.Fprintf(os.Stderr, "Delete evaluator '%s'? [y/N] ", name)
				var confirm string
				_, _ = fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					exitError("aborted")
				}
			}

			deleted := 0
			for _, rule := range matching {
				if err := c.RawDelete(ctx, fmt.Sprintf("/runs/rules/%s", rule.ID), nil); err != nil {
					exitErrorf("deleting evaluator %s: %v", rule.ID, err)
				}
				deleted++
			}

			output.OutputJSON(map[string]any{
				"status": "deleted",
				"name":   name,
				"count":  deleted,
			}, "")
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	return cmd
}

func findEvaluator(rules []evaluatorRule, name, datasetID, projectID string) *evaluatorRule {
	for _, rule := range rules {
		if rule.DisplayName != name {
			continue
		}
		if datasetID != "" && rule.DatasetID == datasetID {
			return &rule
		}
		if projectID != "" && rule.SessionID == projectID {
			return &rule
		}
	}
	return nil
}
