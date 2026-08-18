package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/xlab/treeprint"
)

// OutputJSON writes data as indented JSON to stdout or a file.
// If filePath is non-empty, writes to file and prints status to stderr.
func OutputJSON(data any, filePath string) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		PrintError(fmt.Sprintf("JSON encoding error: %v", err))
		return
	}

	if filePath != "" {
		if err := os.WriteFile(filePath, jsonBytes, 0644); err != nil {
			PrintError(fmt.Sprintf("write error: %v", err))
			return
		}
		fmt.Fprintf(os.Stderr, `{"status": "written", "path": %q}`+"\n", filePath)
	} else {
		fmt.Println(string(jsonBytes))
	}
}

// OutputJSONL writes items as JSONL (one JSON object per line).
func OutputJSONL(items []map[string]any, filePath string) {
	if filePath != "" {
		f, err := os.Create(filePath)
		if err != nil {
			PrintError(fmt.Sprintf("write error: %v", err))
			return
		}
		defer f.Close()
		for _, item := range items {
			line, _ := json.Marshal(item)
			_, _ = f.Write(line)
			_, _ = f.WriteString("\n")
		}
		fmt.Fprintf(os.Stderr, `{"status": "written", "path": %q, "count": %d}`+"\n", filePath, len(items))
	} else {
		for _, item := range items {
			line, _ := json.Marshal(item)
			fmt.Println(string(line))
		}
	}
}

// OutputTable prints a table to stdout using tablewriter.
func OutputTable(columns []string, rows [][]string, title string) {
	if title != "" {
		fmt.Println(title)
		fmt.Println(strings.Repeat("─", len(title)))
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(columns)
	table.SetBorder(false)
	table.SetColumnSeparator("  ")
	table.SetHeaderLine(true)
	table.SetAutoWrapText(false)
	table.AppendBulk(rows)
	table.Render()
}

// RunTreeData holds the data needed for tree rendering.
type RunTreeData struct {
	ID          string
	ParentRunID string
	Name        string
	RunType     string
	DurationMs  *int64
	HasError    bool
}

// OutputTree prints a trace hierarchy tree.
func OutputTree(runs []RunTreeData, rootID string) {
	if len(runs) == 0 {
		fmt.Println("No runs found")
		return
	}

	// Build parent → children mapping
	childrenMap := make(map[string][]RunTreeData)
	runMap := make(map[string]RunTreeData)
	for _, r := range runs {
		runMap[r.ID] = r
		childrenMap[r.ParentRunID] = append(childrenMap[r.ParentRunID], r)
	}

	// Sort children by name for deterministic output
	for pid := range childrenMap {
		sort.Slice(childrenMap[pid], func(i, j int) bool {
			return childrenMap[pid][i].Name < childrenMap[pid][j].Name
		})
	}

	// Find roots
	var roots []RunTreeData
	if rootID != "" {
		if r, ok := runMap[rootID]; ok {
			roots = []RunTreeData{r}
		}
	}
	if len(roots) == 0 {
		roots = childrenMap[""]
	}
	if len(roots) == 0 && len(runs) > 0 {
		roots = runs[:1]
	}

	for _, root := range roots {
		tree := treeprint.New()
		label := fmt.Sprintf("%s (%s) [%s]", root.Name, root.RunType, FormatDuration(root.DurationMs))
		tree.SetValue(label)
		addChildren(tree, root.ID, childrenMap)
		fmt.Print(tree.String())
	}
}

func addChildren(node treeprint.Tree, parentID string, childrenMap map[string][]RunTreeData) {
	for _, child := range childrenMap[parentID] {
		label := fmt.Sprintf("%s (%s) [%s]", child.Name, child.RunType, FormatDuration(child.DurationMs))
		if child.HasError {
			label = "ERROR: " + label
		}
		childNode := node.AddBranch(label)
		addChildren(childNode, child.ID, childrenMap)
	}
}

// PrintOutput dispatches to JSON or pretty output.
func PrintOutput(data any, format string, filePath string) {
	if format == "pretty" {
		// Pretty mode: just pretty-print JSON to stdout
		jsonBytes, _ := json.MarshalIndent(data, "", "  ")
		if filePath != "" {
			_ = os.WriteFile(filePath, jsonBytes, 0644)
		} else {
			fmt.Println(string(jsonBytes))
		}
	} else {
		OutputJSON(data, filePath)
	}
}

// PrintError prints an error to stderr.
func PrintError(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}

// PrintRunsTable prints a table of runs in pretty format.
func PrintRunsTable(w io.Writer, runs []map[string]any, includeMetadata bool, includeFeedback bool, title string) {
	if title != "" {
		fmt.Fprintln(w, title)
		fmt.Fprintln(w, strings.Repeat("─", len(title)))
	}

	columns := []string{"Time", "Name", "Type", "Trace ID", "Run ID"}
	if includeMetadata {
		columns = append(columns, "Duration", "Status", "Tokens")
	}
	if includeFeedback {
		columns = append(columns, "Feedback")
	}

	table := tablewriter.NewWriter(w)
	table.SetHeader(columns)
	table.SetBorder(false)
	table.SetColumnSeparator("  ")
	table.SetHeaderLine(true)
	table.SetAutoWrapText(false)

	for _, r := range runs {
		timeStr := "N/A"
		if st, ok := r["start_time"].(string); ok && st != "" {
			if len(st) > 19 {
				timeStr = st[11:19]
			}
		}

		name := getStr(r, "name")
		if len(name) > 40 {
			name = name[:40]
		}

		traceID := getStr(r, "trace_id")

		runID := getStr(r, "run_id")

		row := []string{timeStr, name, getStr(r, "run_type"), traceID, runID}

		if includeMetadata {
			duration := "N/A"
			if d, ok := r["duration_ms"]; ok && d != nil {
				duration = FormatDuration(toInt64Ptr(d))
			}
			status := getStr(r, "status")
			if status == "" {
				status = "N/A"
			}
			tokens := "N/A"
			if tu, ok := r["token_usage"].(map[string]any); ok && tu != nil {
				if tt, ok := tu["total_tokens"]; ok {
					tokens = fmt.Sprintf("%v", tt)
				}
			}
			row = append(row, duration, status, tokens)
		}

		if includeFeedback {
			row = append(row, FormatFeedbackStats(r["feedback_stats"]))
		}

		table.Append(row)
	}

	table.Render()
}

// FormatFeedbackStats formats feedback_stats into a readable string.
func FormatFeedbackStats(v any) string {
	if v == nil {
		return "N/A"
	}

	statsMap := make(map[string]map[string]any)
	switch m := v.(type) {
	case map[string]map[string]any:
		statsMap = m
	case map[string]any:
		for k, inner := range m {
			if innerMap, ok := inner.(map[string]any); ok {
				statsMap[k] = innerMap
			}
		}
	default:
		return "N/A"
	}

	if len(statsMap) == 0 {
		return "N/A"
	}

	keys := make([]string, 0, len(statsMap))
	for k := range statsMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		inner := statsMap[k]
		avgStr := formatStatNum(inner["avg"])
		countStr := formatStatCount(inner)
		if avgStr != "" && countStr != "" {
			parts = append(parts, fmt.Sprintf("%s: %s (%s)", k, avgStr, countStr))
		} else if avgStr != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", k, avgStr))
		} else if countStr != "" {
			parts = append(parts, fmt.Sprintf("%s: (%s)", k, countStr))
		} else {
			parts = append(parts, k)
		}
	}

	if len(parts) == 0 {
		return "N/A"
	}
	return strings.Join(parts, ", ")
}

