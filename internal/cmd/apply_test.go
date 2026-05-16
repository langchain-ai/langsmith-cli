package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadManifests_SingleFile_MultipleDocuments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "alerts.yaml")
	contents := `apiVersion: langsmith.langchain.com/v1
kind: Alert
metadata:
  name: a
  project: p
spec:
  description: a
  type: threshold
  attribute: error_count
  aggregation: sum
  window_minutes: 15
  operator: gte
  threshold: 10
  actions:
    - target: webhook
      config:
        url: https://example.test/a
---
apiVersion: langsmith.langchain.com/v1
kind: Alert
metadata:
  name: b
  project: p
spec:
  description: b
  type: change
  attribute: total_cost
  aggregation: sum
  window_minutes: 15
  operator: gte
  threshold_window_minutes: 60
  threshold_multiplier: 1.5
  actions:
    - target: webhook
      config:
        url: https://example.test/b
`
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))

	manifests, err := loadManifests([]string{path})
	require.NoError(t, err)
	require.Len(t, manifests, 2)
	assert.Equal(t, "a", manifests[0].Metadata.Name)
	assert.Equal(t, "b", manifests[1].Metadata.Name)
}

func TestLoadManifests_DirectoryGlobsBothExtensions(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.yaml"), minimalAlertYAML("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.yml"), minimalAlertYAML("b"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("nope"), 0o644))

	manifests, err := loadManifests([]string{dir})
	require.NoError(t, err)
	names := []string{}
	for _, m := range manifests {
		names = append(names, m.Metadata.Name)
	}
	assert.ElementsMatch(t, []string{"a", "b"}, names)
}

func TestManifestToWriteRequest_ThresholdAlert(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.yaml")
	require.NoError(t, os.WriteFile(path, minimalAlertYAML("alpha"), 0o644))
	manifests, err := loadManifests([]string{path})
	require.NoError(t, err)
	require.Len(t, manifests, 1)

	req, err := manifestToWriteRequest(manifests[0])
	require.NoError(t, err)
	assert.Equal(t, "alpha", req.Rule.Name)
	assert.Equal(t, "threshold", req.Rule.Type)
	require.NotNil(t, req.Rule.Threshold)
	assert.Equal(t, float64(10), *req.Rule.Threshold)
	require.Len(t, req.Actions, 1)
	// Wire format: config is a JSON string, not a nested object.
	assert.Contains(t, req.Actions[0].Config, `"url":"https://example.test/hook"`)
}

func TestValidateAlertSpec_RejectsBadEnum(t *testing.T) {
	s := AlertSpec{
		Description:   "x",
		Type:          "threshold",
		Attribute:     "not_a_thing",
		Aggregation:   "sum",
		WindowMinutes: 15,
		Operator:      "gte",
		Threshold:     fptr(1),
		Actions:       []AlertActionSpec{{Target: "webhook", Config: map[string]any{}}},
	}
	err := validateAlertSpec(s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "attribute")
}

func TestValidateAlertSpec_ThresholdRequiresThreshold(t *testing.T) {
	s := AlertSpec{
		Description:   "x",
		Type:          "threshold",
		Attribute:     "error_count",
		Aggregation:   "sum",
		WindowMinutes: 15,
		Operator:      "gte",
		Actions:       []AlertActionSpec{{Target: "webhook", Config: map[string]any{}}},
	}
	err := validateAlertSpec(s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "threshold is required")
}

func TestValidateAlertSpec_ChangeRequiresWindowAndMultiplier(t *testing.T) {
	s := AlertSpec{
		Description:   "x",
		Type:          "change",
		Attribute:     "error_count",
		Aggregation:   "sum",
		WindowMinutes: 15,
		Operator:      "gte",
		Actions:       []AlertActionSpec{{Target: "webhook", Config: map[string]any{}}},
	}
	err := validateAlertSpec(s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "threshold_window_minutes")
}

func TestDiffAlert_NoChange(t *testing.T) {
	desired := writeReqWithThreshold("a", 10)
	existing := AlertResponse{
		Rule:    desired.Rule,
		Actions: desired.Actions,
	}
	assert.Empty(t, diffAlert(desired, existing))
}

func TestDiffAlert_ThresholdChange(t *testing.T) {
	desired := writeReqWithThreshold("a", 20)
	existing := AlertResponse{Rule: writeReqWithThreshold("a", 10).Rule, Actions: writeReqWithThreshold("a", 10).Actions}
	got := diffAlert(desired, existing)
	assert.Contains(t, got, "threshold")
}

func TestDiffAlert_ActionsContentChange(t *testing.T) {
	desired := writeReqWithThreshold("a", 10)
	existing := AlertResponse{Rule: desired.Rule, Actions: []AlertAction{{
		Target: "webhook",
		Config: `{"url":"https://example.test/other"}`,
	}}}
	got := diffAlert(desired, existing)
	assert.Contains(t, got, "actions")
}

// --- helpers ---

func minimalAlertYAML(name string) []byte {
	return []byte(`apiVersion: langsmith.langchain.com/v1
kind: Alert
metadata:
  name: ` + name + `
  project: p
spec:
  description: ` + name + `
  type: threshold
  attribute: error_count
  aggregation: sum
  window_minutes: 15
  operator: gte
  threshold: 10
  actions:
    - target: webhook
      config:
        url: https://example.test/hook
`)
}

func writeReqWithThreshold(name string, threshold float64) AlertWriteRequest {
	t := threshold
	return AlertWriteRequest{
		Rule: AlertRule{
			Name:          name,
			Description:   name,
			Type:          "threshold",
			Attribute:     "error_count",
			Aggregation:   "sum",
			WindowMinutes: 15,
			Operator:      "gte",
			Threshold:     &t,
		},
		Actions: []AlertAction{{
			Target: "webhook",
			Config: `{"url":"https://example.test/hook"}`,
		}},
	}
}

func fptr(v float64) *float64 { return &v }
