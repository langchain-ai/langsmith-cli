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

func newExampleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "example",
		Short: "Manage individual examples within datasets",
		Long: `Manage individual examples within datasets.

Examples are the individual input/output pairs stored in a dataset.
Use these commands to list, add, or remove examples.

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
		Short: "List examples in a dataset",
		Run: func(cmd *cobra.Command, args []string) {
			c := mustGetClient()
			ctx := context.Background()

			ds, err := resolveDataset(ctx, c, datasetName)
			if err != nil {
				exitErrorf("%v", err)
			}

			var examples []langsmith.Example
			remaining := limit
			pageOffset := offset
			for remaining > 0 {
				pageSize := remaining
				if pageSize > 100 {
					pageSize = 100
				}
				resp, err := c.SDK.Examples.List(ctx, langsmith.ExampleListParams{
					Dataset: langsmith.F(ds.ID),
					Limit:   langsmith.F(int64(pageSize)),
					Offset:  langsmith.F(int64(pageOffset)),
				})
				if err != nil {
					exitErrorf("listing examples: %v", err)
				}
				examples = append(examples, resp.Items...)
				if len(resp.Items) < pageSize {
					break
				}
				remaining -= len(resp.Items)
				pageOffset += len(resp.Items)
			}
			fmt_ := getFormat()

			// Filter by split if specified
			if split != "" {
				var filtered []langsmith.Example
				for _, ex := range examples {
					// The split field may be in metadata
					if ex.Metadata != nil {
						if s, ok := ex.Metadata["split"].(string); ok && s == split {
							filtered = append(filtered, ex)
						}
					}
				}
				examples = filtered
			}

			if fmt_ == "pretty" {
				columns := []string{"ID", "Split", "Created", "Inputs Preview"}
				var rows [][]string
				for _, ex := range examples {
					id := ex.ID
					if len(id) > 16 {
						id = id[:16] + "..."
					}
					splitVal := "N/A"
					if ex.Metadata != nil {
						if s, ok := ex.Metadata["split"].(string); ok {
							splitVal = s
						}
					}
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
				output.OutputJSON(data, outputFile)
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
		Run: func(cmd *cobra.Command, args []string) {
			c := mustGetClient()
			ctx := context.Background()

			// Parse JSON inputs
			var parsedInputs map[string]any
			if err := json.Unmarshal([]byte(inputs), &parsedInputs); err != nil {
				output.OutputJSON(map[string]any{"error": fmt.Sprintf("Invalid JSON for --inputs: %v", err)}, "")
				return
			}

			var parsedOutputs map[string]any
			if outputs != "" {
				if err := json.Unmarshal([]byte(outputs), &parsedOutputs); err != nil {
					output.OutputJSON(map[string]any{"error": fmt.Sprintf("Invalid JSON for --outputs: %v", err)}, "")
					return
				}
			}

			var parsedMetadata map[string]any
			if metadata != "" {
				if err := json.Unmarshal([]byte(metadata), &parsedMetadata); err != nil {
					output.OutputJSON(map[string]any{"error": fmt.Sprintf("Invalid JSON for --metadata: %v", err)}, "")
					return
				}
			}

			// Resolve dataset
			ds, err := resolveDataset(ctx, c, datasetName)
			if err != nil {
				exitErrorf("%v", err)
			}

			params := langsmith.ExampleNewParams{
				DatasetID: langsmith.F(ds.ID),
				Inputs:    langsmith.F(parsedInputs),
			}
			if parsedOutputs != nil {
				params.Outputs = langsmith.F(parsedOutputs)
			}
			if parsedMetadata != nil {
				if split != "" {
					parsedMetadata["split"] = split
				}
				params.Metadata = langsmith.F(parsedMetadata)
			} else if split != "" {
				params.Metadata = langsmith.F(map[string]any{"split": split})
			}

			ex, err := c.SDK.Examples.New(ctx, params)
			if err != nil {
				exitErrorf("creating example: %v", err)
			}

			output.OutputJSON(map[string]any{
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
					exitError("aborted")
				}
			}

			c := mustGetClient()
			ctx := context.Background()

			_, err := c.SDK.Examples.Delete(ctx, exampleID)
			if err != nil {
				exitErrorf("deleting example: %v", err)
			}

			output.OutputJSON(map[string]any{
				"status": "deleted",
				"id":     exampleID,
			}, "")
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	return cmd
}
