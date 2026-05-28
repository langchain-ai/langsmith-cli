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
		Use:   "issues",
		Short: "Manage issues for a tracing project",
		Long: `Manage Issues Board issues for a tracing project.

Examples:
  langsmith project issues list --project my-app
  langsmith project issues list --project my-app --status open --priority high
  langsmith project issues events --project my-app`,
	}

	cmd.AddCommand(newProjectIssuesListCmd())
	cmd.AddCommand(newProjectIssuesEventsCmd())
	cmd.AddCommand(newProjectIssuesUpdateCmd())
	cmd.AddCommand(newProjectIssuesRunsCmd())
	cmd.AddCommand(newProjectIssuesExamplesCmd())
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
		Short: "List issues for a tracing project",
		Long: `List forge issues associated with a tracing project.

Fetches issues from the Issues Board for the specified project. Results
can be filtered by status (open/closed) and priority (high/medium/low).
Output is JSON by default; pass --format pretty for a human-readable table.

Examples:
  langsmith project issues list --project my-app
  langsmith project issues list --project my-app --status open
  langsmith project issues list --project my-app --priority high --limit 10
  langsmith project issues list --project my-app --format pretty`,
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
				data := []map[string]any{}
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
		Short: "List issue events for a tracing project",
		Long: `List issue events for the Issues Board of a tracing project.

Issue events record user and agent actions on issues: status changes, severity
edits, evaluator deployments, and issue creation. The ABM agent reads these on
cron runs to update the User Preferences section of the Agent Overview.

Output is JSON by default; pass --format pretty for a human-readable table.

Examples:
  langsmith project issues events --project my-app
  langsmith project issues events --project my-app --look-back-minutes 1440
  langsmith project issues events --project my-app --limit 50 --format pretty`,
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
				columns := []string{"EVENT TYPE", "TO", "REASON", "ACTOR", "ISSUE ID", "CREATED"}
				var rows [][]string
				for _, e := range events {
					issueRef := ""
					if e.IssueID != nil {
						issueRef = (*e.IssueID)[:8]
					}
					p := parseEventPayload(e.Payload)
					rows = append(rows, []string{
						e.EventType,
						p.to,
						truncate(p.reason, 60),
						e.Actor,
						issueRef,
						formatIssueTime(e.CreatedAt),
					})
				}
				output.OutputTable(columns, rows, fmt.Sprintf("Issue events for %s", projectName))
			} else {
				data := []map[string]any{}
				for _, e := range events {
					issueID := any(nil)
					if e.IssueID != nil {
						issueID = *e.IssueID
					}
					p := parseEventPayload(e.Payload)
					m := map[string]any{
						"id":         e.ID,
						"session_id": e.SessionID,
						"issue_id":   issueID,
						"event_type": e.EventType,
						"actor":      e.Actor,
						"created_at": formatTimeISO(e.CreatedAt),
					}
					if p.to != "" {
						m["to"] = p.to
					}
					if p.reason != "" {
						m["reason"] = p.reason
					}
					data = append(data, m)
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
		name        string
		description string
		proposedFix string
		evaluator   string
		status      string
		outputFile  string
	)

	cmd := &cobra.Command{
		Use:   "update <issue-id>",
		Short: "Update an existing issue's name, description, proposed fix, evaluator, or status",
		Long: `Update an existing issue.

To link runs as evidence, use 'langsmith project issues runs add' instead.

The issue ID is the UUID returned by 'langsmith project issues list'.

--name and --description are for factual corrections only (when new evidence disproves the original finding).
--proposed-fix updates the suggested code fix shown to users.
--evaluator replaces the suggested evaluator. Pass the evaluator config as JSON — the CLI wraps it automatically.
--status reopens a resolved issue. The only accepted value is 'open' — closing an issue is a human action done via the UI.

Examples:
  langsmith project issues update <id> --name "Corrected name" --description "New finding..."
  langsmith project issues update <id> --status open
  langsmith project issues update <id> --proposed-fix "Root cause: missing null check.\n\` + "`" + `` + "`" + `diff\n-if result:\n+if result is not None:\n` + "`" + `` + "`" + `"
  langsmith project issues update <id> --evaluator '{"type":"llm","display_name":"no_hallucination","prompt":[["system","Evaluate whether the response contains hallucinated facts. Score 1 if grounded, 0 if not."],["user","Evaluate and score."]],"schema":{"type":"object","properties":{"score":{"type":"integer","minimum":0,"maximum":1},"reasoning":{"type":"string"}},"required":["score","reasoning"]}}'
  langsmith project issues update <id> --evaluator '{"type":"code","display_name":"no_tool_errors","code_evaluators":[{"code":"def perform_eval(run, example=None):\n    out = str((run.outputs or {}).get(\"output\",\"\")).lower()\n    return {\"score\": 0 if \"error\" in out else 1, \"key\": \"no_tool_errors\"}","language":"python"}]}'`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			issueID := args[0]
			if name == "" && description == "" && proposedFix == "" && evaluator == "" && status == "" {
				ExitError("at least one of --name, --description, --proposed-fix, --evaluator, or --status is required")
			}
			if status != "" && status != "open" {
				ExitError("--status only accepts 'open' — closing an issue is done via the UI")
			}

			c := MustGetClient()
			ctx := context.Background()

			body := map[string]any{}
			if name != "" {
				body["name"] = name
			}
			if description != "" {
				body["description"] = description
			}
			if proposedFix != "" {
				body["proposed_fix"] = proposedFix
			}
			if status != "" {
				body["status"] = status
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

	cmd.Flags().StringVar(&name, "name", "", "Corrected name (use only when original is factually wrong)")
	cmd.Flags().StringVar(&description, "description", "", "Corrected description (use only when original is factually wrong)")
	cmd.Flags().StringVar(&proposedFix, "proposed-fix", "", "Updated proposed fix (markdown with code diff)")
	cmd.Flags().StringVar(&status, "status", "", "Reopen a resolved issue. Only 'open' is accepted — closing is done via the UI")
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
	m := map[string]any{
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
	// Include actions (evaluator spec) and traces so callers can read the
	// existing evaluator and run it against new evidence traces.
	if len(issue.Actions) > 0 {
		var actions any
		if err := json.Unmarshal(issue.Actions, &actions); err == nil {
			m["actions"] = actions
		}
	}
	if len(issue.Traces) > 0 {
		var traces any
		if err := json.Unmarshal(issue.Traces, &traces); err == nil {
			m["traces"] = traces
		}
	}
	return m
}

// eventPayloadFields holds the fields we extract from an event payload.
type eventPayloadFields struct {
	to     string // new status or severity value
	reason string // user-provided reason for the change
}

// parseEventPayload decodes the raw event payload and extracts "to" and "reason"
// so callers don't have to parse raw JSON.
func parseEventPayload(raw json.RawMessage) eventPayloadFields {
	if len(raw) == 0 {
		return eventPayloadFields{}
	}
	var p map[string]any
	if err := json.Unmarshal(raw, &p); err != nil {
		return eventPayloadFields{}
	}
	var result eventPayloadFields
	if v, ok := p["to"]; ok {
		result.to = fmt.Sprintf("%v", v)
	}
	if v, ok := p["reason"]; ok {
		if s, ok := v.(string); ok {
			result.reason = s
		}
	}
	return result
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
		Use:   "runs",
		Short: "Manage linked runs for an issue",
		Long: `Link and unlink runs to/from an issue.

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

func newProjectIssuesExamplesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "examples",
		Short: "Manage regression examples for an issue",
		Long: `Propose regression examples for an issue.

Examples:
  langsmith project issues examples propose <issue-id> --run-id <run-id> --assertion correctness="Response must be factually correct"
  langsmith project issues examples propose <issue-id> --run-id <run-id> --assertion correctness="Must be correct" --assertion format="Output must be JSON"`,
	}

	cmd.AddCommand(newProjectIssuesProposeExampleCmd())
	return cmd
}

