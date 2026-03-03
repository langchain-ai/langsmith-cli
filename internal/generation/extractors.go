package generation

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Common field names for heuristic extraction.
var (
	CommonInputFields  = []string{"query", "input", "question", "message", "prompt", "text"}
	CommonOutputFields = []string{"answer", "output", "response", "result"}
)

// ExtractFromMessages extracts content from a messages array by role.
func ExtractFromMessages(messages []any, role string) string {
	if len(messages) == 0 {
		return ""
	}

	if role == "human" || role == "user" {
		for _, msg := range messages {
			if m, ok := msg.(map[string]any); ok {
				msgType := getStr(m, "type")
				if msgType == "" {
					msgType = getStr(m, "role")
				}
				if msgType == "human" || msgType == "user" {
					return extractContent(m)
				}
			} else if s, ok := msg.(string); ok {
				return s
			}
		}
	} else if role == "ai" || role == "assistant" {
		// Search from the end for AI messages
		for i := len(messages) - 1; i >= 0; i-- {
			if m, ok := messages[i].(map[string]any); ok {
				msgType := getStr(m, "type")
				if msgType == "" {
					msgType = getStr(m, "role")
				}
				if msgType == "ai" || msgType == "assistant" {
					content := extractContent(m)
					if content != "" && content != "None" {
						return content
					}
				}
			}
		}
	} else {
		// Default: last message's content
		if len(messages) > 0 {
			last := messages[len(messages)-1]
			if m, ok := last.(map[string]any); ok {
				return fmt.Sprintf("%v", m["content"])
			}
			return fmt.Sprintf("%v", last)
		}
	}

	return ""
}

