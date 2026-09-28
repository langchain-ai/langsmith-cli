package cmd

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	"github.com/langchain-ai/langsmith-cli/internal/langgraphapi"
	"github.com/langchain-ai/langsmith-cli/internal/structured"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	deployLogLevels          = []string{"DEBUG", "INFO", "WARNING", "ERROR", "CRITICAL"}
	deployLogsFollowInterval = 2 * time.Second
)

type deployListInput struct {
	NameContains     string
	AgentID          string
	AgentEnvironment string
	Limit            int
}

var deployListCommand = structured.Command[*deployListInput]{
	Use:   "list",
	Short: "List deployments",
	Long: `List LangSmith Deployments in the current workspace.

Examples:
  langsmith deploy list
  langsmith deploy list --name-contains agent
  langsmith deploy list --format json`,
	Args: cobra.NoArgs,
	Input: func(cmd *cobra.Command) *deployListInput {
		in := &deployListInput{Limit: langgraphapi.MaxPageSize}
		cmd.Flags().StringVar(&in.NameContains, "name-contains", "", "Only show deployments whose names contain this value")
		cmd.Flags().StringVar(&in.AgentID, "agent-id", "", "Only show deployments of this logical agent, private beta [env: LANGSMITH_AGENT_ID]")
		cmd.Flags().StringVar(&in.AgentEnvironment, "agent-environment", "", "Only show deployments in this agent environment, private beta [env: LANGSMITH_AGENT_ENVIRONMENT]")
		cmd.Flags().IntVar(&in.Limit, "limit", in.Limit, "Maximum number of deployments to return")
		return in
	},
	Action: func(ctx context.Context, cmd *cobra.Command, in *deployListInput, args []string) (any, error) {
		if in.Limit < 1 {
			return nil, errors.New("--limit must be at least 1")
		}
		filter := langgraphapi.DeploymentFilter{
			NameContains:     in.NameContains,
			AgentID:          envOrFlag(cmd, "agent-id", in.AgentID, "LANGSMITH_AGENT_ID"),
			AgentEnvironment: envOrFlag(cmd, "agent-environment", in.AgentEnvironment, "LANGSMITH_AGENT_ENVIRONMENT"),
		}
		if cmd.Flags().Changed("agent-id") && strings.TrimSpace(filter.AgentID) == "" {
			return nil, errors.New("--agent-id must not be empty")
		}
		if filter.AgentEnvironment != "" && !slices.Contains(deployAgentEnvs, filter.AgentEnvironment) {
			return nil, fmt.Errorf("--agent-environment must be one of %s (got %q)", strings.Join(deployAgentEnvs, ", "), filter.AgentEnvironment)
		}
		hc, err := newLangGraphAPIClient(cmd)
		if err != nil {
			return nil, err
		}
		deployments := []langgraphapi.Deployment{}
		for len(deployments) < in.Limit {
			filter.Limit = min(in.Limit-len(deployments), langgraphapi.MaxPageSize)
			filter.Offset = len(deployments)
			page, err := hc.ListDeployments(ctx, filter)
			if err != nil {
				return nil, fmt.Errorf("listing deployments: %w", err)
			}
			deployments = append(deployments, page...)
			if len(page) < filter.Limit {
				break
			}
		}
		return deployments, nil
	},
	Render: structured.Table{
		Title: "Deployments",
		Rows:  ".",
		Columns: []structured.Column{
			{Header: "ID", Template: "{{.ID}}"},
			{Header: "Name", Template: "{{dash .Name}}"},
			{Header: "URL", Template: "{{dash .SourceConfig.CustomURL}}"},
		},
	},
}

var deployRevisionsCommand = structured.Parent{
	Use:   "revisions",
	Short: "Manage deployment revisions",
	Children: []func() *cobra.Command{
		deployRevisionsListCommand.Cobra,
	},
}

type deployRevisionsListInput struct {
	Limit int
}

