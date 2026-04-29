package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

type workspaceListItem struct {
	ID             string   `json:"id"`
	OrganizationID string   `json:"organization_id,omitempty"`
	DisplayName    string   `json:"display_name"`
	TenantHandle   string   `json:"tenant_handle,omitempty"`
	CreatedAt      string   `json:"created_at,omitempty"`
	IsPersonal     bool     `json:"is_personal"`
	IsDeleted      bool     `json:"is_deleted"`
	ReadOnly       bool     `json:"read_only"`
	RoleID         string   `json:"role_id,omitempty"`
	RoleName       string   `json:"role_name,omitempty"`
	Permissions    []string `json:"permissions,omitempty"`
	DataPlaneURL   string   `json:"data_plane_url,omitempty"`
}

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
			endpoint := "/api/v1/workspaces"
			if includeDeleted {
				q := url.Values{"include_deleted": {"true"}}
				endpoint += "?" + q.Encode()
			}

			var workspaces []workspaceListItem
			if err := c.RawGet(context.Background(), endpoint, &workspaces); err != nil {
				ExitErrorf("listing workspaces: %v", err)
			}

			if GetFormat() == "pretty" {
				renderWorkspaceTable(cmd, workspaces)
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

func renderWorkspaceTable(cmd *cobra.Command, workspaces []workspaceListItem) {
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

func workspaceRole(ws workspaceListItem) string {
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
