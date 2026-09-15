package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"github.com/spf13/cobra"
)

type insightsCreateOptions struct {
	name, model, start, end, filter, summaryPrompt, userContext string
	lastNHours, sample                                          int64
}

func chooseInsightsProvider(cmd *cobra.Command, explicit string) (string, error) {
	if explicit != "" || cmd.Flags().Changed("model") {
		return explicit, nil
	}
	if GetFormat() != "pretty" || !inputIsTerminal(cmd.InOrStdin()) {
		return "", fmt.Errorf("--model is required for non-interactive or JSON output; choose openai or anthropic")
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "Insights uses workspace model credentials and may incur usage; it does not inherit your local Gateway connection.")
	fmt.Fprintln(cmd.ErrOrStderr(), "Supported providers (workspace availability is validated by the service):")
	fmt.Fprintln(cmd.ErrOrStderr(), "  1. OpenAI\n  2. Anthropic")
	fmt.Fprint(cmd.ErrOrStderr(), "Choose provider [1/2, no default]: ")
	scanner := bufio.NewScanner(cmd.InOrStdin())
	scanner.Buffer(make([]byte, 1024), 1024)
	if !scanner.Scan() {
		return "", fmt.Errorf("provider selection cancelled or unreadable; pass --model openai or --model anthropic")
	}
	switch strings.ToLower(strings.TrimSpace(scanner.Text())) {
	case "1", "openai":
		return "openai", nil
	case "2", "anthropic":
		return "anthropic", nil
	default:
		return "", fmt.Errorf("invalid provider selection; choose 1/openai or 2/anthropic")
	}
}

func insightsCreateParams(o insightsCreateOptions) (langsmith.SessionInsightNewParams, error) {
	request := langsmith.CreateRunClusteringJobRequestParam{}
	fail := func(message string) (langsmith.SessionInsightNewParams, error) {
		return langsmith.SessionInsightNewParams{}, fmt.Errorf("%s", message)
	}
	if o.model != "openai" && o.model != "anthropic" {
		return fail("--model must be openai or anthropic")
	}
	if o.sample <= 0 {
		return fail("--sample must be a positive number of traces")
	}
	if o.lastNHours < 0 || (o.start == "" && o.lastNHours == 0) {
		return fail("specify --start-time or a positive --last-n-hours")
	}
	if o.start != "" && o.lastNHours != 0 {
		return fail("--start-time and --last-n-hours are mutually exclusive")
	}
	if o.end != "" && o.start == "" {
		return fail("--end-time requires --start-time")
	}
	if o.start != "" {
		start, err := time.Parse(time.RFC3339Nano, o.start)
		if err != nil {
			return fail("--start-time must be an RFC3339 timestamp with timezone")
		}
		request.StartTime = langsmith.F(start)
		if o.end != "" {
			end, err := time.Parse(time.RFC3339Nano, o.end)
			if err != nil {
				return fail("--end-time must be an RFC3339 timestamp with timezone")
			}
			if !end.After(start) {
				return fail("--end-time must be after --start-time")
			}
			request.EndTime = langsmith.F(end)
		}
	} else {
		request.LastNHours = langsmith.F(o.lastNHours)
	}
	request.Model = langsmith.F(langsmith.CreateRunClusteringJobRequestModel(o.model))
	request.Sample = langsmith.F(float64(o.sample))
	if o.name != "" {
		if strings.TrimSpace(o.name) == "" {
			return fail("--name must not be blank")
		}
		request.Name = langsmith.F(o.name)
	}
	if o.filter != "" {
		request.Filter = langsmith.F(o.filter)
	}
	if o.summaryPrompt != "" {
		request.SummaryPrompt = langsmith.F(o.summaryPrompt)
	}
	if o.userContext != "" {
		var context map[string]string
		if err := json.Unmarshal([]byte(o.userContext), &context); err != nil {
			return fail("--user-context must be a JSON object of question-to-answer strings")
		}
		hasAnswer := false
		for _, answer := range context {
			hasAnswer = hasAnswer || strings.TrimSpace(answer) != ""
		}
		if !hasAnswer {
			return fail("--user-context must contain at least one non-empty answer")
		}
		request.UserContext = langsmith.F(context)
	}
	return langsmith.SessionInsightNewParams{CreateRunClusteringJobRequest: request}, nil
}

func newInsightsCreateCmd() *cobra.Command {
	var project, projectID string
	var options insightsCreateOptions
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Start a one-off Insights analysis of selected project traces",
		Long: `Start a one-off Insights job. This can incur model usage in the workspace.

Choose the provider, sample size, and time window explicitly. The provider uses
the workspace's server-side model configuration; it does not inherit a local
LLM Gateway connection. Model-secret validation remains enabled on the service.
If --model is omitted in an interactive terminal with pretty output, a selection
prompt is shown. Scripts and JSON output require --model and never prompt.

The sample is a trace count, not a percentage or cost budget. Service limits apply.
Without --filter the service selects root runs. --user-context accepts a JSON
object mapping business questions to answers. Creation does not schedule recurrence.

The response contains the actual job ID and status, not an assertion of completion.
Use 'langsmith insights get <id> --project-id <project-id>' to inspect the report.`,
		Example: `  langsmith insights create --project-id <uuid> --last-n-hours 24 --sample 20 --model openai
  langsmith insights create --project my-app --start-time 2026-09-01T00:00:00Z --end-time 2026-09-02T00:00:00Z --sample 100 --model anthropic --user-context '{"Business goal":"Resolve eligible refund requests"}'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			model, err := chooseInsightsProvider(cmd, options.model)
			if err != nil {
				return err
			}
			options.model = model
			params, err := insightsCreateParams(options)
			if err != nil {
				return err
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			id, err := resolveSessionID(cmd.Context(), c, project, projectID, "insights create")
			if err != nil {
				return err
			}
			// A retried POST could create a second paid job after an ambiguous response.
			job, err := c.SDK.Sessions.Insights.New(cmd.Context(), id, params, option.WithMaxRetries(0))
			if err != nil {
				return fmt.Errorf("creating Insights job: %w", err)
			}
			return output.OutputJSON(map[string]any{
				"id": job.ID, "name": job.Name, "status": job.Status,
				"error": nilStr(job.Error), "project_id": id,
			}, "")
		},
	}
	addProjectFlags(cmd, &project, &projectID)
	cmd.Flags().StringVar(&options.name, "name", "", "Optional report name")
	cmd.Flags().StringVar(&options.model, "model", "", "Workspace provider: openai or anthropic; prompts only in interactive pretty mode")
	cmd.Flags().Int64Var(&options.sample, "sample", 0, "Number of traces to sample (required, positive)")
	cmd.Flags().Int64Var(&options.lastNHours, "last-n-hours", 0, "Look back this many hours; cannot combine with --start-time")
	cmd.Flags().StringVar(&options.start, "start-time", "", "Start timestamp in RFC3339 format")
	cmd.Flags().StringVar(&options.end, "end-time", "", "End timestamp in RFC3339 format; requires --start-time")
	cmd.Flags().StringVar(&options.filter, "filter", "", "LangSmith run filter DSL; defaults to root runs on the service")
	cmd.Flags().StringVar(&options.summaryPrompt, "summary-prompt", "", "Instructions for the report summary")
	cmd.Flags().StringVar(&options.userContext, "user-context", "", "JSON object mapping business questions to answers")
	_ = cmd.MarkFlagRequired("sample")
	cmd.MarkFlagsMutuallyExclusive("start-time", "last-n-hours")
	return cmd
}
