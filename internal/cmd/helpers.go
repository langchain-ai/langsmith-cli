package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/extract"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"

	"github.com/google/uuid"
)

// resolveSessionID resolves the target session (project) UUID for a command from
// either an explicit project ID or a project name. Precedence:
//
//	--project-id (an explicit session UUID; takes precedence) →
//	--project / $LANGSMITH_PROJECT (a project name, looked up via the API).
//
// projectID, when set, must be a well-formed UUID and is returned as-is without a
// name lookup (saving a Sessions.List round-trip). cmdName is used only to build
// the "required" error message.
func resolveSessionID(ctx context.Context, c *client.Client, projectName, projectID, cmdName string) (string, error) {
	if projectID != "" {
		if _, err := uuid.Parse(projectID); err != nil {
			return "", fmt.Errorf("invalid --project-id %q: must be a project (session) UUID", projectID)
		}
		return projectID, nil
	}
	name := ResolveProject(projectName)
	if name == "" {
		return "", fmt.Errorf("--project or --project-id is required for %s (or set LANGSMITH_PROJECT)", cmdName)
	}
	return c.ResolveSessionID(ctx, name)
}

// queryRuns queries runs with the given params, scoped to sessionID when non-empty.
// The caller is responsible for resolving sessionID (see resolveSessionID).
// minTokens > 0 enables client-side filtering by total_tokens (not supported server-side).
func queryRuns(ctx context.Context, c *client.Client, params langsmith.RunQueryParams, sessionID string, limit int, minTokens int) ([]langsmith.RunSchema, error) {
	if sessionID != "" {
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
		if resp.Cursors.Next == "" || remaining <= 0 {
			break
		}
		params.Cursor = langsmith.F(resp.Cursors.Next)
	}

	return allRuns, nil
}

// queryRunsV2 queries runs against the v2 (SmithDB) endpoint, resolves the
// project name to a session UUID, paginates on next_cursor, and normalizes
// each v2 Run back into v1's RunSchema so downstream rendering is unchanged.
// minTokens > 0 enables client-side filtering by total_tokens.
func queryRunsV2(ctx context.Context, c *client.Client, params langsmith.RunQueryV2Params, sessionID string, limit int, minTokens int) ([]langsmith.RunSchema, error) {
	if sessionID != "" {
		params.ProjectIDs = langsmith.F([]string{sessionID})
	}

	var allRuns []langsmith.RunSchema

	iter := c.SDK.Runs.QueryV2AutoPaging(ctx, params)
	for iter.Next() {
		if len(allRuns) >= limit {
			break
		}
		run := runV2ToSchema(iter.Current())
		if minTokens > 0 && run.TotalTokens < int64(minTokens) {
			continue
		}
		allRuns = append(allRuns, run)
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("querying runs (v2): %w", err)
	}

	return allRuns, nil
}

// buildRunQueryV2Params translates FilterFlags into a v2 query body. The
// project name is left for queryRunsV2 to resolve into ProjectIDs.
func buildRunQueryV2Params(f *FilterFlags, isRoot bool, defaultLimit int) langsmith.RunQueryV2Params {
	var params langsmith.RunQueryV2Params

	limit := defaultLimit
	if f.Limit > 0 {
		limit = f.Limit
	}
	params.PageSize = langsmith.F(int64(limit))

	if isRoot {
		params.IsRoot = langsmith.F(true)
	}

	params.MinStartTime = langsmith.F(resolveStartTime(f.Since, f.LastNMinutes))
	if f.Before != "" {
		t, err := time.Parse(time.RFC3339, f.Before)
		if err != nil {
			t, err = time.Parse("2006-01-02T15:04:05", f.Before)
			if err != nil {
				ExitErrorf("invalid --before timestamp: %s", f.Before)
			}
		}
		params.MaxStartTime = langsmith.F(t.UTC())
	}

	if f.RunType != "" {
		params.RunType = langsmith.F(langsmith.RunQueryV2ParamsRunType(strings.ToUpper(f.RunType)))
	}

	if f.ErrorFlag {
		params.HasError = langsmith.F(true)
	} else if f.NoErrorFlag {
		params.HasError = langsmith.F(false)
	}

	if f.TraceIDs != "" {
		ids := splitTrim(f.TraceIDs)
		if len(ids) == 1 {
			params.TraceID = langsmith.F(ids[0])
		}
	}

	if s := buildFilterDSL(f); s != "" {
		params.Filter = langsmith.F(s)
	}

	return params
}

