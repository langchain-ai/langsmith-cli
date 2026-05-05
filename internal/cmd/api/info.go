package api

import (
	"context"

	"github.com/langchain-ai/langsmith-cli/internal/cache"
	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	"github.com/langchain-ai/langsmith-cli/internal/structured"
	"github.com/spf13/cobra"
)

type infoInput struct {
	Refresh bool
}

var infoCommand = structured.Command[*infoInput]{
	Use:   "info METHOD PATH",
	Short: "Show details for a specific API endpoint",
	Long: `Show full details for a specific API endpoint including parameters,
request body schema, and response schema.

Examples:
  langsmith api info GET /api/v1/sessions
  langsmith api info GET sessions
  langsmith api info POST runs/query`,
	Args: cobra.ExactArgs(2),
	Input: func(cmd *cobra.Command) *infoInput {
		in := &infoInput{}
		cmd.Flags().BoolVar(&in.Refresh, "refresh", false, "Force re-fetch of the OpenAPI spec")
		return in
	},
	Action: func(_ context.Context, cmd *cobra.Command, in *infoInput, args []string) (any, error) {
		spec, err := loadSpec(cmdutil.ResolveAPIURL(cmd), cache.DefaultDir(), in.Refresh)
		if err != nil {
			return nil, err
		}
		return spec.LookupEndpoint(args[0], args[1])
	},
	Render: structured.Template(`{{.Method}} {{.Path}}
Tag: {{.Tag}}
Summary: {{.Summary}}{{if .Description}}
Description: {{.Description}}{{end}}{{if .Parameters}}

Parameters:
{{range .Parameters}}  {{printf "%-20s %-10s %s" .Name .Type .Description}}{{if .Required}} (required){{end}}
{{end}}{{end}}{{if .RequestBody}}
Request Body:
  {{jsonIndent .RequestBody "  " "  "}}
{{end}}{{if .Response}}
Response Schema:
  {{jsonIndent .Response "  " "  "}}
{{end}}`),
}

func newInfoCmd() *cobra.Command {
	return infoCommand.Cobra()
}
