package cmd

import (
	"context"
	"fmt"
	"time"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/extract"
	"github.com/langchain-ai/langsmith-cli/internal/output"

	"github.com/google/uuid"
)

// queryRuns queries runs with the given params and optional session resolution.
// minTokens > 0 enables client-side filtering by total_tokens (not supported server-side).
func queryRuns(ctx context.Context, c *client.Client, params langsmith.RunQueryParams, projectName string, limit int, minTokens int) ([]langsmith.RunQueryResponseRun, error) {
	// Resolve project name → session ID
	if projectName != "" {
		sessionID, err := c.ResolveSessionID(ctx, projectName)
		if err != nil {
			return nil, err
		}
		params.Session = langsmith.F([]string{sessionID})
	}

	var allRuns []langsmith.RunQueryResponseRun
	remaining := limit

	for {
		resp, err := c.SDK.Runs.Query(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("querying runs: %w", err)
		}

		for _, run := range resp.Runs {
			if remaining <= 0 {
				return allRuns, nil
			}
			// Client-side token filter
			if minTokens > 0 && run.TotalTokens < int64(minTokens) {
				continue
			}
			allRuns = append(allRuns, run)
			remaining--
		}

		// Check for next cursor
		if resp.Cursors == nil || resp.Cursors["next"] == "" || remaining <= 0 {
			break
		}
		params.Cursor = langsmith.F(resp.Cursors["next"])
	}

	return allRuns, nil
}

// buildRunSelect returns the Select fields needed for the given include flags.
// Returns nil when neither IO nor feedback is requested, letting the API use its defaults.
// When set, includes all base/metadata fields so they aren't stripped from the response.
func buildRunSelect(includeIO, includeFeedback bool) []langsmith.RunQueryParamsSelect {
	if !includeIO && !includeFeedback {
		return nil
	}

	fields := []langsmith.RunQueryParamsSelect{
		// Base fields
		langsmith.RunQueryParamsSelectID,
		langsmith.RunQueryParamsSelectTraceID,
		langsmith.RunQueryParamsSelectName,
		langsmith.RunQueryParamsSelectRunType,
		langsmith.RunQueryParamsSelectParentRunID,
		langsmith.RunQueryParamsSelectStartTime,
		langsmith.RunQueryParamsSelectEndTime,
		langsmith.RunQueryParamsSelectStatus,
		// Metadata fields
		langsmith.RunQueryParamsSelectExtra,
		langsmith.RunQueryParamsSelectPromptTokens,
		langsmith.RunQueryParamsSelectCompletionTokens,
		langsmith.RunQueryParamsSelectTotalTokens,
		langsmith.RunQueryParamsSelectPromptCost,
		langsmith.RunQueryParamsSelectCompletionCost,
		langsmith.RunQueryParamsSelectTotalCost,
		langsmith.RunQueryParamsSelectTags,
	}

	if includeIO {
		fields = append(fields,
			langsmith.RunQueryParamsSelectInputs,
			langsmith.RunQueryParamsSelectOutputs,
			langsmith.RunQueryParamsSelectError,
		)
	}

	if includeFeedback {
		fields = append(fields,
			langsmith.RunQueryParamsSelectFeedbackStats,
		)
	}

	return fields
}

// extractRunsToMaps extracts a slice of runs to maps.
func extractRunsToMaps(runs []langsmith.RunQueryResponseRun, includeMetadata, includeIO, includeFeedback bool) []map[string]any {
	result := make([]map[string]any, 0, len(runs))
	for _, r := range runs {
		result = append(result, extract.ExtractRun(r, includeMetadata, includeIO, includeFeedback))
	}
	return result
}

// runsToTreeData converts runs to tree data for output.
func runsToTreeData(runs []langsmith.RunQueryResponseRun) []output.RunTreeData {
	var treeData []output.RunTreeData
	for _, r := range runs {
		var durationMs *int64
		if !r.StartTime.IsZero() && !r.EndTime.IsZero() {
			ms := int64(r.EndTime.Sub(r.StartTime).Milliseconds())
			durationMs = &ms
		}
		treeData = append(treeData, output.RunTreeData{
			ID:          r.ID,
			ParentRunID: r.ParentRunID,
			Name:        r.Name,
			RunType:     string(r.RunType),
			DurationMs:  durationMs,
			HasError:    r.Error != "",
		})
	}
	return treeData
}

// resolveDataset resolves a dataset by name or UUID.
func resolveDataset(ctx context.Context, c *client.Client, nameOrID string) (*langsmith.Dataset, error) {
	// Try UUID first
	if _, err := uuid.Parse(nameOrID); err == nil {
		ds, err := c.SDK.Datasets.Get(ctx, nameOrID)
		if err != nil {
			return nil, fmt.Errorf("fetching dataset by ID: %w", err)
		}
		return ds, nil
	}
	// Fall back to name lookup
	resp, err := c.SDK.Datasets.List(ctx, langsmith.DatasetListParams{
		Name: langsmith.F(nameOrID),
		Limit:       langsmith.F(int64(1)),
	})
	if err != nil {
		return nil, fmt.Errorf("searching dataset by name: %w", err)
	}
	if len(resp.Items) == 0 {
		return nil, fmt.Errorf("dataset not found: %s", nameOrID)
	}
	return &resp.Items[0], nil
}

// formatTimedelta formats a duration as a human-readable string.
func formatTimedelta(seconds float64) string {
	if seconds < 1 {
		return fmt.Sprintf("%.0fms", seconds*1000)
	} else if seconds < 60 {
		return fmt.Sprintf("%.1fs", seconds)
	}
	minutes := int(seconds / 60)
	secs := seconds - float64(minutes)*60
	return fmt.Sprintf("%dm %.0fs", minutes, secs)
}

// formatTimeISO formats a time as ISO string or "N/A".
func formatTimeISO(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.Format(time.RFC3339)
}

// formatTimeShort formats a time as "YYYY-MM-DD HH:MM" or "N/A".
func formatTimeShort(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}
	return t.Format("2006-01-02 15:04")
}
