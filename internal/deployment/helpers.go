package deployment

import (
	"fmt"
	"strings"
	"time"
)

// ResolveDeploymentID resolves a deployment ID from --deployment-id or --name.
func ResolveDeploymentID(client *HostBackendClient, deploymentID, name string) (string, error) {
	if deploymentID != "" {
		return deploymentID, nil
	}
	if name == "" {
		return "", fmt.Errorf("either --deployment-id or --name is required")
	}
	existing, err := client.ListDeployments(name)
	if err != nil {
		return "", err
	}
	resources, ok := existing["resources"].([]interface{})
	if !ok {
		return "", fmt.Errorf("deployment '%s' not found", name)
	}
	for _, r := range resources {
		dep, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		if dep["name"] == name {
			if id, ok := dep["id"].(string); ok && id != "" {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("deployment '%s' not found", name)
}

// FormatTimestamp converts a timestamp (epoch ms or string) to a readable string.
func FormatTimestamp(ts interface{}) string {
	switch v := ts.(type) {
	case float64:
		t := time.UnixMilli(int64(v))
		return t.UTC().Format("2006-01-02 15:04:05")
	case int64:
		t := time.UnixMilli(v)
		return t.UTC().Format("2006-01-02 15:04:05")
	case string:
		return v
	default:
		return ""
	}
}

// FormatLogEntry formats a single log entry for display.
func FormatLogEntry(entry map[string]interface{}) string {
	ts := FormatTimestamp(entry["timestamp"])
	level, _ := entry["level"].(string)
	message, _ := entry["message"].(string)
	if ts != "" && level != "" {
		return fmt.Sprintf("[%s] [%s] %s", ts, level, message)
	}
	if ts != "" {
		return fmt.Sprintf("[%s] %s", ts, message)
	}
	return message
}

// LevelColor returns an ANSI color code for a log level.
func LevelColor(level string) string {
	switch strings.ToUpper(level) {
	case "ERROR", "CRITICAL":
		return "\033[31m" // red
	case "WARNING":
		return "\033[33m" // yellow
	default:
		return ""
	}
}

// ColorReset is the ANSI reset code.
const ColorReset = "\033[0m"

// FormatDeploymentsTable formats deployments as a text table.
func FormatDeploymentsTable(deployments []map[string]interface{}) string {
	headers := []string{"Deployment ID", "Deployment Name", "Deployment URL"}

	var rows [][]string
	for _, dep := range deployments {
		id := stringOrDash(dep["id"])
		name := stringOrDash(dep["name"])
		url := extractDeploymentURL(dep)
		rows = append(rows, []string{id, name, url})
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	formatRow := func(row []string) string {
		parts := make([]string, len(row))
		for i, cell := range row {
			parts[i] = padRight(cell, widths[i])
		}
		return strings.Join(parts, "  ")
	}

	var lines []string
	lines = append(lines, formatRow(headers))
	dashes := make([]string, len(headers))
	for i, w := range widths {
		dashes[i] = strings.Repeat("-", w)
	}
	lines = append(lines, formatRow(dashes))
	for _, row := range rows {
		lines = append(lines, formatRow(row))
	}
	return strings.Join(lines, "\n")
}

// CleanEmptyLines removes empty lines from a string.
func CleanEmptyLines(s string) string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func extractDeploymentURL(dep map[string]interface{}) string {
	sc, ok := dep["source_config"].(map[string]interface{})
	if !ok {
		return "-"
	}
	url, ok := sc["custom_url"].(string)
	if !ok || url == "" {
		return "-"
	}
	return url
}

func stringOrDash(v interface{}) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return "-"
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