var deployRevisionsListCommand = structured.Command[*deployRevisionsListInput]{
	Use:   "list <deployment-id>",
	Short: "List revisions of a deployment, newest first",
	Long: `List revisions of a deployment, newest first.

Revisions that were deployed and later superseded show as REPLACED. Use
'langsmith deploy list' to find deployment IDs.

Examples:
  langsmith deploy revisions list <deployment-id>
  langsmith deploy revisions list <deployment-id> --limit 50`,
	Args: cobra.ExactArgs(1),
	Input: func(cmd *cobra.Command) *deployRevisionsListInput {
		in := &deployRevisionsListInput{Limit: 10}
		cmd.Flags().IntVar(&in.Limit, "limit", in.Limit, "Maximum number of revisions to return (max 100)")
		return in
	},
	Action: func(ctx context.Context, cmd *cobra.Command, in *deployRevisionsListInput, args []string) (any, error) {
		if in.Limit < 1 || in.Limit > langgraphapi.MaxPageSize {
			return nil, fmt.Errorf("--limit must be between 1 and %d", langgraphapi.MaxPageSize)
		}
		hc, err := newLangGraphAPIClient(cmd)
		if err != nil {
			return nil, err
		}
		revisions, err := hc.ListRevisions(ctx, args[0], in.Limit)
		if err != nil {
			return nil, fmt.Errorf("listing revisions: %w", err)
		}
		return revisions, nil
	},
	Render: revisionsTable{},
}

type revisionRow struct {
	ID        string
	Status    string
	CreatedAt string
}

// revisionsTable shows superseded DEPLOYED revisions as REPLACED.
type revisionsTable struct{}

func (revisionsTable) RenderText(w io.Writer, model any) error {
	revisions, ok := model.([]langgraphapi.Revision)
	if !ok {
		return fmt.Errorf("unexpected revisions model %T", model)
	}
	rows := make([]revisionRow, 0, len(revisions))
	seenDeployed := false
	for _, rev := range revisions {
		status := rev.Status
		if status == "DEPLOYED" {
			if seenDeployed {
				status = "REPLACED"
			}
			seenDeployed = true
		}
		rows = append(rows, revisionRow{ID: rev.ID, Status: status, CreatedAt: rev.CreatedAt})
	}
	return structured.Table{
		Title: "Revisions",
		Rows:  ".",
		Columns: []structured.Column{
			{Header: "ID", Template: "{{.ID}}"},
			{Header: "Status", Template: "{{dash .Status}}"},
			{Header: "Created", Template: "{{formatTime .CreatedAt}}"},
		},
	}.RenderText(w, rows)
}

type deployDeleteInput struct {
	Yes bool
}

type deployMessage struct {
	DeploymentID string `json:"deployment_id"`
	Message      string `json:"message"`
}

var deployDeleteCommand = structured.Command[*deployDeleteInput]{
	Use:   "delete <deployment-id>",
	Short: "Delete a deployment",
	Long: `Delete a LangSmith Deployment. Use 'langsmith deploy list' to find deployment IDs.

Examples:
  langsmith deploy delete <deployment-id>
  langsmith deploy delete <deployment-id> --yes`,
	Args: cobra.ExactArgs(1),
	Input: func(cmd *cobra.Command) *deployDeleteInput {
		in := &deployDeleteInput{}
		cmd.Flags().BoolVar(&in.Yes, "yes", false, "Skip confirmation prompt")
		cmd.Flags().BoolVar(&in.Yes, "force", false, "Alias for --yes")
		_ = cmd.Flags().MarkHidden("force")
		return in
	},
	Action: func(ctx context.Context, cmd *cobra.Command, in *deployDeleteInput, args []string) (any, error) {
		hc, err := newLangGraphAPIClient(cmd)
		if err != nil {
			return nil, err
		}
		id := args[0]
		if !in.Yes {
			deployment, err := hc.GetDeployment(ctx, id)
			if err != nil {
				return nil, err
			}
			if err := confirmDelete(cmd, deleteConfirmation{
				target:   "deployment " + id,
				identity: "Name: " + deployment.Name,
			}); err != nil {
				return nil, err
			}
		}
		if err := hc.DeleteDeployment(ctx, id); err != nil {
			return nil, fmt.Errorf("deleting deployment: %w", err)
		}
		return deployMessage{DeploymentID: id, Message: fmt.Sprintf("Deleted deployment %s.", id)}, nil
	},
	Render: structured.Template("{{.Message}}\n"),
}

type deployLogsInput struct {
	Name         string
	DeploymentID string
	Type         string
	RevisionID   string
	Level        string
	Limit        int
	Query        string
	StartTime    string
	EndTime      string
	Follow       bool
}

