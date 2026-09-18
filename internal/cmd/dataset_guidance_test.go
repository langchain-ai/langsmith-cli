package cmd

import (
	"strings"
	"testing"
)

func TestDatasetAddMissingPlanDiagnostic(t *testing.T) {
	cmd := newDatasetAddCmd()
	cmd.SetArgs([]string{"--dataset", workflowDataset})
	err := cmd.Execute()
	d, ok := err.(commandDiagnostic)
	if !ok || d.code != "selection_required" || !strings.Contains(d.next, "Preview") {
		t.Fatalf("missing actionable diagnostic: %v", err)
	}
}
