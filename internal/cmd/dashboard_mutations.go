package cmd

import (
	"strings"

	"github.com/spf13/cobra"
)

func newDashboardCloneCmd() *cobra.Command {
	var id string
	var dryRun bool
	cmd := &cobra.Command{Use: "clone", Short: "Clone a workspace dashboard and its charts", Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if err := dashboardUUID(id); err != nil {
			return err
		}
		return createDashboardResource(cmd, "/api/v1/charts/section/clone", map[string]any{"section_id": id}, dryRun)
	}
	cmd.Flags().StringVar(&id, "dashboard-id", "", "Source dashboard UUID")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the clone request without writing")
	_ = cmd.MarkFlagRequired("dashboard-id")
	return cmd
}

func dashboardResourcePath(id string, chart bool) string {
	if chart {
		return "/api/v1/charts/" + id
	}
	return "/api/v1/charts/section/" + id
}

func newDashboardUpdateCmd(chart bool) *cobra.Command {
	var config string
	var dryRun bool
	cmd := &cobra.Command{Use: "update ID", Short: "Update dashboard settings or layout from JSON", Args: cobra.ExactArgs(1)}
	if chart {
		cmd.Short = "Update chart settings, series, or Markdown from JSON"
	}
	cmd.Long = cmd.Short + ". Omitted fields remain unchanged. Supplied arrays replace their existing values; read the dashboard before editing series or layout. Supply chart_type together with series to check visualization compatibility. Dry-run validates supplied fields only, without reading existing state. No automatic retries."
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := dashboardUUID(args[0]); err != nil {
			return err
		}
		body, err := readDashboardObject(config)
		if err != nil {
			return err
		}
		allowed := map[string]bool{"title": true, "description": true, "index": true, "layout": !chart}
		if chart {
			for _, key := range []string{"chart_type", "series", "section_id", "metadata", "common_filters", "markdown"} {
				allowed[key] = true
			}
		}
		for key := range body {
			if !allowed[key] {
				return dashboardInvalid("Unsupported update field; pass only writable dashboard or chart fields.")
			}
		}
		if value, exists := body["title"]; exists {
			if title, ok := value.(string); !ok || strings.TrimSpace(title) == "" {
				return dashboardInvalid("title must be a nonblank string.")
			}
		}
		if value, exists := body["section_id"]; exists {
			id, ok := value.(string)
			if !ok {
				return dashboardInvalid("section_id must be a dashboard UUID.")
			}
			if err := dashboardUUID(id); err != nil {
				return err
			}
		}
		if chart {
			if err := validateDashboardChart(body, true); err != nil {
				return err
			}
		}
		if dryRun {
			return dashboardResult(cmd, map[string]any{"status": "dry_run", "id": args[0], "workspace_id": dashboardWorkspaceID(), "request": body, "validation_scope": "supplied_fields_only", "message": "Existing state, server semantics, and rendering were not checked. Supply chart_type and series together to validate their compatibility."})
		}
		c, err := getClient()
		if err != nil {
			return err
		}
		var result map[string]any
		if err := c.RawPatch(cmd.Context(), dashboardResourcePath(args[0], chart), body, &result); err != nil {
			return dashboardRequestError(err, true)
		}
		id, ok := result["id"].(string)
		if !ok || !sameDashboardID(id, args[0]) {
			return dashboardInvalidResponse(true)
		}
		return dashboardResult(cmd, map[string]any{"status": "updated", "workspace_id": dashboardWorkspaceID(), "resource": result})
	}
	cmd.Flags().StringVar(&config, "config", "", "Partial update: inline JSON, file.json, or @file.json")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the update without writing")
	_ = cmd.MarkFlagRequired("config")
	return cmd
}

func newDashboardDeleteCmd(chart bool) *cobra.Command {
	var yes, dryRun bool
	cmd := &cobra.Command{Use: "delete ID", Short: "Delete a dashboard and its charts", Args: cobra.ExactArgs(1)}
	if chart {
		cmd.Short = "Delete one dashboard chart or text block"
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := dashboardUUID(args[0]); err != nil {
			return err
		}
		if dryRun {
			return dashboardResult(cmd, map[string]any{"status": "dry_run", "id": args[0], "workspace_id": dashboardWorkspaceID(), "operation": "delete"})
		}
		if !yes {
			return dashboardDiagnostic{"confirmation_required", "Deletion requires --yes.", "Use --dry-run to inspect the exact target first. Deleting a dashboard also deletes its charts."}
		}
		c, err := getClient()
		if err != nil {
			return err
		}
		if err := c.RawDelete(cmd.Context(), dashboardResourcePath(args[0], chart), nil); err != nil {
			return dashboardRequestError(err, true)
		}
		return dashboardResult(cmd, map[string]any{"status": "deleted", "id": args[0], "workspace_id": dashboardWorkspaceID()})
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm deletion of this exact UUID")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the deletion target without writing")
	return cmd
}
