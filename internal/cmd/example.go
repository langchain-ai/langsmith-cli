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

func newExampleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "example",
		Short: "Manage individual examples within datasets",
		Long: `Manage individual examples within datasets.

Examples are the individual input/output pairs stored in a dataset.
Use these commands to list, add, or remove examples.

List results are paginated and return at most 20 examples by default
(use --limit to change). Use --offset to paginate through results.

Examples:
  langsmith example list --dataset my-dataset
  langsmith example create --dataset my-dataset --inputs '{"question": "What is LangSmith?"}'
  langsmith example delete <example-id> --yes`,
	}

	cmd.AddCommand(newExampleListCmd())
	cmd.AddCommand(newExampleCreateCmd())
	cmd.AddCommand(newExampleDeleteCmd())
	return cmd
}

func newExampleListCmd() *cobra.Command {
	var (
		datasetName string
		limit       int
		offset      int
		split       string
		outputFile  string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List examples in a dataset (default: 20)",
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			ctx := context.Background()

			ds, err := resolveDataset(ctx, c, datasetName)
			if err != nil {
				ExitErrorf("%v", err)
			}

			pageSize := int64(20)
			if limit > 0 && int64(limit) < pageSize {
				pageSize = int64(limit)
			}
			params := exampleListParams(ds.ID, pageSize, offset, split)
			var examples []langsmith.Example
			pager := c.SDK.Examples.ListAutoPaging(ctx, params)
			for pager.Next() {
				examples = append(examples, pager.Current())
				if limit > 0 && len(examples) >= limit {
					break
				}
			}
			if err := pager.Err(); err != nil {
				ExitErrorf("listing examples: %v", err)
			}
			fmt_ := GetFormat()

			if fmt_ == "pretty" {
				columns := []string{"ID", "Split", "Created", "Inputs Preview"}
				var rows [][]string
				for _, ex := range examples {
					id := ex.ID
					splitVal := exampleSplitDisplay(ex.Metadata)
					created := "N/A"
					if !ex.CreatedAt.IsZero() {
						created = ex.CreatedAt.Format("2006-01-02")
					}
					inputsPreview := "N/A"
					if ex.Inputs != nil {
						b, _ := json.Marshal(ex.Inputs)
						inputsPreview = string(b)
						if len(inputsPreview) > 60 {
							inputsPreview = inputsPreview[:60] + "..."
						}
					}
					rows = append(rows, []string{id, splitVal, created, inputsPreview})
				}
				output.OutputTable(columns, rows, fmt.Sprintf("Examples in %s", ds.Name))
			} else {
				var data []map[string]any
				for _, ex := range examples {
					entry := map[string]any{
						"id":         ex.ID,
						"inputs":     ex.Inputs,
						"outputs":    ex.Outputs,
						"metadata":   ex.Metadata,
						"created_at": formatTimeISO(ex.CreatedAt),
					}
					data = append(data, entry)
				}
				if err := output.OutputJSON(data, outputFile); err != nil {
					ExitErrorf("%v", err)
				}
			}
		},
	}

	cmd.Flags().StringVar(&datasetName, "dataset", "", "Dataset name or UUID (required)")
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "Maximum number of examples to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "Number of examples to skip (pagination)")
	cmd.Flags().StringVar(&split, "split", "", "Filter by split (train, test, validation)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	_ = cmd.MarkFlagRequired("dataset")

	return cmd
}

func exampleSplitDisplay(metadata map[string]any) string {
	if metadata == nil {
		return "N/A"
	}
	switch splits := metadata["dataset_split"].(type) {
	case []any:
		values := make([]string, 0, len(splits))
		for _, split := range splits {
			if value, ok := split.(string); ok {
				values = append(values, value)
			}
		}
		if len(values) > 0 {
			return strings.Join(values, ", ")
		}
	case []string:
		if len(splits) > 0 {
			return strings.Join(splits, ", ")
		}
	case string:
		if splits != "" {
			return splits
		}
	}
	return "N/A"
}

