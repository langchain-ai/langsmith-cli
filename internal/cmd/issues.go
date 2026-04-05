package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

// forgeIssue mirrors the JSON shape returned by GET /v1/platform/forge-issues.
type forgeIssue struct {
	ID           string    `json:"id"`
	IssueBoardID string    `json:"issue_board_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Priority     string    `json:"priority"`
	Status       string    `json:"status"`
	Category     *string   `json:"category"`
	TraceIDs     []string  `json:"trace_ids"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	ResolvedAt   *string   `json:"resolved_at"`
}

func newIssuesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issues",
		Short: "Manage issues on the LangSmith Issues Board",
		Long: `Manage issues on the LangSmith Issues Board.

The Issues Board surfaces problems detected in your tracing projects —
e.g. repeated failures, regressions, or patterns flagged by the Issues
Agent. Use this command group to list and inspect issues programmatically.

Examples:
  langsmith issues list --project my-app
  langsmith issues list --project my-app --status open --priority high
  langsmith issues list --project my-app --format pretty`,
	}

	cmd.AddCommand(newIssuesListCmd())
	return cmd
}

func newIssuesListCmd() *cobra.Command {
	var (
		project    string
		status     string
		priority   string
		limit      int
		outputFile string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List forge issues for a tracing project",
		Long: `List forge issues associated with a tracing project.

Fetches issues from the Issues Board for the specified project. Results
can be filtered by status (open/closed) and priority (high/medium/low).
Output is JSON by default; pass --format pretty for a human-readable table.

Examples:
  langsmith issues list --project my-app
  langsmith issues list --project my-app --status open
  langsmith issues list --project my-app --priority high --limit 10
  langsmith issues list --project my-app --format pretty -o issues.json`,
		Run: func(cmd *cobra.Command, args []string) {
			c := mustGetClient()
			ctx := context.Background()

			projectName := ResolveProject(project)
			if projectName == "" {
				exitError("--project is required (or set LANGSMITH_PROJECT)")
			}

			// Build query path with optional filters.
			path := fmt.Sprintf("/v1/platform/forge-issues?session_name=%s", urlEscape(projectName))
			if status != "" {
				path += "&status=" + urlEscape(status)
			}
			if priority != "" {
				path += "&priority=" + urlEscape(priority)
			}

			var issues []forgeIssue
			if err := c.RawGet(ctx, path, &issues); err != nil {
				exitErrorf("listing issues: %v", err)
			}

			// Apply client-side limit.
			if limit > 0 && len(issues) > limit {
				issues = issues[:limit]
			}

			fmt_ := getFormat()

			if fmt_ == "pretty" {
				columns := []string{"TITLE", "PRIORITY", "STATUS", "CATEGORY", "TRACES", "CREATED"}
				var rows [][]string
				for _, issue := range issues {
					category := ""
					if issue.Category != nil {
						category = *issue.Category
					}
					rows = append(rows, []string{
						truncate(issue.Title, 60),
						issue.Priority,
						issue.Status,
						category,
						fmt.Sprintf("%d", len(issue.TraceIDs)),
						formatIssueTime(issue.CreatedAt),
					})
				}
				output.OutputTable(columns, rows, fmt.Sprintf("Issues for %s", projectName))
			} else {
				var data []map[string]any
				for _, issue := range issues {
					data = append(data, issueToMap(issue))
				}
				output.OutputJSON(data, outputFile)
			}
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name [env: LANGSMITH_PROJECT]")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status: open or closed")
	cmd.Flags().StringVar(&priority, "priority", "", "Filter by priority: high, medium, or low")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of issues to return")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
}

// issueToMap converts a forgeIssue to a plain map for JSON output.
func issueToMap(issue forgeIssue) map[string]any {
	m := map[string]any{
		"id":             issue.ID,
		"issue_board_id": issue.IssueBoardID,
		"title":          issue.Title,
		"description":    issue.Description,
		"priority":       issue.Priority,
		"status":         issue.Status,
		"trace_ids":      issue.TraceIDs,
		"created_at":     formatTimeISO(issue.CreatedAt),
		"updated_at":     formatTimeISO(issue.UpdatedAt),
	}
	if issue.Category != nil {
		m["category"] = *issue.Category
	} else {
		m["category"] = nil
	}
	if issue.ResolvedAt != nil {
		m["resolved_at"] = *issue.ResolvedAt
	} else {
		m["resolved_at"] = nil
	}
	return m
}

// formatIssueTime formats a time.Time for the pretty table.
func formatIssueTime(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}
	return t.Format("2006-01-02 15:04")
}

// urlEscape percent-encodes a string for use in a query parameter value.
// Only encodes characters that would break URL parsing (space, &, =, +, #).
func urlEscape(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, " ", "%20")
	s = strings.ReplaceAll(s, "&", "%26")
	s = strings.ReplaceAll(s, "=", "%3D")
	s = strings.ReplaceAll(s, "+", "%2B")
	s = strings.ReplaceAll(s, "#", "%23")
	return s
}

// truncate shortens a string to at most n runes, appending "…" if cut.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}
