package structured

import (
	"context"

	"github.com/spf13/cobra"
)

type Command[I any] struct {
	Use     string
	Aliases []string
	Short   string
	Long    string
	Args    cobra.PositionalArgs
	Input   func(*cobra.Command) I
	Action  func(context.Context, *cobra.Command, I, []string) (any, error)
	Render  Spec
}

func (c Command[I]) Cobra() *cobra.Command {
	var input I
	cmd := &cobra.Command{
		Use:     c.Use,
		Aliases: c.Aliases,
		Short:   c.Short,
		Long:    c.Long,
		Args:    c.Args,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := c.Action(cmd.Context(), cmd, input, args)
			if err != nil {
				return err
			}
			return Render(cmd, result, c.Render)
		},
	}
	if c.Input != nil {
		input = c.Input(cmd)
	}
	cmd.Flags().String("jq", "", "Filter JSON output using a jq expression")
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
