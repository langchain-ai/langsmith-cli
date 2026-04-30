package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

func newWorkspaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "List and select LangSmith workspaces",
	}
	cmd.AddCommand(newWorkspaceListCmd())
	cmd.AddCommand(newWorkspaceSetDefaultCmd())
	return cmd
}

func newWorkspaceListCmd() *cobra.Command {
	var includeDeleted bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workspaces visible to the current profile",
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			params := langsmith.WorkspaceListParams{}
			if includeDeleted {
				params.IncludeDeleted = langsmith.F(true)
			}

			workspaces, err := c.SDK.Workspaces.List(context.Background(), params)
			if err != nil {
				ExitErrorf("listing workspaces: %v", err)
			}
			if workspaces == nil {
				ExitErrorf("listing workspaces: empty response")
			}

			if GetFormat() == "pretty" {
				renderWorkspaceTable(cmd, *workspaces)
				return
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if err := enc.Encode(workspaces); err != nil {
				ExitErrorf("encoding workspaces: %v", err)
			}
		},
	}
	cmd.Flags().BoolVar(&includeDeleted, "include-deleted", false, "Include deleted workspaces")
	return cmd
}

func newWorkspaceSetDefaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-default WORKSPACE_ID",
		Short: "Set the default workspace for the selected profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileSetWorkspace(cmd, args[0])
		},
	}
}

func renderWorkspaceTable(cmd *cobra.Command, workspaces []langsmith.WorkspaceListResponse) {
	table := tablewriter.NewWriter(cmd.OutOrStdout())
	table.SetHeader([]string{"Name", "ID", "Handle", "Role", "Deleted"})
	table.SetBorder(false)
	table.SetColumnSeparator("  ")
	table.SetHeaderLine(true)
	table.SetAutoWrapText(false)
	for _, ws := range workspaces {
		deleted := "false"
		if ws.IsDeleted {
			deleted = "true"
		}
		table.Append([]string{
			ws.DisplayName,
			ws.ID,
			ws.TenantHandle,
			workspaceRole(ws),
			deleted,
		})
	}
	table.Render()
}

func workspaceRole(ws langsmith.WorkspaceListResponse) string {
	if ws.RoleName != "" {
		return ws.RoleName
	}
	if ws.ReadOnly {
		return "Read only"
	}
	if len(ws.Permissions) > 0 {
		return fmt.Sprintf("%d permissions", len(ws.Permissions))
	}
	return ""
}
