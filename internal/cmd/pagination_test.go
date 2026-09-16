package cmd

import (
	"encoding/json"
	"testing"
)

func TestResultWorkspaceID(t *testing.T) {
	isolateConfig(t)
	previous := flagWorkspaceID
	t.Cleanup(func() { flagWorkspaceID = previous })
	t.Setenv("LANGSMITH_WORKSPACE_ID", "")
	flagWorkspaceID = ""
	if resultWorkspaceID() != nil {
		t.Fatal("missing workspace should remain null")
	}
	flagWorkspaceID = "11111111-1111-4111-8111-111111111111"
	if got := resultWorkspaceID(); got == nil || *got != flagWorkspaceID {
		t.Fatal("explicit workspace missing")
	}
}

func TestOffsetPageCompleteness(t *testing.T) {
	for _, count := range []int{0, 1, 3} {
		page := describeOffsetPage(count, 3, 6)
		if page.Returned != count {
			t.Fatal("wrong count")
		}
		if count < 3 {
			if page.HasMore == nil || *page.HasMore || page.NextOffset != nil {
				t.Fatalf("short page %+v", page)
			}
		} else if page.HasMore != nil || page.NextOffset == nil || *page.NextOffset != 9 {
			t.Fatalf("full page %+v", page)
		}
		b, err := json.Marshal(page)
		if err != nil {
			t.Fatal(err)
		}
		var obj map[string]any
		if err := json.Unmarshal(b, &obj); err != nil {
			t.Fatal(err)
		}
		if _, ok := obj["has_more"]; !ok {
			t.Fatal("uncertainty field omitted")
		}
	}
}
