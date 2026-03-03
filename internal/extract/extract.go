package extract

import (
	"fmt"
	"strconv"
	"time"

	langsmith "github.com/langchain-ai/langsmith-go"
)

// ExtractRun normalizes a LangSmith RunQueryResponseRun to a flat map.
// This mirrors the Python extract_run() function exactly.
func ExtractRun(run langsmith.RunQueryResponseRun, includeMetadata, includeIO bool) map[string]any {
	result := map[string]any{
		"run_id":        run.ID,
		"trace_id":      run.TraceID,
		"name":          run.Name,
		"run_type":      string(run.RunType),
		"parent_run_id": nilIfEmpty(run.ParentRunID),
		"start_time":    formatTime(run.StartTime),
		"end_time":      formatTimeNullable(run.EndTime),
	}

	if includeMetadata {
		durationMs := calcDuration(run.StartTime, run.EndTime)

		var customMetadata map[string]any
		if run.Extra != nil {
			if md, ok := run.Extra["metadata"]; ok {
				if mdMap, ok := md.(map[string]any); ok {
					customMetadata = mdMap
				}
			}
		}

		var tokenUsage map[string]any
		if run.PromptTokens > 0 || run.CompletionTokens > 0 || run.TotalTokens > 0 {
			tokenUsage = map[string]any{}
			if run.PromptTokens > 0 {
				tokenUsage["prompt_tokens"] = run.PromptTokens
			}
			if run.CompletionTokens > 0 {
				tokenUsage["completion_tokens"] = run.CompletionTokens
			}
			if run.TotalTokens > 0 {
				tokenUsage["total_tokens"] = run.TotalTokens
			}
		}

		var costs map[string]any
		if run.PromptCost != "" || run.CompletionCost != "" || run.TotalCost != "" {
			costs = map[string]any{}
			if f, err := strconv.ParseFloat(run.PromptCost, 64); err == nil && f > 0 {
				costs["prompt_cost"] = f
			}
			if f, err := strconv.ParseFloat(run.CompletionCost, 64); err == nil && f > 0 {
				costs["completion_cost"] = f
			}
			if f, err := strconv.ParseFloat(run.TotalCost, 64); err == nil && f > 0 {
				costs["total_cost"] = f
			}
		}
		if len(costs) == 0 {
			costs = nil
		}

		var tags any
		if len(run.Tags) > 0 {
			tags = run.Tags
		}

		result["status"] = run.Status
		result["duration_ms"] = durationMs
		result["custom_metadata"] = customMetadata
		result["token_usage"] = tokenUsage
		result["costs"] = costs
		result["tags"] = tags
	}

	if includeIO {
		result["inputs"] = nilIfEmptyMap(run.Inputs)
		result["outputs"] = nilIfEmptyMap(run.Outputs)
		result["error"] = nilIfEmpty(run.Error)
	}

	return result
}

// calcDuration returns duration in milliseconds, or nil.
func calcDuration(start time.Time, end time.Time) any {
	if start.IsZero() || end.IsZero() {
		return nil
	}
	return int64(end.Sub(start).Milliseconds())
}

func formatTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.Format(time.RFC3339Nano)
}

func formatTimeNullable(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.Format(time.RFC3339Nano)
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nilIfEmptyMap(m map[string]any) any {
	if len(m) == 0 {
		return nil
	}
	return m
}

// FormatDurationHuman formats milliseconds as human-readable string.
func FormatDurationHuman(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.2fs", float64(ms)/1000.0)
}
