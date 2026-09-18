package cmd

import (
	"strings"
	"testing"
)

func TestEmptyResultGuidancePreservesFields(t *testing.T) {
	for _, count := range []int{0, 1} {
		result := map[string]any{"items": []string{}, "offset": 20, "next_cursor": "next"}
		emptyResultGuidance(result, count, "No results on this page.", "Check the offset.")
		if result["offset"] != 20 || result["next_cursor"] != "next" {
			t.Fatal("pagination changed")
		}
		if (result["message"] != nil) != (count == 0) {
			t.Fatal("incorrect empty guidance")
		}
	}
}

func TestReadNextStepQuotesCustomConfig(t *testing.T) {
	defer setupTestEnv(t, "https://example.invalid")()
	path := "missing config's $(echo unsafe).json"
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	got := readNextStep("model", "list")
	if !strings.HasPrefix(got, "env "+shellQuote("LANGSMITH_CONFIG_FILE="+path)+" langsmith ") {
		t.Fatalf("custom config must be a single quoted argument: %s", got)
	}
	t.Setenv("LANGSMITH_CONFIG_FILE", "")
	if got := readNextStep("model", "list"); !strings.HasPrefix(got, "langsmith ") {
		t.Fatalf("unexpected environment prefix: %s", got)
	}
}
