package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newDatasetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dataset",
		Short: "Create, manage, and inspect evaluation datasets",
		Long: `Create, manage, and inspect evaluation datasets.

Datasets are collections of input/output examples used for evaluating
LLM applications. They can be created manually, uploaded from files,
or exported to local files.

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
		Short: "List all datasets in the workspace",
		Run: func(cmd *cobra.Command, args []string) {
			c := mustGetClient()
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
				exitErrorf("listing datasets: %v", err)
			}
			fmt_ := getFormat()

			if fmt_ == "pretty" {
				columns := []string{"Name", "ID", "Description", "Examples", "Created"}
				var rows [][]string
				for _, ds := range datasets {
					id := ds.ID
					if len(id) > 16 {
						id = id[:16] + "..."
					}
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
			c := mustGetClient()
			ctx := context.Background()

			ds, err := resolveDataset(ctx, c, args[0])
			if err != nil {
				exitErrorf("%v", err)
			}

			data := map[string]any{
				"id":            ds.ID,
				"name":          ds.Name,
				"description":   nilStr(ds.Description),
				"data_type":     nilStr(string(ds.DataType)),
				"example_count": ds.ExampleCount,
				"created_at":    formatTimeISO(ds.CreatedAt),
			}

			fmt_ := getFormat()
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
			c := mustGetClient()
			ctx := context.Background()

			params := langsmith.DatasetNewParams{
				Name: langsmith.F(name),
			}
			if description != "" {
				params.Description = langsmith.F(description)
			}

			ds, err := c.SDK.Datasets.New(ctx, params)
			if err != nil {
				exitErrorf("creating dataset: %v", err)
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
			c := mustGetClient()
			ctx := context.Background()

			ds, err := resolveDataset(ctx, c, args[0])
			if err != nil {
				exitErrorf("%v", err)
			}

			if !yes {
				fmt.Fprintf(os.Stderr, "Delete dataset '%s' (%s)? [y/N] ", ds.Name, ds.ID)
				var confirm string
				_, _ = fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					exitError("aborted")
				}
			}

			_, err = c.SDK.Datasets.Delete(ctx, ds.ID)
			if err != nil {
				exitErrorf("deleting dataset: %v", err)
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

			c := mustGetClient()
			ctx := context.Background()

			ds, err := resolveDataset(ctx, c, nameOrID)
			if err != nil {
				exitErrorf("%v", err)
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
				exitErrorf("listing examples: %v", err)
			}

			var data []map[string]any
			for _, ex := range allExamples {
				data = append(data, map[string]any{
					"inputs":  ex.Inputs,
					"outputs": ex.Outputs,
				})
			}

			jsonBytes, _ := json.MarshalIndent(data, "", "  ")
			if err := os.WriteFile(outputFile, jsonBytes, 0644); err != nil {
				exitErrorf("writing file: %v", err)
			}

			output.OutputJSON(map[string]any{
				"status":  "exported",
				"dataset": ds.Name,
				"count":   len(data),
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

			c := mustGetClient()
			ctx := context.Background()

			fileData, err := os.ReadFile(filePath)
			if err != nil {
				exitErrorf("reading file: %v", err)
			}

			var rawData any
			if err := json.Unmarshal(fileData, &rawData); err != nil {
				exitErrorf("parsing JSON: %v", err)
			}

			var items []map[string]any
			switch v := rawData.(type) {
			case []any:
				for _, item := range v {
					if m, ok := item.(map[string]any); ok {
						items = append(items, m)
					}
				}
			case map[string]any:
				items = []map[string]any{v}
			default:
				exitError("JSON file must be an array or object")
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
				exitErrorf("creating dataset: %v", err)
			}

			// Create examples
			for _, item := range items {
				var inputs, outputs map[string]any
				if inp, ok := item["inputs"].(map[string]any); ok {
					inputs = inp
				} else {
					inputs = item
				}
				if out, ok := item["outputs"].(map[string]any); ok {
					outputs = out
				}

				exParams := langsmith.ExampleNewParams{
					DatasetID: langsmith.F(ds.ID),
					Inputs:    langsmith.F(inputs),
				}
				if outputs != nil {
					exParams.Outputs = langsmith.F(outputs)
				}

				_, err := c.SDK.Examples.New(ctx, exParams)
				if err != nil {
					exitErrorf("creating example: %v", err)
				}
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
