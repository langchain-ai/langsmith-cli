// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/autocomplete"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
	docs "github.com/urfave/cli-docs/v3"
	"github.com/urfave/cli/v3"
)

var (
	Command            *cli.Command
	CommandErrorBuffer bytes.Buffer
)

func init() {
	Command = &cli.Command{
		Name:      "langsmith",
		Usage:     "CLI for the langChain API",
		Suggest:   true,
		Version:   Version,
		ErrWriter: &CommandErrorBuffer,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "Enable debug logging",
			},
			&cli.StringFlag{
				Name:        "base-url",
				DefaultText: "url",
				Usage:       "Override the base URL for API requests",
				Validator: func(baseURL string) error {
					return ValidateBaseURL(baseURL, "--base-url")
				},
			},
			&cli.StringFlag{
				Name:  "format",
				Usage: "The format for displaying response data (one of: " + strings.Join(OutputFormats, ", ") + ")",
				Value: "auto",
				Validator: func(format string) error {
					if !slices.Contains(OutputFormats, strings.ToLower(format)) {
						return fmt.Errorf("format must be one of: %s", strings.Join(OutputFormats, ", "))
					}
					return nil
				},
			},
			&cli.StringFlag{
				Name:  "format-error",
				Usage: "The format for displaying error data (one of: " + strings.Join(OutputFormats, ", ") + ")",
				Value: "auto",
				Validator: func(format string) error {
					if !slices.Contains(OutputFormats, strings.ToLower(format)) {
						return fmt.Errorf("format must be one of: %s", strings.Join(OutputFormats, ", "))
					}
					return nil
				},
			},
			&cli.StringFlag{
				Name:  "transform",
				Usage: "The GJSON transformation for data output.",
			},
			&cli.StringFlag{
				Name:  "transform-error",
				Usage: "The GJSON transformation for errors.",
			},
			&cli.BoolFlag{
				Name:    "raw-output",
				Aliases: []string{"r"},
				Usage:   "If the result is a string, print it without JSON quotes. This can be useful for making output transforms talk to non-JSON-based systems.",
			},
			&requestflag.Flag[string]{
				Name:    "api-key",
				Sources: cli.EnvVars("LANGSMITH_API_KEY"),
			},
			&requestflag.Flag[string]{
				Name:    "tenant-id",
				Sources: cli.EnvVars("LANGSMITH_TENANT_ID"),
			},
		},
		Commands: []*cli.Command{
			{
				Name:     "product-feedback",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&productFeedbackCreate,
					&productFeedbackRetrieve,
				},
			},
			{
				Name:     "fleet:threads",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&fleetThreadsActivateSandbox,
				},
			},
			{
				Name:     "sessions",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&sessionsCreate,
					&sessionsRetrieve,
					&sessionsUpdate,
					&sessionsList,
					&sessionsDelete,
					&sessionsDashboard,
				},
			},
			{
				Name:     "sessions:insights",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&sessionsInsightsCreate,
					&sessionsInsightsUpdate,
					&sessionsInsightsList,
					&sessionsInsightsDelete,
					&sessionsInsightsRetrieveJob,
					&sessionsInsightsRetrieveRuns,
				},
			},
			{
				Name:     "sessions:insights:configs",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&sessionsInsightsConfigsCreate,
					&sessionsInsightsConfigsUpdate,
					&sessionsInsightsConfigsList,
					&sessionsInsightsConfigsDelete,
				},
			},
			{
				Name:     "examples",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&examplesCreate,
					&examplesRetrieve,
					&examplesUpdate,
					&examplesList,
					&examplesDelete,
					&examplesDeleteAll,
					&examplesRetrieveCount,
					&examplesUploadFromCsv,
				},
			},
			{
				Name:     "examples:bulk",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&examplesBulkCreate,
					&examplesBulkPatchAll,
				},
			},
			{
				Name:     "examples:validate",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&examplesValidateCreate,
					&examplesValidateBulk,
				},
			},
			{
				Name:     "datasets",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&datasetsCreate,
					&datasetsRetrieve,
					&datasetsUpdate,
					&datasetsList,
					&datasetsDelete,
					&datasetsClone,
					&datasetsRetrieveCsv,
					&datasetsRetrieveJSONL,
					&datasetsRetrieveOpenAI,
					&datasetsRetrieveOpenAIFt,
					&datasetsRetrieveVersion,
					&datasetsUpdateTags,
					&datasetsUpload,
				},
			},
			{
				Name:     "datasets:examples",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&datasetsExamplesDelete,
				},
			},
			{
				Name:     "datasets:versions",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&datasetsVersionsList,
					&datasetsVersionsRetrieveDiff,
				},
			},
			{
				Name:     "datasets:runs",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&datasetsRunsQuery,
				},
			},
			{
				Name:     "datasets:experiment-runs",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&datasetsExperimentRunsQuery,
				},
			},
			{
				Name:     "datasets:share",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&datasetsShareCreate,
					&datasetsShareRetrieve,
					&datasetsShareDeleteAll,
				},
			},
			{
				Name:     "datasets:comparative",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&datasetsComparativeCreate,
					&datasetsComparativeDelete,
				},
			},
			{
				Name:     "datasets:splits",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&datasetsSplitsCreate,
					&datasetsSplitsRetrieve,
				},
			},
			{
				Name:     "runs",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&runsCreate,
					&runsUpdate,
					&runsGetURL,
					&runsIngestBatch,
					&runsQueryV1,
					&runsQueryV2,
					&runsRetrieveV1,
					&runsRetrieveV2,
					&runsStats,
					&runsUpdate2,
				},
			},
			{
				Name:     "runs:share",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&runsShareCreate,
					&runsShareDelete,
				},
			},
			{
				Name:     "threads",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&threadsAggregateStats,
					&threadsListTraces,
					&threadsQuery,
					&threadsStats,
				},
			},
			{
				Name:     "threads:share",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&threadsShareCreate,
					&threadsShareRetrieve,
					&threadsShareDelete,
				},
			},
			{
				Name:     "charts",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&chartsPreview,
				},
			},
			{
				Name:     "traces",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&tracesListRuns,
					&tracesQuery,
				},
			},
			{
				Name:     "evaluators",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&evaluatorsList,
				},
			},
			{
				Name:     "online-evaluators",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&onlineEvaluatorsCreate,
					&onlineEvaluatorsRetrieve,
					&onlineEvaluatorsUpdate,
					&onlineEvaluatorsList,
					&onlineEvaluatorsDelete,
					&onlineEvaluatorsBulkDelete,
					&onlineEvaluatorsSpend,
				},
			},
			{
				Name:     "feedback",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&feedbackCreate,
					&feedbackRetrieve,
					&feedbackUpdate,
					&feedbackList,
					&feedbackDelete,
				},
			},
			{
				Name:     "feedback:tokens",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&feedbackTokensCreate,
					&feedbackTokensRetrieve,
					&feedbackTokensUpdate,
					&feedbackTokensList,
				},
			},
			{
				Name:     "feedback:configs",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&feedbackConfigsDelete,
				},
			},
			{
				Name:     "public",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&publicRetrieveFeedbacks,
				},
			},
			{
				Name:     "public:datasets",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&publicDatasetsList,
					&publicDatasetsListComparative,
					&publicDatasetsListFeedback,
					&publicDatasetsListSessions,
					&publicDatasetsRetrieveSessionsBulk,
				},
			},
			{
				Name:     "public:runs",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&publicRunsRetrieve,
					&publicRunsQuery,
				},
			},
			{
				Name:     "annotation-queues",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&annotationQueuesRetrieve,
					&annotationQueuesUpdate,
					&annotationQueuesDelete,
					&annotationQueuesAnnotationQueues,
					&annotationQueuesCreateRunStatus,
					&annotationQueuesExport,
					&annotationQueuesPopulate,
					&annotationQueuesRetrieveAnnotationQueues,
					&annotationQueuesRetrieveQueues,
					&annotationQueuesRetrieveRun,
					&annotationQueuesRetrieveSize,
					&annotationQueuesRetrieveTotalArchived,
					&annotationQueuesRetrieveTotalSize,
				},
			},
			{
				Name:     "annotation-queues:runs",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&annotationQueuesRunsCreate,
					&annotationQueuesRunsUpdate,
					&annotationQueuesRunsList,
					&annotationQueuesRunsCreateByKey,
					&annotationQueuesRunsDeleteAll,
					&annotationQueuesRunsDeleteQueue,
				},
			},
			{
				Name:     "annotation-queues:items",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&annotationQueuesItemsCreate,
					&annotationQueuesItemsUpdate,
					&annotationQueuesItemsList,
					&annotationQueuesItemsCreateStatus,
					&annotationQueuesItemsDeleteAll,
					&annotationQueuesItemsRetrieveCount,
					&annotationQueuesItemsRetrievePlacement,
				},
			},
			{
				Name:     "prompt-webhooks",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&promptWebhooksCreate,
					&promptWebhooksRetrieve,
					&promptWebhooksUpdate,
					&promptWebhooksList,
					&promptWebhooksDelete,
					&promptWebhooksTest,
				},
			},
			{
				Name:     "info",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&infoList,
				},
			},
			{
				Name:     "workspaces",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&workspacesCreate,
					&workspacesRetrieve,
					&workspacesUpdate,
					&workspacesList,
					&workspacesDelete,
				},
			},
			{
				Name:     "repos",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&reposCreate,
					&reposRetrieve,
					&reposUpdate,
					&reposList,
					&reposDelete,
				},
			},
			{
				Name:     "repos:directories",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&reposDirectoriesList,
					&reposDirectoriesDelete,
					&reposDirectoriesCommit,
				},
			},
			{
				Name:     "commits",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&commitsCreate,
					&commitsRetrieve,
					&commitsList,
				},
			},
			{
				Name:     "settings",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&settingsList,
				},
			},
			{
				Name:     "issues",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&issuesRetrieve,
					&issuesList,
				},
			},
			{
				Name:     "sandboxes",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&sandboxesListUsageCosts,
				},
			},
			{
				Name:     "sandboxes:boxes",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&sandboxesBoxesCreate,
					&sandboxesBoxesRetrieve,
					&sandboxesBoxesUpdate,
					&sandboxesBoxesList,
					&sandboxesBoxesDelete,
					&sandboxesBoxesCreateSnapshot,
					&sandboxesBoxesDeleteServiceURL,
					&sandboxesBoxesGenerateDownloadURL,
					&sandboxesBoxesGenerateServiceURL,
					&sandboxesBoxesGetStatus,
					&sandboxesBoxesListServiceURLs,
					&sandboxesBoxesStart,
					&sandboxesBoxesStop,
				},
			},
			{
				Name:     "sandboxes:registries",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&sandboxesRegistriesCreate,
					&sandboxesRegistriesRetrieve,
					&sandboxesRegistriesUpdate,
					&sandboxesRegistriesList,
					&sandboxesRegistriesDelete,
				},
			},
			{
				Name:     "sandboxes:snapshots",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&sandboxesSnapshotsCreate,
					&sandboxesSnapshotsRetrieve,
					&sandboxesSnapshotsList,
					&sandboxesSnapshotsDelete,
					&sandboxesSnapshotsRetrieveByName,
				},
			},
			{
				Name:            "@manpages",
				Usage:           "Generate documentation for 'man'",
				UsageText:       "langsmith @manpages [-o langsmith.1] [--gzip]",
				Hidden:          true,
				Action:          generateManpages,
				HideHelpCommand: true,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "write manpages to the given folder",
						Value:   "man",
					},
					&cli.BoolFlag{
						Name:    "gzip",
						Aliases: []string{"z"},
						Usage:   "output gzipped manpage files to .gz",
						Value:   true,
					},
					&cli.BoolFlag{
						Name:    "text",
						Aliases: []string{"z"},
						Usage:   "output uncompressed text files",
						Value:   false,
					},
				},
			},
			{
				Name:            "__complete",
				Hidden:          true,
				HideHelpCommand: true,
				Action:          autocomplete.ExecuteShellCompletion,
			},
			{
				Name:            "@completion",
				Hidden:          true,
				HideHelpCommand: true,
				Action:          autocomplete.OutputCompletionScript,
			},
		},
		HideHelpCommand: true,
	}
}

func generateManpages(ctx context.Context, c *cli.Command) error {
	manpage, err := docs.ToManWithSection(Command, 1)
	if err != nil {
		return err
	}
	dir := c.String("output")
	err = os.MkdirAll(filepath.Join(dir, "man1"), 0755)
	if err != nil {
		// handle error
	}
	if c.Bool("text") {
		file, err := os.Create(filepath.Join(dir, "man1", "langsmith.1"))
		if err != nil {
			return err
		}
		defer file.Close()
		if _, err := file.WriteString(manpage); err != nil {
			return err
		}
	}
	if c.Bool("gzip") {
		file, err := os.Create(filepath.Join(dir, "man1", "langsmith.1.gz"))
		if err != nil {
			return err
		}
		defer file.Close()
		gzWriter := gzip.NewWriter(file)
		defer gzWriter.Close()
		_, err = gzWriter.Write([]byte(manpage))
		if err != nil {
			return err
		}
	}
	fmt.Printf("Wrote manpages to %s\n", dir)
	return nil
}
