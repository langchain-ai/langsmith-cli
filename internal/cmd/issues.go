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
	ProposedFix *string         `json:"proposed_fix"`
	Actions     json.RawMessage `json:"actions"`
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
Output is a human-readable table by default; pass --json for machine-readable JSON.

Examples:
  langsmith project issues list --project my-app
  langsmith project issues list --project my-app --status open
  langsmith project issues list --project my-app --priority high --limit 10
  langsmith project issues list --project my-app --json`,
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

Output is a human-readable table by default; pass --json for machine-readable JSON.

Examples:
  langsmith project issues events --project my-app
  langsmith project issues events --project my-app --look-back-minutes 1440
  langsmith project issues events --project my-app --limit 50 --json`,
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			ctx := context.Background()

			projectName := ResolveProject(project)
			if projectName == "" {
				ExitError("--project is required (or set LANGSMITH_PROJECT)")
			}

			sessionID, err := c.ResolveSessionID(ctx, projectName)
			if err != nil {
				ExitErrorf("resolving project %q: %v", projectName, err)
			}

			path := fmt.Sprintf("/v1/platform/sessions/%s/issue-events?look_back_minutes=%d&limit=%d",
				sessionID, lookBackMinutes, limit)

			var events []issueEvent
			if err := c.RawGet(ctx, path, &events); err != nil {
				ExitErrorf("listing issue events: %v", err)
			}

			fmt_ := GetFormat()

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
		title       string
		description string
		proposedFix string
		evaluator   string
		outputFile  string
	)

	cmd := &cobra.Command{
		Use:   "update <issue-id>",
		Short: "[Private Beta] Update an existing issue's title, description, proposed fix, or evaluator",
		Long: `[Private Beta] Update an existing issue.

To link runs as evidence, use 'langsmith project issues runs add' instead.

The issue ID is the UUID returned by 'langsmith project issues list'.

--title and --description are for factual corrections only (when new evidence disproves the original finding).
--proposed-fix updates the suggested code fix shown to users.
--evaluator replaces the suggested evaluator. Pass the evaluator config as JSON — the CLI wraps it automatically.

Examples:
  langsmith project issues update <id> --title "Corrected title" --description "New finding..."
  langsmith project issues update <id> --proposed-fix "Root cause: missing null check.\n\` + "`" + `` + "`" + `diff\n-if result:\n+if result is not None:\n` + "`" + `` + "`" + `"
  langsmith project issues update <id> --evaluator '{"type":"llm","display_name":"no_hallucination","prompt":[["system","Evaluate whether the response contains hallucinated facts. Score 1 if grounded, 0 if not."],["user","Evaluate and score."]],"schema":{"type":"object","properties":{"score":{"type":"integer","minimum":0,"maximum":1},"reasoning":{"type":"string"}},"required":["score","reasoning"]}}'
  langsmith project issues update <id> --evaluator '{"type":"code","display_name":"no_tool_errors","code_evaluators":[{"code":"def perform_eval(run, example=None):\n    out = str((run.outputs or {}).get(\"output\",\"\")).lower()\n    return {\"score\": 0 if \"error\" in out else 1, \"key\": \"no_tool_errors\"}","language":"python"}]}'`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			issueID := args[0]
			if title == "" && description == "" && proposedFix == "" && evaluator == "" {
				ExitError("at least one of --title, --description, --proposed-fix, or --evaluator is required")
			}

			c := MustGetClient()
			ctx := context.Background()

			body := map[string]any{}
			if title != "" {
				body["name"] = title
			}
			if description != "" {
				body["description"] = description
			}
			if proposedFix != "" {
				body["proposed_fix"] = proposedFix
			}
			if evaluator != "" {
				var evalConfig map[string]any
				if err := json.Unmarshal([]byte(evaluator), &evalConfig); err != nil {
					ExitErrorf("--evaluator must be valid JSON: %v", err)
				}
				evalType, _ := evalConfig["type"].(string)
				if evalType != "llm" && evalType != "code" {
					ExitError(`--evaluator must have "type": "llm" or "type": "code"`)
				}
				if _, ok := evalConfig["display_name"]; !ok {
					ExitError(`--evaluator must have a "display_name" field`)
				}
				// Inject standard fields if not provided.
				if _, ok := evalConfig["session_id"]; !ok {
					evalConfig["session_id"] = "{{session_id}}"
				}
				if _, ok := evalConfig["sampling_rate"]; !ok {
					evalConfig["sampling_rate"] = 1.0
				}
				displayName, _ := evalConfig["display_name"].(string)
				body["actions"] = []map[string]any{{
					"reason": fmt.Sprintf("Add %s evaluator", displayName),
					"method": "POST",
					"url":    "/api/v1/runs/rules",
					"body":   evalConfig,
				}}
			}

			path := fmt.Sprintf("/v1/platform/issues/%s", issueID)

			var issue forgeIssue
			if err := c.RawPatch(ctx, path, body, &issue); err != nil {
				ExitErrorf("updating issue: %v", err)
			}

			output.OutputJSON(issueToMap(issue), outputFile)
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Corrected title (use only when original is factually wrong)")
	cmd.Flags().StringVar(&description, "description", "", "Corrected description (use only when original is factually wrong)")
	cmd.Flags().StringVar(&proposedFix, "proposed-fix", "", "Updated proposed fix (markdown with code diff)")
	cmd.Flags().StringVar(&evaluator, "evaluator", "", `Replace the suggested evaluator. JSON with "type" ("llm" or "code"), "display_name", and type-specific fields`)
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
  langsmith project issues runs add <issue-id> --run-id <run-id> --start-time 2026-04-10T00:00:00Z --comment "updated"
  langsmith project issues runs remove <issue-id> <run-id>`,
	}

	cmd.AddCommand(newProjectIssuesRunsAddCmd())
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
		Short: "Link a run to an issue (or update its comment if already linked)",
		Long: `Link a run to an issue as evidence. If the run is already linked, its comment
is updated instead.

The server validates the run_id and start_time against the runs database and
resolves the trace_id automatically.

Examples:
  langsmith project issues runs add <issue-id> --run-id <run-id> --start-time 2026-04-10T00:00:00Z
  langsmith project issues runs add <issue-id> --run-id <run-id> --start-time 2026-04-10T00:00:00Z --comment "evidence"`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			issueID := args[0]
			if runID == "" || startTime == "" {
				ExitError("--run-id and --start-time are required")
			}

			c := MustGetClient()
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
				ExitErrorf("linking run: %v", err)
			}
			fmt.Printf("Run %s linked to issue %s\n", runID, issueID)
		},
	}

	cmd.Flags().StringVar(&runID, "run-id", "", "Run ID to link (required)")
	cmd.Flags().StringVar(&startTime, "start-time", "", "Run start time in RFC3339 format (required)")
	cmd.Flags().StringVar(&comment, "comment", "", "Optional comment explaining why this run is evidence")
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

			c := MustGetClient()
			ctx := context.Background()

			path := fmt.Sprintf("/v1/platform/issues/%s/runs/%s", issueID, runID)
			if err := c.RawDelete(ctx, path, nil); err != nil {
				ExitErrorf("unlinking run: %v", err)
			}
			fmt.Printf("Run %s unlinked from issue %s\n", runID, issueID)
		},
	}

	return cmd
}
