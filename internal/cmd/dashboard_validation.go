package cmd

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

func dashboardInvalid(message string) error {
	return dashboardDiagnostic{"invalid_dashboard_input", message, "Check dashboard --help and the JSON field types; fix the input before retrying. Dry-run checks local structure only."}
}

func dashboardInvalidResponse(write bool) error {
	next := "Check the selected workspace and read the dashboard again."
	if write {
		next = "The write may have succeeded. Use dashboard list/get to reconcile current state before retrying; do not create duplicates."
	}
	return dashboardDiagnostic{"invalid_dashboard_response", "The service returned no matching resource or an empty response.", next}
}

func dashboardRequestError(err error, write bool) error {
	code, message := "dashboard_request_failed", "The dashboard request failed."
	next := "Check connectivity, workspace access, and the requested time window."
	var status interface{ StatusCode() int }
	statusCode := 0
	if errors.As(err, &status) {
		statusCode = status.StatusCode()
	}
	switch {
	case statusCode == 401:
		code, message = "authentication_required", "The dashboard service rejected this login."
		next = "Refresh the selected profile with auth login, then retry the read."
	case statusCode == 429:
		code, message = "rate_limited", "The dashboard service is rate limiting requests."
		next = "Wait before retrying; reduce query frequency and time-window size."
	case statusCode == 403:
		code, message = "permission_denied", "This login cannot perform the dashboard operation in the selected workspace."
		next = "Select the intended workspace/profile or ask its administrator for dashboard access."
	case statusCode == 404:
		code, message = "dashboard_not_found", "The dashboard endpoint or resource was not found, or its query was rejected."
		next = "Verify the ID with dashboard list in the same workspace. For data reads, provide start_time and a positive stride; check deployment support."
	case statusCode == 400 || statusCode == 422:
		code, message = "dashboard_request_rejected", "The service rejected the dashboard definition or query."
		next = "Check metric names, feedback keys, filters, grouping, and time-window fields. Preview data series before creating them."
	case statusCode == 409:
		code, message = "dashboard_conflict", "The dashboard operation conflicts with existing state."
		next = "Read the current dashboard and reconcile its IDs and layout before retrying."
	}
	if write {
		next += " Inspect current state before retrying a write; it may already have succeeded."
	}
	return dashboardDiagnostic{code, message, next}
}

func dashboardNonblank(value any) bool {
	s, ok := value.(string)
	return ok && strings.TrimSpace(s) != ""
}