func newExampleCreateCmd() *cobra.Command {
	var (
		datasetName string
		inputs      string
		outputs     string
		metadata    string
		split       string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new example in a dataset",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := MustGetClient()
			ctx := context.Background()

			// Parse JSON inputs
			var parsedInputs map[string]any
			if err := json.Unmarshal([]byte(inputs), &parsedInputs); err != nil {
				return fmt.Errorf("Invalid JSON for --inputs: %v", err)
			}

			var parsedOutputs map[string]any
			if outputs != "" {
				if err := json.Unmarshal([]byte(outputs), &parsedOutputs); err != nil {
					return fmt.Errorf("Invalid JSON for --outputs: %v", err)
				}
			}

			var parsedMetadata map[string]any
			if metadata != "" {
				if err := json.Unmarshal([]byte(metadata), &parsedMetadata); err != nil {
					return fmt.Errorf("Invalid JSON for --metadata: %v", err)
				}
			}

			ds, err := resolveDataset(ctx, c, datasetName)
			if err != nil {
				ExitErrorf("%v", err)
			}

			params := exampleCreateParams(ds.ID, parsedInputs, parsedOutputs, parsedMetadata, split)

			ex, err := c.SDK.Examples.New(ctx, params)
			if err != nil {
				ExitErrorf("creating example: %v", err)
			}
			return output.OutputJSON(map[string]any{
				"status":     "created",
				"id":         ex.ID,
				"dataset_id": ex.DatasetID,
				"inputs":     ex.Inputs,
				"outputs":    ex.Outputs,
			}, "")
		},
	}

	cmd.Flags().StringVar(&datasetName, "dataset", "", "Dataset name (required)")
	cmd.Flags().StringVar(&inputs, "inputs", "", "JSON string of input fields (required)")
	cmd.Flags().StringVar(&outputs, "outputs", "", "JSON string of output fields")
	cmd.Flags().StringVar(&metadata, "metadata", "", "JSON string of metadata")
	cmd.Flags().StringVar(&split, "split", "", "Assign to a split (train, test, validation)")
	_ = cmd.MarkFlagRequired("dataset")
	_ = cmd.MarkFlagRequired("inputs")

	return cmd
}

func exampleListParams(datasetID string, pageSize int64, offset int, split string) langsmith.ExampleListParams {
	params := langsmith.ExampleListParams{Dataset: langsmith.F(datasetID), Limit: langsmith.F(pageSize)}
	if offset > 0 {
		params.Offset = langsmith.F(int64(offset))
	}
	if split != "" {
		params.Splits = langsmith.F([]string{split})
	}
	return params
}

func exampleCreateParams(datasetID string, inputs, outputs, metadata map[string]any, split string) langsmith.ExampleNewParams {
	params := langsmith.ExampleNewParams{DatasetID: langsmith.F(datasetID), Inputs: langsmith.F(inputs)}
	if outputs != nil {
		params.Outputs = langsmith.F(outputs)
	}
	if metadata != nil {
		params.Metadata = langsmith.F(metadata)
	}
	if split != "" {
		params.Split = langsmith.F[langsmith.ExampleNewParamsSplitUnion](shared.UnionString(split))
	}
	return params
}

func newExampleDeleteCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete EXAMPLE_ID",
		Short: "Delete an example by its UUID",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			exampleID := args[0]

			if !yes {
				fmt.Fprintf(os.Stderr, "Delete example %s? [y/N] ", exampleID)
				var confirm string
				_, _ = fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					ExitError("aborted")
				}
			}

			c := MustGetClient()
			ctx := context.Background()

			_, err := c.SDK.Examples.Delete(ctx, exampleID)
			if err != nil {
				ExitErrorf("deleting example: %v", err)
			}
			if err := output.OutputJSON(map[string]any{
				"status": "deleted",
				"id":     exampleID,
			}, ""); err != nil {
				ExitErrorf("%v", err)
			}
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	return cmd
}
