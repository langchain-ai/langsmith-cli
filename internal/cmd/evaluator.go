package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

func newEvaluatorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "evaluator",
		Short: "Manage evaluators and the rules that attach them to projects and datasets",
		Long: `Manage evaluators and their rules.

An evaluator is the shared scorer (LLM-as-judge prompt or code) shown on the
Evaluators page. A rule attaches an evaluator to one project (online) or
dataset (offline) with its own sampling rate and filters. One evaluator can
have many rules, and a rule's name can differ from its evaluator's name.

  list, get, delete   Act on evaluators
  rule                Act on rules (list, get, delete)
  upload              Create a code evaluator rule from a Python or JavaScript/TypeScript file
  create-llm          Create an LLM-as-judge evaluator rule (--model-config required)

Examples:
  langsmith evaluator list
  langsmith evaluator get ready_for_task_grade
  langsmith evaluator delete <evaluator-id> --delete-rules --yes
  langsmith evaluator rule list --project my-app
  langsmith evaluator rule delete accuracy --project my-app --yes
  langsmith evaluator upload eval.py --name accuracy --function check_accuracy --dataset my-eval-set
  langsmith evaluator create-llm --name relevance --project my-app --prompt prompt.json --schema schema.json --model-config model.json`,
	}

	cmd.AddCommand(newEvaluatorGetCmd())
	cmd.AddCommand(newEvaluatorListCmd())
	cmd.AddCommand(newEvaluatorDeleteCmd())
	cmd.AddCommand(newEvaluatorRuleCmd())
	cmd.AddCommand(newEvaluatorUploadCmd())
	cmd.AddCommand(newEvaluatorCreateLLMCmd())
	return cmd
}

