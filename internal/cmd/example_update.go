package cmd

import (
	"fmt"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"github.com/spf13/cobra"
)

func newExampleUpdateCmd() *cobra.Command {
	var inputs, outputs string
	cmd := &cobra.Command{Use: "update ID", Short: "Update example inputs or reference outputs", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := resourceUUID(args[0]); err != nil {
			return err
		}
		if !cmd.Flags().Changed("inputs") && !cmd.Flags().Changed("outputs") {
			return fmt.Errorf("provide --inputs or --outputs")
		}
		p := langsmith.ExampleUpdateParams{}
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
			return err
		}
		return output.OutputJSON(map[string]any{"example_id": args[0], "result": result}, "")
	}}
	cmd.Flags().StringVar(&inputs, "inputs", "", "Replacement inputs JSON object or @file")
	cmd.Flags().StringVar(&outputs, "outputs", "", "Replacement reference outputs JSON object or @file")
	return cmd
}
