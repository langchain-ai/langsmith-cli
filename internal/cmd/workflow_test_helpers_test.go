package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

const workflowDataset = "11111111-1111-4111-8111-111111111111"
const workflowExample = "22222222-2222-4222-8222-222222222222"
const workflowTime = "2026-09-16T00:00:00Z"

func workflowFile(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "edit.json")
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