var deployLogsCommand = structured.Command[*deployLogsInput]{
	Use:   "logs",
	Short: "Fetch deployment runtime or build logs",
	Long: `Fetch LangSmith Deployment logs, oldest first.

--type deploy (the default) shows agent server runtime logs; --type build shows
the remote build logs of a revision, the latest one unless --revision-id is set.

The deployment is found by --deployment-id, else by --name, which defaults to
LANGSMITH_DEPLOYMENT_NAME and then to the current directory's name.

With --format json, logs print as a JSON array, or as one JSON object per line
with --follow.

Examples:
  langsmith deploy logs
  langsmith deploy logs --name my-agent --level ERROR --limit 50
  langsmith deploy logs --type build
  langsmith deploy logs --follow`,
	Args: cobra.NoArgs,
	Input: func(cmd *cobra.Command) *deployLogsInput {
		in := &deployLogsInput{Type: "deploy", Limit: 100}
		f := cmd.Flags()
		f.StringVar(&in.Name, "name", "", "Deployment name [env: LANGSMITH_DEPLOYMENT_NAME] (default: current directory name)")
		f.StringVar(&in.DeploymentID, "deployment-id", "", "Deployment ID (instead of --name)")
		f.StringVar(&in.Type, "type", in.Type, "Log stream: deploy (runtime) or build (remote build)")
		f.StringVar(&in.RevisionID, "revision-id", "", "Revision ID; build logs default to the latest revision")
		f.StringVar(&in.Level, "level", "", "Minimum level to show: DEBUG, INFO, WARNING, ERROR, or CRITICAL")
		f.IntVar(&in.Limit, "limit", in.Limit, "Maximum number of log entries to fetch")
		f.StringVarP(&in.Query, "query", "q", "", "Only show entries matching this search string")
		f.StringVar(&in.StartTime, "start-time", "", "ISO 8601 start time (e.g. 2026-03-08T00:00:00Z)")
		f.StringVar(&in.EndTime, "end-time", "", "ISO 8601 end time (e.g. 2026-03-08T00:00:00Z)")
		f.BoolVarP(&in.Follow, "follow", "f", false, "Keep polling for new log entries")
		return in
	},
	CustomOutput: true,
	Action: func(ctx context.Context, cmd *cobra.Command, in *deployLogsInput, args []string) (any, error) {
		return nil, runDeployLogs(ctx, cmd, in)
	},
}

