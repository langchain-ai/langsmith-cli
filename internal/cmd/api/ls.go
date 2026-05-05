package api

import (
	"context"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/cache"
	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	"github.com/langchain-ai/langsmith-cli/internal/structured"
	"github.com/spf13/cobra"
)

type lsInput struct {
	Tag     string
	Search  string
	Refresh bool
}

var lsCommand = structured.Command[*lsInput]{
	Use:   "ls",
	Short: "List available API endpoints from the OpenAPI spec",
	Long: `List all available LangSmith API endpoints.

The endpoint list is fetched from the OpenAPI spec and cached locally for 24 hours.

Examples:
  langsmith api ls
  langsmith api ls --tag datasets
  langsmith api ls --search create
  langsmith api ls --tag run --search query
  langsmith api ls --refresh`,
	Input: func(cmd *cobra.Command) *lsInput {
		in := &lsInput{}
		cmd.Flags().StringVarP(&in.Tag, "tag", "t", "", "Filter by tag")
		cmd.Flags().StringVarP(&in.Search, "search", "s", "", "Search path, summary, or tag (case-insensitive)")
		cmd.Flags().BoolVar(&in.Refresh, "refresh", false, "Force re-fetch of the OpenAPI spec")
		return in
	},
	Action: func(_ context.Context, cmd *cobra.Command, in *lsInput, _ []string) (any, error) {
		spec, err := loadSpec(cmdutil.ResolveAPIURL(cmd), cache.DefaultDir(), in.Refresh)
		if err != nil {
			return nil, err
		}

		endpoints := spec.Endpoints()
		if in.Tag != "" || in.Search != "" {
			var filtered []Endpoint
			for _, e := range endpoints {
				if in.Tag != "" && e.Tag != in.Tag {
					continue
				}
				if in.Search != "" {
					q := strings.ToLower(in.Search)
					if !strings.Contains(strings.ToLower(e.Path), q) &&
						!strings.Contains(strings.ToLower(e.Summary), q) &&
						!strings.Contains(strings.ToLower(e.Tag), q) {
						continue
					}
				}
				filtered = append(filtered, e)
			}
			endpoints = filtered
		}

		return endpoints, nil
	},
	Render: structured.Table{
		Columns: []structured.Column{
			{Header: "Method", Template: "{{.Method}}"},
			{Header: "Path", Template: "{{.Path}}"},
			{Header: "Tag", Template: "{{.Tag}}"},
			{Header: "Summary", Template: "{{.Summary}}"},
		},
		Footer: structured.Template("({{len .}} endpoints)\n"),
	},
}

func newLsCmd() *cobra.Command {
	return lsCommand.Cobra()
}
