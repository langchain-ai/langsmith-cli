package structured

import (
	"encoding/json"
	"fmt"
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
	// A bare string, such as a token, prints as is unless JSON was asked for,
	// so $(langsmith ...) captures the value rather than a quoted JSON string.
	if s, ok := model.(string); ok && !cmdutil.FormatChanged(cmd) {
		_, err := fmt.Fprintln(w, s)
		return err
	}
	if cmdutil.ResolveFormat(cmd) != "pretty" || spec == nil {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(model)
	}
	return spec.RenderText(w, model)
}