// exampleAssertion is a single key=comment assertion for a proposed regression example.
type exampleAssertion struct {
	Key     string `json:"key"`
	Comment string `json:"comment"`
}

// parseAssertion parses a "key=comment" string into an exampleAssertion.
// The first '=' is the delimiter; the comment may contain '=' characters.
func parseAssertion(s string) (exampleAssertion, error) {
	idx := strings.IndexByte(s, '=')
	if idx < 1 {
		return exampleAssertion{}, fmt.Errorf("assertion %q must be in key=comment format", s)
	}
	key := strings.TrimSpace(s[:idx])
	comment := strings.TrimSpace(s[idx+1:])
	if key == "" {
		return exampleAssertion{}, fmt.Errorf("assertion key cannot be empty in %q", s)
	}
	if comment == "" {
		return exampleAssertion{}, fmt.Errorf("assertion comment cannot be empty in %q", s)
	}
	return exampleAssertion{Key: key, Comment: comment}, nil
}

func newProjectIssuesProposeExampleCmd() *cobra.Command {
	var (
		runID      string
		assertions []string
		outputFile string
	)

	cmd := &cobra.Command{
		Use:   "propose <issue-id>",
		Short: "Propose a regression example for an issue",
		Long: `Propose a run as a regression example for an issue, with assertions that
the run should satisfy. The issues agent uses these to generate evaluators
and test cases for the issue.

Each --assertion flag takes a key=comment pair. The key is a short identifier
for the assertion (e.g. "correctness"), and the comment describes what the run
should demonstrate. You may specify up to 10 assertions; keys must be unique.

Examples:
  langsmith project issues examples propose <issue-id> \
    --run-id <run-id> \
    --assertion correctness="Response must be factually correct"

  langsmith project issues examples propose <issue-id> \
    --run-id <run-id> \
    --assertion correctness="Must cite sources" \
    --assertion format="Output must be valid JSON"`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			issueID := args[0]
			if runID == "" {
				ExitError("--run-id is required")
			}
			if len(assertions) == 0 {
				ExitError("at least one --assertion is required")
			}
			if len(assertions) > 10 {
				ExitError("at most 10 assertions are allowed")
			}

			parsed := make([]exampleAssertion, 0, len(assertions))
			seen := make(map[string]bool, len(assertions))
			for _, raw := range assertions {
				a, err := parseAssertion(raw)
				if err != nil {
					ExitErrorf("invalid assertion: %v", err)
				}
				if seen[a.Key] {
					ExitErrorf("duplicate assertion key %q", a.Key)
				}
				seen[a.Key] = true
				parsed = append(parsed, a)
			}

			c := MustGetClient()
			ctx := context.Background()

			body := map[string]any{
				"run_id":     runID,
				"assertions": parsed,
			}

			path := fmt.Sprintf("/v1/platform/issues/%s/proposed-examples", issueID)
			var result map[string]any
			if err := c.RawPost(ctx, path, body, &result); err != nil {
				ExitErrorf("proposing example: %v", err)
			}

			output.OutputJSON(result, outputFile)
		},
	}

	cmd.Flags().StringVar(&runID, "run-id", "", "Run ID to propose as a regression example (required)")
	cmd.Flags().StringArrayVar(&assertions, "assertion", nil, `Assertion in key=comment format. May be repeated up to 10 times.`)
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	return cmd
}
