package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/olekukonko/tablewriter"
	"github.com/xlab/treeprint"
)

// OutputJSON writes data as indented JSON to stdout or a file.
// If filePath is non-empty, writes to file and prints status to stderr.
func OutputJSON(data any, filePath string) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		Errorf("JSON encoding error: %v\n", err)
		return
	}

	if filePath != "" {
		if err := os.WriteFile(filePath, jsonBytes, 0644); err != nil {
			Errorf("write error: %v\n", err)
			return
		}
		Errorf(`{"status": "written", "path": %q}`+"\n", filePath)
	} else {
		PrintLines(string(jsonBytes) + "\n")
	}
}

// OutputJSONL writes items as JSONL (one JSON object per line).
func OutputJSONL(items []map[string]any, filePath string) {
	if filePath != "" {
		f, err := os.Create(filePath)
		if err != nil {
			Errorf("write error: %v\n", err)
			return
		}
		defer f.Close()
		for _, item := range items {
			line, _ := json.Marshal(item)
			_, _ = f.Write(line)
			_, _ = f.WriteString("\n")
		}
		Errorf(`{"status": "written", "path": %q, "count": %d}`+"\n", filePath, len(items))
	} else {
		for _, item := range items {
			line, _ := json.Marshal(item)
			PrintLines(string(line) + "\n")
		}
	}
}

// OutputTable prints a table to stdout using tablewriter.
func OutputTable(columns []string, rows [][]string, title string) {
	TableTo(os.Stdout, columns, rows, title)
}

func TableTo(w io.Writer, columns []string, rows [][]string, title string) {
	printTitle(w, title)

	table := newTable(w, sanitizeRow(columns))
	for _, row := range rows {
		table.Append(sanitizeRow(row))
	}
	table.Render()
}

func newTable(w io.Writer, columns []string) *tablewriter.Table {
	table := tablewriter.NewWriter(w)
	table.SetHeader(columns)
	table.SetBorder(false)
	table.SetColumnSeparator("  ")
	table.SetHeaderLine(true)
	table.SetAutoWrapText(false)
	return table
}

func printTitle(w io.Writer, title string) {
	if title == "" {
		return
	}
	title = SanitizeTerminal(title)
	Fprintln(w, title)
	Fprintln(w, strings.Repeat("─", utf8.RuneCountInString(title)))
}

func sanitizeRow(cells []string) []string {
	out := make([]string, len(cells))
	for i, c := range cells {
		out[i] = SanitizeTerminal(c)
	}
	return out
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
		Println("No runs found")
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
		tree.SetValue(treeLabel(root))
		addChildren(tree, root.ID, childrenMap)
		PrintLines(tree.String())
	}
}

func addChildren(node treeprint.Tree, parentID string, childrenMap map[string][]RunTreeData) {
	for _, child := range childrenMap[parentID] {
		label := treeLabel(child)
		if child.HasError {
			label = "ERROR: " + label
		}
		childNode := node.AddBranch(label)
		addChildren(childNode, child.ID, childrenMap)
	}
}

func treeLabel(r RunTreeData) string {
	return fmt.Sprintf("%s (%s) [%s]",
		SanitizeTerminal(r.Name), SanitizeTerminal(r.RunType), FormatDuration(r.DurationMs))
}

// PrintOutput dispatches to JSON or pretty output.
func PrintOutput(data any, format string, filePath string) {
	if format == "pretty" {
		// Pretty mode: just pretty-print JSON to stdout
		jsonBytes, _ := json.MarshalIndent(data, "", "  ")
		if filePath != "" {
			_ = os.WriteFile(filePath, jsonBytes, 0644)
		} else {
			PrintLines(string(jsonBytes) + "\n")
		}
	} else {
		OutputJSON(data, filePath)
	}
}

// PrintRunsTable prints a table of runs in pretty format.
func PrintRunsTable(w io.Writer, runs []map[string]any, includeMetadata bool, title string) {
	printTitle(w, title)

	columns := []string{"Time", "Name", "Type", "Trace ID", "Run ID"}
	if includeMetadata {
		columns = append(columns, "Duration", "Status", "Tokens")
	}

	table := newTable(w, columns)

	for _, r := range runs {
		timeStr := "N/A"
		if st, ok := r["start_time"].(string); ok {
			if clock := clockTime(st); clock != "" {
				timeStr = clock
			}
		}

		name := TruncateRunes(getStr(r, "name"), 40)

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

		table.Append(row)
	}

	table.Render()
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
		return SanitizeTerminal(v)
	}
	return "N/A"
}

func clockTime(s string) string {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("15:04:05")
		}
	}
	return ""
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
