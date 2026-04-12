package api

import "github.com/spf13/cobra"

// newTestRoot wraps the api command under a minimal root that defines the same
// persistent flags as the real root command. This avoids flag shadowing while
// allowing tests to pass --api-key, --api-url, and --format.
func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "langsmith"}
	root.PersistentFlags().String("api-key", "", "")
	root.PersistentFlags().String("api-url", "", "")
	root.PersistentFlags().String("format", "json", "")
	root.AddCommand(NewCmd())
	return root
}
