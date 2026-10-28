package cmd

import (
	"fmt"
	"os"
	"regexp"
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
	ProjectID    string
	LastNMinutes int
	Since        string
	Before       string
	Cursor       string
	ErrorFlag    bool
	NoErrorFlag  bool
	Name         string
	RunType      string
	MinLatency   float64
	MaxLatency   float64
	MinTokens    int
	Tags         string
	Metadata     string
	RawFilter    string

	FeedbackSince  string
	FeedbackBefore string
}

const (
	projectFlagUsage   = "Project name [env: LANGSMITH_PROJECT]"
	projectIDFlagUsage = "Project (session) UUID; skips the name lookup. Mutually exclusive with --project; overrides $LANGSMITH_PROJECT"
)

// addProjectFlags attaches the --project / --project-id pair to a command that
// does not use the shared filter flags. Every command that resolves a project
// takes both, so an agent building a command line never has to quote a project
// name it did not choose. Resolve them with resolveSessionID.
func addProjectFlags(cmd *cobra.Command, project, projectID *string) {
	cmd.Flags().StringVar(project, "project", "", projectFlagUsage)
	cmd.Flags().StringVar(projectID, "project-id", "", projectIDFlagUsage)
	cmd.MarkFlagsMutuallyExclusive("project", "project-id")
}

// addCommonFilterFlags attaches shared filter flags to a command.
func addCommonFilterFlags(cmd *cobra.Command, f *FilterFlags, includeRunType bool) {
	cmd.Flags().StringVar(&f.TraceIDs, "trace-ids", "", "Comma-separated trace IDs to filter by")
	cmd.Flags().IntVarP(&f.Limit, "limit", "n", 0, "Maximum number of results to return")
	addProjectFlags(cmd, &f.Project, &f.ProjectID)
	cmd.Flags().IntVar(&f.LastNMinutes, "last-n-minutes", 0, "Only include runs from the last N minutes, e.g. 60 (overrides 7-day default)")
	cmd.Flags().StringVar(&f.Since, "since", "", "Only include runs after this timestamp, e.g. 2024-01-15T00:00:00Z (overrides 7-day default)")
	cmd.Flags().StringVar(&f.Before, "before", "", "Only include runs before this timestamp, e.g. 2024-01-15T00:00:00Z (for pagination)")
	cmd.Flags().BoolVar(&f.ErrorFlag, "error", false, "Filter for failed runs only")
	cmd.Flags().BoolVar(&f.NoErrorFlag, "no-error", false, "Filter for successful runs only")
	cmd.Flags().StringVar(&f.Name, "name", "", "Filter by run name (exact match)")
	cmd.Flags().Float64Var(&f.MinLatency, "min-latency", 0, "Minimum latency in seconds")
	cmd.Flags().Float64Var(&f.MaxLatency, "max-latency", 0, "Maximum latency in seconds")
	cmd.Flags().IntVar(&f.MinTokens, "min-tokens", 0, "Minimum total tokens")
	cmd.Flags().StringVar(&f.Tags, "tags", "", "Comma-separated tags (OR logic)")
	cmd.Flags().StringVar(&f.Metadata, "metadata", "", "Filter by metadata key=value (e.g. revision_id=abc123)")
	cmd.Flags().StringVar(&f.RawFilter, "filter", "", filterFlagUsage)

	if includeRunType {
		cmd.Flags().StringVar(&f.RunType, "run-type", "", "Filter by run type (llm, chain, tool, retriever, prompt, parser)")
	}
}

func addFeedbackWindowFlags(cmd *cobra.Command, f *FilterFlags) {
	cmd.Flags().StringVar(&f.FeedbackSince, "feedback-since", "", "Only match feedback modified at or after this timestamp, e.g. 2024-01-15T00:00:00Z (requires a feedback condition in --filter)")
	cmd.Flags().StringVar(&f.FeedbackBefore, "feedback-before", "", "Only match feedback modified before this timestamp, e.g. 2024-01-15T00:00:00Z (requires a feedback condition in --filter)")
}

func feedbackWindow(f *FilterFlags) (since, before time.Time) {
	return parseFeedbackBound("--feedback-since", f.FeedbackSince), parseFeedbackBound("--feedback-before", f.FeedbackBefore)
}

func parseFeedbackBound(flag, value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	t, err := parseFlexTime(value)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05", value)
	}
	if err != nil {
		ExitErrorf("invalid %s timestamp: %s", flag, value)
	}
	return t
}

