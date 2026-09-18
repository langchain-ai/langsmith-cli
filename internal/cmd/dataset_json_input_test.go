package cmd

import (
	"encoding/json"
	"testing"
)

func TestJSONInputSelectionAndAssertions(t *testing.T) {
	selection := testTraceSelection()
	encoded, err := json.Marshal(selection)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range jsonInputForms(t, string(encoded)) {
		got, err := readTraceSelection(input)
		if err != nil || got.DatasetID != selection.DatasetID || len(got.Examples) != len(selection.Examples) {
			t.Fatalf("selection=%#v err=%v", got, err)
		}
	}
	for _, input := range jsonInputForms(t, `[{"key":"helpful","comment":"Respond to the request"}]`) {
		got, err := readDatasetAssertions(input)
		if err != nil || len(got) != 1 {
			t.Fatalf("assertions=%#v err=%v", got, err)
		}
	}
}