func extractContent(m map[string]any) string {
	content := m["content"]
	if content == nil {
		return ""
	}

	// Content can be a list of content blocks
	if contentList, ok := content.([]any); ok {
		var parts []string
		for _, part := range contentList {
			if pm, ok := part.(map[string]any); ok {
				if getStr(pm, "type") == "text" {
					parts = append(parts, getStr(pm, "text"))
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, " ")
		}
		return fmt.Sprintf("%v", content)
	}

	s := fmt.Sprintf("%v", content)
	if s == "" || s == "None" {
		return ""
	}
	return s
}

// ExtractValue extracts a value from a dict using a priority chain.
func ExtractValue(data map[string]any, fields []string, commonFields []string, messageRole string, fallbackToRaw bool) any {
	if len(data) == 0 {
		return nil
	}

	// 1. User-specified fields
	if len(fields) > 0 {
		for _, field := range fields {
			if val, ok := data[field]; ok && val != nil {
				return val
			}
		}
	}

	// 2. Messages extraction
	if msgs, ok := data["messages"].([]any); ok && len(msgs) > 0 {
		result := ExtractFromMessages(msgs, messageRole)
		if result != "" {
			return result
		}
	}

	// 3. Common fields
	if len(commonFields) > 0 {
		for _, field := range commonFields {
			if val, ok := data[field]; ok && val != nil {
				return val
			}
		}
	}

	// 4. Fallback
	if fallbackToRaw {
		var nonNilValues []any
		for _, v := range data {
			if v != nil {
				nonNilValues = append(nonNilValues, v)
			}
		}
		if len(nonNilValues) == 1 {
			if s, ok := nonNilValues[0].(string); ok {
				return s
			}
		}
		for _, v := range nonNilValues {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return data
	}

	return nil
}

// ExtractTraceInputs extracts the primary input from a trace's root run.
func ExtractTraceInputs(root RunData, inputFields []string, asDict bool) any {
	if len(root.Inputs) == 0 {
		return nil
	}

	if len(inputFields) > 0 {
		return ExtractValue(root.Inputs, inputFields, CommonInputFields, "human", true)
	}

	if asDict {
		return root.Inputs
	}

	return ExtractValue(root.Inputs, nil, CommonInputFields, "human", true)
}

// ExtractTraceOutput extracts the primary output from a trace's root run.
func ExtractTraceOutput(root RunData, outputFields []string, messagesOnly bool) string {
	if len(root.Outputs) == 0 {
		return ""
	}

	var commonFields []string
	if !messagesOnly {
		commonFields = CommonOutputFields
	}

	result := ExtractValue(root.Outputs, outputFields, commonFields, "ai", !messagesOnly)
	if result == nil {
		return ""
	}

	if m, ok := result.(map[string]any); ok {
		b, _ := json.Marshal(m)
		return string(b)
	}

	return fmt.Sprintf("%v", result)
}

// ExtractFinalOutput searches all runs newest-first for any run with matching outputs.
func ExtractFinalOutput(runs []RunData, outputFields []string) string {
	sortedRuns := make([]RunData, len(runs))
	copy(sortedRuns, runs)
	sort.Slice(sortedRuns, func(i, j int) bool {
		return parseTime(sortedRuns[i].StartTime).After(parseTime(sortedRuns[j].StartTime))
	})

	for _, run := range sortedRuns {
		if len(run.Outputs) == 0 {
			continue
		}
		result := ExtractValue(run.Outputs, outputFields, CommonOutputFields, "ai", true)
		if result != nil {
			if m, ok := result.(map[string]any); ok {
				b, _ := json.Marshal(m)
				return string(b)
			}
			s := fmt.Sprintf("%v", result)
			if s != "" {
				return s
			}
		}
	}

	return ""
}

// ExtractToolSequence extracts the sequence of tool names from runs.
func ExtractToolSequence(runs []RunData, depth *int) []string {
	var toolRuns []RunData
	for _, r := range runs {
		if r.RunType == "tool" {
			toolRuns = append(toolRuns, r)
		}
	}

	sort.Slice(toolRuns, func(i, j int) bool {
		return parseTime(toolRuns[i].StartTime).Before(parseTime(toolRuns[j].StartTime))
	})

	if depth != nil {
		runMap := make(map[string]RunData)
		for _, r := range runs {
			rid := r.RunID
			if rid == "" {
				continue
			}
			runMap[rid] = r
		}

		var filtered []RunData
		for _, r := range toolRuns {
			d := 0
			current := r
			for {
				pid := current.ParentRunID
				if pid == "" {
					break
				}
				parent, ok := runMap[pid]
				if !ok {
					break
				}
				current = parent
				d++
			}
			if d <= *depth {
				filtered = append(filtered, r)
			}
		}
		toolRuns = filtered
	}

	var names []string
	for _, r := range toolRuns {
		name := r.Name
		if name == "" {
			name = "unknown"
		}
		names = append(names, strings.ToLower(name))
	}
	return names
}

// GetNodeIO gets input/output for runs matching a name.
func GetNodeIO(runs []RunData, runName string) []map[string]any {
	var matching []RunData
	for _, r := range runs {
		if runName != "" && r.Name != runName {
			continue
		}
		if len(r.Outputs) == 0 {
			continue
		}
		matching = append(matching, r)
	}

	sort.Slice(matching, func(i, j int) bool {
		return parseTime(matching[i].StartTime).Before(parseTime(matching[j].StartTime))
	})

	var results []map[string]any
	for _, r := range matching {
		rid := r.RunID
		if rid == "" {
			rid = r.TraceID
		}
		results = append(results, map[string]any{
			"node_name": r.Name,
			"inputs":    r.Inputs,
			"outputs":   r.Outputs,
			"run_id":    rid,
		})
	}
	return results
}

// ExtractDocuments extracts document text from retriever outputs.
func ExtractDocuments(outputs map[string]any) []string {
	if len(outputs) == 0 {
		return nil
	}

	var docs []any

	if d, ok := outputs["documents"].([]any); ok {
		docs = d
	} else if d, ok := outputs["output"].([]any); ok {
		docs = d
	} else {
		// Try the whole outputs as the document list
		for _, v := range outputs {
			if dl, ok := v.([]any); ok {
				docs = dl
				break
			}
		}
	}

	if docs == nil {
		return nil
	}

	var chunks []string
	for _, doc := range docs {
		switch d := doc.(type) {
		case map[string]any:
			text := getStr(d, "page_content")
			if text == "" {
				text = getStr(d, "content")
			}
			if text == "" {
				text = getStr(d, "text")
			}
			if text != "" {
				chunks = append(chunks, text)
			} else {
				b, _ := json.Marshal(d)
				chunks = append(chunks, string(b))
			}
		case string:
			chunks = append(chunks, d)
		default:
			chunks = append(chunks, fmt.Sprintf("%v", d))
		}
	}

	return chunks
}

// FindRetrievalData finds retrieval data from retriever runs.
func FindRetrievalData(runs []RunData) map[string]any {
	var retrieverRuns []RunData
	for _, r := range runs {
		if r.RunType == "retriever" {
			retrieverRuns = append(retrieverRuns, r)
		}
	}

	sort.Slice(retrieverRuns, func(i, j int) bool {
		return parseTime(retrieverRuns[i].StartTime).Before(parseTime(retrieverRuns[j].StartTime))
	})

	var query string
	var allChunks []string

	if len(retrieverRuns) > 0 {
		firstInputs := retrieverRuns[0].Inputs
		if len(firstInputs) > 0 {
			val := ExtractValue(firstInputs, nil, CommonInputFields, "human", true)
			if val != nil {
				query = fmt.Sprintf("%v", val)
			}
		}

		for _, r := range retrieverRuns {
			if len(r.Outputs) > 0 {
				chunks := ExtractDocuments(r.Outputs)
				allChunks = append(allChunks, chunks...)
			}
		}
	}

	answer := ExtractFinalOutput(runs, nil)

	return map[string]any{
		"query":            query,
		"retrieved_chunks": allChunks,
		"answer":           answer,
	}
}
