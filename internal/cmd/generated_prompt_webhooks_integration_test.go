//go:build integration

package cmd

import (
	"testing"
)

// TestPromptWebhooksLifecycleIntegration creates, reads, updates, lists, and
// deletes a webhook. It creates no prompt commits, so the webhook never fires.
func TestPromptWebhooksLifecycleIntegration(t *testing.T) {
	requireIntegrationEnv(t)
	url := "https://example.com/" + randomHandle("langsmith-cli-it")

	out, code := runGeneratedLive(t, "prompt-webhooks", "create", "--url", url, "--trigger", "commit")
	if code != 0 {
		t.Fatalf("create exited %d", code)
	}
	created := decodeLive[map[string]any](t, out)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("create returned no id: %s", out)
	}
	deleted := false
	t.Cleanup(func() {
		if !deleted {
			runGeneratedLive(t, "prompt-webhooks", "delete", "--webhook-id", id, "--yes")
		}
	})

	out, code = runGeneratedLive(t, "prompt-webhooks", "retrieve", "--webhook-id", id)
	if got := decodeLive[map[string]any](t, out)["url"]; code != 0 || got != url {
		t.Fatalf("retrieve: exit %d, url %v, want %s", code, got, url)
	}

	newURL := url + "/updated"
	out, code = runGeneratedLive(t, "prompt-webhooks", "update", "--webhook-id", id, "--url", newURL)
	if got := decodeLive[map[string]any](t, out)["url"]; code != 0 || got != newURL {
		t.Fatalf("update: exit %d, url %v, want %s", code, got, newURL)
	}

	out, code = runGeneratedLive(t, "prompt-webhooks", "list")
	found := false
	for _, webhook := range decodeLive[[]map[string]any](t, out) {
		found = found || webhook["id"] == id
	}
	if code != 0 || !found {
		t.Fatalf("list: exit %d, webhook %s present = %v", code, id, found)
	}

	if _, code = runGeneratedLive(t, "prompt-webhooks", "delete", "--webhook-id", id, "--yes"); code != 0 {
		t.Fatalf("delete exited %d", code)
	}
	deleted = true

	if _, code = runGeneratedLive(t, "prompt-webhooks", "retrieve", "--webhook-id", id); code == 0 {
		t.Fatalf("retrieve after delete succeeded, want an error")
	}
}
