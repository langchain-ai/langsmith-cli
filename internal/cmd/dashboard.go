package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Dashboards are workspace-scoped "sections" in the charts API. These routes
// are not exposed by the pinned Go SDK.
func newDashboardCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "dashboard", Short: "Create and inspect workspace dashboards"}
	cmd.AddCommand(newDashboardCreateCmd(), newDashboardListCmd(), newDashboardGetCmd(), newDashboardCloneCmd(), newDashboardUpdateCmd(false), newDashboardDeleteCmd(false))
	chart := &cobra.Command{Use: "chart", Short: "Preview and create dashboard charts"}
	chart.AddCommand(newDashboardChartCmd(false), newDashboardChartCmd(true), newDashboardUpdateCmd(true), newDashboardDeleteCmd(true))
	cmd.AddCommand(chart)
	return cmd
}

func dashboardResult(cmd *cobra.Command, result any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	if GetFormat() != "json" {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(result)
}

func readDashboardObject(input string) (map[string]any, error) {
	var value map[string]any
	data, err := readDashboardJSONInput(input, 1024*1024)
	if err != nil {
		return nil, dashboardInvalid("Cannot read dashboard JSON. Supply inline JSON, a readable file.json, or @file.json.")
	}
	if len(data) > 1024*1024 {
		return nil, dashboardInvalid("Dashboard JSON exceeds 1 MiB.")
	}
	if err := checkDashboardJSONKeys(json.NewDecoder(strings.NewReader(string(data))), 0); err != nil {
		return nil, dashboardInvalid("Dashboard JSON contains invalid, duplicate, or excessively nested fields.")
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil || !json.Valid(data) {
		return nil, dashboardInvalid("Dashboard JSON must contain exactly one object.")
	}
	if len(value) == 0 {
		return nil, dashboardInvalid("Dashboard JSON must be a nonempty object.")
	}
	return value, nil
}

func newDashboardCreateCmd() *cobra.Command {
	var title, description string
	var dryRun bool
	cmd := &cobra.Command{Use: "create", Short: "Create an empty workspace dashboard", Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if strings.TrimSpace(title) == "" {
			return dashboardInvalid("--title must not be blank.")
		}
		body := map[string]any{"title": title}
		if cmd.Flags().Changed("description") {
			body["description"] = description
		}
		return createDashboardResource(cmd, "/api/v1/charts/section", body, dryRun)
	}
	cmd.Flags().StringVar(&title, "title", "", "Dashboard title (required)")
	cmd.Flags().StringVar(&description, "description", "", "Dashboard description")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the request without creating a dashboard")
	_ = cmd.MarkFlagRequired("title")
	return cmd
}

func createDashboardResource(cmd *cobra.Command, path string, body map[string]any, dryRun bool) error {
	if dryRun {
		return dashboardResult(cmd, map[string]any{"status": "dry_run", "workspace_id": dashboardWorkspaceID(), "request": body})
	}
	c, err := getClient()
	if err != nil {
		return err
	}
	var result map[string]any
	if err := c.RawPost(cmd.Context(), path, body, &result); err != nil {
		return dashboardRequestError(err, true)
	}
	id, ok := result["id"].(string)
	if !ok || dashboardUUID(id) != nil {
		return dashboardInvalidResponse(true)
	}
	next := dashboardReadNextStep("dashboard", "get", id)
	if path == "/api/v1/charts/create" {
		next = dashboardReadNextStep("dashboard", "get", body["section_id"].(string))
	}
	return dashboardResult(cmd, map[string]any{"status": "created", "workspace_id": dashboardWorkspaceID(), "resource": result, "next_steps": []string{next}})
}

func newDashboardListCmd() *cobra.Command {
	var limit, offset int64
	var title string
	cmd := &cobra.Command{Use: "list", Short: "List a bounded page of workspace dashboards", Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if limit < 1 || limit > 100 || offset < 0 || offset > (1<<63-1)-limit {
			return dashboardInvalid("--limit must be 1–100; --offset must be nonnegative and leave room for the next page.")
		}
		query := url.Values{"limit": {strconv.FormatInt(limit, 10)}, "offset": {strconv.FormatInt(offset, 10)}}
		if title != "" {
			query.Set("title_contains", title)
		}
		c, err := getClient()
		if err != nil {
			return err
		}
		items := []map[string]any{}
		if err := c.RawGet(cmd.Context(), "/api/v1/charts/section?"+query.Encode(), &items); err != nil {
			return dashboardRequestError(err, false)
		}
		return dashboardResult(cmd, map[string]any{"workspace_id": dashboardWorkspaceID(), "items": items, "pagination": describeDashboardPage(len(items), limit, offset)})
	}
	cmd.Flags().Int64Var(&limit, "limit", 20, "Page size (1–100)")
	cmd.Flags().Int64Var(&offset, "offset", 0, "Page offset")
	cmd.Flags().StringVar(&title, "title-contains", "", "Filter dashboard titles by substring")
	return cmd
}

func newDashboardGetCmd() *cobra.Command {
	var query string
	cmd := &cobra.Command{Use: "get ID", Short: "Read a dashboard definition, optionally with chart data", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := dashboardUUID(args[0]); err != nil {
			return err
		}
		// Some deployments require a time window even when data is omitted.
		now := time.Now().UTC()
		body := map[string]any{"omit_data": true, "start_time": now.Add(-24 * time.Hour).Format(time.RFC3339), "end_time": now.Format(time.RFC3339)}
		if cmd.Flags().Changed("query") {
			var err error
			body, err = readDashboardObject(query)
			if err != nil {
				return err
			}
		}
		if err := validateDashboardQuery(body); err != nil {
			return err
		}
		c, err := getClient()
		if err != nil {
			return err
		}
		var result map[string]any
		if err := c.RawPost(cmd.Context(), "/api/v1/charts/section/"+args[0], body, &result); err != nil {
			return dashboardRequestError(err, false)
		}
		if id, ok := result["id"].(string); !ok || !sameDashboardID(id, args[0]) {
			return dashboardInvalidResponse(false)
		}
		return dashboardResult(cmd, map[string]any{"workspace_id": dashboardWorkspaceID(), "dashboard": result})
	}
	cmd.Flags().StringVar(&query, "query", "", "Query JSON: inline, file.json, or @file.json; supply start_time and stride to include data")
	return cmd
}

func newDashboardChartCmd(preview bool) *cobra.Command {
	var config, dashboardID, query string
	var dryRun bool
	cmd := &cobra.Command{Use: "create", Short: "Create a chart from its API definition", Args: cobra.NoArgs}
	if preview {
		cmd.Use = "preview"
		cmd.Short = "Query chart data without saving a chart"
	}
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		body, err := readDashboardObject(config)
		if err != nil {
			return err
		}
		if err := validateDashboardChart(body, false); err != nil {
			return err
		}
		if preview {
			bucket, err := readDashboardObject(query)
			if err != nil {
				return err
			}
			if err := validateDashboardQuery(bucket); err != nil {
				return err
			}
			series, ok := body["series"].([]any)
			if !ok || len(series) == 0 {
				return dashboardInvalid("Chart preview requires data series; text blocks cannot be previewed.")
			}
			for i, item := range series {
				definition, ok := item.(map[string]any)
				if !ok {
					return dashboardInvalid("Series entries must be objects.")
				}
				// Preview accepts unsaved string IDs; creation allocates real IDs.
				if _, exists := definition["id"]; !exists {
					definition["id"] = fmt.Sprintf("preview-%d", i)
				}
			}
			chart := map[string]any{"series": series}
			if filters, ok := body["common_filters"]; ok {
				chart["common_filters"] = filters
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			var result map[string]any
			if err := c.RawPost(cmd.Context(), "/api/v1/charts/preview", map[string]any{"chart": chart, "bucket_info": bucket}, &result); err != nil {
				return dashboardRequestError(err, false)
			}
			if _, ok := result["data"].([]any); !ok {
				return dashboardInvalidResponse(false)
			}
			return dashboardResult(cmd, map[string]any{"status": "preview", "workspace_id": dashboardWorkspaceID(), "chart": result, "message": "Data query completed; this does not validate chart rendering or saving. Missing scores remain missing."})
		}
		if err := dashboardUUID(dashboardID); err != nil {
			return err
		}
		if id, exists := body["section_id"]; exists && id != dashboardID {
			return dashboardInvalid("Config section_id conflicts with --dashboard-id.")
		}
		body["section_id"] = dashboardID
		return createDashboardResource(cmd, "/api/v1/charts/create", body, dryRun)
	}
	cmd.Flags().StringVar(&config, "config", "", "Chart definition: inline JSON, file.json, or @file.json; modern chart types require metric/filter definitions")
	_ = cmd.MarkFlagRequired("config")
	if preview {
		cmd.Flags().StringVar(&query, "query", "", "Time-window JSON with start_time and stride: inline, file.json, or @file.json")
		_ = cmd.MarkFlagRequired("query")
	} else {
		cmd.Flags().StringVar(&dashboardID, "dashboard-id", "", "Target workspace dashboard UUID")
		_ = cmd.MarkFlagRequired("dashboard-id")
		cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show request without writing; does not validate server-side chart semantics")
	}
	return cmd
}
