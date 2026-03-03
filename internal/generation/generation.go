package generation

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"

	langsmith "github.com/langchain-ai/langsmith-go"
)

// GenerateDataset generates a dataset from traces.
func GenerateDataset(traces []Trace, datasetType string, runName string, depth *int, inputFields []string, outputFields []string, messagesOnly bool, samplePerTrace *int) []map[string]any {
	var dataset []map[string]any

	for _, trace := range traces {
		switch datasetType {
		case "rag":
			retrieval := FindRetrievalData(trace.Runs)
			query, _ := retrieval["query"].(string)
			answer, _ := retrieval["answer"].(string)
			if query == "" || answer == "" {
				continue
			}
			chunks, _ := retrieval["retrieved_chunks"].([]string)
			chunksText := strings.Join(chunks, "\n\n")
			var citedChunks []string
			if len(chunks) > 3 {
				citedChunks = chunks[:3]
			} else {
				citedChunks = chunks
			}
			cited, _ := json.Marshal(citedChunks)
			dataset = append(dataset, map[string]any{
				"trace_id":         trace.TraceID,
				"question":         query,
				"retrieved_chunks": chunksText,
				"answer":           answer,
				"cited_chunks":     string(cited),
			})

		case "final_response":
			inputs := ExtractTraceInputs(trace.Root, inputFields, len(inputFields) == 0)
			output := ExtractTraceOutput(trace.Root, outputFields, messagesOnly)
			if output == "" {
				continue
			}
			var inputsMap map[string]any
			if s, ok := inputs.(string); ok {
				inputsMap = map[string]any{"expected_input": s}
			} else if m, ok := inputs.(map[string]any); ok {
				inputsMap = m
			}
			dataset = append(dataset, map[string]any{
				"trace_id": trace.TraceID,
				"inputs":   inputsMap,
				"outputs":  map[string]any{"expected_response": output},
			})

		case "single_step":
			nodeResults := GetNodeIO(trace.Runs, runName)

			if samplePerTrace != nil && len(nodeResults) > *samplePerTrace {
				rand.Shuffle(len(nodeResults), func(i, j int) {
					nodeResults[i], nodeResults[j] = nodeResults[j], nodeResults[i]
				})
				nodeResults = nodeResults[:*samplePerTrace]
			}

			for i, node := range nodeResults {
				dataset = append(dataset, map[string]any{
					"trace_id":   trace.TraceID,
					"run_id":     node["run_id"],
					"node_name":  node["node_name"],
					"occurrence": i + 1,
					"inputs":     node["inputs"],
					"outputs":    map[string]any{"expected_output": node["outputs"]},
				})
			}

		case "trajectory":
			inputs := ExtractTraceInputs(trace.Root, inputFields, len(inputFields) == 0)
			tools := ExtractToolSequence(trace.Runs, depth)
			var inputsMap map[string]any
			if s, ok := inputs.(string); ok {
				inputsMap = map[string]any{"expected_input": s}
			} else if m, ok := inputs.(map[string]any); ok {
				inputsMap = m
			}
			dataset = append(dataset, map[string]any{
				"trace_id": trace.TraceID,
				"inputs":   inputsMap,
				"outputs":  map[string]any{"expected_trajectory": tools},
			})
		}
	}

	return dataset
}

// ExportToFile exports a generated dataset to a file (JSON or CSV).
func ExportToFile(dataset []map[string]any, outputPath string) {
	ext := strings.ToLower(filepath.Ext(outputPath))

	if ext == ".csv" {
		if len(dataset) == 0 {
			return
		}

		// Collect all keys
		keySet := make(map[string]bool)
		for _, item := range dataset {
			for k := range item {
				keySet[k] = true
			}
		}
		allKeys := make([]string, 0, len(keySet))
		for k := range keySet {
			allKeys = append(allKeys, k)
		}
		sort.Strings(allKeys)

		f, err := os.Create(outputPath)
		if err != nil {
			return
		}
		defer f.Close()

		writer := csv.NewWriter(f)
		_ = writer.Write(allKeys)

		for _, row := range dataset {
			var record []string
			for _, k := range allKeys {
				v := row[k]
				switch val := v.(type) {
				case map[string]any, []any:
					b, _ := json.Marshal(val)
					record = append(record, string(b))
				case string:
					record = append(record, val)
				default:
					record = append(record, fmt.Sprintf("%v", val))
				}
			}
			_ = writer.Write(record)
		}
		writer.Flush()
	} else {
		// Default: JSON
		jsonBytes, _ := json.MarshalIndent(dataset, "", "  ")
		_ = os.WriteFile(outputPath, jsonBytes, 0644)
	}
}

// ExportToLangSmith uploads a generated dataset to LangSmith.
func ExportToLangSmith(ctx context.Context, sdk *langsmith.Client, dataset []map[string]any, datasetName string, datasetType string) {
	// Create or read existing dataset
	ds, err := sdk.Datasets.New(ctx, langsmith.DatasetNewParams{
		Name: langsmith.F(datasetName),
	})
	if err != nil {
		// Dataset might already exist, try to find it
		resp, listErr := sdk.Datasets.List(ctx, langsmith.DatasetListParams{
			Name: langsmith.F(datasetName),
			Limit:       langsmith.F(int64(1)),
		})
		if listErr != nil || len(resp.Items) == 0 {
			return
		}
		ds = &resp.Items[0]
	}

	// Create examples
	for _, ex := range dataset {
		var inputs, outputs map[string]any

		if datasetType == "rag" {
			inputs = map[string]any{
				"question":         ex["question"],
				"retrieved_chunks": ex["retrieved_chunks"],
			}
			outputs = map[string]any{
				"answer":       ex["answer"],
				"cited_chunks": ex["cited_chunks"],
			}
		} else {
			if inp, ok := ex["inputs"].(map[string]any); ok {
				inputs = inp
			} else {
				inputs = map[string]any{}
			}
			if out, ok := ex["outputs"].(map[string]any); ok {
				outputs = out
			} else {
				outputs = map[string]any{}
			}
		}

		params := langsmith.ExampleNewParams{
			DatasetID: langsmith.F(ds.ID),
			Inputs:    langsmith.F(inputs),
		}
		if len(outputs) > 0 {
			params.Outputs = langsmith.F(outputs)
		}

		if _, err := sdk.Examples.New(ctx, params); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to create example: %v\n", err)
		}
	}
}
