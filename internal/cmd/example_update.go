package cmd

import (
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"github.com/spf13/cobra"
)

func newExampleUpdateCmd() *cobra.Command {
	var inputs, outputs, metadata string
	var splits []string
	var clearSplits bool
	cmd := &cobra.Command{Use: "update ID", Short: "Update example inputs, reference outputs, metadata, or splits", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := resourceUUID(args[0]); err != nil {
			return err
		}
		if !cmd.Flags().Changed("inputs") && !cmd.Flags().Changed("outputs") && !cmd.Flags().Changed("metadata") && !cmd.Flags().Changed("split") && !clearSplits {
			return datasetInputError("provide --inputs, --outputs, --metadata, --split, or --clear-splits")
		}
		p := langsmith.ExampleUpdateParams{}
		if clearSplits && cmd.Flags().Changed("split") {
			return datasetInputError("--split and --clear-splits are mutually exclusive")
		}
		if err := validateExampleSplits(splits); err != nil {
			return err
		}
		if cmd.Flags().Changed("split") || clearSplits {
			if splits == nil {
				splits = []string{}
			}
			p.Split = langsmith.F[langsmith.ExampleUpdateParamsSplitUnion](langsmith.ExampleUpdateParamsSplitArray(splits))
		}
		if cmd.Flags().Changed("metadata") {
			v, err := resourceObject(metadata)
			if err != nil {
				return err
			}
			p.Metadata = langsmith.F(v)
		}
		if cmd.Flags().Changed("inputs") {
			v, err := resourceObject(inputs)
			if err != nil {
				return err
			}
			p.Inputs = langsmith.F(v)
		}
		if cmd.Flags().Changed("outputs") {
			v, err := resourceObject(outputs)
			if err != nil {
				return err
			}
			p.Outputs = langsmith.F(v)
		}
		c, err := getClient()
		if err != nil {
			return err
		}
		result, err := c.SDK.Examples.Update(cmd.Context(), args[0], p, option.WithMaxRetries(0))
		if err != nil {
			return datasetWriteError()
		}
		return output.OutputJSON(map[string]any{"status": "updated", "verification": "acknowledged_not_read_back", "workspace_id": resultWorkspaceID(), "example_id": args[0], "result": result,
			"message":    "Example update acknowledged. Existing experiments were not rerun.",
			"next_steps": []string{readNextStep("api", "examples/"+args[0])}}, "")
	}}
	cmd.Flags().StringVar(&inputs, "inputs", "", "Replacement inputs JSON object or @file")
	cmd.Flags().StringVar(&outputs, "outputs", "", "Replacement reference outputs JSON object or @file")
	cmd.Flags().StringVar(&metadata, "metadata", "", "Replacement metadata JSON object or @file; include existing keys to preserve them, including dataset_split")
	cmd.Flags().StringSliceVar(&splits, "split", nil, "Replacement split memberships (repeat or comma-separate)")
	cmd.Flags().BoolVar(&clearSplits, "clear-splits", false, "Remove all split memberships")
	return cmd
}
