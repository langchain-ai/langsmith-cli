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
func OutputJSON(data any, filePath string) error {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}

	if filePath != "" {
		if err := os.WriteFile(filePath, jsonBytes, 0644); err != nil {
			return fmt.Errorf("write JSON to %q: %w", filePath, err)
		}
		fmt.Fprintf(os.Stderr, `{"status": "written", "path": %q}`+"\n", filePath)
	} else {
		fmt.Println(string(jsonBytes))
	}
	return nil
}

// OutputJSONL writes items as JSONL (one JSON object per line).
func OutputJSONL(items []map[string]any, filePath string) error {
	if filePath != "" {
		if err := writeJSONLFile(items, filePath); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, `{"status": "written", "path": %q, "count": %d}`+"\n", filePath, len(items))
	} else {
		for _, item := range items {
			line, err := json.Marshal(item)
			if err != nil {
				return fmt.Errorf("encode JSONL item: %w", err)
			}
			fmt.Println(string(line))
		}
	}
	return nil
}

func writeJSONLFile(items []map[string]any, filePath string) (err error) {
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("create JSONL file %q: %w", filePath, err)
	}
	defer func() {
		if closeErr := f.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close JSONL file %q: %w", filePath, closeErr)
		}
	}()

	for _, item := range items {
		line, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("encode JSONL item: %w", err)
		}
		if _, err := fmt.Fprintln(f, string(line)); err != nil {
			return fmt.Errorf("write JSONL to %q: %w", filePath, err)
		}
	}
	return nil
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
func PrintOutput(data any, format string, filePath string) error {
	if format == "pretty" {
		// Pretty mode: just pretty-print JSON to stdout
		jsonBytes, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("encode JSON: %w", err)
		}
		if filePath != "" {
			if err := os.WriteFile(filePath, jsonBytes, 0644); err != nil {
				return fmt.Errorf("write JSON to %q: %w", filePath, err)
			}
		} else {
			fmt.Println(string(jsonBytes))
		}
		return nil
	} else {
		return OutputJSON(data, filePath)
	}
}

// PrintError prints an error to stderr.
func PrintError(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}

// PrintRunsTable prints a table of runs in pretty format.
func PrintRunsTable(w io.Writer, runs []map[string]any, includeMetadata, includeFeedback bool, title string) {
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
			feedback := "N/A"
			if fb, ok := r["feedback_stats"].(map[string]map[string]interface{}); ok {
				feedback = formatFeedbackStats(fb)
			}
			row = append(row, feedback)
		}

		table.Append(row)
	}

	table.Render()
}

// formatFeedbackStats renders a compact, single-line summary of a run's
// feedback_stats for the table's "Feedback" column: "key=avg (n=N)" for
// numeric feedback, or "key{label=count,...}" for categorical feedback.
// Multiple keys are joined with "; ". Keys with no recorded feedback points
// are skipped.
func formatFeedbackStats(fb map[string]map[string]interface{}) string {
	if len(fb) == 0 {
		return "N/A"
	}

	keys := make([]string, 0, len(fb))
	for k := range fb {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		stat := fb[k]

		if values, ok := stat["values"].(map[string]interface{}); ok && len(values) > 0 {
			labels := make([]string, 0, len(values))
			for label := range values {
				labels = append(labels, label)
			}
			sort.Strings(labels)

			counts := make([]string, 0, len(labels))
			for _, label := range labels {
				counts = append(counts, fmt.Sprintf("%s=%v", label, values[label]))
			}
			parts = append(parts, fmt.Sprintf("%s{%s}", k, strings.Join(counts, ",")))
			continue
		}

		n := feedbackStatFloat(stat, "n")
		if n == 0 {
			continue
		}
		avg := feedbackStatFloat(stat, "avg")
		parts = append(parts, fmt.Sprintf("%s=%.2g (n=%d)", k, avg, int64(n)))
	}

	if len(parts) == 0 {
		return "N/A"
	}
	return strings.Join(parts, "; ")
}

// feedbackStatFloat safely reads a numeric field out of a single feedback
// key's loosely-typed stats map, returning 0 if absent or non-numeric.
func feedbackStatFloat(stat map[string]interface{}, key string) float64 {
	if v, ok := stat[key].(float64); ok {
		return v
	}
	return 0
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
