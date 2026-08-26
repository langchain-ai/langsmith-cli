package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	langsmith "github.com/langchain-ai/langsmith-go"
)

type llmEvaluatorTarget struct {
	datasetID, projectID string
}

func loadJSONFile(path string, dest any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("parsing JSON in %s: %w", path, err)
	}
	return nil
}

// Accepts a plain schema file or one already saved from the LangSmith UI.
func loadEvaluatorSchema(path string) (map[string]any, error) {
	var raw map[string]any
	if err := loadJSONFile(path, &raw); err != nil {
		return nil, err
	}
	if _, ok := raw["parameters"]; ok {
		return raw, nil
	}
	return map[string]any{"name": "eval", "description": "", "parameters": raw}, nil
}

func validatePromptMessagePair(i int, path string, msg []string) error {
	if len(msg) != 2 || msg[0] == "" || msg[1] == "" {
		return fmt.Errorf("prompt message %d in %s must be [role, content] with non-empty values", i, path)
	}
	return nil
}

// Loads the judge's system/user messages from a prompt file.
func loadPromptMessagesFromJSON(path string, data []byte) ([][]string, error) {
	var pairs [][]string
	if err := json.Unmarshal(data, &pairs); err == nil && len(pairs) > 0 {
		for i, msg := range pairs {
			if err := validatePromptMessagePair(i, path, msg); err != nil {
				return nil, err
			}
		}
		return pairs, nil
	}
	var objects []struct {
		Role, Content string
	}
	if err := json.Unmarshal(data, &objects); err != nil || len(objects) == 0 {
		return nil, fmt.Errorf(
			"prompt file %s: expected [[role,content],...] or [{role,content},...]", path,
		)
	}
	pairs = make([][]string, len(objects))
	for i, obj := range objects {
		if err := validatePromptMessagePair(i, path, []string{obj.Role, obj.Content}); err != nil {
			return nil, err
		}
		pairs[i] = []string{obj.Role, obj.Content}
	}
	return pairs, nil
}

// Maps {{placeholders}} in the prompt to fields on each trace.
func parseVariableMapping(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if strings.HasPrefix(raw, "@") {
		var mapping map[string]string
		if err := loadJSONFile(raw[1:], &mapping); err != nil {
			return nil, err
		}
		return mapping, nil
	}
	var mapping map[string]string
	if err := json.Unmarshal([]byte(raw), &mapping); err != nil {
		return nil, fmt.Errorf("parsing --variable-mapping: %w", err)
	}
	return mapping, nil
}

func validateEvaluatorTargetFlags(dataset, project, projectID string) error {
	set := 0
	for _, v := range []string{dataset, project, projectID} {
		if v != "" {
			set++
		}
	}
	if set == 0 {
		return fmt.Errorf("must specify --dataset, --project, or --project-id (global evaluators not supported)")
	}
	if set > 1 {
		return fmt.Errorf("specify only one of --dataset, --project, or --project-id")
	}
	return nil
}

// Finds the dataset or project this evaluator should run on.
func resolveLLMEvaluatorTarget(ctx context.Context, c *client.Client, dataset, project, projectID string) (llmEvaluatorTarget, error) {
	if err := validateEvaluatorTargetFlags(dataset, project, projectID); err != nil {
		return llmEvaluatorTarget{}, err
	}
	var target llmEvaluatorTarget
	if dataset != "" {
		ds, err := resolveDataset(ctx, c, dataset)
		if err != nil {
			return llmEvaluatorTarget{}, err
		}
		target.datasetID = ds.ID
	}
	if project != "" || projectID != "" {
		sid, err := resolveSessionID(ctx, c, project, projectID, "evaluator llm create")
		if err != nil {
			return llmEvaluatorTarget{}, err
		}
		target.projectID = sid
	}
	return target, nil
}

// Finds an existing evaluator with the same name on this dataset or project.
// When the evaluator already exists and --replace is not set, returns the existing
// rule alongside the error so callers can reuse it without another list call.
func findLLMEvaluatorForCreate(ctx context.Context, c *client.Client, name string, target llmEvaluatorTarget, replace, yes bool) (*langsmith.Evaluator, error) {
	rules, err := c.SDK.Evaluators.List(ctx, langsmith.EvaluatorListParams{})
	if err != nil {
		return nil, fmt.Errorf("checking existing evaluators: %w", err)
	}
	existing := findEvaluator(*rules, name, target.datasetID, target.projectID)
	if existing == nil {
		return nil, nil
	}
	if !replace {
		return existing, fmt.Errorf("evaluator %q already exists (use --replace to overwrite)", name)
	}
	if !yes {
		fmt.Fprintf(os.Stderr, "Replace existing evaluator '%s'? [y/N] ", name)
		var confirm string
		_, _ = fmt.Scanln(&confirm)
		if strings.ToLower(confirm) != "y" {
			return nil, fmt.Errorf("aborted")
		}
	}
	return existing, nil
}

// Packages prompt, schema, model, and targeting into the create-evaluator request.
func buildLLMEvaluatorPayload(
	name string,
	target llmEvaluatorTarget,
	samplingRate float64,
	traceFilter, hubRef, promptPath, schemaPath, modelConfigPath string,
	variableMapping map[string]string,
) (map[string]any, error) {
	if modelConfigPath == "" {
		return nil, fmt.Errorf("--model-config is required")
	}
	var model map[string]any
	if err := loadJSONFile(modelConfigPath, &model); err != nil {
		return nil, err
	}

	structured := map[string]any{"model": model}
	if hubRef != "" {
		structured["hub_ref"] = hubRef
	} else {
		if promptPath == "" || schemaPath == "" {
			return nil, fmt.Errorf("--prompt and --schema are required unless --hub-ref is set")
		}
		data, err := os.ReadFile(promptPath)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", promptPath, err)
		}
		messages, err := loadPromptMessagesFromJSON(promptPath, data)
		if err != nil {
			return nil, err
		}
		schema, err := loadEvaluatorSchema(schemaPath)
		if err != nil {
			return nil, err
		}
		structured["prompt"] = messages
		structured["schema"] = schema
		structured["template_format"] = "mustache"
	}
	if len(variableMapping) > 0 {
		structured["variable_mapping"] = variableMapping
	}
	payload := map[string]any{
		"display_name": name, "sampling_rate": samplingRate, "is_enabled": true,
		"include_extended_stats": false,
		"evaluators":             []map[string]any{{"structured": structured}},
	}
	if traceFilter != "" {
		payload["trace_filter"] = traceFilter
	}
	if target.datasetID != "" {
		payload["dataset_id"] = target.datasetID
	}
	if target.projectID != "" {
		payload["session_id"] = target.projectID
	}
	return payload, nil
}
