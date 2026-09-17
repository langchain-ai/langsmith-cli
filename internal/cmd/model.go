package cmd

import (
	"context"

	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

// Keep raw settings private: they can contain credentials and arbitrary headers.
type modelPreset struct {
	ID            string         `json:"id"`
	Name          *string        `json:"name"`
	Settings      map[string]any `json:"settings"`
	Evaluators    *bool          `json:"available_in_evaluators"`
	Playground    *bool          `json:"available_in_playground"`
	InsightsHeavy *bool          `json:"available_in_insights_heavy"`
	InsightsLight *bool          `json:"available_in_insights_light"`
	OAuth         bool           `json:"oauth_enabled"`
}

func (p modelPreset) view() map[string]any {
	var model *string
	if kwargs, ok := p.Settings["kwargs"].(map[string]any); ok {
		for _, key := range []string{"model", "model_name"} {
			if value, ok := kwargs[key].(string); ok && value != "" {
				model = &value
				break
			}
		}
	}
	return map[string]any{"id": p.ID, "name": p.Name, "model": model, "available_in_evaluators": p.Evaluators,
		"available_in_playground": p.Playground, "available_in_insights_heavy": p.InsightsHeavy,
		"available_in_insights_light": p.InsightsLight, "inference_verified": false}
}

func presetReadError() error {
	return commandDiagnostic{"model_preset_read_failed", "Could not read saved model configurations", "Check the workspace, preset ID, authentication, and model-read permissions. Raw server details are omitted to protect configuration secrets."}
}

func getModelPreset(ctx context.Context, c *client.Client, id string) (modelPreset, error) {
	var p modelPreset
	parsed, err := uuid.Parse(id)
	if err != nil {
		return p, commandDiagnostic{"invalid_model_preset", "Saved model ID must be a UUID", "Use model list to find a saved configuration ID, not a provider model name."}
	}
	// The pinned SDK does not expose playground-settings.
	if err := c.RawGet(ctx, "/api/v1/playground-settings/"+parsed.String(), &p); err != nil {
		return p, presetReadError()
	}
	if p.ID != parsed.String() {
		return p, presetReadError()
	}
	return p, nil
}

func (p modelPreset) evaluatorModel() (map[string]any, error) {
	if p.Evaluators == nil || !*p.Evaluators {
		return nil, commandDiagnostic{"model_preset_unavailable", "Preset is not explicitly enabled for evaluators", "Enable it for evaluators in workspace model settings, or choose another preset."}
	}
	_, kwargsOK := p.Settings["kwargs"].(map[string]any)
	ids, idsOK := p.Settings["id"].([]any)
	for _, id := range ids {
		value, ok := id.(string)
		if !ok || value == "" {
			idsOK = false
		}
	}
	if p.OAuth || p.Settings["lc"] != float64(1) || p.Settings["type"] != "constructor" || !kwargsOK || !idsOK || len(ids) == 0 || !validModelSerialization(p.Settings, 0) {
		return nil, commandDiagnostic{"model_preset_unsupported", "Saved configuration cannot be copied into this evaluator", "Choose a non-OAuth configuration with supported LC v1 constructor/secret nodes. Unknown or malformed serialized nodes are rejected. No evaluator was written; provider compatibility and credentials are not validated."}
	}
	return p.Settings, nil
}

// Validate serialized nodes recursively without instantiating models or exposing
// their contents. This checks structure, not provider availability or capabilities.
func validModelSerialization(value any, depth int) bool {
	if depth > 64 {
		return false
	}
	switch v := value.(type) {
	case map[string]any:
		if lc, serialized := v["lc"]; serialized {
			if lc != float64(1) {
				return false
			}
			ids, ok := v["id"].([]any)
			if !ok || len(ids) == 0 {
				return false
			}
			for _, id := range ids {
				s, ok := id.(string)
				if !ok || s == "" {
					return false
				}
			}
			switch v["type"] {
			case "constructor":
				if _, ok := v["kwargs"].(map[string]any); !ok {
					return false
				}
			case "secret":
				if len(ids) != 1 || len(v) != 3 {
					return false
				}
			default:
				return false
			}
		}
		for _, child := range v {
			if !validModelSerialization(child, depth+1) {
				return false
			}
		}
	case []any:
		for _, child := range v {
			if !validModelSerialization(child, depth+1) {
				return false
			}
		}
	}
	return true
}

func newModelCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "model", Short: "Inspect saved workspace model configurations",
		Long:    "List and inspect saved workspace model configurations, not the provider's full model catalog. IDs identify saved configurations; availability does not prove inference access. Credentials and raw settings are never printed.",
		Example: "  langsmith model list --format json\n  langsmith model get <configuration-uuid> --format json"}
	cmd.AddCommand(newModelListCmd("models"), newModelGetCmd("model"))
	preset := &cobra.Command{Use: "preset", Hidden: true}
	preset.AddCommand(newModelListCmd("presets"), newModelGetCmd("preset"))
	cmd.AddCommand(preset)
	return cmd
}

func newModelListCmd(resultKey string) *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List saved model configurations (not a provider model catalog)", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			var presets []modelPreset
			if err := c.RawGet(cmd.Context(), "/api/v1/playground-settings", &presets); err != nil {
				return presetReadError()
			}
			views := make([]map[string]any, 0, len(presets))
			for _, p := range presets {
				views = append(views, p.view())
			}
			return output.OutputJSON(emptyResultGuidance(map[string]any{"workspace_id": resultWorkspaceID(), resultKey: views}, len(views),
				"No saved model configurations found in this workspace. Provider credential availability is unknown.",
				"Save a model configuration in LangSmith, then list it here. Keep provider keys out of agent messages.",
				readNextStep("model", "list")), "")
		}}
}

func newModelGetCmd(resultKey string) *cobra.Command {
	return &cobra.Command{Use: "get ID", Short: "Read a saved model summary by configuration UUID; does not verify inference", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			p, err := getModelPreset(cmd.Context(), c, args[0])
			if err != nil {
				return err
			}
			return output.OutputJSON(map[string]any{"workspace_id": resultWorkspaceID(), resultKey: p.view()}, "")
		}}
}
