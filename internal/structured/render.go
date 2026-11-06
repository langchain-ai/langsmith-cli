package structured

import (
	"encoding/json"
	"io"

	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

type Spec interface {
	RenderText(io.Writer, any) error
}

func Render(cmd *cobra.Command, model any, spec Spec) error {
	w := cmd.OutOrStdout()
	if expr := cmdutil.ResolveJQ(cmd); expr != "" {
		return output.RenderJQ(w, model, expr)
	}
	if cmdutil.ResolveFormat(cmd) != "pretty" || spec == nil {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(model)
	}
	return spec.RenderText(w, model)
}
