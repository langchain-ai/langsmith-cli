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
		return validateProjectID(projectID)
	}
	name := ResolveProject(projectName)
	if name == "" {
		return "", fmt.Errorf("--project or --project-id is required for %s (or set LANGSMITH_PROJECT)", cmdName)
	}
	return c.ResolveSessionID(ctx, name)
}

// validateProjectID returns a --project-id value unchanged once it is a
// well-formed UUID, so a malformed one fails here rather than as a server 4xx.
// Callers that filter by session ID without going through resolveSessionID
// (see `project issues list`) use this directly.
func validateProjectID(projectID string) (string, error) {
	if _, err := uuid.Parse(projectID); err != nil {
		return "", fmt.Errorf("invalid --project-id %q: must be a project (session) UUID", projectID)
	}
	return projectID, nil
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

// requireV2Feature exits with a clear error when the deployment lacks the v2
// (SmithDB) API a v2-only feature needs (older self-hosted).
func requireV2Feature(ctx context.Context, c *client.Client, feature string) {
	useV2, err := c.UseV2API(ctx)
	if err != nil {
		ExitErrorf("%v", err)
	}
	if !useV2 {
		ExitErrorf("%s is only available on LangSmith Cloud or self-hosted >= 0.16 (SmithDB); this deployment does not support it", feature)
	}
}

// queryRunsAuto queries runs via the v2 (SmithDB) backend when the deployment
// supports it, else v1. params is the canonical v1 query; always pass a base
// v2Selects set (see buildRunSelectV2) or v2 returns only run IDs.
func queryRunsAuto(ctx context.Context, c *client.Client, params langsmith.RunQueryParams, v2Selects []langsmith.RunSelectField, sessionID string, limit, minTokens int) ([]langsmith.RunSchema, error) {
	useV2, err := c.UseV2API(ctx)
	if err != nil {
		return nil, err
	}
	if useV2 {
		// total_tokens is filterable on the v2 (SmithDB) path, so push the bound
		// into the query and skip the client-side pass. v1 does not accept it,
		// and an attribute it does not recognise fails the whole query rather
		// than being ignored — so the clause must not reach that path. useV2API
		// routes v1 only below 0.16, which is also below any release that
		// accepts the attribute, so nothing is lost by gating on v2.
		if minTokens > 0 {
			addFilterClause(&params, fmt.Sprintf("gte(total_tokens, %d)", minTokens))
			minTokens = 0
		}
		return queryRunsV2(ctx, c, toV2Params(params, v2Selects), sessionID, limit, minTokens)
	}
	return queryRuns(ctx, c, params, sessionID, limit, minTokens)
}

// addFilterClause ANDs clause into params.Filter, preserving anything already
// there from buildFilterDSL.
func addFilterClause(params *langsmith.RunQueryParams, clause string) {
	if params.Filter.Present && params.Filter.Value != "" {
		params.Filter = langsmith.F(fmt.Sprintf("and(%s, %s)", params.Filter.Value, clause))
		return
	}
	params.Filter = langsmith.F(clause)
}

// toV2Params translates canonical v1 RunQueryParams to v2. Order is dropped (no
// v2 equivalent; v2 is newest-first); Session is applied by queryRunsV2.
func toV2Params(p langsmith.RunQueryParams, selects []langsmith.RunSelectField) langsmith.RunQueryV2Params {
	var v2 langsmith.RunQueryV2Params
	if len(selects) > 0 {
		v2.Selects = langsmith.F(selects)
	}
	if p.Trace.Present {
		v2.TraceID = langsmith.F(p.Trace.Value)
	}
	if p.IsRoot.Present {
		v2.IsRoot = langsmith.F(p.IsRoot.Value)
	}
	if p.RunType.Present {
		v2.RunType = langsmith.F(langsmith.RunType(strings.ToUpper(string(p.RunType.Value))))
	}
	if p.Error.Present {
		v2.HasError = langsmith.F(p.Error.Value)
	}
	if p.StartTime.Present {
		v2.MinStartTime = langsmith.F(p.StartTime.Value)
	}
	if p.EndTime.Present {
		v2.MaxStartTime = langsmith.F(p.EndTime.Value)
	}
	if p.Filter.Present {
		v2.Filter = langsmith.F(p.Filter.Value)
	}
	if len(p.ID.Value) > 0 {
		v2.IDs = langsmith.F(p.ID.Value)
	}
	if p.Limit.Present {
		v2.PageSize = langsmith.F(p.Limit.Value)
	}
	return v2
}

// threadListSelect and threadListSelectV2 add thread_id to the base select
// sets. `thread list` groups runs by thread_id, so omitting it yields zero
// threads rather than an error; the base sets leave it out because no other
// command reads it.
func threadListSelect() []langsmith.RunQueryParamsSelect {
	return append(buildRunSelect(true, false), langsmith.RunQueryParamsSelectThreadID)
}

func threadListSelectV2() []langsmith.RunSelectField {
	return append(buildRunSelectV2(true, false), langsmith.RunSelectFieldThreadID)
}

// buildRunSelectV2 returns the v2 select-field set covering the same base
// fields used by the downstream RunSchema pipeline plus the optional groups
// requested by the include flags.
func buildRunSelectV2(includeIO, includeFeedback bool) []langsmith.RunSelectField {
	fields := []langsmith.RunSelectField{
		langsmith.RunSelectFieldID,
		langsmith.RunSelectFieldTraceID,
		langsmith.RunSelectFieldName,
		langsmith.RunSelectFieldRunType,
		langsmith.RunSelectFieldStartTime,
		langsmith.RunSelectFieldEndTime,
		langsmith.RunSelectFieldParentRunIDs,
		langsmith.RunSelectFieldProjectID,
		langsmith.RunSelectFieldDottedOrder,
		langsmith.RunSelectFieldIsRoot,
		langsmith.RunSelectFieldExtra,
		langsmith.RunSelectFieldMetadata,
		langsmith.RunSelectFieldTags,
		langsmith.RunSelectFieldPromptTokens,
		langsmith.RunSelectFieldCompletionTokens,
		langsmith.RunSelectFieldTotalTokens,
		langsmith.RunSelectFieldPromptCost,
		langsmith.RunSelectFieldCompletionCost,
		langsmith.RunSelectFieldTotalCost,
		langsmith.RunSelectFieldLatencySeconds,
		langsmith.RunSelectFieldAppPath,
		langsmith.RunSelectFieldFirstTokenTime,
	}
	if includeIO {
		fields = append(fields,
			langsmith.RunSelectFieldInputs,
			langsmith.RunSelectFieldOutputs,
			langsmith.RunSelectFieldError,
			langsmith.RunSelectFieldEvents,
		)
	}
	if includeFeedback {
		fields = append(fields, langsmith.RunSelectFieldFeedbackStats)
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
		Events:             convertEvents(r.Events),
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

// convertEvents round-trips the generated v2 event shape into the
// loosely-typed maps the legacy RunSchema.Events field expects. Returns nil on
// empty input or any marshalling error.
func convertEvents(events []langsmith.RunEvent) []map[string]interface{} {
	if len(events) == 0 {
		return nil
	}
	raw, err := json.Marshal(events)
	if err != nil {
		return nil
	}
	var m []map[string]interface{}
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
		langsmith.RunQueryParamsSelectFirstTokenTime,
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
			langsmith.RunQueryParamsSelectEvents,
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
