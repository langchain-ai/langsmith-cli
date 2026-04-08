package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

// forgeIssue mirrors the JSON shape returned by GET /v1/platform/issues.
type forgeIssue struct {
	ID          string          `json:"id"`
	SessionID   string          `json:"session_id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Severity    int             `json:"severity"`
	Status      string          `json:"status"`
	Tags        []string        `json:"tags"`
	FixBranch   *string         `json:"fix_branch"`
	FixPrompt   *string         `json:"fix_prompt"`
	FixPRNumber *int            `json:"fix_pr_number"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	Traces      json.RawMessage `json:"traces"`
}

var severityLabels = map[int]string{
	0: "URGENT",
	1: "HIGH",
	2: "MEDIUM",
	3: "LOW",
}

func newProjectIssuesCmd() *cobra.Command {
	var (
		project    string
		status     string
		priority   string
		limit      int
		outputFile string
	)

	cmd := &cobra.Command{
		Use:    "issues",
		Short:  "[Private Beta] List issues for a tracing project",
		Hidden: true,
		Long: `[Private Beta] List forge issues associated with a tracing project.
This feature is currently in private beta and may not be available to all users.

Fetches issues from the Issues Board for the specified project. Results
can be filtered by status (open/closed) and priority (high/medium/low).
Output is JSON by default; pass --format pretty for a human-readable table.

Examples:
  langsmith project issues --project my-app
  langsmith project issues --project my-app --status open
  langsmith project issues --project my-app --priority high --limit 10
  langsmith project issues --project my-app --format pretty`,
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			ctx := context.Background()

			projectName := ResolveProject(project)
			if projectName == "" {
				ExitError("--project is required (or set LANGSMITH_PROJECT)")
			}

			path := fmt.Sprintf("/v1/platform/issues?session_name=%s", urlEscape(projectName))
			if status != "" {
				path += "&status=" + urlEscape(status)
			}
			if priority != "" {
				sev := priorityToSeverity(priority)
				if sev >= 0 {
					path += fmt.Sprintf("&severity=%d", sev)
				}
			}

			var issues []forgeIssue
			if err := c.RawGet(ctx, path, &issues); err != nil {
				ExitErrorf("listing issues: %v", err)
			}

			if limit > 0 && len(issues) > limit {
				issues = issues[:limit]
			}

			fmt_ := GetFormat()

			if fmt_ == "pretty" {
				columns := []string{"NAME", "SEVERITY", "STATUS", "TAGS", "CREATED"}
				var rows [][]string
				for _, issue := range issues {
					sevLabel := severityLabels[issue.Severity]
					if sevLabel == "" {
						sevLabel = fmt.Sprintf("%d", issue.Severity)
					}
					rows = append(rows, []string{
						truncate(issue.Name, 60),
						sevLabel,
						issue.Status,
						strings.Join(issue.Tags, ", "),
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

func priorityToSeverity(p string) int {
	switch strings.ToLower(p) {
	case "urgent":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return -1
	}
}

func issueToMap(issue forgeIssue) map[string]any {
	return map[string]any{
		"id":            issue.ID,
		"session_id":    issue.SessionID,
		"name":          issue.Name,
		"description":   issue.Description,
		"severity":      issue.Severity,
		"status":        issue.Status,
		"tags":          issue.Tags,
		"fix_branch":    issue.FixBranch,
		"fix_prompt":    issue.FixPrompt,
		"fix_pr_number": issue.FixPRNumber,
		"created_at":    formatTimeISO(issue.CreatedAt),
		"updated_at":    formatTimeISO(issue.UpdatedAt),
	}
}

func formatIssueTime(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}
	return t.Format("2006-01-02 15:04")
}

func urlEscape(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, " ", "%20")
	s = strings.ReplaceAll(s, "&", "%26")
	s = strings.ReplaceAll(s, "=", "%3D")
	s = strings.ReplaceAll(s, "+", "%2B")
	s = strings.ReplaceAll(s, "#", "%23")
	return s
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}
