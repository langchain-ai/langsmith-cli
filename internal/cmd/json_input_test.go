package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func jsonInputForms(t *testing.T, content string) []string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input with spaces.json")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return []string{content, path, "@" + path}
}

func TestJSONInputFormsAcrossReaders(t *testing.T) {
	for name, read := range map[string]func(string, any) error{
		"evaluator": loadJSONFile, "insights": readInsightsJSON, "dataset and rubric": readDatasetEditFile,
	} {
		t.Run(name, func(t *testing.T) {
			for _, input := range jsonInputForms(t, `{"question":"input.message"}`) {
				var got map[string]string
				if err := read(input, &got); err != nil {
					t.Fatal(err)
				}
				if got["question"] != "input.message" {
					t.Fatalf("unexpected value: %#v", got)
				}
			}
		})
	}
	for _, input := range jsonInputForms(t, `{"question":"input.message"}`) {
		if got, err := parseVariableMapping(input); err != nil || got["question"] != "input.message" {
			t.Fatalf("mapping=%v err=%v", got, err)
		}
		if got, err := resourceObject(input); err != nil || got["question"] != "input.message" {
			t.Fatalf("object=%v err=%v", got, err)
		}
	}
}

func TestJSONInputEvaluatorPayloadEquivalence(t *testing.T) {
	prompts := jsonInputForms(t, `[["human","Question: {{question}}"]]`)
	schemas := jsonInputForms(t, `{"type":"object","properties":{"score":{"type":"number"}}}`)
	models := jsonInputForms(t, `{"id":["test-model"]}`)
	var expected map[string]any
	for i := range prompts {
		got, err := buildLLMEvaluatorPayload("test", llmEvaluatorTarget{projectID: deleteTestProjectID}, 1, "", "", prompts[i], schemas[i], models[i], nil)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			expected = got
		} else if !reflect.DeepEqual(got, expected) {
			t.Fatal("file and inline payloads differ")
		}
	}
}

func TestJSONInputBoundsAndSafeErrors(t *testing.T) {
	for _, input := range jsonInputForms(t, `{"private":"secret-marker"}`) {
		if _, err := readJSONInput(input, 5); err == nil {
			t.Fatal("accepted oversized input")
		}
	}
	for _, input := range []string{`{"private":"secret-marker",`, "missing-secret-marker.json"} {
		var got map[string]any
		err := loadJSONFile(input, &got)
		if err == nil || strings.Contains(err.Error(), "secret-marker") {
			t.Fatalf("unsafe error: %v", err)
		}
	}
	for _, read := range []func(string, any) error{readInsightsJSON, readDatasetEditFile, loadJSONFile} {
		for _, input := range []string{`{"x":1,"x":2}`, `{"x":1} {"x":2}`} {
			var got map[string]any
			if err := read(input, &got); err == nil {
				t.Fatal("accepted ambiguous inline JSON")
			}
		}
	}
}