// Validate stable structural requirements locally; the service owns metric,
// grouping, filter, and deployment-specific semantics.
func validateDashboardChart(body map[string]any, update bool) error {
	allowed := map[string]bool{"title": true, "description": true, "index": true, "chart_type": true, "series": true, "section_id": true, "metadata": true, "common_filters": true, "markdown": true}
	for key := range body {
		if !allowed[key] {
			return dashboardInvalid("Unknown chart field. Pass writable chart fields only, not a full read response.")
		}
	}
	kindValue, hasKind := body["chart_type"]
	kind, _ := kindValue.(string)
	if hasKind || !update {
		switch kind {
		case "line", "bar", "table", "kpi", "top-k", "pie", "text":
		default:
			return dashboardInvalid("chart_type must be line, bar, table, kpi, top-k, pie, or text.")
		}
	}
	if kind == "text" && update {
		return dashboardInvalid("Text blocks cannot be converted through chart_type updates. To edit text, supply markdown, index, or metadata only.")
	}
	if value, exists := body["markdown"]; exists {
		if _, ok := value.(string); !ok {
			return dashboardInvalid("markdown must be a string.")
		}
		if kind != "" && kind != "text" {
			return dashboardInvalid("Data charts cannot contain markdown; create a separate text block.")
		}
		if update {
			for key := range body {
				if key != "markdown" && key != "index" && key != "metadata" {
					return dashboardInvalid("Text updates accept only markdown, index, and metadata; do not mix text and data-chart fields.")
				}
			}
		}
	}
	if kind == "text" {
		if _, ok := body["markdown"].(string); !ok {
			return dashboardInvalid("Text blocks require a markdown string.")
		}
		for key := range body {
			if key != "chart_type" && key != "section_id" && key != "markdown" && key != "index" && key != "metadata" {
				return dashboardInvalid("Text blocks accept only chart_type, section_id, markdown, index, and metadata.")
			}
		}
		return nil
	}
	if title, exists := body["title"]; exists || !update {
		if !dashboardNonblank(title) {
			return dashboardInvalid("Data charts require a nonblank title string.")
		}
	}
	if value, exists := body["series"]; exists || !update {
		series, ok := value.([]any)
		if !ok || len(series) < 1 {
			return dashboardInvalid("Data charts require a nonempty series array.")
		}
		for _, value := range series {
			s, ok := value.(map[string]any)
			if !ok || !dashboardNonblank(s["name"]) {
				return dashboardInvalid("Each series requires a nonblank name string.")
			}
			if id, exists := s["id"]; exists && id != nil {
				value, ok := id.(string)
				if !ok || dashboardUUID(value) != nil {
					return dashboardInvalid("Series id must be a stored UUID, not an expanded UUID:group display ID.")
				}
			}
			definition, hasDefinition := s["metric_definition"].(map[string]any)
			if !dashboardNonblank(s["metric"]) && (!hasDefinition || len(definition) == 0) {
				return dashboardInvalid("Each series requires a metric string or nonempty metric_definition object.")
			}
			if metric, _ := s["metric"].(string); metric == "feedback" || metric == "feedback_score_avg" || metric == "feedback_values" {
				if !dashboardNonblank(s["feedback_key"]) {
					return dashboardInvalid("Feedback metrics require a nonblank feedback_key in the same series.")
				}
			}
		}
		return validateDashboardVisualization(kind, series)
	}
	return nil
}

// Check renderer constraints only when the relevant fields are supplied. PATCH
// validation cannot infer omitted fields without reading the existing resource.
func validateDashboardVisualization(kind string, series []any) error {
	modernOnly := kind == "pie" || kind == "top-k" || kind == "kpi" || kind == "table"
	if modernOnly && len(series) != 1 {
		return dashboardInvalid("Donut, ranked bar, KPI, and table charts require exactly one metric. Use line or bar for multiple metrics.")
	}
	for _, value := range series {
		s := value.(map[string]any) // Structural validation precedes this check.
		metric, _ := s["metric_definition"].(map[string]any)
		filter, _ := s["filter_definition"].(map[string]any)
		if raw := s["metric_definition"]; raw != nil && !dashboardNonblank(metric["type"]) {
			return dashboardInvalid("metric_definition requires a nonblank type string.")
		}
		if raw := s["filter_definition"]; raw != nil {
			if err := validateDashboardSource(filter); err != nil {
				return err
			}
		}
		if modernOnly && (len(metric) == 0 || len(filter) == 0) {
			return dashboardInvalid("This visualization requires metric_definition and filter_definition for modern rendering. Legacy metric/filters alone are insufficient.")
		}
		groups, _ := s["group_by_definitions"].([]any)
		if raw, exists := s["group_by_definitions"]; exists && raw != nil {
			if _, ok := raw.([]any); !ok || len(groups) > 1 {
				return dashboardInvalid("group_by_definitions must be an array with at most one grouping attribute.")
			}
		}
		if len(groups) > 0 {
			group, ok := groups[0].(map[string]any)
			if !ok || !dashboardNonblank(group["attribute"]) {
				return dashboardInvalid("Each grouping requires an attribute string.")
			}
			attribute, _ := group["attribute"].(string)
			if (attribute == "metadata" || attribute == "feedback_label") && !dashboardNonblank(group["path"]) {
				return dashboardInvalid("Metadata and feedback-label grouping require a nonblank path.")
			}
		}
		if len(groups) > 0 && len(series) != 1 {
			return dashboardInvalid("Grouping cannot be combined with multiple metrics. Use one grouped metric or separate ungrouped metrics.")
		}
		if (kind == "pie" || kind == "top-k") && len(groups) != 1 {
			return dashboardInvalid("Donut and ranked bar charts require one group_by_definitions attribute. Input/output comparisons should use line or bar.")
		}
		if kind == "kpi" && len(groups) != 0 {
			return dashboardInvalid("KPI charts cannot be grouped; use a ranked bar or donut for breakdowns.")
		}
		if kind == "table" && (metric["type"] != "count" || metric["entity"] != nil) {
			return dashboardInvalid("Table charts support run count only, not latency, cost, or feedback metrics. Use line or ranked bar for those comparisons.")
		}
		if filter["source_type"] == "dataset" {
			if (kind != "" && kind != "table") || len(series) != 1 || len(groups) != 0 {
				return dashboardInvalid("Dataset charts require a single-series table without grouping.")
			}
			for _, key := range []string{"run_filter", "trace_filter", "tree_filter", "project_ids"} {
				if filter[key] != nil {
					return dashboardInvalid("Dataset sources cannot include project IDs or run/trace/tree filters.")
				}
			}
		}
	}
	return nil
}

