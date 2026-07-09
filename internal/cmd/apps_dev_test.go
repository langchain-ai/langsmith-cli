package cmd

import (
	"strings"
	"testing"
)

func TestWebAppURLFromAPIURL_StripsAPIPrefix(t *testing.T) {
	got := webAppURLFromAPIURL("https://api.smith.langchain.com")
	if got != "https://smith.langchain.com" {
		t.Errorf("got %q", got)
	}
}

func TestWebAppURLFromAPIURL_SingleOriginPassesThrough(t *testing.T) {
	got := webAppURLFromAPIURL("http://localhost:1980")
	if got != "http://localhost:1980" {
		t.Errorf("got %q", got)
	}
}

func TestBuildAppsDevPreviewURL_IncludesWorkspaceAndLinkInfo(t *testing.T) {
	link := &appLink{AppID: "app_1", ContextType: "annotation_queue"}
	got, err := buildAppsDevPreviewURL("https://smith.langchain.com", "ws_1", "http://localhost:5173", link)
	if err != nil {
		t.Fatalf("buildAppsDevPreviewURL: %v", err)
	}
	if !strings.HasPrefix(got, "https://smith.langchain.com/o/ws_1/custom-apps/dev?") {
		t.Errorf("unexpected base/path: %s", got)
	}
	for _, want := range []string{"dev_url=http%3A%2F%2Flocalhost%3A5173", "app_id=app_1", "context_type=annotation_queue"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in %s", want, got)
		}
	}
}

func TestBuildAppsDevPreviewURL_WorksWithoutLink(t *testing.T) {
	got, err := buildAppsDevPreviewURL("https://smith.langchain.com", "", "http://localhost:5173", nil)
	if err != nil {
		t.Fatalf("buildAppsDevPreviewURL: %v", err)
	}
	if !strings.HasPrefix(got, "https://smith.langchain.com/custom-apps/dev?") {
		t.Errorf("unexpected URL without workspace/link: %s", got)
	}
}

func TestAppsDev_RejectsNonLocalhostURL(t *testing.T) {
	cmd := newAppsCmd()
	cmd.SetArgs([]string{"dev", "--url", "https://evil.example.com", "--no-open"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "localhost") {
		t.Errorf("expected localhost-only error, got %v", err)
	}
}

func TestAppsDev_RequiresURL(t *testing.T) {
	cmd := newAppsCmd()
	cmd.SetArgs([]string{"dev", "--no-open"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when --url is missing")
	}
}
