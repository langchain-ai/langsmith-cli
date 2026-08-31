package api

import (
	"fmt"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

// NewCmd creates the top-level `langsmith api` command.
func NewCmd() *cobra.Command {
	var (
		body    string
		headers []string
		include bool
		input   string
		method  string
		fields  []string
		raw     []string
	)

	cmd := &cobra.Command{
		Use:   "api",
		Short: "Browse API endpoints and make authenticated requests",
		Long: `Browse LangSmith API endpoints and make authenticated HTTP requests.

Browse endpoints:
  langsmith api ls                              List all endpoints
  langsmith api ls --tag datasets               Filter by tag
  langsmith api ls --search create              Search endpoints
  langsmith api info GET sessions               Show endpoint details

Make requests:
  langsmith api sessions?limit=5
  langsmith api runs/query -F 'session[]=abc' -F limit=10
  langsmith api sessions/abc-123 -X DELETE
  langsmith api datasets --input body.json
  echo '{"name":"x"}' | langsmith api sessions --input -
  langsmith api sessions --include`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}

			params, err := parseFields(raw, fields)
			if err != nil {
				return err
			}

			methodPassed := cmd.Flags().Changed("method")
			method := strings.ToUpper(method)
			if !methodPassed && (len(params) > 0 || input != "" || body != "") {
				method = "POST"
			}
			if !isHTTPMethod(method) {
				return fmt.Errorf("invalid HTTP method: %q\nRun 'langsmith api --help' for usage", method)
			}

			path := args[0]

			c, err := cmdutil.GetClient(cmd)
			if err != nil {
				return err
			}
			if err := blockRawTracingProjectDelete(c.APIURL(), method, path); err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			statusCode, err := runRequest(c, method, path, body, input, params, headers, include, w)
			if err != nil {
				return err
			}
			if statusCode >= 400 {
				return fmt.Errorf("HTTP %d", statusCode)
			}
			return nil
		},
	}

	// Flags for request mode
	cmd.Flags().StringVar(&body, "body", "", `Request body (JSON string, @file, or @- for stdin)`)
	cmd.Flags().StringVar(&input, "input", "", `File to use as body for the HTTP request (use "-" to read from stdin)`)
	cmd.Flags().StringArrayVarP(&fields, "field", "F", nil, `Add a typed JSON field in key=value format (use "@<path>" or "@-" to read value from file or stdin)`)
	cmd.Flags().StringArrayVarP(&raw, "raw-field", "f", nil, "Add a string JSON field in key=value format")
	cmd.Flags().StringArrayVarP(&headers, "header", "H", nil, "Additional headers (Key:Value, repeatable)")
	cmd.Flags().BoolVarP(&include, "include", "i", false, "Include HTTP response headers in output")
	cmd.Flags().StringVarP(&method, "method", "X", "GET", "HTTP method")

	cmd.AddCommand(newLsCmd())
	cmd.AddCommand(newInfoCmd())

	return cmd
}