func validateDashboardSource(filter map[string]any) error {
	key := "project_ids"
	switch filter["source_type"] {
	case "tracing_project":
		if filter["dataset_ids"] != nil {
			return dashboardInvalid("Tracing-project sources cannot include dataset_ids.")
		}
	case "dataset":
		key = "dataset_ids"
	default:
		return dashboardInvalid("filter_definition requires source_type tracing_project or dataset.")
	}
	ids, ok := filter[key].([]any)
	if !ok || len(ids) == 0 || (key == "dataset_ids" && len(ids) != 1) {
		return dashboardInvalid("Sources require nonempty project_ids or exactly one dataset_ids entry.")
	}
	for _, raw := range ids {
		id, ok := raw.(string)
		if !ok || dashboardUUID(id) != nil {
			return dashboardInvalid("Source IDs must be UUIDs. Resolve project or dataset names before creating charts.")
		}
	}
	return nil
}

func validateDashboardQuery(body map[string]any) error {
	for key := range body {
		switch key {
		case "start_time", "end_time", "stride", "timezone", "omit_data", "group_by":
		default:
			return dashboardInvalid("Unknown query field. Use start_time, end_time, stride, timezone, omit_data, or group_by.")
		}
	}
	if value, ok := body["omit_data"]; ok {
		if _, valid := value.(bool); !valid {
			return dashboardInvalid("omit_data must be a boolean.")
		}
	}
	var start time.Time
	for _, key := range []string{"start_time", "end_time"} {
		value, exists := body[key]
		if !exists && (key == "end_time" || body["omit_data"] == true) {
			continue
		}
		s, ok := value.(string)
		parsed, err := time.Parse(time.RFC3339, s)
		if !ok || err != nil {
			return dashboardInvalid("Data queries require an RFC3339 start_time; end_time, when supplied, must also be RFC3339.")
		}
		if key == "start_time" {
			start = parsed
		} else if !start.IsZero() && !parsed.After(start) {
			return dashboardInvalid("end_time must be later than start_time.")
		}
	}
	if value, exists := body["stride"]; exists {
		stride, ok := value.(map[string]any)
		if !ok {
			return dashboardInvalid("stride must be an object with days, hours, or minutes.")
		}
		positive := false
		for key, value := range stride {
			number, ok := value.(json.Number)
			n, err := number.Int64()
			if (key != "days" && key != "hours" && key != "minutes") || !ok || err != nil || n < 0 {
				return dashboardInvalid("stride accepts nonnegative integer days, hours, and minutes only.")
			}
			positive = positive || n > 0
		}
		if !positive {
			return dashboardInvalid("stride must contain a positive duration.")
		}
	}
	return nil
}
