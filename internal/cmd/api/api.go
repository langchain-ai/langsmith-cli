package api

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewCmd creates the top-level `langsmith api` command.
func NewCmd() *cobra.Command {
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
			if len(args) == 0 {
				return cmd.Help()
			}
			return fmt.Errorf("unknown subcommand or HTTP method: %q\nRun 'langsmith api --help' for usage", args[0])
		},
	}

	cmd.AddCommand(newLsCmd())
	cmd.AddCommand(newInfoCmd())
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"} {
		cmd.AddCommand(requestCommand(method).Cobra())
	}

	return cmd
}
