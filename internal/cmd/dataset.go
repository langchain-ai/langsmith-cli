package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/shared"
	"github.com/spf13/cobra"
)

const datasetExportVersion = 1

type datasetTransferFile struct {
	Version  int                      `json:"version"`
	Examples []datasetTransferExample `json:"examples"`
}

type datasetTransferExample struct {
	Inputs   map[string]any `json:"inputs"`
	Outputs  map[string]any `json:"outputs,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Splits   []string       `json:"splits,omitempty"`
}

func newDatasetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dataset",
		Short: "Create, manage, and inspect evaluation datasets",
		Long: `Create, manage, and inspect evaluation datasets.

Datasets are collections of input/output examples used for evaluating
LLM applications. They can be created manually, uploaded from files,
or exported to local files.

List results are paginated and return at most 100 datasets by default
(use --limit to change).

Examples:
  langsmith dataset list
  langsmith dataset get my-dataset
  langsmith dataset create --name my-dataset
  langsmith dataset export my-dataset ./export.json
  langsmith dataset upload data.json --name new-dataset`,
	}

	cmd.AddCommand(newDatasetListCmd())
	cmd.AddCommand(newDatasetGetCmd())
	cmd.AddCommand(newDatasetCreateCmd())
	cmd.AddCommand(newDatasetDeleteCmd())
	cmd.AddCommand(newDatasetExportCmd())
	cmd.AddCommand(newDatasetUploadCmd())

	return cmd
}

func newDatasetListCmd() *cobra.Command {
	var (
		limit        int
		nameContains string
		outputFile   string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all datasets in the workspace (default: 100)",
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			ctx := context.Background()

			pageSize := int64(20)
			if limit > 0 && int64(limit) < pageSize {
				pageSize = int64(limit)
			}
			params := langsmith.DatasetListParams{
				Limit: langsmith.F(pageSize),
			}
			if nameContains != "" {
				params.NameContains = langsmith.F(nameContains)
			}

			var datasets []langsmith.Dataset
			pager := c.SDK.Datasets.ListAutoPaging(ctx, params)
			for pager.Next() {
				datasets = append(datasets, pager.Current())
				if limit > 0 && len(datasets) >= limit {
					break
				}
			}
			if err := pager.Err(); err != nil {
				ExitErrorf("listing datasets: %v", err)
			}
			fmt_ := GetFormat()

			if fmt_ == "pretty" {
				columns := []string{"Name", "ID", "Description", "Examples", "Created"}
				var rows [][]string
				for _, ds := range datasets {
					id := ds.ID
					desc := ds.Description
					if len(desc) > 50 {
						desc = desc[:50]
					}
					created := "N/A"
					if !ds.CreatedAt.IsZero() {
						created = ds.CreatedAt.Format("2006-01-02")
					}
					rows = append(rows, []string{
						ds.Name, id, desc,
						fmt.Sprintf("%d", ds.ExampleCount),
						created,
					})
				}
				output.OutputTable(columns, rows, "Datasets")
			} else {
				var data []map[string]any
				for _, ds := range datasets {
					data = append(data, map[string]any{
						"id":            ds.ID,
						"name":          ds.Name,
						"description":   nilStr(ds.Description),
						"data_type":     nilStr(string(ds.DataType)),
						"example_count": ds.ExampleCount,
						"created_at":    formatTimeISO(ds.CreatedAt),
					})
				}
				output.OutputJSON(data, outputFile)
			}
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 100, "Maximum number of datasets to return")
	cmd.Flags().StringVar(&nameContains, "name-contains", "", "Filter datasets by name substring")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
}

func newDatasetGetCmd() *cobra.Command {
	var outputFile string

	cmd := &cobra.Command{
		Use:   "get NAME_OR_ID",
		Short: "Get dataset details by name or UUID",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			ctx := context.Background()

			ds, err := resolveDataset(ctx, c, args[0])
			if err != nil {
				ExitErrorf("%v", err)
			}

			data := map[string]any{
				"id":            ds.ID,
				"name":          ds.Name,
				"description":   nilStr(ds.Description),
				"data_type":     nilStr(string(ds.DataType)),
				"example_count": ds.ExampleCount,
				"created_at":    formatTimeISO(ds.CreatedAt),
			}

			fmt_ := GetFormat()
			if fmt_ == "pretty" {
				output.PrintOutput(data, "pretty", outputFile)
			} else {
				output.OutputJSON(data, outputFile)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	return cmd
}

func newDatasetCreateCmd() *cobra.Command {
	var (
		name        string
		description string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new empty dataset",
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			ctx := context.Background()

			params := langsmith.DatasetNewParams{
				Name: langsmith.F(name),
			}
			if description != "" {
				params.Description = langsmith.F(description)
			}

			ds, err := c.SDK.Datasets.New(ctx, params)
			if err != nil {
				ExitErrorf("creating dataset: %v", err)
			}

			output.OutputJSON(map[string]any{
				"status":      "created",
				"id":          ds.ID,
				"name":        ds.Name,
				"description": nilStr(ds.Description),
				"created_at":  formatTimeISO(ds.CreatedAt),
			}, "")
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name for the new dataset (required)")
	cmd.Flags().StringVar(&description, "description", "", "Optional description")
	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func newDatasetDeleteCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete NAME_OR_ID",
		Short: "Delete a dataset by name or UUID",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			ctx := context.Background()

			ds, err := resolveDataset(ctx, c, args[0])
			if err != nil {
				ExitErrorf("%v", err)
			}

			if !yes {
				fmt.Fprintf(os.Stderr, "Delete dataset '%s' (%s)? [y/N] ", ds.Name, ds.ID)
				var confirm string
				_, _ = fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					ExitError("aborted")
				}
			}

			_, err = c.SDK.Datasets.Delete(ctx, ds.ID)
			if err != nil {
				ExitErrorf("deleting dataset: %v", err)
			}

			output.OutputJSON(map[string]any{
				"status": "deleted",
				"id":     ds.ID,
				"name":   ds.Name,
			}, "")
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	return cmd
}

func newDatasetExportCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "export NAME_OR_ID OUTPUT_FILE",
		Short: "Export dataset examples to a JSON file",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			nameOrID := args[0]
			outputFile := args[1]

			c := MustGetClient()
			ctx := context.Background()

			ds, err := resolveDataset(ctx, c, nameOrID)
			if err != nil {
				ExitErrorf("%v", err)
			}

			exportPageSize := int64(20)
			if limit > 0 && int64(limit) < exportPageSize {
				exportPageSize = int64(limit)
			}
			var allExamples []langsmith.Example
			pager := c.SDK.Examples.ListAutoPaging(ctx, langsmith.ExampleListParams{
				Dataset: langsmith.F(ds.ID),
				Limit:   langsmith.F(exportPageSize),
			})
			for pager.Next() {
				allExamples = append(allExamples, pager.Current())
				if limit > 0 && len(allExamples) >= limit {
					break
				}
			}
			if err := pager.Err(); err != nil {
				ExitErrorf("listing examples: %v", err)
			}

			splitsByExample := make(map[string][]string)
			splitNames, err := c.SDK.Datasets.Splits.Get(ctx, ds.ID, langsmith.DatasetSplitGetParams{})
			if err != nil {
				ExitErrorf("listing dataset splits: %v", err)
			}
			for _, splitName := range *splitNames {
				splitPager := c.SDK.Examples.ListAutoPaging(ctx, langsmith.ExampleListParams{
					Dataset: langsmith.F(ds.ID), Splits: langsmith.F([]string{splitName}), Limit: langsmith.F(int64(100)),
				})
				for splitPager.Next() {
					ex := splitPager.Current()
					splitsByExample[ex.ID] = append(splitsByExample[ex.ID], splitName)
				}
				if err := splitPager.Err(); err != nil {
					ExitErrorf("listing examples in split %q: %v", splitName, err)
				}
			}

			data := datasetTransferFile{Version: datasetExportVersion}
			for _, ex := range allExamples {
				data.Examples = append(data.Examples, datasetTransferExample{
					Inputs: ex.Inputs, Outputs: ex.Outputs, Metadata: ex.Metadata, Splits: splitsByExample[ex.ID],
				})
			}

			jsonBytes, err := json.MarshalIndent(data, "", "  ")
			if err != nil {
				ExitErrorf("serializing dataset: %v", err)
			}
			if err := os.WriteFile(outputFile, jsonBytes, 0644); err != nil {
				ExitErrorf("writing file: %v", err)
			}

			output.OutputJSON(map[string]any{
				"status":  "exported",
				"dataset": ds.Name,
				"count":   len(data.Examples),
				"path":    outputFile,
			}, "")
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 100, "Maximum number of examples to export")
	return cmd
}

func newDatasetUploadCmd() *cobra.Command {
	var (
		name        string
		description string
	)

	cmd := &cobra.Command{
		Use:   "upload FILE_PATH",
		Short: "Upload a JSON file as a new dataset",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			filePath := args[0]

			c := MustGetClient()
			ctx := context.Background()

			fileData, err := os.ReadFile(filePath)
			if err != nil {
				ExitErrorf("reading file: %v", err)
			}

			items, err := parseDatasetUpload(fileData)
			if err != nil {
				ExitErrorf("parsing JSON: %v", err)
			}

			// Create dataset
			dsParams := langsmith.DatasetNewParams{
				Name: langsmith.F(name),
			}
			if description != "" {
				dsParams.Description = langsmith.F(description)
			}

			ds, err := c.SDK.Datasets.New(ctx, dsParams)
			if err != nil {
				ExitErrorf("creating dataset: %v", err)
			}

			if err := uploadExamplesWithCleanup(ctx, c.SDK, ds.ID, items); err != nil {
				ExitError(err.Error())
			}

			output.OutputJSON(map[string]any{
				"status":        "uploaded",
				"dataset_id":    ds.ID,
				"dataset_name":  name,
				"example_count": len(items),
			}, "")
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name for the new dataset (required)")
	cmd.Flags().StringVar(&description, "description", "", "Optional description")
	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func uploadExamplesWithCleanup(ctx context.Context, sdk *langsmith.Client, datasetID string, items []datasetTransferExample) error {
	bulk := langsmith.ExampleBulkNewParams{Body: make([]langsmith.ExampleBulkNewParamsBody, 0, len(items))}
	for _, item := range items {
		body := langsmith.ExampleBulkNewParamsBody{DatasetID: langsmith.F(datasetID), Inputs: langsmith.F(item.Inputs)}
		if item.Outputs != nil {
			body.Outputs = langsmith.F(item.Outputs)
		}
		if item.Metadata != nil {
			body.Metadata = langsmith.F(item.Metadata)
		}
		if len(item.Splits) == 1 {
			body.Split = langsmith.F[langsmith.ExampleBulkNewParamsBodySplitUnion](shared.UnionString(item.Splits[0]))
		} else if len(item.Splits) > 1 {
			body.Split = langsmith.F[langsmith.ExampleBulkNewParamsBodySplitUnion](langsmith.ExampleBulkNewParamsBodySplitArray(item.Splits))
		}
		bulk.Body = append(bulk.Body, body)
	}
	if _, err := sdk.Examples.Bulk.New(ctx, bulk); err != nil {
		if _, cleanupErr := sdk.Datasets.Delete(ctx, datasetID); cleanupErr != nil {
			return fmt.Errorf("creating examples: %v; cleaning up dataset %s: %v", err, datasetID, cleanupErr)
		}
		return fmt.Errorf("creating examples: %v (new dataset was removed)", err)
	}
	return nil
}

func parseDatasetUpload(data []byte) ([]datasetTransferExample, error) {
	var envelope datasetTransferFile
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Version != 0 {
		if envelope.Version != datasetExportVersion {
			return nil, fmt.Errorf("unsupported dataset export version %d", envelope.Version)
		}
		return validateDatasetTransferExamples(envelope.Examples)
	}
	var rawItems []json.RawMessage
	if err := json.Unmarshal(data, &rawItems); err != nil {
		return nil, fmt.Errorf("expected a versioned dataset export or an array of examples: %w", err)
	}
	items := make([]datasetTransferExample, 0, len(rawItems))
	for i, raw := range rawItems {
		var item datasetTransferExample
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, fmt.Errorf("example at index %d must be an object: %w", i, err)
		}
		if len(raw) == 0 || raw[0] != '{' {
			return nil, fmt.Errorf("example at index %d must be an object", i)
		}
		items = append(items, item)
	}
	return validateDatasetTransferExamples(items)
}

func validateDatasetTransferExamples(items []datasetTransferExample) ([]datasetTransferExample, error) {
	for i, item := range items {
		if item.Inputs == nil {
			return nil, fmt.Errorf("example at index %d must contain an object-valued inputs field", i)
		}
	}
	return items, nil
}
