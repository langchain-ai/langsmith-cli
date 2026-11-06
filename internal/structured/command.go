package structured

import (
	"context"

	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

type Command[I any] struct {
	Use          string
	Short        string
	Long         string
	Args         cobra.PositionalArgs
	Input        func(*cobra.Command) I
	Action       func(context.Context, *cobra.Command, I, []string) (any, error)
	Render       Spec
	CustomOutput bool
}

func (c Command[I]) Cobra() *cobra.Command {
	var input I
	cmd := &cobra.Command{
		Use:   c.Use,
		Short: c.Short,
		Long:  c.Long,
		Args:  c.Args,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := c.Action(cmd.Context(), cmd, input, args)
			if err != nil {
				return err
			}
			if c.CustomOutput {
				return nil
			}
			return Render(cmd, result, c.Render)
		},
	}
	if c.Input != nil {
		input = c.Input(cmd)
	}
	if c.CustomOutput {
		cmd.Annotations = map[string]string{cmdutil.NoJSONOutputAnnotation: "true"}
	} else {
		cmd.Flags().String("jq", "", cmdutil.JQFlagUsage)
	}
	return cmd
}

type Parent struct {
	Use      string
	Short    string
	Long     string
	Children []func() *cobra.Command
}

func (p Parent) Cobra() *cobra.Command {
	cmd := &cobra.Command{
		Use:   p.Use,
		Short: p.Short,
		Long:  p.Long,
	}
	for _, child := range p.Children {
		cmd.AddCommand(child())
	}
	return cmd
}