func formatStatNum(v any) string {
	if v == nil {
		return ""
	}
	switch n := v.(type) {
	case float64:
		return fmt.Sprintf("%.2f", n)
	case float32:
		return fmt.Sprintf("%.2f", n)
	case int:
		return fmt.Sprintf("%d", n)
	case int64:
		return fmt.Sprintf("%d", n)
	case json.Number:
		if f, err := n.Float64(); err == nil {
			return fmt.Sprintf("%.2f", f)
		}
	}
	return fmt.Sprintf("%v", v)
}

func formatStatCount(inner map[string]any) string {
	for _, countKey := range []string{"n", "count"} {
		if val, ok := inner[countKey]; ok && val != nil {
			switch n := val.(type) {
			case float64:
				return fmt.Sprintf("%.0f", n)
			case float32:
				return fmt.Sprintf("%.0f", n)
			case int, int64:
				return fmt.Sprintf("%d", n)
			case json.Number:
				return n.String()
			default:
				return fmt.Sprintf("%v", n)
			}
		}
	}
	return ""
}

// FormatDuration formats milliseconds as human-readable duration.
func FormatDuration(ms *int64) string {
	if ms == nil {
		return "N/A"
	}
	v := *ms
	if v < 1000 {
		return fmt.Sprintf("%dms", v)
	}
	return fmt.Sprintf("%.2fs", float64(v)/1000.0)
}

func getStr(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return "N/A"
}

func toInt64Ptr(v any) *int64 {
	switch n := v.(type) {
	case int64:
		return &n
	case float64:
		i := int64(n)
		return &i
	case int:
		i := int64(n)
		return &i
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return &i
		}
	}
	return nil
}
