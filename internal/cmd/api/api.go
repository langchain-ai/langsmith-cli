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
  langsmith api GET sessions?limit=5
  langsmith api POST runs/query --body '{"session_id":"abc"}'
  langsmith api DELETE sessions/abc-123
  langsmith api POST datasets --body @body.json
  echo '{"name":"x"}' | langsmith api POST sessions --body @-
  langsmith api GET sessions --include`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}

			method := strings.ToUpper(args[0])
			if !isHTTPMethod(method) {
				return fmt.Errorf("unknown subcommand or HTTP method: %q\nRun 'langsmith api --help' for usage", args[0])
			}

			path := args[1]

			c, err := cmdutil.GetClient(cmd)
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			statusCode, err := runRequest(c, method, path, body, headers, include, w)
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
	cmd.Flags().StringArrayVarP(&headers, "header", "H", nil, "Additional headers (Key:Value, repeatable)")
	cmd.Flags().BoolVarP(&include, "include", "i", false, "Include HTTP response headers in output")

	cmd.AddCommand(newLsCmd())
	cmd.AddCommand(newInfoCmd())

	return cmd
}
