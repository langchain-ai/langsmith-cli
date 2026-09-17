package cmd

import (
	"encoding/json"
	"math"
	"time"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

func addEvaluatorSettingsFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.String("group-by", "", "Evaluate whole threads with thread_id; none selects individual runs")
	f.String("filter", "", "Run filter DSL (thread selection semantics are service-defined)")
	f.String("tree-filter", "", "Filter DSL matching runs within a trace tree")
	f.Bool("enabled", true, "Enable evaluation; false creates or leaves the rule paused")
	f.Bool("extend-trace-retention", false, "Explicitly enable/disable extended retention for evaluated traces (project only)")
	f.Bool("trace-evaluator-runs", true, "Trace the judge's own execution")
	f.Bool("include-extended-stats", false, "Include feedback, cost and token stats (run mode only)")
	f.String("backfill-from", "", "Evaluate historical data from an RFC3339 timestamp; may incur cost")
	f.Float64("spend-limit", 0, "Positive weekly USD rule limit; omitted inherits service behavior")
	f.Bool("dry-run", false, "Validate and preview rule settings without saving or running a judge; does not validate model credentials")
}

func evaluatorSettingsError(message string) error {
	return commandDiagnostic{"invalid_evaluator_settings", message, "Run 'langsmith evaluator create-llm --help'. Thread grouping requires a project and does not support extended stats."}
}

// Only explicitly supplied advanced settings are sent, preserving existing values
// during replacement. Legacy creation defaults are supplied by the payload builder.
func applyEvaluatorSettings(cmd *cobra.Command, payload map[string]any) error {
	f := cmd.Flags()
	rate, _ := f.GetFloat64("sampling-rate")
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 || rate > 1 {
		return evaluatorSettingsError("--sampling-rate must be a finite number between 0 and 1")
	}
	group, _ := f.GetString("group-by")
	if f.Changed("group-by") {
		switch group {
		case "thread_id":
			if _, offline := payload["dataset_id"]; offline {
				return evaluatorSettingsError("thread grouping requires a project, not a dataset")
			}
			payload["group_by"] = group
		case "none":
			payload["group_by"] = nil
		default:
			return evaluatorSettingsError("--group-by must be thread_id or none")
		}
	}
	for flag, key := range map[string]string{"filter": "filter", "tree-filter": "tree_filter", "trace-filter": "trace_filter"} {
		if f.Changed(flag) {
			payload[key], _ = f.GetString(flag)
		}
	}
	for flag, key := range map[string]string{"enabled": "is_enabled", "extend-trace-retention": "extend_evaluator_trace_retention", "include-extended-stats": "include_extended_stats"} {
		if f.Changed(flag) {
			payload[key], _ = f.GetBool(flag)
		}
	}
	if f.Changed("extend-trace-retention") {
		if _, offline := payload["dataset_id"]; offline {
			return evaluatorSettingsError("--extend-trace-retention is only supported for project evaluators")
		}
	}
	if f.Changed("trace-evaluator-runs") {
		trace, _ := f.GetBool("trace-evaluator-runs")
		payload["is_tracing_disabled"] = !trace
	}
	if payload["group_by"] == "thread_id" && payload["include_extended_stats"] == true {
		return evaluatorSettingsError("thread evaluators cannot include extended stats")
	}
	if f.Changed("backfill-from") {
		value, _ := f.GetString("backfill-from")
		timestamp, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return evaluatorSettingsError("--backfill-from must be an RFC3339 timestamp with a timezone")
		}
		payload["backfill_from"] = timestamp.Format(time.RFC3339Nano)
	}
	if f.Changed("spend-limit") {
		limit, _ := f.GetFloat64("spend-limit")
		if math.IsNaN(limit) || math.IsInf(limit, 0) || limit <= 0 {
			return evaluatorSettingsError("--spend-limit must be a finite positive USD amount")
		}
		payload["spend_limit"] = map[string]any{"limit_usd": limit, "window": "weekly"}
	}
	return nil
}

// Exclude judge definitions/model configuration: these can contain credentials.
func evaluatorSettingsView(payload map[string]any) map[string]any {
	settings := map[string]any{}
	for _, key := range []string{"session_id", "dataset_id", "group_by", "filter", "trace_filter", "tree_filter", "sampling_rate", "is_enabled", "extend_evaluator_trace_retention", "is_tracing_disabled", "include_extended_stats", "backfill_from", "spend_limit"} {
		if value, ok := payload[key]; ok {
			settings[key] = value
		}
	}
	return settings
}

func evaluatorRuleSettings(rule langsmith.Evaluator) map[string]any {
	var raw map[string]any
	// Raw response preserves missing/null and fields not modeled by the pinned SDK.
	if json.Unmarshal([]byte(rule.JSON.RawJSON()), &raw) != nil {
		return nil
	}
	return evaluatorSettingsView(raw)
}
