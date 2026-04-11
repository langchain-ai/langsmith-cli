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
	cmd := &cobra.Command{
		Use:    "issues",
		Short:  "[Private Beta] Manage issues for a tracing project",
		Hidden: true,
		Long: `[Private Beta] Manage Issues Board issues for a tracing project.
This feature is currently in private beta and may not be available to all users.

Examples:
  langsmith project issues list --project my-app
  langsmith project issues list --project my-app --status open --priority high
  langsmith project issues events --project my-app`,
	}

	cmd.AddCommand(newProjectIssuesListCmd())
	cmd.AddCommand(newProjectIssuesEventsCmd())
	cmd.AddCommand(newProjectIssuesUpdateCmd())
	cmd.AddCommand(newProjectIssuesRunsCmd())
	return cmd
}

func newProjectIssuesListCmd() *cobra.Command {
	var (
		project    string
		status     string
		priority   string
		limit      int
		outputFile string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "[Private Beta] List issues for a tracing project",
		Long: `[Private Beta] List forge issues associated with a tracing project.

Fetches issues from the Issues Board for the specified project. Results
can be filtered by status (open/closed) and priority (high/medium/low).
Output is JSON by default; pass --format pretty for a human-readable table.

Examples:
  langsmith project issues list --project my-app
  langsmith project issues list --project my-app --status open
  langsmith project issues list --project my-app --priority high --limit 10
  langsmith project issues list --project my-app --format pretty`,
		Run: func(cmd *cobra.Command, args []string) {
			c := mustGetClient()
			ctx := context.Background()

			projectName := ResolveProject(project)
			if projectName == "" {
				exitError("--project is required (or set LANGSMITH_PROJECT)")
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
				exitErrorf("listing issues: %v", err)
			}

			if limit > 0 && len(issues) > limit {
				issues = issues[:limit]
			}

			fmt_ := getFormat()

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

// issueEvent mirrors the JSON shape returned by GET /v1/platform/sessions/{id}/issue-events.
type issueEvent struct {
	ID        string          `json:"id"`
	TenantID  string          `json:"tenant_id"`
	SessionID string          `json:"session_id"`
	IssueID   *string         `json:"issue_id"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	Actor     string          `json:"actor"`
	CreatedAt time.Time       `json:"created_at"`
}

func newProjectIssuesEventsCmd() *cobra.Command {
	var (
		project         string
		lookBackMinutes int
		limit           int
		outputFile      string
	)

	cmd := &cobra.Command{
		Use:   "events",
		Short: "[Private Beta] List issue events for a tracing project",
		Long: `[Private Beta] List issue events for the Issues Board of a tracing project.

Issue events record user and agent actions on issues: status changes, severity
edits, evaluator deployments, and issue creation. The ABM agent reads these on
cron runs to update the User Preferences section of the Agent Overview.

Output is JSON by default; pass --format pretty for a human-readable table.

Examples:
  langsmith project issues events --project my-app
  langsmith project issues events --project my-app --look-back-minutes 1440
  langsmith project issues events --project my-app --limit 50 --format pretty`,
		Run: func(cmd *cobra.Command, args []string) {
			c := mustGetClient()
			ctx := context.Background()

			projectName := ResolveProject(project)
			if projectName == "" {
				exitError("--project is required (or set LANGSMITH_PROJECT)")
			}

			sessionID, err := c.ResolveSessionID(ctx, projectName)
			if err != nil {
				exitErrorf("resolving project %q: %v", projectName, err)
			}

			path := fmt.Sprintf("/v1/platform/sessions/%s/issue-events?look_back_minutes=%d&limit=%d",
				sessionID, lookBackMinutes, limit)

			var events []issueEvent
			if err := c.RawGet(ctx, path, &events); err != nil {
				exitErrorf("listing issue events: %v", err)
			}

			fmt_ := getFormat()

			if fmt_ == "pretty" {
				columns := []string{"EVENT TYPE", "ACTOR", "ISSUE ID", "CREATED"}
				var rows [][]string
				for _, e := range events {
					issueRef := ""
					if e.IssueID != nil {
						issueRef = (*e.IssueID)[:8]
					}
					rows = append(rows, []string{
						e.EventType,
						e.Actor,
						issueRef,
						formatIssueTime(e.CreatedAt),
					})
				}
				output.OutputTable(columns, rows, fmt.Sprintf("Issue events for %s", projectName))
			} else {
				var data []map[string]any
				for _, e := range events {
					issueID := any(nil)
					if e.IssueID != nil {
						issueID = *e.IssueID
					}
					data = append(data, map[string]any{
						"id":         e.ID,
						"session_id": e.SessionID,
						"issue_id":   issueID,
						"event_type": e.EventType,
						"payload":    e.Payload,
						"actor":      e.Actor,
						"created_at": formatTimeISO(e.CreatedAt),
					})
				}
				output.OutputJSON(data, outputFile)
			}
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name [env: LANGSMITH_PROJECT]")
	cmd.Flags().IntVar(&lookBackMinutes, "look-back-minutes", 10080, "Look-back window in minutes (default 10080 = 7 days)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of events to return")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
}

func newProjectIssuesUpdateCmd() *cobra.Command {
	var (
		addTraces   []string
		title       string
		description string
		outputFile  string
	)

	cmd := &cobra.Command{
		Use:   "update <issue-id>",
		Short: "[Private Beta] Update an existing issue (add evidence or correct a disproven finding)",
		Long: `[Private Beta] Update an existing issue.

Two use cases:
  1. Add trace IDs as new supporting evidence (--add-traces)
  2. Correct the title or description when evidence disproves the original finding
     (--title / --description)

The issue ID is the UUID returned by 'langsmith project issues list'.

Examples:
  langsmith project issues update <id> --add-traces <trace-id1>,<trace-id2>
  langsmith project issues update <id> --title "Corrected title" --description "New finding..."`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			issueID := args[0]
			if len(addTraces) == 0 && title == "" && description == "" {
				exitError("at least one of --add-traces, --title, or --description is required")
			}

			c := mustGetClient()
			ctx := context.Background()

			body := map[string]any{}

			if len(addTraces) > 0 {
				type traceInput struct {
					TraceID   string `json:"trace_id"`
					StartTime string `json:"start_time"`
				}
				traces := make([]traceInput, 0, len(addTraces))
				for _, tid := range addTraces {
					traces = append(traces, traceInput{TraceID: tid, StartTime: time.Now().UTC().Format(time.RFC3339)})
				}
				body["traces"] = traces
			}
			if title != "" {
				body["name"] = title
			}
			if description != "" {
				body["description"] = description
			}

			path := fmt.Sprintf("/v1/platform/issues/%s", issueID)

			var issue forgeIssue
			if err := c.RawPatch(ctx, path, body, &issue); err != nil {
				exitErrorf("updating issue: %v", err)
			}

			output.OutputJSON(issueToMap(issue), outputFile)
		},
	}

	cmd.Flags().StringArrayVar(&addTraces, "add-traces", nil, "Trace IDs to add as evidence (repeatable)")
	cmd.Flags().StringVar(&title, "title", "", "Corrected title (use only when original is factually wrong)")
	cmd.Flags().StringVar(&description, "description", "", "Corrected description (use only when original is factually wrong)")
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

func newProjectIssuesRunsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "runs",
		Short:  "[Private Beta] Manage linked runs for an issue",
		Hidden: true,
		Long: `[Private Beta] Link and unlink runs to/from an issue.

Examples:
  langsmith project issues runs add <issue-id> --run-id <run-id> --start-time 2026-04-10T00:00:00Z
  langsmith project issues runs update <issue-id> <run-id> --comment "new comment"
  langsmith project issues runs remove <issue-id> <run-id>`,
	}

	cmd.AddCommand(newProjectIssuesRunsAddCmd())
	cmd.AddCommand(newProjectIssuesRunsUpdateCmd())
	cmd.AddCommand(newProjectIssuesRunsRemoveCmd())
	return cmd
}

func newProjectIssuesRunsAddCmd() *cobra.Command {
	var (
		runID     string
		startTime string
		comment   string
	)

	cmd := &cobra.Command{
		Use:   "add <issue-id>",
		Short: "Link a run to an issue",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			issueID := args[0]
			if runID == "" || startTime == "" {
				exitError("--run-id and --start-time are required")
			}

			c := mustGetClient()
			ctx := context.Background()

			body := map[string]any{
				"run_id":     runID,
				"start_time": startTime,
			}
			if comment != "" {
				body["comment"] = comment
			}

			path := fmt.Sprintf("/v1/platform/issues/%s/runs", issueID)
			if err := c.RawPost(ctx, path, body, nil); err != nil {
				exitErrorf("linking run: %v", err)
			}
			fmt.Printf("Run %s linked to issue %s\n", runID, issueID)
		},
	}

	cmd.Flags().StringVar(&runID, "run-id", "", "Run ID to link (required)")
	cmd.Flags().StringVar(&startTime, "start-time", "", "Run start time in RFC3339 format (required)")
	cmd.Flags().StringVar(&comment, "comment", "", "Optional comment explaining why this run is evidence")
	return cmd
}

func newProjectIssuesRunsUpdateCmd() *cobra.Command {
	var comment string

	cmd := &cobra.Command{
		Use:   "update <issue-id> <run-id>",
		Short: "Update the comment on a linked run",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			issueID := args[0]
			runID := args[1]

			c := mustGetClient()
			ctx := context.Background()

			body := map[string]any{"comment": comment}
			path := fmt.Sprintf("/v1/platform/issues/%s/runs/%s", issueID, runID)
			if err := c.RawPatch(ctx, path, body, nil); err != nil {
				exitErrorf("updating linked run: %v", err)
			}
			fmt.Printf("Updated comment on run %s for issue %s\n", runID, issueID)
		},
	}

	cmd.Flags().StringVar(&comment, "comment", "", "Updated comment (pass empty string to clear)")
	return cmd
}

func newProjectIssuesRunsRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <issue-id> <run-id>",
		Short: "Unlink a run from an issue",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			issueID := args[0]
			runID := args[1]

			c := mustGetClient()
			ctx := context.Background()

			path := fmt.Sprintf("/v1/platform/issues/%s/runs/%s", issueID, runID)
			if err := c.RawDelete(ctx, path, nil); err != nil {
				exitErrorf("unlinking run: %v", err)
			}
			fmt.Printf("Run %s unlinked from issue %s\n", runID, issueID)
		},
	}

	return cmd
}
