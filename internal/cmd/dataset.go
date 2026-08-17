package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/shared"
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

			data := datasetExport{Version: 1, Dataset: datasetExportDataset{Name: ds.Name, Description: ds.Description}}
			for _, ex := range allExamples {
				item := datasetExportExample{ID: ex.ID, Inputs: ex.Inputs, Outputs: ex.Outputs, Metadata: ex.Metadata, AttachmentURLs: ex.AttachmentURLs}
				if !ex.CreatedAt.IsZero() {
					item.CreatedAt = ex.CreatedAt.Format(time.RFC3339Nano)
				}
				item.Split = exampleSplit(ex)
				data.Examples = append(data.Examples, item)
			}

			jsonBytes, err := json.MarshalIndent(data, "", "  ")
			if err != nil {
				ExitErrorf("encoding export: %v", err)
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
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]

			fileData, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}
			items, err := parseDatasetExport(fileData)
			if err != nil {
				return err
			}
			if len(items) == 0 {
				return fmt.Errorf("dataset export contains no examples")
			}

			c := MustGetClient()
			ctx := context.Background()

			// Create dataset
			dsParams := langsmith.DatasetNewParams{
				Name: langsmith.F(name),
			}
			if description != "" {
				dsParams.Description = langsmith.F(description)
			}

			ds, err := c.SDK.Datasets.New(ctx, dsParams)
			if err != nil {
				return fmt.Errorf("creating dataset: %w", err)
			}

			bodies := make([]langsmith.ExampleBulkNewParamsBody, len(items))
			for i, item := range items {
				body := langsmith.ExampleBulkNewParamsBody{DatasetID: langsmith.F(ds.ID), Inputs: langsmith.F(item.Inputs)}
				if item.ID != "" {
					body.ID = langsmith.F(item.ID)
				}
				if item.CreatedAt != "" {
					body.CreatedAt = langsmith.F(item.CreatedAt)
				}
				if item.Outputs != nil {
					body.Outputs = langsmith.F(item.Outputs)
				}
				if item.Metadata != nil {
					body.Metadata = langsmith.F(item.Metadata)
				}
				if len(item.Split) == 1 {
					body.Split = langsmith.F[langsmith.ExampleBulkNewParamsBodySplitUnion](shared.UnionString(item.Split[0]))
				} else if len(item.Split) > 1 {
					body.Split = langsmith.F[langsmith.ExampleBulkNewParamsBodySplitUnion](langsmith.ExampleBulkNewParamsBodySplitArray(item.Split))
				}
				bodies[i] = body
			}
			if _, err := c.SDK.Examples.Bulk.New(ctx, langsmith.ExampleBulkNewParams{Body: bodies}); err != nil {
				_, cleanupErr := c.SDK.Datasets.Delete(ctx, ds.ID)
				if cleanupErr != nil {
					return fmt.Errorf("creating examples: %v; cleanup dataset %s failed: %v", err, ds.ID, cleanupErr)
				}
				return fmt.Errorf("creating examples: %v; dataset %s was removed", err, ds.ID)
			}

			output.OutputJSON(map[string]any{
				"status":        "uploaded",
				"dataset_id":    ds.ID,
				"dataset_name":  name,
				"example_count": len(items),
			}, "")
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name for the new dataset (required)")
	cmd.Flags().StringVar(&description, "description", "", "Optional description")
	_ = cmd.MarkFlagRequired("name")

	return cmd
}

