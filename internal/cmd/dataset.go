package cmd

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-cli/internal/generation"
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
or generated from production traces.

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
	cmd.AddCommand(newDatasetGenerateCmd())
	cmd.AddCommand(newDatasetViewFileCmd())
	cmd.AddCommand(newDatasetStructureCmd())

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

			params := langsmith.DatasetListParams{
				Limit: langsmith.F(int64(limit)),
			}
			if nameContains != "" {
				params.NameContains = langsmith.F(nameContains)
			}

			resp, err := c.SDK.Datasets.List(ctx, params)
			if err != nil {
				exitErrorf("listing datasets: %v", err)
			}

			datasets := resp.Items
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

			resp, err := c.SDK.Examples.List(ctx, langsmith.ExampleListParams{
				Dataset: langsmith.F(ds.ID),
				Limit:   langsmith.F(int64(limit)),
			})
			if err != nil {
				exitErrorf("listing examples: %v", err)
			}

			var data []map[string]any
			for _, ex := range resp.Items {
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

func newDatasetGenerateCmd() *cobra.Command {
	var (
		inputPath      string
		datasetType    string
		outputPath     string
		uploadName     string
		runName        string
		depth          int
		inputFields    string
		outputFields   string
		messagesOnly   bool
		samplePerTrace int
		sortOrder      string
		replace        bool
		yes            bool
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate evaluation datasets from exported trace files",
		Long: `Generate evaluation datasets from exported trace files.

Reads JSONL trace files (produced by 'trace export') and extracts
structured input/output pairs suitable for evaluation.

Dataset types:
  final_response  Extract root input -> root output pairs
  single_step     Extract individual node I/O (use --run-name to target)
  trajectory      Extract input -> tool call sequence
  rag             Extract question -> retrieved chunks -> answer

Examples:
  langsmith dataset generate -i ./traces -o eval.json --type final_response
  langsmith dataset generate -i ./traces -o eval.json --type rag --upload my-rag-eval`,
		Run: func(cmd *cobra.Command, args []string) {
			// Parse field lists
			var inFields, outFields []string
			if inputFields != "" {
				inFields = splitTrim(inputFields)
			}
			if outputFields != "" {
				outFields = splitTrim(outputFields)
			}

			// Load traces
			var traces []generation.Trace
			var err error

			fi, statErr := os.Stat(inputPath)
			if statErr != nil {
				exitErrorf("input path error: %v", statErr)
			}

			if fi.IsDir() {
				traces, err = generation.LoadTracesFromDir(inputPath, sortOrder)
			} else {
				traces, err = generation.LoadTracesFromFile(inputPath, sortOrder)
			}
			if err != nil {
				exitErrorf("loading traces: %v", err)
			}

			if len(traces) == 0 {
				output.OutputJSON(map[string]any{"error": "No traces found"}, "")
				return
			}

			// Generate dataset
			var depthPtr *int
			if cmd.Flags().Changed("depth") {
				depthPtr = &depth
			}
			var samplePtr *int
			if cmd.Flags().Changed("sample-per-trace") {
				samplePtr = &samplePerTrace
			}

			dataset := generation.GenerateDataset(traces, datasetType, runName, depthPtr, inFields, outFields, messagesOnly, samplePtr)

			if len(dataset) == 0 {
				output.OutputJSON(map[string]any{"error": "No examples generated"}, "")
				return
			}

			// Handle replace for output file
			if _, err := os.Stat(outputPath); err == nil && !replace {
				output.OutputJSON(map[string]any{
					"error": fmt.Sprintf("Output file exists: %s. Use --replace to overwrite.", outputPath),
				}, "")
				return
			}

			// Export to file
			generation.ExportToFile(dataset, outputPath)

			result := map[string]any{
				"status": "generated",
				"type":   datasetType,
				"count":  len(dataset),
				"output": outputPath,
			}

			// Upload to LangSmith if requested
			if uploadName != "" {
				c := mustGetClient()
				ctx := context.Background()

				if replace {
					// Try to delete existing dataset
					ds, err := resolveDataset(ctx, c, uploadName)
					if err == nil {
						if !yes {
							fmt.Fprintf(os.Stderr, "Delete existing dataset '%s'? [y/N] ", uploadName)
							var confirm string
							_, _ = fmt.Scanln(&confirm)
							if strings.ToLower(confirm) != "y" {
								exitError("aborted")
							}
						}
						_, _ = c.SDK.Datasets.Delete(ctx, ds.ID)
					}
				}

				generation.ExportToLangSmith(ctx, c.SDK, dataset, uploadName, datasetType)
				result["uploaded_to"] = uploadName
			}

			output.OutputJSON(result, "")
		},
	}

	cmd.Flags().StringVarP(&inputPath, "input", "i", "", "Path to JSONL trace files or directory (required)")
	cmd.Flags().StringVar(&datasetType, "type", "", "Dataset type: final_response, single_step, trajectory, rag (required)")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path (required)")
	cmd.Flags().StringVar(&uploadName, "upload", "", "Also upload to LangSmith with this name")
	cmd.Flags().StringVar(&runName, "run-name", "", "For single_step: target runs by name")
	cmd.Flags().IntVar(&depth, "depth", 0, "For trajectory: max hierarchy depth")
	cmd.Flags().StringVar(&inputFields, "input-fields", "", "Comma-separated field names for inputs")
	cmd.Flags().StringVar(&outputFields, "output-fields", "", "Comma-separated field names for outputs")
	cmd.Flags().BoolVar(&messagesOnly, "messages-only", false, "For final_response: only extract from messages")
	cmd.Flags().IntVar(&samplePerTrace, "sample-per-trace", 0, "For single_step: max examples per trace")
	cmd.Flags().StringVar(&sortOrder, "sort", "newest", "Sort order: newest, oldest, alphabetical, reverse-alphabetical")
	cmd.Flags().BoolVar(&replace, "replace", false, "Overwrite existing output file/dataset")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompts")

	_ = cmd.MarkFlagRequired("input")
	_ = cmd.MarkFlagRequired("type")
	_ = cmd.MarkFlagRequired("output")

	return cmd
}

func newDatasetViewFileCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "view-file FILE_PATH",
		Short: "Preview examples from a local dataset file (JSON or CSV)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			filePath := args[0]
			ext := strings.ToLower(filepath.Ext(filePath))
			fmt_ := getFormat()

			switch ext {
			case ".json":
				fileData, err := os.ReadFile(filePath)
				if err != nil {
					exitErrorf("reading file: %v", err)
				}

				var data []any
				if err := json.Unmarshal(fileData, &data); err != nil {
					// Try as single object
					var obj any
					if err2 := json.Unmarshal(fileData, &obj); err2 != nil {
						exitErrorf("parsing JSON: %v", err)
					}
					data = []any{obj}
				}

				fmt.Fprintf(os.Stderr, "File: %s\n", filepath.Base(filePath))
				fmt.Fprintf(os.Stderr, "Total: %d examples\n", len(data))

				examples := data
				if limit < len(examples) {
					examples = examples[:limit]
				}

				if fmt_ == "pretty" {
					output.PrintOutput(examples, "pretty", "")
				} else {
					output.OutputJSON(examples, "")
				}

			case ".csv":
				f, err := os.Open(filePath)
				if err != nil {
					exitErrorf("opening file: %v", err)
				}
				defer f.Close()

				reader := csv.NewReader(f)
				records, err := reader.ReadAll()
				if err != nil {
					exitErrorf("parsing CSV: %v", err)
				}

				if len(records) == 0 {
					exitError("empty CSV file")
				}

				headers := records[0]
				var rows []map[string]any
				for _, record := range records[1:] {
					row := make(map[string]any)
					for i, h := range headers {
						if i < len(record) {
							row[h] = record[i]
						}
					}
					rows = append(rows, row)
				}

				fmt.Fprintf(os.Stderr, "File: %s\n", filepath.Base(filePath))
				fmt.Fprintf(os.Stderr, "Total: %d rows\n", len(rows))

				examples := rows
				if limit < len(examples) {
					examples = examples[:limit]
				}

				if fmt_ == "pretty" {
					if len(examples) > 0 {
						columns := headers
						var tableRows [][]string
						for _, row := range examples {
							var r []string
							for _, c := range columns {
								v := fmt.Sprintf("%v", row[c])
								if len(v) > 100 {
									v = v[:100]
								}
								r = append(r, v)
							}
							tableRows = append(tableRows, r)
						}
						output.OutputTable(columns, tableRows, filepath.Base(filePath))
					}
				} else {
					output.OutputJSON(examples, "")
				}

			default:
				output.OutputJSON(map[string]any{"error": fmt.Sprintf("Unsupported file type: %s", ext)}, "")
			}
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 5, "Number of examples to display")
	return cmd
}

func newDatasetStructureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "structure FILE_PATH",
		Short: "Analyze the structure and field coverage of a local dataset file",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			filePath := args[0]
			ext := strings.ToLower(filepath.Ext(filePath))

			switch ext {
			case ".json":
				fileData, err := os.ReadFile(filePath)
				if err != nil {
					exitErrorf("reading file: %v", err)
				}

				var data []map[string]any
				if err := json.Unmarshal(fileData, &data); err != nil {
					// Try single object
					var obj map[string]any
					if err2 := json.Unmarshal(fileData, &obj); err2 != nil {
						exitErrorf("parsing JSON: %v", err)
					}
					data = []map[string]any{obj}
				}

				firstPreview := "N/A"
				if len(data) > 0 {
					b, _ := json.Marshal(data[0])
					firstPreview = string(b)
					if len(firstPreview) > 500 {
						firstPreview = firstPreview[:500]
					}
				}

				fieldCounts := make(map[string]int)
				for _, item := range data {
					for key := range item {
						fieldCounts[key]++
					}
				}

				total := len(data)
				coverage := make(map[string]string)
				keys := make([]string, 0, len(fieldCounts))
				for k := range fieldCounts {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, field := range keys {
					count := fieldCounts[field]
					pct := 0
					if total > 0 {
						pct = count * 100 / total
					}
					coverage[field] = fmt.Sprintf("%d/%d (%d%%)", count, total, pct)
				}

				output.OutputJSON(map[string]any{
					"format":                ext[1:],
					"example_count":         total,
					"first_example_preview": firstPreview,
					"field_coverage":        coverage,
				}, "")

			case ".csv":
				f, err := os.Open(filePath)
				if err != nil {
					exitErrorf("opening file: %v", err)
				}
				defer f.Close()

				reader := csv.NewReader(f)
				records, err := reader.ReadAll()
				if err != nil {
					exitErrorf("parsing CSV: %v", err)
				}

				if len(records) == 0 {
					exitError("empty CSV file")
				}

				headers := records[0]
				colCounts := make(map[string]int)
				for _, record := range records[1:] {
					for i, h := range headers {
						if i < len(record) && record[i] != "" {
							colCounts[h]++
						}
					}
				}

				total := len(records) - 1
				coverage := make(map[string]string)
				sortedCols := make([]string, len(headers))
				copy(sortedCols, headers)
				sort.Strings(sortedCols)
				for _, col := range sortedCols {
					count := colCounts[col]
					pct := 0
					if total > 0 {
						pct = count * 100 / total
					}
					coverage[col] = fmt.Sprintf("%d/%d (%d%%)", count, total, pct)
				}

				output.OutputJSON(map[string]any{
					"format":          ext[1:],
					"row_count":       total,
					"column_coverage": coverage,
				}, "")

			default:
				output.OutputJSON(map[string]any{"error": fmt.Sprintf("Unsupported file type: %s", ext)}, "")
			}
		},
	}

	return cmd
}
