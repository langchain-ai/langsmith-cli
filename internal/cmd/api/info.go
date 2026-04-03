package api

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// Test overrides.
var (
	infoAPIURL   string
	infoCacheDir string
	infoFormat   string
)

func newInfoCmd() *cobra.Command {
	var refresh bool

	cmd := &cobra.Command{
		Use:   "info METHOD PATH",
		Short: "Show details for a specific API endpoint",
		Long: `Show full details for a specific API endpoint including parameters,
request body schema, and response schema.

Examples:
  langsmith api info GET /api/v1/sessions
  langsmith api info GET sessions
  langsmith api info POST runs/query`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			method := args[0]
			path := args[1]

			apiURL := infoAPIURL
			if apiURL == "" {
				apiURL = resolveAPIURL(cmd)
			}
			cacheDir := infoCacheDir
			if cacheDir == "" {
				cacheDir = defaultCacheDir()
			}
			format := infoFormat
			if format == "" {
				format = resolveFormat(cmd)
			}

			spec, err := loadSpec(apiURL, cacheDir, refresh)
			if err != nil {
				return err
			}

			detail, err := spec.LookupEndpoint(method, path)
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()

			if format == "pretty" {
				fmt.Fprintf(w, "%s %s\n", detail.Method, detail.Path)
				fmt.Fprintf(w, "Tag: %s\n", detail.Tag)
				fmt.Fprintf(w, "Summary: %s\n", detail.Summary)
				if detail.Description != "" {
					fmt.Fprintf(w, "Description: %s\n", detail.Description)
				}
				if len(detail.Parameters) > 0 {
					fmt.Fprintf(w, "\nParameters:\n")
					for _, p := range detail.Parameters {
						req := ""
						if p.Required {
							req = " (required)"
						}
						fmt.Fprintf(w, "  %-20s %-10s %s%s\n", p.Name, p.Type, p.Description, req)
					}
				}
				if detail.RequestBody != nil {
					fmt.Fprintf(w, "\nRequest Body:\n")
					b, _ := json.MarshalIndent(detail.RequestBody, "  ", "  ")
					fmt.Fprintf(w, "  %s\n", b)
				}
				if detail.Response != nil {
					fmt.Fprintf(w, "\nResponse Schema:\n")
					b, _ := json.MarshalIndent(detail.Response, "  ", "  ")
					fmt.Fprintf(w, "  %s\n", b)
				}
			} else {
				data, _ := json.MarshalIndent(detail, "", "  ")
				fmt.Fprintln(w, string(data))
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&refresh, "refresh", false, "Force re-fetch of the OpenAPI spec")

	return cmd
}