// resolveStartTime returns the start time for a query.
// Priority: lastNMinutes > since > default (7 days ago).
func resolveStartTime(since string, lastNMinutes int) time.Time {
	if lastNMinutes > 0 {
		return time.Now().UTC().Add(-time.Duration(lastNMinutes) * time.Minute)
	}
	if since != "" {
		// Date-only is accepted so --since matches --before (parseFlexTime) and
		// the "RFC3339 or YYYY-MM-DD" contract the flag help advertises.
		for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
			if t, err := time.Parse(layout, since); err == nil {
				return t
			}
		}
		ExitErrorf("invalid --since timestamp: %s", since)
	}
	return time.Now().UTC().Add(-7 * 24 * time.Hour)
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
	params.Order = langsmith.F(langsmith.RunQueryParamsOrderDesc)

	// Start time
	params.StartTime = langsmith.F(resolveStartTime(f.Since, f.LastNMinutes))

	// End time (--before flag for forward pagination)
	if f.Before != "" {
		params.EndTime = langsmith.F(parseBefore(f.Before))
	}

	// Run type
	if f.RunType != "" {
		params.RunType = langsmith.F(langsmith.RunTypeEnum(f.RunType))
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

	checkFilterFlags(f, params.StartTime.Value)

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

	// Name filter (exact match)
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

	// Note: --min-tokens is not emitted here. total_tokens is filterable on the
	// v2 (SmithDB) path but not on v1, and buildFilterDSL cannot see which
	// backend a deployment uses. queryRunsAuto and queryTracesAuto add the
	// clause when v2 is in play and fall back to the client-side bound otherwise.

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

	// Metadata key=value filter
	if f.Metadata != "" {
		key, val, ok := strings.Cut(f.Metadata, "=")
		if !ok {
			// key-only: filter by key existence
			parts = append(parts, fmt.Sprintf("eq(metadata_key, %q)", key))
		} else {
			parts = append(parts, fmt.Sprintf("and(eq(metadata_key, %q), eq(metadata_value, %q))", key, val))
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

// filterFlagUsage is the --filter help shared by every command that takes one.
const filterFlagUsage = `LangSmith filter DSL, e.g. 'and(eq(run_type, "llm"), gte(latency, 5))'. Match metadata with 'and(eq(metadata_key, "user_id"), eq(metadata_value, "u1"))', and inputs or outputs with input_key/input_value or output_key/output_value; metadata.x and inputs.x paths are not supported`

// parseBefore parses --before as RFC 3339, with or without a zone.
func parseBefore(before string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, before); err == nil {
			return t
		}
	}
	ExitErrorf("invalid --before timestamp: %s", before)
	return time.Time{}
}

// checkFilterFlags rejects filter flags the server would reject, with the fix.
// Every command that sends the shared filter flags calls it.
func checkFilterFlags(f *FilterFlags, start time.Time) {
	if f.Before != "" {
		if end := parseBefore(f.Before); !end.After(start) {
			ExitErrorf("--before (%s) must be later than the start of the window (%s); set --since or --last-n-minutes to an earlier time",
				end.Format(time.RFC3339), start.Format(time.RFC3339))
		}
	}
	if err := validateRawFilter(f.RawFilter); err != nil {
		ExitErrorf("%v", err)
	}
}

var (
	quotedString = regexp.MustCompile(`"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'`)
	nestedPath   = regexp.MustCompile(`\b(metadata|inputs|outputs|extra)\.[A-Za-z_]`)
)

// validateRawFilter catches the filter mistakes agents make most, which the
// server rejects with a parse error: nested paths such as metadata.user_id.
func validateRawFilter(filter string) error {
	m := nestedPath.FindStringSubmatch(quotedString.ReplaceAllString(filter, `""`))
	if m == nil {
		return nil
	}
	if m[1] == "metadata" || m[1] == "extra" {
		return fmt.Errorf(`--filter can't read %s.<key>; use and(eq(metadata_key, "key"), eq(metadata_value, "value")) (or --metadata key=value on run and trace lists)`, m[1])
	}
	attr := strings.TrimSuffix(m[1], "s")
	return fmt.Errorf(`--filter can't read %s.<key>; use and(eq(%s_key, "key"), eq(%s_value, "value")), or search("text") for a full-text match`, m[1], attr, attr)
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
