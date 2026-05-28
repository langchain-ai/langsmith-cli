package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/extract"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/lib/runsv2"

	"github.com/google/uuid"
)

// queryRuns queries runs with the given params and optional session resolution.
// minTokens > 0 enables client-side filtering by total_tokens (not supported server-side).
func queryRuns(ctx context.Context, c *client.Client, params langsmith.RunQueryParams, projectName string, limit int, minTokens int) ([]langsmith.RunSchema, error) {
	// Resolve project name → session ID
	if projectName != "" {
		sessionID, err := c.ResolveSessionID(ctx, projectName)
		if err != nil {
			return nil, err
		}
		params.Session = langsmith.F([]string{sessionID})
	}

	var allRuns []langsmith.RunSchema
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

// queryRunsV2 queries runs against the v2 (SmithDB) endpoint, resolves the
// project name to a session UUID, paginates on next_cursor, and normalizes
// each v2 Run back into v1's RunSchema so downstream rendering is unchanged.
// minTokens > 0 enables client-side filtering by total_tokens.
func queryRunsV2(ctx context.Context, c *client.Client, body runsv2.QueryRequest, projectName string, limit int, minTokens int) ([]langsmith.RunSchema, error) {
	if projectName != "" {
		sessionID, err := c.ResolveSessionID(ctx, projectName)
		if err != nil {
			return nil, err
		}
		body.ProjectIDs = []string{sessionID}
	}

	v2Client := runsv2.NewClient(c.APIURL(), c.APIKey())

	var allRuns []langsmith.RunSchema
	remaining := limit

	for {
		resp, err := v2Client.Query(ctx, body)
		if err != nil {
			return nil, fmt.Errorf("querying runs (v2): %w", err)
		}

		for i := range resp.Items {
			if remaining <= 0 {
				return allRuns, nil
			}
			run := runV2ToSchema(resp.Items[i])
			if minTokens > 0 && run.TotalTokens < int64(minTokens) {
				continue
			}
			allRuns = append(allRuns, run)
			remaining--
		}

		if !resp.HasMore || resp.NextCursor == nil || *resp.NextCursor == "" || remaining <= 0 {
			break
		}
		body.Cursor = resp.NextCursor
	}

	return allRuns, nil
}

// buildRunQueryV2Params translates FilterFlags into a v2 query body. The
// project name is left for queryRunsV2 to resolve into ProjectIDs.
func buildRunQueryV2Params(f *FilterFlags, isRoot bool, defaultLimit int) runsv2.QueryRequest {
	body := runsv2.QueryRequest{
		SortOrder: runsv2.Ptr(runsv2.SortOrderDesc),
	}

	limit := defaultLimit
	if f.Limit > 0 {
		limit = f.Limit
	}
	body.PageSize = runsv2.Ptr(uint64(limit))

	if isRoot {
		body.IsRoot = runsv2.Ptr(true)
	}

	body.MinStartTime = runsv2.Ptr(resolveStartTime(f.Since, f.LastNMinutes).Format(time.RFC3339))
	if f.Before != "" {
		t, err := time.Parse(time.RFC3339, f.Before)
		if err != nil {
			t, err = time.Parse("2006-01-02T15:04:05", f.Before)
			if err != nil {
				ExitErrorf("invalid --before timestamp: %s", f.Before)
			}
		}
		body.MaxStartTime = runsv2.Ptr(t.UTC().Format(time.RFC3339))
	}

	if f.RunType != "" {
		body.RunType = runsv2.Ptr(f.RunType)
	}

	if f.ErrorFlag {
		body.HasError = runsv2.Ptr(true)
	} else if f.NoErrorFlag {
		body.HasError = runsv2.Ptr(false)
	}

	if f.TraceIDs != "" {
		ids := splitTrim(f.TraceIDs)
		if len(ids) == 1 {
			body.TraceID = runsv2.Ptr(ids[0])
		}
	}

	if s := buildFilterDSL(f); s != "" {
		body.Filter = runsv2.Ptr(s)
	}

	return body
}

// buildRunSelectV2 returns the v2 select-field set covering the same base
// fields used by the downstream RunSchema pipeline plus the optional groups
// requested by the include flags.
func buildRunSelectV2(includeIO, includeFeedback bool) []runsv2.SelectField {
	fields := []runsv2.SelectField{
		runsv2.SelectID,
		runsv2.SelectTraceID,
		runsv2.SelectName,
		runsv2.SelectRunType,
		runsv2.SelectStatus,
		runsv2.SelectStartTime,
		runsv2.SelectEndTime,
		runsv2.SelectParentRunIDs,
		runsv2.SelectProjectID,
		runsv2.SelectDottedOrder,
		runsv2.SelectIsRoot,
		runsv2.SelectExtra,
		runsv2.SelectMetadata,
		runsv2.SelectTags,
		runsv2.SelectPromptTokens,
		runsv2.SelectCompletionTokens,
		runsv2.SelectTotalTokens,
		runsv2.SelectPromptCost,
		runsv2.SelectCompletionCost,
		runsv2.SelectTotalCost,
		runsv2.SelectLatencySeconds,
		runsv2.SelectAppPath,
	}
	if includeIO {
		fields = append(fields, runsv2.SelectInputs, runsv2.SelectOutputs, runsv2.SelectError)
	}
	if includeFeedback {
		fields = append(fields, runsv2.SelectFeedbackStats)
	}
	return fields
}

// runV2ToSchema converts a v2 Run into the legacy v1 RunSchema shape so the
// existing extract/output pipeline can consume it unchanged.
func runV2ToSchema(r runsv2.Run) langsmith.RunSchema {
	out := langsmith.RunSchema{
		Inputs:        decodeJSONMap(r.Inputs),
		Outputs:       decodeJSONMap(r.Outputs),
		Extra:         decodeJSONMap(r.Extra),
		FeedbackStats: decodeFeedbackStats(r.FeedbackStats),
	}
	if r.ID != nil {
		out.ID = *r.ID
	}
	if r.TraceID != nil {
		out.TraceID = *r.TraceID
	}
	if r.Name != nil {
		out.Name = *r.Name
	}
	if r.RunType != nil {
		out.RunType = langsmith.RunTypeEnum(*r.RunType)
	}
	if r.Status != nil {
		out.Status = *r.Status
	}
	if r.StartTime != nil {
		if t, err := time.Parse(time.RFC3339, *r.StartTime); err == nil {
			out.StartTime = t
		}
	}
	if r.EndTime != nil {
		if t, err := time.Parse(time.RFC3339, *r.EndTime); err == nil {
			out.EndTime = t
		}
	}
	if r.FirstTokenTime != nil {
		if t, err := time.Parse(time.RFC3339, *r.FirstTokenTime); err == nil {
			out.FirstTokenTime = t
		}
	}
	if r.ProjectID != nil {
		out.SessionID = *r.ProjectID
	}
	if len(r.ParentRunIDs) > 0 {
		out.ParentRunIDs = r.ParentRunIDs
		out.ParentRunID = r.ParentRunIDs[len(r.ParentRunIDs)-1]
	}
	if r.DottedOrder != nil {
		out.DottedOrder = *r.DottedOrder
	}
	if len(r.Tags) > 0 {
		out.Tags = r.Tags
	}
	if r.ThreadID != nil {
		out.ThreadID = *r.ThreadID
	}
	if r.AppPath != nil {
		out.AppPath = *r.AppPath
	}
	if r.TotalTokens != nil {
		out.TotalTokens = *r.TotalTokens
	}
	if r.PromptTokens != nil {
		out.PromptTokens = *r.PromptTokens
	}
	if r.CompletionTokens != nil {
		out.CompletionTokens = *r.CompletionTokens
	}
	if r.TotalCost != nil {
		out.TotalCost = *r.TotalCost
	}
	if r.PromptCost != nil {
		out.PromptCost = *r.PromptCost
	}
	if r.CompletionCost != nil {
		out.CompletionCost = *r.CompletionCost
	}
	if r.LatencySeconds != nil {
		// duration_ms is derived from EndTime-StartTime by the extractor;
		// nothing to set on RunSchema for latency directly.
		_ = r.LatencySeconds
	}
	if r.Error != nil {
		out.Error = *r.Error
	}
	if r.InputsPreview != nil {
		out.InputsPreview = *r.InputsPreview
	}
	if r.OutputsPreview != nil {
		out.OutputsPreview = *r.OutputsPreview
	}
	if r.ReferenceExampleID != nil {
		out.ReferenceExampleID = *r.ReferenceExampleID
	}
	if r.ReferenceDatasetID != nil {
		out.ReferenceDatasetID = *r.ReferenceDatasetID
	}
	if r.PriceModelID != nil {
		out.PriceModelID = *r.PriceModelID
	}
	if r.IsInDataset != nil {
		out.InDataset = *r.IsInDataset
	}
	return out
}

func decodeJSONMap(raw json.RawMessage) map[string]interface{} {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
}

func decodeFeedbackStats(raw json.RawMessage) map[string]map[string]interface{} {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
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
func extractRunsToMaps(runs []langsmith.RunSchema, includeMetadata, includeIO, includeFeedback bool) []map[string]any {
	result := make([]map[string]any, 0, len(runs))
	for _, r := range runs {
		result = append(result, extract.ExtractRun(r, includeMetadata, includeIO, includeFeedback))
	}
	return result
}

// runsToTreeData converts runs to tree data for output.
func runsToTreeData(runs []langsmith.RunSchema) []output.RunTreeData {
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
		Name:  langsmith.F(nameOrID),
		Limit: langsmith.F(int64(1)),
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