type datasetExport struct {
	Version  int                    `json:"version"`
	Dataset  datasetExportDataset   `json:"dataset"`
	Examples []datasetExportExample `json:"examples"`
}
type datasetExportDataset struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}
type datasetExportExample struct {
	ID             string         `json:"id,omitempty"`
	CreatedAt      string         `json:"created_at,omitempty"`
	Inputs         map[string]any `json:"inputs"`
	Outputs        map[string]any `json:"outputs,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	Split          []string       `json:"split,omitempty"`
	AttachmentURLs map[string]any `json:"attachment_urls,omitempty"`
}

func exampleSplit(ex langsmith.Example) []string {
	var raw struct {
		Split any `json:"split"`
	}
	if json.Unmarshal([]byte(ex.JSON.RawJSON()), &raw) != nil {
		return nil
	}
	switch split := raw.Split.(type) {
	case string:
		if split != "" {
			return []string{split}
		}
	case []any:
		values := make([]string, 0, len(split))
		for _, value := range split {
			if s, ok := value.(string); ok && s != "" {
				values = append(values, s)
			}
		}
		return values
	}
	return nil
}

func parseDatasetExport(data []byte) ([]datasetExportExample, error) {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}
	if envelope, ok := raw.(map[string]any); ok && envelope["version"] != nil {
		if v, ok := envelope["version"].(float64); !ok || v != 1 {
			return nil, fmt.Errorf("unsupported dataset export version")
		}
		examples, ok := envelope["examples"].([]any)
		if !ok {
			return nil, fmt.Errorf("dataset export examples must be an array")
		}
		return validateDatasetExamples(examples)
	}
	if arr, ok := raw.([]any); ok {
		return validateDatasetExamples(arr)
	}
	if obj, ok := raw.(map[string]any); ok {
		return validateDatasetExamples([]any{obj})
	}
	return nil, fmt.Errorf("JSON file must be an array or versioned export object")
}

func validateDatasetExamples(raw []any) ([]datasetExportExample, error) {
	items := make([]datasetExportExample, len(raw))
	for i, value := range raw {
		obj, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("item at index %d must be an object", i)
		}
		encoded, _ := json.Marshal(obj)
		var parsed struct {
			ID             string         `json:"id,omitempty"`
			CreatedAt      string         `json:"created_at,omitempty"`
			Inputs         map[string]any `json:"inputs"`
			Outputs        map[string]any `json:"outputs,omitempty"`
			Metadata       map[string]any `json:"metadata,omitempty"`
			Split          any            `json:"split,omitempty"`
			AttachmentURLs map[string]any `json:"attachment_urls,omitempty"`
		}
		if err := json.Unmarshal(encoded, &parsed); err != nil {
			return nil, fmt.Errorf("item at index %d: %w", i, err)
		}
		items[i] = datasetExportExample{ID: parsed.ID, CreatedAt: parsed.CreatedAt, Inputs: parsed.Inputs, Outputs: parsed.Outputs, Metadata: parsed.Metadata, AttachmentURLs: parsed.AttachmentURLs}
		if items[i].Inputs == nil {
			return nil, fmt.Errorf("item at index %d: inputs must be an object", i)
		}
		if items[i].AttachmentURLs != nil && len(items[i].AttachmentURLs) > 0 {
			return nil, fmt.Errorf("item at index %d contains attachments that cannot be restored by dataset upload", i)
		}
		if items[i].ID != "" {
			if _, err := uuid.Parse(items[i].ID); err != nil {
				return nil, fmt.Errorf("item at index %d: invalid id: %w", i, err)
			}
		}
		if items[i].CreatedAt != "" {
			if _, err := time.Parse(time.RFC3339, items[i].CreatedAt); err != nil {
				return nil, fmt.Errorf("item at index %d: invalid created_at: %w", i, err)
			}
		}
		if parsed.Split != nil {
			switch split := parsed.Split.(type) {
			case string:
				if split == "" {
					return nil, fmt.Errorf("item at index %d: split must not be empty", i)
				}
				items[i].Split = []string{split}
			case []any:
				if len(split) == 0 {
					return nil, fmt.Errorf("item at index %d: split must not be empty", i)
				}
				items[i].Split = make([]string, len(split))
				for j, s := range split {
					value, ok := s.(string)
					if !ok {
						return nil, fmt.Errorf("item at index %d: split values must be strings", i)
					}
					items[i].Split[j] = value
				}
			default:
				return nil, fmt.Errorf("item at index %d: split must be a string or array", i)
			}
		}
	}
	return items, nil
}