func runDeployLogs(ctx context.Context, cmd *cobra.Command, in *deployLogsInput) error {
	if in.Type != "deploy" && in.Type != "build" {
		return fmt.Errorf("--type must be deploy or build (got %q)", in.Type)
	}
	level := strings.ToUpper(in.Level)
	if level != "" && !slices.Contains(deployLogLevels, level) {
		return fmt.Errorf("--level must be one of %s (got %q)", strings.Join(deployLogLevels, ", "), in.Level)
	}
	if in.Limit < 1 {
		return errors.New("--limit must be at least 1")
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	hc, err := newLangGraphAPIClient(cmd)
	if err != nil {
		return err
	}
	deploymentID := in.DeploymentID
	if deploymentID == "" {
		envVars, err := readDotenv(".env")
		if err != nil {
			return err
		}
		explicit := cmp.Or(envOrFlag(cmd, "name", in.Name, deploymentNameEnv), envVars[deploymentNameEnv])
		name := normalizeDeploymentName(cmp.Or(explicit, cwdDeploymentName()))
		found, err := findDeploymentByName(ctx, hc, name)
		if err != nil {
			return err
		}
		if found == nil && explicit == "" {
			return fmt.Errorf("no deployment named '%s' (the current directory's name); pass --name or --deployment-id, or run 'langsmith deploy list' to see deployments", name)
		}
		if found == nil {
			return fmt.Errorf("deployment '%s' not found; run 'langsmith deploy list' to see deployments", name)
		}
		deploymentID = found.ID
	}

	revisionID := in.RevisionID
	if in.Type == "build" && revisionID == "" {
		revisions, err := hc.ListRevisions(ctx, deploymentID, 1)
		if err != nil {
			return err
		}
		if len(revisions) == 0 {
			return errors.New("no revisions found for this deployment, so there are no build logs")
		}
		revisionID = revisions[0].ID
		fmt.Fprintf(cmd.ErrOrStderr(), "Using latest revision: %s\n", revisionID)
	}

	req := langgraphapi.LogsRequest{
		Limit:     in.Limit,
		Order:     "desc",
		Level:     level,
		Query:     in.Query,
		StartTime: in.StartTime,
		EndTime:   in.EndTime,
	}
	fetch := func(req langgraphapi.LogsRequest) ([]langgraphapi.LogEntry, error) {
		var resp *langgraphapi.LogsResponse
		var err error
		if in.Type == "build" {
			resp, err = hc.BuildLogs(ctx, deploymentID, revisionID, req)
		} else {
			resp, err = hc.DeployLogs(ctx, deploymentID, revisionID, req)
		}
		if err != nil {
			return nil, err
		}
		return resp.Logs, nil
	}

	entries, err := fetch(req)
	if err != nil {
		return err
	}
	slices.Reverse(entries)
	w := cmd.OutOrStdout()
	color := wantsColor(w)
	jsonOutput := cmdutil.ResolveFormat(cmd) != "pretty" || cmdutil.ResolveJQ(cmd) != ""

	if !in.Follow {
		if jsonOutput {
			return structured.Render(cmd, entries, nil)
		}
		if len(entries) == 0 {
			fmt.Fprintln(cmd.ErrOrStderr(), "No log entries found.")
		}
		for _, entry := range entries {
			fmt.Fprintln(w, formatLogEntry(entry, color))
		}
		return nil
	}

	seen := map[string]bool{}
	emit := func(batch []langgraphapi.LogEntry) error {
		for _, entry := range batch {
			if entry.ID != "" {
				if seen[entry.ID] {
					continue
				}
				seen[entry.ID] = true
			}
			if jsonOutput {
				if err := json.NewEncoder(w).Encode(entry); err != nil {
					return err
				}
				continue
			}
			fmt.Fprintln(w, formatLogEntry(entry, color))
		}
		return nil
	}
	if err := emit(entries); err != nil {
		return err
	}
	req.Order = "asc"
	if len(entries) > 0 {
		req.StartTime = logStartTime(entries[len(entries)-1].Timestamp, req.StartTime)
	}
	for {
		timer := time.NewTimer(deployLogsFollowInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
		batch, err := fetch(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if err := emit(batch); err != nil {
			return err
		}
		if len(batch) > 0 {
			req.StartTime = logStartTime(batch[len(batch)-1].Timestamp, req.StartTime)
		}
	}
}

// logStartTime turns an entry's timestamp into the next poll's start_time.
func logStartTime(ts any, fallback string) string {
	switch v := ts.(type) {
	case float64:
		return time.UnixMilli(int64(v)).UTC().Format(time.RFC3339Nano)
	case string:
		return cmp.Or(v, fallback)
	default:
		return fallback
	}
}

func formatLogTimestamp(ts any) string {
	switch v := ts.(type) {
	case float64:
		return time.UnixMilli(int64(v)).UTC().Format(time.DateTime)
	case string:
		return v
	default:
		return ""
	}
}

func formatLogEntry(entry langgraphapi.LogEntry, color bool) string {
	ts := formatLogTimestamp(entry.Timestamp)
	var line string
	switch {
	case ts != "" && entry.Level != "":
		line = fmt.Sprintf("[%s] [%s] %s", ts, entry.Level, entry.Message)
	case ts != "":
		line = fmt.Sprintf("[%s] %s", ts, entry.Message)
	default:
		line = entry.Message
	}
	if code := logLevelColor(entry.Level); color && code != "" {
		return code + line + ansiReset
	}
	return line
}

const (
	ansiRed    = "\x1b[31m"
	ansiYellow = "\x1b[33m"
	ansiReset  = "\x1b[0m"
)

func logLevelColor(level string) string {
	switch strings.ToUpper(level) {
	case "ERROR", "CRITICAL":
		return ansiRed
	case "WARNING":
		return ansiYellow
	default:
		return ""
	}
}

func wantsColor(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && os.Getenv("NO_COLOR") == "" && term.IsTerminal(int(f.Fd()))
}