func newEvaluatorListCmd() *cobra.Command {
	var (
		outputFile   string
		evalType     string
		nameContains string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List evaluators and the projects or datasets each one is attached to",
		Long: `List evaluators in the workspace. Each entry includes the rules that attach
it to projects or datasets. Use 'langsmith evaluator rule list' for rule details.

Examples:
  langsmith evaluator list
  langsmith evaluator list --type llm
  langsmith evaluator list --name-contains grade`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			ctx := context.Background()

			params := langsmith.OnlineEvaluatorListParams{}
			if evalType != "" {
				params.Type = langsmith.F(evalType)
			}
			if nameContains != "" {
				params.NameContains = langsmith.F(nameContains)
			}
			evaluators, err := listOnlineEvaluators(ctx, c, params)
			if err != nil {
				return err
			}

			if GetFormat() == "pretty" {
				columns := []string{"Name", "Type", "Attached To", "ID"}
				var rows [][]string
				for _, ev := range evaluators {
					rows = append(rows, []string{ev.Name, string(ev.Type), ruleTargetsSummary(ev.RunRules), ev.ID})
				}
				output.OutputTable(columns, rows, "Evaluators")
				return nil
			}
			data := make([]map[string]any, 0, len(evaluators))
			for _, ev := range evaluators {
				data = append(data, evaluatorEntry(ev))
			}
			return output.OutputJSON(data, outputFile)
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	cmd.Flags().StringVar(&evalType, "type", "", "Filter by evaluator type (llm or code)")
	cmd.Flags().StringVar(&nameContains, "name-contains", "", "Filter by name substring")
	return cmd
}

func newEvaluatorGetCmd() *cobra.Command {
	var outputFile string

	cmd := &cobra.Command{
		Use:   "get NAME_OR_ID",
		Short: "Get an evaluator by name or ID",
		Long: `Get an evaluator, including its prompt or code and the rules that attach
it to projects or datasets. NAME is the evaluator name shown in the UI.

Examples:
  langsmith evaluator get ready_for_task_grade
  langsmith evaluator get <evaluator-id>`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			ctx := context.Background()

			id, err := resolveOnlineEvaluatorID(ctx, c, args[0])
			if err != nil {
				return err
			}
			ev, err := c.SDK.OnlineEvaluators.Get(ctx, id)
			if err != nil {
				return fmt.Errorf("fetching evaluator %s: %w", id, err)
			}
			return output.OutputJSON(evaluatorDetail(*ev), outputFile)
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	return cmd
}

func newEvaluatorDeleteCmd() *cobra.Command {
	var deleteRules, yes bool

	cmd := &cobra.Command{
		Use:   "delete EVALUATOR_ID",
		Short: "Delete an evaluator by ID",
		Long: `Delete an evaluator. This affects every project and dataset it is attached
to, so it takes an ID rather than a name. An evaluator that still has rules is
only deleted with --delete-rules. To detach it from one project or dataset,
use 'langsmith evaluator rule delete' instead.

Examples:
  langsmith evaluator delete <evaluator-id>
  langsmith evaluator delete <evaluator-id> --delete-rules --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if _, err := uuid.Parse(id); err != nil {
				return fmt.Errorf("evaluator delete takes an evaluator ID (see 'langsmith evaluator list'); to delete one rule, use 'langsmith evaluator rule delete'")
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			ctx := context.Background()

			ev, err := c.SDK.OnlineEvaluators.Get(ctx, id)
			if err != nil {
				return fmt.Errorf("fetching evaluator %s: %w", id, err)
			}
			if len(ev.RunRules) > 0 && !deleteRules {
				return fmt.Errorf("evaluator %q is attached to %s; pass --delete-rules to delete those rules too", ev.Name, ruleTargetsSummary(ev.RunRules))
			}
			if !yes {
				target := "the evaluator"
				if len(ev.RunRules) > 0 {
					target = fmt.Sprintf("the evaluator and its %d rule(s)", len(ev.RunRules))
				}
				if err := confirmDelete(cmd, deleteConfirmation{
					target:   target,
					identity: fmt.Sprintf("Evaluator: %q (id: %s, attached to: %s)", ev.Name, ev.ID, ruleTargetsSummary(ev.RunRules)),
				}); err != nil {
					return err
				}
			}

			if err := c.SDK.OnlineEvaluators.Delete(ctx, id, langsmith.OnlineEvaluatorDeleteParams{
				DeleteRunRules: langsmith.F(deleteRules),
			}); err != nil {
				return fmt.Errorf("deleting evaluator %s: %w", id, err)
			}
			return output.OutputJSON(map[string]any{
				"status":        "deleted",
				"id":            ev.ID,
				"name":          ev.Name,
				"rules_deleted": len(ev.RunRules),
			}, "")
		},
	}

	cmd.Flags().BoolVar(&deleteRules, "delete-rules", false, "Also delete the rules that attach the evaluator to projects and datasets")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	return cmd
}

func listOnlineEvaluators(ctx context.Context, c *client.Client, params langsmith.OnlineEvaluatorListParams) ([]langsmith.OnlineEvaluator, error) {
	params.Limit = langsmith.F(int64(100))
	pager := c.SDK.OnlineEvaluators.ListAutoPaging(ctx, params)
	var evaluators []langsmith.OnlineEvaluator
	for pager.Next() {
		evaluators = append(evaluators, pager.Current())
	}
	if err := pager.Err(); err != nil {
		return nil, fmt.Errorf("listing evaluators: %w", err)
	}
	return evaluators, nil
}

// resolveOnlineEvaluatorID accepts an evaluator ID or an exact evaluator name.
// The API's name filter is a substring match that also matches creator names,
// so the exact comparison happens here.
func resolveOnlineEvaluatorID(ctx context.Context, c *client.Client, nameOrID string) (string, error) {
	if _, err := uuid.Parse(nameOrID); err == nil {
		return nameOrID, nil
	}
	candidates, err := listOnlineEvaluators(ctx, c, langsmith.OnlineEvaluatorListParams{
		NameContains: langsmith.F(nameOrID),
	})
	if err != nil {
		return "", err
	}
	var ids []string
	for _, ev := range candidates {
		if ev.Name == nameOrID {
			ids = append(ids, ev.ID)
		}
	}
	switch len(ids) {
	case 1:
		return ids[0], nil
	case 0:
		hints := ruleNameHints(ctx, c, nameOrID, nil)
		if len(hints) == 0 {
			return "", fmt.Errorf("evaluator %q not found", nameOrID)
		}
		return "", fmt.Errorf("no evaluator named %q; that is a rule name, and its rules use evaluator %s", nameOrID, strings.Join(hints, ", "))
	default:
		msg := fmt.Sprintf("%d evaluators are named %q; pass an ID instead: %s", len(ids), nameOrID, strings.Join(ids, ", "))
		if hints := ruleNameHints(ctx, c, nameOrID, ids); len(hints) > 0 {
			msg += fmt.Sprintf(". %q is also a rule name, and its rules use evaluator %s", nameOrID, strings.Join(hints, ", "))
		}
		return "", errors.New(msg)
	}
}

// ruleNameHints names the evaluators behind rules called name, skipping
// excludeIDs. A rule keeps its name when its evaluator is renamed, and the UI
// shows only evaluator names, so a name taken from rule output may refer to a
// different evaluator. Lookup failures yield no hints.
func ruleNameHints(ctx context.Context, c *client.Client, name string, excludeIDs []string) []string {
	rules, err := c.SDK.Evaluators.List(ctx, langsmith.EvaluatorListParams{})
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	for _, id := range excludeIDs {
		seen[id] = true
	}
	var hints []string
	for _, r := range *rules {
		if r.DisplayName != name || r.EvaluatorID == "" || seen[r.EvaluatorID] {
			continue
		}
		seen[r.EvaluatorID] = true
		hints = append(hints, fmt.Sprintf("%q (%s)", r.EvaluatorName, r.EvaluatorID))
	}
	return hints
}

func evaluatorEntry(ev langsmith.OnlineEvaluator) map[string]any {
	rules := make([]map[string]any, 0, len(ev.RunRules))
	for _, r := range ev.RunRules {
		rules = append(rules, map[string]any{
			"rule_id":    r.ID,
			"session_id": nilStr(r.SessionID),
			"project":    nilStr(r.SessionName),
			"dataset_id": nilStr(r.DatasetID),
			"dataset":    nilStr(r.DatasetName),
		})
	}
	return map[string]any{
		"id":            ev.ID,
		"name":          ev.Name,
		"type":          string(ev.Type),
		"feedback_keys": ev.FeedbackKeys,
		"rules":         rules,
	}
}

func evaluatorDetail(ev langsmith.OnlineEvaluator) map[string]any {
	entry := evaluatorEntry(ev)
	if raw := ev.JSON.LlmEvaluator.Raw(); raw != "" && raw != "null" {
		entry["llm_evaluator"] = json.RawMessage(raw)
	}
	if raw := ev.JSON.CodeEvaluator.Raw(); raw != "" && raw != "null" {
		entry["code_evaluator"] = json.RawMessage(raw)
	}
	return entry
}

func ruleTargetsSummary(rules []langsmith.OnlineEvaluatorRunRule) string {
	if len(rules) == 0 {
		return "nothing"
	}
	targets := make([]string, 0, len(rules))
	for _, r := range rules {
		label := targetLabel(r.SessionName, r.SessionID, r.DatasetName, r.DatasetID)
		if label == "" {
			label = "rule " + r.ID
		}
		targets = append(targets, label)
	}
	return strings.Join(targets, ", ")
}

// targetLabel names the project or dataset a rule runs on, preferring names over IDs.
func targetLabel(sessionName, sessionID, datasetName, datasetID string) string {
	switch {
	case sessionName != "":
		return "project " + sessionName
	case datasetName != "":
		return "dataset " + datasetName
	case sessionID != "":
		return "project " + sessionID
	case datasetID != "":
		return "dataset " + datasetID
	default:
		return ""
	}
}

func newEvaluatorUploadCmd() *cobra.Command {
	var (
		name            string
		funcName        string
		targetDataset   string
		targetProject   string
		targetProjectID string
		samplingRate    float64
		traceFilter     string
		replace         bool
		yes             bool
	)

	cmd := &cobra.Command{
		Use:   "upload EVALUATOR_FILE",
		Short: "Upload an evaluator function to LangSmith",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			evaluatorFile := args[0]

			if err := validateEvaluatorTargetFlags(targetDataset, targetProject, targetProjectID); err != nil {
				return err
			}

			c := MustGetClient()
			ctx := context.Background()

			var datasetID, projectID string
			if targetDataset != "" {
				ds, err := resolveDataset(ctx, c, targetDataset)
				if err != nil {
					ExitErrorf("%v", err)
				}
				datasetID = ds.ID
			}
			if targetProject != "" || targetProjectID != "" {
				sid, err := resolveSessionID(ctx, c, targetProject, targetProjectID, "evaluator upload")
				if err != nil {
					ExitErrorf("%v", err)
				}
				projectID = sid
			}

			// Read and prepare function source
			source, err := os.ReadFile(evaluatorFile)
			if err != nil {
				ExitErrorf("reading evaluator file: %v", err)
			}

			language, evalFuncName := detectLanguage(evaluatorFile)
			if language == "" {
				ExitErrorf("unsupported file extension: %s (use .py, .js, .ts, .tsx, or .mjs)", evaluatorFile)
			}

			var sourceStr string
			switch language {
			case "python":
				sourceStr = extractPythonFunction(string(source), funcName)
				if sourceStr == "" {
					ExitErrorf("function %q not found in %s", funcName, evaluatorFile)
				}
				re := regexp.MustCompile(`\bdef\s+` + regexp.QuoteMeta(funcName) + `\s*\(`)
				sourceStr = re.ReplaceAllString(sourceStr, "def "+evalFuncName+"(")
			case "javascript":
				sourceStr = extractJSFunction(string(source), funcName)
				if sourceStr == "" {
					ExitErrorf("function %q not found in %s", funcName, evaluatorFile)
				}
				sourceStr = renameJSFunction(sourceStr, funcName)
			}

			payload := map[string]any{
				"display_name":           name,
				"sampling_rate":          samplingRate,
				"is_enabled":             true,
				"include_extended_stats": false,
				"code_evaluators": []map[string]any{
					{"code": sourceStr, "language": language},
				},
			}
			if datasetID != "" {
				payload["dataset_id"] = datasetID
			}
			if projectID != "" {
				payload["session_id"] = projectID
			}
			if traceFilter != "" {
				payload["trace_filter"] = traceFilter
			}

			// Prepare the new evaluator before touching any existing one. If this is
			// a replacement, the old evaluator stays in place until the new version is ready.
			rules, err := c.SDK.Evaluators.List(ctx, langsmith.EvaluatorListParams{})
			if err != nil {
				ExitErrorf("checking existing evaluators: %v", err)
			}

			existing := findEvaluator(*rules, name, datasetID, projectID)
			if existing != nil {
				if !replace {
					return fmt.Errorf("Evaluator '%s' already exists (use --replace to overwrite)", name)
				}
				if !yes {
					fmt.Fprintf(os.Stderr, "Replace existing evaluator '%s'? [y/N] ", name)
					var confirm string
					_, _ = fmt.Scanln(&confirm)
					if strings.ToLower(confirm) != "y" {
						ExitError("aborted")
					}
				}
			}

			var result map[string]any
			if existing != nil {
				if err := c.RawPatch(ctx, fmt.Sprintf("/api/v1/runs/rules/%s", existing.ID), payload, &result); err != nil {
					ExitErrorf("replacing evaluator: %v", err)
				}
			} else if err := c.RawPost(ctx, "/api/v1/runs/rules", payload, &result); err != nil {
				ExitErrorf("uploading evaluator: %v", err)
			}

			targetLabel := "project"
			if datasetID != "" {
				targetLabel = "dataset"
			}
			return output.OutputJSON(map[string]any{
				"status":       "uploaded",
				"rule_id":      result["id"],
				"evaluator_id": result["evaluator_id"],
				"name":         name,
				"target":       targetLabel,
			}, "")
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Display name for the evaluator (required)")
	cmd.Flags().StringVar(&funcName, "function", "", "Name of the function to upload (required)")
	cmd.Flags().StringVar(&targetDataset, "dataset", "", "Target dataset name (offline evaluator)")
	cmd.Flags().StringVar(&targetProject, "project", "", "Target project name (online evaluator)")
	cmd.Flags().StringVar(&targetProjectID, "project-id", "", "Target project (session) UUID; skips the name lookup")
	cmd.MarkFlagsMutuallyExclusive("project", "project-id")
	cmd.Flags().Float64Var(&samplingRate, "sampling-rate", 1.0, "Fraction of runs to evaluate (0.0-1.0)")
	cmd.Flags().StringVar(&traceFilter, "trace-filter", "", "Filter expression for which runs to evaluate")
	cmd.Flags().BoolVar(&replace, "replace", false, "Replace existing evaluator with same name")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt when replacing")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("function")

	return cmd
}

// Registers create-llm for LLM-as-judge evaluators.
func newEvaluatorCreateLLMCmd() *cobra.Command {
	var (
		name            string
		targetDataset   string
		targetProject   string
		targetProjectID string
		samplingRate    float64
		traceFilter     string
		hubRef          string
		promptPath      string
		schemaPath      string
		modelConfigPath string
		variableMapping string
		replace         bool
		yes             bool
	)

	cmd := &cobra.Command{
		Use:   "create-llm",
		Short: "Create an LLM-as-judge evaluator rule",
		Long: `Create an LLM-as-judge run rule. --model-config is always required.

Provide the judge prompt inline with --prompt and --schema, or point at Prompt Hub
with --hub-ref (which replaces --prompt and --schema). The model is configured separately.

Examples:
  langsmith evaluator create-llm --name relevance --project my-app \
    --prompt prompt.json --schema schema.json --model-config model.json
  langsmith evaluator create-llm --name relevance --project my-app \
    --hub-ref my-org/relevance:latest --model-config model.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := MustGetClient()
			ctx := context.Background()

			target, err := resolveEvaluatorTarget(ctx, c, targetDataset, targetProject, targetProjectID)
			if err != nil {
				ExitErrorf("%v", err)
			}
			mapping, err := parseVariableMapping(variableMapping)
			if err != nil {
				ExitErrorf("%v", err)
			}
			payload, err := buildLLMEvaluatorPayload(
				name, target, samplingRate, traceFilter, hubRef,
				promptPath, schemaPath, modelConfigPath, mapping,
			)
			if err != nil {
				ExitErrorf("%v", err)
			}
			existing, err := findLLMEvaluatorForCreate(ctx, c, name, target, replace, yes)
			if err != nil {
				if err.Error() == "aborted" {
					ExitError("aborted")
				}
				if existing != nil {
					return err
				}
				ExitErrorf("%v", err)
			}

			var result map[string]any
			if existing != nil {
				if err := c.RawPatch(ctx, fmt.Sprintf("/api/v1/runs/rules/%s", existing.ID), payload, &result); err != nil {
					ExitErrorf("replacing LLM evaluator: %v", err)
				}
			} else if err := c.RawPost(ctx, "/api/v1/runs/rules", payload, &result); err != nil {
				ExitErrorf("creating LLM evaluator: %v", err)
			}
			targetLabel := "project"
			if target.datasetID != "" {
				targetLabel = "dataset"
			}
			return output.OutputJSON(map[string]any{
				"status": "created", "type": "llm",
				"rule_id": result["id"], "evaluator_id": result["evaluator_id"],
				"name": name, "target": targetLabel,
			}, "")
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Display name (required)")
	cmd.Flags().StringVar(&targetDataset, "dataset", "", "Target dataset name")
	cmd.Flags().StringVar(&targetProject, "project", "", "Target project name")
	cmd.Flags().StringVar(&targetProjectID, "project-id", "", "Target project (session) UUID; skips the name lookup")
	cmd.MarkFlagsMutuallyExclusive("project", "project-id")
	cmd.Flags().Float64Var(&samplingRate, "sampling-rate", 1.0, "Fraction of runs to evaluate (0.0-1.0)")
	cmd.Flags().StringVar(&traceFilter, "trace-filter", "", "Filter expression for which runs to evaluate")
	cmd.Flags().StringVar(&hubRef, "hub-ref", "", "Prompt Hub reference; replaces --prompt and --schema (e.g. my-org/prompt:latest)")
	cmd.Flags().StringVar(&promptPath, "prompt", "", "Prompt JSON file ([[role,content],...] or [{role,content},...]); omit if --hub-ref is set")
	cmd.Flags().StringVar(&schemaPath, "schema", "", "JSON schema file for structured output; omit if --hub-ref is set")
	cmd.Flags().StringVar(&modelConfigPath, "model-config", "", "Serialized LangChain model JSON (required; copy from UI or GET /runs/rules)")
	cmd.Flags().StringVar(&variableMapping, "variable-mapping", "", `Map prompt vars to trace paths (JSON or @file.json)`)
	cmd.Flags().BoolVar(&replace, "replace", false, "Replace existing evaluator with same name")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation when replacing")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("model-config")

	return cmd
}

// extractPythonFunction extracts a single top-level function from Python source.
// It finds "def funcName(" and collects all lines until the next top-level
// definition (def/class at column 0) or end of file.
func extractPythonFunction(source string, funcName string) string {
	lines := strings.Split(source, "\n")
	defPattern := regexp.MustCompile(`^def\s+` + regexp.QuoteMeta(funcName) + `\s*\(`)

	startIdx := -1
	for i, line := range lines {
		if defPattern.MatchString(line) {
			startIdx = i
			break
		}
	}
	if startIdx < 0 {
		return ""
	}

	// Collect lines: the def line plus all following indented/blank lines,
	// stopping at the next top-level definition.
	topLevelDef := regexp.MustCompile(`^(def |class )\S`)
	endIdx := len(lines)
	for i := startIdx + 1; i < len(lines); i++ {
		if topLevelDef.MatchString(lines[i]) {
			endIdx = i
			break
		}
	}

	// Trim trailing blank lines
	for endIdx > startIdx+1 && strings.TrimSpace(lines[endIdx-1]) == "" {
		endIdx--
	}

	return strings.Join(lines[startIdx:endIdx], "\n") + "\n"
}

// detectLanguage returns the API language value and the canonical eval function
// name based on the file extension.
func detectLanguage(filename string) (language, evalFuncName string) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".py":
		return "python", "perform_eval"
	case ".js", ".ts", ".tsx", ".mjs":
		return "javascript", "performEval"
	default:
		return "", ""
	}
}

// extractJSFunction extracts a single top-level function from JavaScript/TypeScript
// source. It matches function declarations and const/let/var arrow/function
// expressions, then uses brace-counting to find the end of the function body.
func extractJSFunction(source string, funcName string) string {
	lines := strings.Split(source, "\n")
	qName := regexp.QuoteMeta(funcName)

	// Pattern 1: (export)? (async)? function funcName(
	funcDeclRe := regexp.MustCompile(`^(export\s+)?(async\s+)?function\s+` + qName + `\s*\(`)
	// Pattern 2: (export)? (const|let|var) funcName =
	varDeclRe := regexp.MustCompile(`^(export\s+)?(const|let|var)\s+` + qName + `\s*=`)

	startIdx := -1
	for i, line := range lines {
		if funcDeclRe.MatchString(line) || varDeclRe.MatchString(line) {
			startIdx = i
			break
		}
	}
	if startIdx < 0 {
		return ""
	}

	// Use brace-counting to find function end
	braceCount := 0
	foundOpen := false
	endIdx := len(lines)
	for i := startIdx; i < len(lines); i++ {
		for _, ch := range lines[i] {
			if ch == '{' {
				braceCount++
				foundOpen = true
			} else if ch == '}' {
				braceCount--
			}
		}
		if foundOpen && braceCount <= 0 {
			endIdx = i + 1
			break
		}
	}

	// Trim trailing blank lines
	for endIdx > startIdx+1 && strings.TrimSpace(lines[endIdx-1]) == "" {
		endIdx--
	}

	return strings.Join(lines[startIdx:endIdx], "\n") + "\n"
}

// renameJSFunction renames the given function to performEval, strips export
// keywords, and converts arrow functions to function declarations.
func renameJSFunction(source string, funcName string) string {
	qName := regexp.QuoteMeta(funcName)

	// Pattern 1: (export)? async function funcName( → async function performEval(
	asyncFuncRe := regexp.MustCompile(`^(export\s+)?async\s+function\s+` + qName + `\s*\(`)
	// Pattern 2: (export)? function funcName( → function performEval(
	funcRe := regexp.MustCompile(`^(export\s+)?function\s+` + qName + `\s*\(`)
	// Pattern 3: (export)? const funcName = async (params) => { → async function performEval(params) {
	asyncArrowRe := regexp.MustCompile(`^(export\s+)?(const|let|var)\s+` + qName + `\s*=\s*async\s*\(([^)]*)\)\s*=>\s*\{`)
	// Pattern 4: (export)? const funcName = (params) => { → function performEval(params) {
	arrowRe := regexp.MustCompile(`^(export\s+)?(const|let|var)\s+` + qName + `\s*=\s*\(([^)]*)\)\s*=>\s*\{`)

	lines := strings.Split(source, "\n")
	if len(lines) == 0 {
		return source
	}

	firstLine := lines[0]
	switch {
	case asyncArrowRe.MatchString(firstLine):
		lines[0] = asyncArrowRe.ReplaceAllString(firstLine, "async function performEval($3) {")
	case arrowRe.MatchString(firstLine):
		lines[0] = arrowRe.ReplaceAllString(firstLine, "function performEval($3) {")
	case asyncFuncRe.MatchString(firstLine):
		lines[0] = asyncFuncRe.ReplaceAllString(firstLine, "async function performEval(")
	case funcRe.MatchString(firstLine):
		lines[0] = funcRe.ReplaceAllString(firstLine, "function performEval(")
	}

	return strings.Join(lines, "\n")
}

func findEvaluator(rules []langsmith.Evaluator, name, datasetID, projectID string) *langsmith.Evaluator {
	matches := findEvaluators(rules, name, datasetID, projectID)
	if len(matches) == 0 {
		return nil
	}
	return &matches[0]
}

// findEvaluators returns every rule with this name on the given dataset or project.
func findEvaluators(rules []langsmith.Evaluator, name, datasetID, projectID string) []langsmith.Evaluator {
	var matches []langsmith.Evaluator
	for _, rule := range rules {
		if rule.DisplayName != name {
			continue
		}
		if (datasetID != "" && rule.DatasetID == datasetID) || (projectID != "" && rule.SessionID == projectID) {
			matches = append(matches, rule)
		}
	}
	return matches
}
