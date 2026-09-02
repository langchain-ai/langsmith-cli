package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/cache"
	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newLsCmd() *cobra.Command {
	var (
		tag     string
		search  string
		refresh bool
	)

	cmd := &cobra.Command{
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
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL := cmdutil.ResolveAPIURL(cmd)
			cacheDir := cache.DefaultDir()
			format := cmdutil.ResolveFormat(cmd)

			spec, err := loadSpec(apiURL, cacheDir, refresh)
			if err != nil {
				return err
			}

			endpoints := spec.Endpoints()

			// Apply filters
			if tag != "" || search != "" {
				var filtered []Endpoint
				for _, e := range endpoints {
					if tag != "" && e.Tag != tag {
						continue
					}
					if search != "" {
						q := strings.ToLower(search)
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

			w := cmd.OutOrStdout()

			if format == "pretty" {
				table := output.NewTable(w, []string{"Method", "Path", "Tag", "Summary"})
				for _, e := range endpoints {
					if err := table.Append([]string{e.Method, e.Path, e.Tag, e.Summary}); err != nil {
						return fmt.Errorf("adding table row: %w", err)
					}
				}
				if err := table.Render(); err != nil {
					return fmt.Errorf("rendering table: %w", err)
				}
				fmt.Fprintf(w, "(%d endpoints)\n", len(endpoints))
			} else {
				data, _ := json.MarshalIndent(endpoints, "", "  ")
				fmt.Fprintln(w, string(data))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&tag, "tag", "t", "", "Filter by tag")
	cmd.Flags().StringVarP(&search, "search", "s", "", "Search path, summary, or tag (case-insensitive)")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Force re-fetch of the OpenAPI spec")

	return cmd
}
