package cmd

import (
	"os"
	"path/filepath"
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

func TestJSONInputFormsAndBounds(t *testing.T) {
	for _, input := range jsonInputForms(t, `{"private":"secret-marker"}`) {
		got, err := readJSONInput(input, 1024)
		if err != nil || string(got) != `{"private":"secret-marker"}` {
			t.Fatal("input forms differ")
		}
		if _, err := readJSONInput(input, 5); err == nil {
			t.Fatal("accepted oversized input")
		}
	}
}
