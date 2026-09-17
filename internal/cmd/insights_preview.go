package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	langsmith "github.com/langchain-ai/langsmith-go"
)

var insightsPromptPath = regexp.MustCompile(`^(run\.[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*|all_thread_messages)$`)

// Inspect interpolation syntax only; never evaluate templates or execute trace data.
func insightsPromptVariables(prompt string) ([]string, error) {
	seen := map[string]bool{}
	remaining := prompt
	for {
		start := strings.Index(remaining, "{{")
		if start < 0 {
			break
		}
		remaining = remaining[start+2:]
		closing := "}}"
		if strings.HasPrefix(remaining, "{") {
			remaining = remaining[1:]
			closing = "}}}"
		}
		end := strings.Index(remaining, closing)
		if end < 0 {
			return nil, fmt.Errorf("summary prompt has an unclosed variable")
		}
		path := strings.TrimSpace(remaining[:end])
		remaining = remaining[end+len(closing):]
		if !insightsPromptPath.MatchString(path) {
			return nil, fmt.Errorf("summary prompt must use simple run paths such as {{run.inputs}} or {{all_thread_messages}}; sections, helpers and unassigned variables are not supported")
		}
		seen[path] = true
	}
	if len(seen) == 0 {
		return nil, fmt.Errorf("summary prompt must include trace data, for example {{run.inputs}} and {{run.outputs}}; omit it to use the service default")
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

func previewInsightsRun(ctx context.Context, c *client.Client, projectID, runID, prompt string) (map[string]any, error) {
	paths := []string{"run.inputs", "run.outputs", "run.error"}
	if prompt != "" {
		var err error
		paths, err = insightsPromptVariables(prompt)
		if err != nil {
			return nil, err
		}
	}
	selects := []langsmith.RunQueryParamsSelect{"id", "trace_id", "parent_run_id", "session_id"}
	seen := map[string]bool{"id": true, "trace_id": true, "parent_run_id": true, "session_id": true}
	for _, path := range paths {
		if path == "all_thread_messages" {
			continue
		}
		field := strings.Split(path, ".")[1]
		sel := langsmith.RunQueryParamsSelect(field)
		if sel.IsKnown() && !seen[field] {
			selects = append(selects, sel)
			seen[field] = true
		}
	}
	runs, err := queryRuns(ctx, c, langsmith.RunQueryParams{ID: langsmith.F([]string{runID}), IsRoot: langsmith.F(true), Select: langsmith.F(selects), Limit: langsmith.F(int64(1))}, projectID, 1, 0)
	if err != nil {
		return nil, fmt.Errorf("reading Insights preview run: %w", err)
	}
	if len(runs) != 1 || runs[0].ID != runID {
		return nil, fmt.Errorf("preview root run not found in selected project")
	}
	// Raw response preserves omitted versus explicitly null fields.
	var run map[string]any
	decoder := json.NewDecoder(strings.NewReader(runs[0].JSON.RawJSON()))
	decoder.UseNumber()
	if err := decoder.Decode(&run); err != nil {
		return nil, fmt.Errorf("invalid preview run response")
	}
	bindings := map[string]any{}
	missing := []string{}
	unchecked := []string{}
	for _, path := range paths {
		if path == "all_thread_messages" || strings.HasPrefix(path, "run.feedback") {
			unchecked = append(unchecked, path)
			continue
		}
		if !langsmith.RunQueryParamsSelect(strings.Split(path, ".")[1]).IsKnown() {
			unchecked = append(unchecked, path)
			continue
		}
		var current any = run
		found := true
		for _, part := range strings.Split(path, ".")[1:] {
			object, ok := current.(map[string]any)
			if !ok {
				found = false
				break
			}
			current, ok = object[part]
			if !ok {
				found = false
				break
			}
		}
		if found {
			bindings[path] = current
		} else {
			missing = append(missing, path)
		}
	}
	return map[string]any{"run_id": runID, "bindings": bindings, "missing_paths": missing, "unchecked_paths": unchecked,
		"paths_validated": len(missing) == 0 && len(unchecked) == 0, "note": "Read-only variable bindings, not an LLM summary. This explicit run is not checked against the analysis filter/time window or guaranteed to be sampled. Thread messages and feedback require separate inspection."}, nil
}