// buildRunSelectV2 returns the v2 select-field set covering the same base
// fields used by the downstream RunSchema pipeline plus the optional groups
// requested by the include flags.
func buildRunSelectV2(includeIO, includeFeedback bool) []langsmith.RunQueryV2ParamsSelect {
	fields := []langsmith.RunQueryV2ParamsSelect{
		langsmith.RunQueryV2ParamsSelectID,
		langsmith.RunQueryV2ParamsSelectTraceID,
		langsmith.RunQueryV2ParamsSelectName,
		langsmith.RunQueryV2ParamsSelectRunType,
		langsmith.RunQueryV2ParamsSelectStartTime,
		langsmith.RunQueryV2ParamsSelectEndTime,
		langsmith.RunQueryV2ParamsSelectParentRunIDs,
		langsmith.RunQueryV2ParamsSelectProjectID,
		langsmith.RunQueryV2ParamsSelectDottedOrder,
		langsmith.RunQueryV2ParamsSelectIsRoot,
		langsmith.RunQueryV2ParamsSelectExtra,
		langsmith.RunQueryV2ParamsSelectMetadata,
		langsmith.RunQueryV2ParamsSelectTags,
		langsmith.RunQueryV2ParamsSelectPromptTokens,
		langsmith.RunQueryV2ParamsSelectCompletionTokens,
		langsmith.RunQueryV2ParamsSelectTotalTokens,
		langsmith.RunQueryV2ParamsSelectPromptCost,
		langsmith.RunQueryV2ParamsSelectCompletionCost,
		langsmith.RunQueryV2ParamsSelectTotalCost,
		langsmith.RunQueryV2ParamsSelectLatencySeconds,
		langsmith.RunQueryV2ParamsSelectAppPath,
	}
	if includeIO {
		fields = append(fields,
			langsmith.RunQueryV2ParamsSelectInputs,
			langsmith.RunQueryV2ParamsSelectOutputs,
			langsmith.RunQueryV2ParamsSelectError,
		)
	}
	if includeFeedback {
		fields = append(fields, langsmith.RunQueryV2ParamsSelectFeedbackStats)
	}
	return fields
}

// runV2ToSchema converts a v2 Run into the legacy v1 RunSchema shape so the
// existing extract/output pipeline can consume it unchanged.
func runV2ToSchema(r langsmith.Run) langsmith.RunSchema {
	extra := asMap(r.Extra)
	if md := asMap(r.Metadata); md != nil {
		if extra == nil {
			extra = map[string]interface{}{}
		}
		if _, ok := extra["metadata"]; !ok {
			extra["metadata"] = md
		}
	}

	out := langsmith.RunSchema{
		ID:                 r.ID,
		TraceID:            r.TraceID,
		Name:               r.Name,
		RunType:            langsmith.RunTypeEnum(strings.ToLower(string(r.RunType))),
		StartTime:          r.StartTime,
		EndTime:            r.EndTime,
		FirstTokenTime:     r.FirstTokenTime,
		SessionID:          r.ProjectID,
		DottedOrder:        r.DottedOrder,
		Tags:               r.Tags,
		ThreadID:           r.ThreadID,
		AppPath:            r.AppPath,
		TotalTokens:        r.TotalTokens,
		PromptTokens:       r.PromptTokens,
		CompletionTokens:   r.CompletionTokens,
		TotalCost:          r.TotalCost,
		PromptCost:         r.PromptCost,
		CompletionCost:     r.CompletionCost,
		Error:              r.Error,
		InputsPreview:      r.InputsPreview,
		OutputsPreview:     r.OutputsPreview,
		ReferenceExampleID: r.ReferenceExampleID,
		ReferenceDatasetID: r.ReferenceDatasetID,
		PriceModelID:       r.PriceModelID,
		InDataset:          r.IsInDataset,
		Inputs:             asMap(r.Inputs),
		Outputs:            asMap(r.Outputs),
		Extra:              extra,
		FeedbackStats:      convertFeedbackStats(r.FeedbackStats),
	}
	if len(r.ParentRunIDs) > 0 {
		out.ParentRunIDs = r.ParentRunIDs
		out.ParentRunID = r.ParentRunIDs[len(r.ParentRunIDs)-1]
	}
	return out
}

// asMap returns v as a map[string]interface{} when it is one, else nil.
func asMap(v interface{}) map[string]interface{} {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	return m
}

// convertFeedbackStats round-trips the generated feedback-stats shape into the
// loosely-typed map the downstream pipeline expects. Returns nil on empty input
// or any marshalling error.
func convertFeedbackStats(stats map[string]langsmith.RunFeedbackStat) map[string]map[string]interface{} {
	if len(stats) == 0 {
		return nil
	}
	raw, err := json.Marshal(stats)
	if err != nil {
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
