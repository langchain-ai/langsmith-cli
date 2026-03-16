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

// PrintError prints a JSON error to stderr.
func PrintError(msg string) {
	fmt.Fprintf(os.Stderr, `{"error": %q}`+"\n", msg)
}

// PrintRunsTable prints a table of runs in pretty format.
func PrintRunsTable(w io.Writer, runs []map[string]any, includeMetadata bool, title string) {
	if title != "" {
		fmt.Fprintln(w, title)
		fmt.Fprintln(w, strings.Repeat("─", len(title)))
	}

	columns := []string{"Time", "Name", "Type", "Trace ID", "Run ID"}
	if includeMetadata {
		columns = append(columns, "Duration", "Status", "Tokens")
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
