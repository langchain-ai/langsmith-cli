package cmd

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"github.com/spf13/cobra"
)

type insightsCreateOptions struct {
	name, model, start, end, filter, summaryPrompt, userContext string
	lastNHours, sample                                          int64
	summaryPromptSet                                            bool
	configID, clusterModel, summaryModel                        string
	categories                                                  map[string]string
	attributes                                                  map[string]insightsAttribute
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
	if o.configID != "" {
		if _, err := uuid.Parse(o.configID); err != nil {
			return langsmith.SessionInsightNewParams{}, fmt.Errorf("--config-id must be a UUID")
		}
		request.ConfigID = langsmith.F(o.configID)
		return langsmith.SessionInsightNewParams{CreateRunClusteringJobRequest: request}, nil
	}
	fail := func(message string) (langsmith.SessionInsightNewParams, error) {
		return langsmith.SessionInsightNewParams{}, fmt.Errorf("%s", message)
	}
	if o.model != "openai" && o.model != "anthropic" {
		return fail("--model must be openai or anthropic")
	}
	if o.sample < 1 || o.sample > 1000 {
		return fail("--sample must be between 1 and 1000 (matching the Insights UI)")
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
	if o.summaryPrompt != "" || o.summaryPromptSet {
		if _, err := insightsPromptVariables(o.summaryPrompt); err != nil {
			return fail(err.Error())
		}
		request.SummaryPrompt = langsmith.F(o.summaryPrompt)
	}
	if o.userContext != "" {
		var context map[string]string
		if err := readInsightsJSON(o.userContext, &context); err != nil {
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
	if err := addInsightsAnalysisParams(o, &request); err != nil {
		return langsmith.SessionInsightNewParams{}, err
	}
	return langsmith.SessionInsightNewParams{CreateRunClusteringJobRequest: request}, nil
}

func newInsightsCreateCmd() *cobra.Command {
	var project, projectID string
	var options insightsCreateOptions
	var file, categoriesFile, attributesFile, outputFile string
	var dryRun bool
	var previewRun string
	var configFile string
	var wait bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Start a one-off Insights analysis of selected project traces",
		Long: `Start a one-off Insights job. This can incur model usage in the workspace.

Choose the provider, sample size, and time window explicitly. The provider uses
the workspace's server-side model configuration; it does not inherit a local
LLM Gateway connection. Model-secret validation remains enabled on the service.
If --model is omitted in an interactive terminal with pretty output, a selection
prompt is shown. Scripts and JSON output require --model and never prompt.

The sample is a requested count from 1 to 1000, not a percentage or cost budget.
The service selects the latest matching root per thread plus unthreaded roots.
Without --filter the service selects root runs. --user-context accepts a JSON
object mapping business questions to answers. Creation does not schedule recurrence.

Use --file for an analysis JSON object with API field names, or --categories and
--attributes for individual JSON files. File input cannot be mixed with analysis
flags. --config-id runs a saved configuration exactly as stored; overrides are
rejected. --dry-run validates the request without creating a job, but cannot
resolve saved configurations or verify model availability and write permission.
With --dry-run --preview-run UUID, inspect variable bindings from one root run.
This does not render a summary or verify that the run will be sampled.
Custom summary prompts summarize each run and must reference {{run.inputs}},
{{run.outputs}}, other simple run paths, or {{all_thread_messages}}.
Omit --summary-prompt to use the service default. Sections/helpers are unsupported.

The response contains the actual job ID and status, not an assertion of completion.
Use 'langsmith insights get <id> --project-id <project-id>' to inspect the report.`,
		Example: `  langsmith insights create --project-id <uuid> --last-n-hours 24 --sample 20 --model openai
  langsmith insights create --project my-app --start-time 2026-09-01T00:00:00Z --end-time 2026-09-02T00:00:00Z --sample 100 --model anthropic --user-context '{"Business goal":"Resolve eligible refund requests"}'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("config") {
				configured := newInsightsConfigCreateCmd()
				for _, name := range []string{"config", "wait", "project", "project-id", "output"} {
					if cmd.Flags().Changed(name) {
						if err := configured.Flags().Set(name, cmd.Flags().Lookup(name).Value.String()); err != nil {
							return err
						}
					}
				}
				configured.SetContext(cmd.Context())
				return configured.RunE(configured, args)
			}
			if wait {
				return commandDiagnostic{"invalid_flag_combination", "--wait requires --config", "Use --config FILE --wait, or poll a one-off report with insights get."}
			}
			options.summaryPromptSet = cmd.Flags().Changed("summary-prompt")
			if cmd.Flags().Changed("preview-run") {
				if !dryRun || options.configID != "" {
					return fmt.Errorf("--preview-run requires --dry-run and cannot use --config-id")
				}
				if _, err := uuid.Parse(previewRun); err != nil {
					return fmt.Errorf("--preview-run must be a UUID")
				}
			}
			for _, name := range []string{"file", "categories", "attributes", "config-id", "cluster-model", "summary-model"} {
				v, _ := cmd.Flags().GetString(name)
				if cmd.Flags().Changed(name) && strings.TrimSpace(v) == "" {
					return fmt.Errorf("--%s must not be blank", name)
				}
			}
			if file != "" {
				var config insightsFileConfig
				if err := readInsightsJSON(file, &config); err != nil {
					return err
				}
				options = config.options()
			}
			if categoriesFile != "" {
				if err := readInsightsJSON(categoriesFile, &options.categories); err != nil {
					return err
				}
			}
			if attributesFile != "" {
				if err := readInsightsJSON(attributesFile, &options.attributes); err != nil {
					return err
				}
			}
			if options.configID == "" {
				if dryRun && options.model == "" {
					return fmt.Errorf("--model is required for a non-interactive dry run")
				}
				model, err := chooseInsightsProvider(cmd, options.model)
				if err != nil {
					return err
				}
				options.model = model
			}
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
			if dryRun {
				result := map[string]any{"status": "dry_run", "project_id": id,
					"next_steps":   []string{"Review the request and optional trace preview. Remove --dry-run only after approving model usage; the workspace needs server-side model credentials."},
					"workspace_id": nilStr(GetWorkspaceID()), "request": params,
					"saved_config_resolved": false, "authorization_validated": false,
					"note": "No job created. Saved configuration, provider availability, service limits and write permission are not validated."}
				if previewRun != "" {
					preview, err := previewInsightsRun(cmd.Context(), c, id, previewRun, options.summaryPrompt)
					if err != nil {
						return err
					}
					result["preview"] = preview
				}
				return output.OutputJSON(result, outputFile)
			}
			// A retried POST could create a second paid job after an ambiguous response.
			job, err := c.SDK.Sessions.Insights.New(cmd.Context(), id, params, option.WithMaxRetries(0))
			if err != nil {
				return commandDiagnostic{"insights_creation_unverified", "creating Insights job: no confirmed job response", "Check insights list for this project before retrying; the paid job may already exist. Verify workspace permissions and model configuration. No automatic creation retry was performed."}
			}
			check := readNextStep("insights", "get", job.ID, "--project-id", id)
			evidence := readNextStep("insights", "runs", job.ID, "--project-id", id)
			message := "Report submitted. Check status before reading results."
			switch string(job.Status) {
			case "queued", "pending":
				message = "Report queued. Results are not ready yet."
			case "running":
				message = "Report running. Results are not ready yet."
			case "success":
				message = "Report complete. Results are ready."
			case "error", "failed":
				message = "Report failed. Inspect the report before retrying."
			}
			if GetFormat() == "pretty" && outputFile == "" {
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\n\nName: %s\nReport ID: %s\nProject ID: %s\nStatus: %s\n\nCheck report:\n  %s\n\nAfter completion, inspect evidence:\n  %s\n", message, job.Name, job.ID, id, job.Status, check, evidence)
				return err
			}
			return output.OutputJSON(map[string]any{
				"message":    message,
				"next_steps": []string{check},
				"id":         job.ID, "name": job.Name, "status": job.Status,
				"error": nilStr(job.Error), "project_id": id, "workspace_id": nilStr(GetWorkspaceID()),
			}, outputFile)
		},
	}
	addProjectFlags(cmd, &project, &projectID)
	cmd.Flags().StringVar(&configFile, "config", "", "Save and start a manual report: inline JSON, file.json, @file.json, or - for stdin")
	cmd.Flags().BoolVar(&wait, "wait", false, "With --config, wait for the report to succeed or fail")
	cmd.Flags().StringVar(&options.name, "name", "", "Optional report name")
	cmd.Flags().StringVar(&options.model, "model", "", "Workspace provider: openai or anthropic; prompts only in interactive pretty mode")
	cmd.Flags().Int64Var(&options.sample, "sample", 0, "Requested sample count (1–1000; required unless using --file or --config-id)")
	cmd.Flags().Int64Var(&options.lastNHours, "last-n-hours", 0, "Look back this many hours; cannot combine with --start-time")
	cmd.Flags().StringVar(&options.start, "start-time", "", "Start timestamp in RFC3339 format")
	cmd.Flags().StringVar(&options.end, "end-time", "", "End timestamp in RFC3339 format; requires --start-time")
	cmd.Flags().StringVar(&options.filter, "filter", "", "LangSmith run filter DSL; defaults to root runs on the service")
	cmd.Flags().StringVar(&options.summaryPrompt, "summary-prompt", "", "Per-run summary template with trace variables; omitted uses service default")
	cmd.Flags().StringVar(&options.userContext, "user-context", "", "Business questions and answers: inline JSON object, file.json, or @file.json")
	cmd.Flags().StringVarP(&file, "file", "f", "", "Analysis: inline JSON, file.json, or @file.json; cannot combine with analysis flags or --config-id")
	cmd.Flags().StringVar(&categoriesFile, "categories", "", "Category names and descriptions (1-10): inline JSON, file.json, or @file.json")
	cmd.Flags().StringVar(&attributesFile, "attributes", "", "Named string/number/boolean attributes: inline JSON, file.json, or @file.json")
	cmd.Flags().StringVar(&options.configID, "config-id", "", "Run a saved configuration UUID without overrides")
	cmd.Flags().StringVar(&options.clusterModel, "cluster-model", "", "Clustering provider or workspace model-settings UUID")
	cmd.Flags().StringVar(&options.summaryModel, "summary-model", "", "Summarization provider or workspace model-settings UUID")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate and show the request without creating a job (may read project metadata)")
	cmd.Flags().StringVar(&previewRun, "preview-run", "", "With --dry-run, inspect prompt variable bindings against this root run UUID")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	cmd.MarkFlagsMutuallyExclusive("file", "config-id")
	for _, name := range []string{"file", "config-id", "dry-run", "preview-run"} {
		cmd.MarkFlagsMutuallyExclusive("config", name)
	}
	for _, name := range []string{"name", "model", "sample", "last-n-hours", "start-time", "end-time", "filter", "summary-prompt", "user-context", "categories", "attributes", "cluster-model", "summary-model"} {
		cmd.MarkFlagsMutuallyExclusive("file", name)
		cmd.MarkFlagsMutuallyExclusive("config", name)
		cmd.MarkFlagsMutuallyExclusive("config-id", name)
	}
	cmd.MarkFlagsMutuallyExclusive("start-time", "last-n-hours")
	return cmd
}
