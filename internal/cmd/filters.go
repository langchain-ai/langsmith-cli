package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

// FilterFlags holds common filter flag values.
type FilterFlags struct {
	TraceIDs     string
	Limit        int
	Project      string
	LastNMinutes int
	Since        string
	ErrorFlag    bool
	NoErrorFlag  bool
	Name         string
	RunType      string
	MinLatency   float64
	MaxLatency   float64
	MinTokens    int
	Tags         string
	RawFilter    string
}

// addCommonFilterFlags attaches shared filter flags to a command.
func addCommonFilterFlags(cmd *cobra.Command, f *FilterFlags, includeRunType bool) {
	cmd.Flags().StringVar(&f.TraceIDs, "trace-ids", "", "Comma-separated trace IDs to filter by")
	cmd.Flags().IntVarP(&f.Limit, "limit", "n", 0, "Maximum number of results to return")
	cmd.Flags().StringVar(&f.Project, "project", "", "Project name [env: LANGSMITH_PROJECT]")
	cmd.Flags().IntVar(&f.LastNMinutes, "last-n-minutes", 0, "Only include runs from the last N minutes")
	cmd.Flags().StringVar(&f.Since, "since", "", "Only include runs after this ISO timestamp")
	cmd.Flags().BoolVar(&f.ErrorFlag, "error", false, "Filter for failed runs only")
	cmd.Flags().BoolVar(&f.NoErrorFlag, "no-error", false, "Filter for successful runs only")
	cmd.Flags().StringVar(&f.Name, "name", "", "Filter by run name (exact match)")
	cmd.Flags().Float64Var(&f.MinLatency, "min-latency", 0, "Minimum latency in seconds")
	cmd.Flags().Float64Var(&f.MaxLatency, "max-latency", 0, "Maximum latency in seconds")
	cmd.Flags().IntVar(&f.MinTokens, "min-tokens", 0, "Minimum total tokens")
	cmd.Flags().StringVar(&f.Tags, "tags", "", "Comma-separated tags (OR logic)")
	cmd.Flags().StringVar(&f.RawFilter, "filter", "", "Raw LangSmith filter DSL string")

	if includeRunType {
		cmd.Flags().StringVar(&f.RunType, "run-type", "", "Filter by run type (llm, chain, tool, retriever, prompt, parser)")
	}
}

// BuildRunQueryParams builds RunQueryParams from FilterFlags.
func BuildRunQueryParams(f *FilterFlags, isRoot bool, defaultLimit int) langsmith.RunQueryParams {
	params := langsmith.RunQueryParams{}

	// Resolve project → session ID (handled separately by caller, set Session field)
	// We build the filter DSL here and return it; the caller resolves session IDs.

	// Limit
	limit := defaultLimit
	if f.Limit > 0 {
		limit = f.Limit
	}
	params.Limit = langsmith.F(int64(limit))

	// Is root
	if isRoot {
		params.IsRoot = langsmith.F(true)
	}

	// Start time
	if f.LastNMinutes > 0 {
		t := time.Now().UTC().Add(-time.Duration(f.LastNMinutes) * time.Minute)
		params.StartTime = langsmith.F(t)
	} else if f.Since != "" {
		t, err := time.Parse(time.RFC3339, f.Since)
		if err != nil {
			// Try without timezone suffix
			t, err = time.Parse("2006-01-02T15:04:05", f.Since)
			if err != nil {
				exitErrorf("invalid --since timestamp: %s", f.Since)
			}
		}
		params.StartTime = langsmith.F(t)
	}

	// Run type
	if f.RunType != "" {
		params.RunType = langsmith.F(langsmith.RunQueryParamsRunType(f.RunType))
	}

	// Error
	if f.ErrorFlag {
		params.Error = langsmith.F(true)
	} else if f.NoErrorFlag {
		params.Error = langsmith.F(false)
	}

	// Trace ID (single)
	if f.TraceIDs != "" {
		ids := splitTrim(f.TraceIDs)
		if len(ids) == 1 {
			params.Trace = langsmith.F(ids[0])
		}
	}

	// Build filter DSL
	filterStr := buildFilterDSL(f)
	if filterStr != "" {
		params.Filter = langsmith.F(filterStr)
	}

	return params
}

// buildFilterDSL builds the LangSmith filter DSL string from filter flags.
func buildFilterDSL(f *FilterFlags) string {
	var parts []string

	// Multiple trace IDs
	if f.TraceIDs != "" {
		ids := splitTrim(f.TraceIDs)
		if len(ids) > 1 {
			quoted := make([]string, len(ids))
			for i, id := range ids {
				quoted[i] = fmt.Sprintf("%q", id)
			}
			parts = append(parts, fmt.Sprintf("in(trace_id, [%s])", strings.Join(quoted, ", ")))
		}
	}

	// Name filter (exact match via eq; use --filter for advanced name queries)
	if f.Name != "" {
		parts = append(parts, fmt.Sprintf("eq(name, %q)", f.Name))
	}

	// Latency filters
	if f.MinLatency > 0 {
		parts = append(parts, fmt.Sprintf("gte(latency, %g)", f.MinLatency))
	}
	if f.MaxLatency > 0 {
		parts = append(parts, fmt.Sprintf("lte(latency, %g)", f.MaxLatency))
	}

	// Note: total_tokens is not accepted as a server-side filter attribute.
	// --min-tokens filtering is applied client-side in queryRuns().

	// Tags
	if f.Tags != "" {
		tagList := splitTrim(f.Tags)
		if len(tagList) == 1 {
			parts = append(parts, fmt.Sprintf("has(tags, %q)", tagList[0]))
		} else if len(tagList) > 1 {
			clauses := make([]string, len(tagList))
			for i, t := range tagList {
				clauses[i] = fmt.Sprintf("has(tags, %q)", t)
			}
			parts = append(parts, fmt.Sprintf("or(%s)", strings.Join(clauses, ", ")))
		}
	}

	// Raw filter passthrough
	if f.RawFilter != "" {
		parts = append(parts, f.RawFilter)
	}

	// Combine
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	default:
		return fmt.Sprintf("and(%s)", strings.Join(parts, ", "))
	}
}

// ResolveProject resolves the project name from flag → env.
func ResolveProject(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv("LANGSMITH_PROJECT")
}

// splitTrim splits a comma-separated string and trims whitespace.
func splitTrim(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
