package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// Test overrides — empty means "use real values from cobra flags".
var (
	lsAPIURL   string
	lsCacheDir string
	lsFormat   string
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
  langsmith api ls --search "create"
  langsmith api ls --tag run --search query
  langsmith api ls --refresh`,
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL := lsAPIURL
			if apiURL == "" {
				apiURL = resolveAPIURL(cmd)
			}
			cacheDir := lsCacheDir
			if cacheDir == "" {
				cacheDir = defaultCacheDir()
			}
			format := lsFormat
			if format == "" {
				format = resolveFormat(cmd)
			}

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
				table := tablewriter.NewWriter(w)
				table.SetHeader([]string{"Method", "Path", "Tag", "Summary"})
				table.SetBorder(false)
				table.SetColumnSeparator("  ")
				table.SetHeaderLine(true)
				table.SetAutoWrapText(false)
				for _, e := range endpoints {
					table.Append([]string{e.Method, e.Path, e.Tag, e.Summary})
				}
				table.Render()
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
