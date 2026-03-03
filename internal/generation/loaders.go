package generation

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// RunData represents a single run loaded from a trace file.
type RunData struct {
	RunID       string         `json:"run_id"`
	TraceID     string         `json:"trace_id"`
	Name        string         `json:"name"`
	RunType     string         `json:"run_type"`
	ParentRunID string         `json:"parent_run_id"`
	StartTime   string         `json:"start_time"`
	EndTime     string         `json:"end_time"`
	Status      string         `json:"status"`
	Inputs      map[string]any `json:"inputs"`
	Outputs     map[string]any `json:"outputs"`
	Error       string         `json:"error"`
	Tags        []string       `json:"tags"`
}

// Trace represents a loaded trace: (trace_id, root_run, all_runs).
type Trace struct {
	TraceID string
	Root    RunData
	Runs    []RunData
}

// LoadTracesFromDir loads traces from a directory of JSONL files.
func LoadTracesFromDir(inputDir string, sortOrder string) ([]Trace, error) {
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return nil, err
	}

	var traces []Trace
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}
		fpath := filepath.Join(inputDir, entry.Name())
		fileTraces, err := loadJSONLFile(fpath)
		if err != nil {
			continue
		}
		traces = append(traces, fileTraces...)
	}

	return sortTraces(traces, sortOrder), nil
}

// LoadTracesFromFile loads traces from a single file.
func LoadTracesFromFile(inputFile string, sortOrder string) ([]Trace, error) {
	ext := filepath.Ext(inputFile)
	var traces []Trace
	var err error

	switch ext {
	case ".jsonl":
		traces, err = loadJSONLFile(inputFile)
	case ".json":
		traces, err = loadJSONFile(inputFile)
	default:
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return sortTraces(traces, sortOrder), nil
}

func loadJSONLFile(fpath string) ([]Trace, error) {
	f, err := os.Open(fpath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	traceRuns := make(map[string][]RunData)

	scanner := bufio.NewScanner(f)
	// Increase buffer size for long lines
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var runData RunData
		if err := json.Unmarshal([]byte(line), &runData); err != nil {
			continue
		}

		tid := runData.TraceID
		if tid == "" {
			tid = runData.RunID
		}
		if tid == "" {
			tid = "unknown"
		}

		traceRuns[tid] = append(traceRuns[tid], runData)
	}

	var traces []Trace
	for tid, runs := range traceRuns {
		root := findRoot(runs)
		traces = append(traces, Trace{
			TraceID: tid,
			Root:    root,
			Runs:    runs,
		})
	}

	return traces, nil
}

func loadJSONFile(fpath string) ([]Trace, error) {
	data, err := os.ReadFile(fpath)
	if err != nil {
		return nil, err
	}

	// Try as array
	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		// Try as single object
		var obj map[string]any
		if err2 := json.Unmarshal(data, &obj); err2 != nil {
			return nil, err
		}
		items = []map[string]any{obj}
	}

	var traces []Trace
	for _, item := range items {
		tid := getStr(item, "trace_id")
		if tid == "" {
			tid = "unknown"
		}

		var runs []RunData
		if runsRaw, ok := item["runs"].([]any); ok {
			for _, r := range runsRaw {
				if rm, ok := r.(map[string]any); ok {
					runs = append(runs, mapToRunData(rm))
				}
			}
		} else {
			runs = []RunData{mapToRunData(item)}
		}

		root := findRoot(runs)
		traces = append(traces, Trace{
			TraceID: tid,
			Root:    root,
			Runs:    runs,
		})
	}

	return traces, nil
}

func findRoot(runs []RunData) RunData {
	for _, r := range runs {
		if r.ParentRunID == "" {
			return r
		}
	}
	if len(runs) > 0 {
		return runs[0]
	}
	return RunData{}
}

func mapToRunData(m map[string]any) RunData {
	rd := RunData{
		RunID:       getStr(m, "run_id"),
		TraceID:     getStr(m, "trace_id"),
		Name:        getStr(m, "name"),
		RunType:     getStr(m, "run_type"),
		ParentRunID: getStr(m, "parent_run_id"),
		StartTime:   getStr(m, "start_time"),
		EndTime:     getStr(m, "end_time"),
		Status:      getStr(m, "status"),
		Error:       getStr(m, "error"),
	}

	if inp, ok := m["inputs"].(map[string]any); ok {
		rd.Inputs = inp
	}
	if out, ok := m["outputs"].(map[string]any); ok {
		rd.Outputs = out
	}
	if tags, ok := m["tags"].([]any); ok {
		for _, t := range tags {
			if s, ok := t.(string); ok {
				rd.Tags = append(rd.Tags, s)
			}
		}
	}

	return rd
}

func sortTraces(traces []Trace, sortOrder string) []Trace {
	switch sortOrder {
	case "newest":
		sort.Slice(traces, func(i, j int) bool {
			return parseTime(traces[i].Root.StartTime).After(parseTime(traces[j].Root.StartTime))
		})
	case "oldest":
		sort.Slice(traces, func(i, j int) bool {
			return parseTime(traces[i].Root.StartTime).Before(parseTime(traces[j].Root.StartTime))
		})
	case "alphabetical":
		sort.Slice(traces, func(i, j int) bool {
			return traces[i].TraceID < traces[j].TraceID
		})
	case "reverse-alphabetical":
		sort.Slice(traces, func(i, j int) bool {
			return traces[i].TraceID > traces[j].TraceID
		})
	}
	return traces
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return time.Time{}
		}
	}
	return t
}

func getStr(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
