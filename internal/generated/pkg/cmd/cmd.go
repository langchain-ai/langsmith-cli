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
					&productFeedbackRetrieve,
				},
			},
			{
				Name:     "agents",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&agentsCreate,
					&agentsRetrieve,
					&agentsUpdate,
					&agentsList,
					&agentsDelete,
				},
			},
			{
				Name:     "sessions",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&sessionsUpdate,
					&sessionsDashboard,
					&sessionsResolve,
				},
			},
			{
				Name:     "sessions:insights",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&sessionsInsightsUpdate,
					&sessionsInsightsDelete,
					&sessionsInsightsRetrieveRuns,
				},
			},
			{
				Name:     "sessions:insights:configs",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&sessionsInsightsConfigsCreate,
				},
			},
			{
				Name:     "examples",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&examplesRetrieve,
					&examplesUpdate,
					&examplesDeleteAll,
					&examplesRetrieveCount,
				},
			},
			{
				Name:     "examples:bulk",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&examplesBulkCreate,
				},
			},
			{
				Name:     "datasets",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&datasetsClone,
					&datasetsRetrieveVersion,
					&datasetsUpdateTags,
				},
			},
			{
				Name:     "datasets:versions",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&datasetsVersionsRetrieveDiff,
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
					&runsGetURL,
					&runsIngestBatch,
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
				Name:     "traces",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&tracesQuery,
				},
			},
			{
				Name:     "online-evaluators",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&onlineEvaluatorsCreate,
					&onlineEvaluatorsUpdate,
					&onlineEvaluatorsSpend,
				},
			},
			{
				Name:     "feedback",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&feedbackRetrieve,
					&feedbackUpdate,
					&feedbackDelete,
				},
			},
			{
				Name:     "feedback:tokens",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&feedbackTokensCreate,
					&feedbackTokensUpdate,
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
					&annotationQueuesPopulate,
					&annotationQueuesRetrieveQueues,
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
				Name:     "workspaces",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&workspacesCreate,
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
					&sandboxesBoxesDeleteServiceURL,
					&sandboxesBoxesListServiceURLs,
				},
			},
			{
				Name:     "sandboxes:registries",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&sandboxesRegistriesCreate,
					&sandboxesRegistriesRetrieve,
					&sandboxesRegistriesList,
					&sandboxesRegistriesDelete,
				},
			},
			{
				Name:     "sandboxes:snapshots",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&sandboxesSnapshotsRetrieveByName,
				},
			},
			{
				Name:     "sandboxes:browsers",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&sandboxesBrowsersCreate,
					&sandboxesBrowsersRetrieve,
					&sandboxesBrowsersLoadExtensions,
				},
			},
			{
				Name:     "sandboxes:browsers:attachments",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&sandboxesBrowsersAttachmentsUpdate,
					&sandboxesBrowsersAttachmentsDelete,
					&sandboxesBrowsersAttachmentsSetController,
				},
			},
			{
				Name:     "sandboxes:browsers:attachments:links",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&sandboxesBrowsersAttachmentsLinksCreate,
				},
			},
			{
				Name:     "sandboxes:browsers:attachments:tabs",
				Category: "API RESOURCE",
				Suggest:  true,
				Commands: []*cli.Command{
					&sandboxesBrowsersAttachmentsTabsCreate,
					&sandboxesBrowsersAttachmentsTabsDelete,
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
