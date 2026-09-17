package cmd

import (
	"context"
	"sort"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
)

// Preview deliberately supports only simple single-run input/output paths. Other
// backend mapping forms need their own context; never pretend to validate them.
func previewEvaluatorRun(ctx context.Context, c *client.Client, projectID, runID string, mapping map[string]string) (map[string]any, error) {
	if err := resourceUUID(runID); err != nil {
		return nil, err
	}
	if len(mapping) == 0 {
		return nil, commandDiagnostic{"preview_mapping_required", "Preview requires an explicit variable mapping.", "Provide --variable-mapping with input/output paths."}
	}
	paths := map[string]string{}
	variables := make([]string, 0, len(mapping))
	for variable, path := range mapping {
		parts := strings.Split(path, ".")
		if parts[0] != "input" && parts[0] != "output" {
			return nil, commandDiagnostic{"preview_mapping_unsupported", "Preview supports simple single-run input/output mappings only.", "Reference, attachment, run-stat, and thread mappings require separate inspection."}
		}
		parts[0] = "run." + parts[0] + "s"
		converted := strings.Join(parts, ".")
		if !insightsPromptPath.MatchString(converted) {
			return nil, commandDiagnostic{"preview_mapping_unsupported", "Preview does not support indexed, wildcard, or escaped mapping paths.", "Use simple dot-separated input/output object paths for this preview."}
		}
		paths[variable] = converted
		variables = append(variables, variable)
	}
	sort.Strings(variables)
	var prompt strings.Builder
	for _, variable := range variables {
		prompt.WriteString("{{" + paths[variable] + "}}")
	}
	raw, err := previewInsightsRun(ctx, c, projectID, runID, prompt.String())
	if err != nil {
		return nil, err
	}
	values := raw["bindings"].(map[string]any)
	bindings := map[string]any{}
	missing, nulls := []string{}, []string{}
	for _, variable := range variables {
		value, exists := values[paths[variable]]
		if !exists {
			missing = append(missing, variable)
			continue
		}
		bindings[variable] = value
		if value == nil {
			nulls = append(nulls, variable)
		}
	}
	return map[string]any{"run_id": runID, "bindings": bindings, "missing_variables": missing, "null_variables": nulls,
		"bindings_validated": len(missing) == 0 && len(nulls) == 0,
		"note":               "Read-only mapping preview for this root run. Values may contain private trace data. Does not validate prompt coverage, model access, filters, future runs, or judge quality."}, nil
}
