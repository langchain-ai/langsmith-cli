package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

func TestProductFeedbackCommandMetadata(t *testing.T) {
	cmd := newProductFeedbackCmd("1.2.3")
	if cmd.Use != "feedback <note>" {
		t.Fatalf("Use = %q", cmd.Use)
	}
	if got := cmd.Flags().Lookup("category").DefValue; got != "usability" {
		t.Fatalf("default category = %q", got)
	}
	if !strings.Contains(cmd.Long, "does not collect command") || !strings.Contains(cmd.Long, "Authentication, routing, idempotency") {
		t.Fatalf("privacy disclosure missing from help: %q", cmd.Long)
	}
}

func TestProductFeedbackValidation(t *testing.T) {
	cmd := newProductFeedbackCmd("1.2.3")
	cmd.SetArgs([]string{"  "})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Fatalf("empty note error = %v", err)
	}

	cmd = newProductFeedbackCmd("1.2.3")
	cmd.SetArgs([]string{"--category", "invalid", "note"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "invalid category") {
		t.Fatalf("invalid category error = %v", err)
	}

	cmd = newProductFeedbackCmd("1.2.3")
	cmd.SetArgs([]string{strings.Repeat("a", 4001)})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "at most 4000 characters") {
		t.Fatalf("long note error = %v", err)
	}
}

func TestProductFeedbackSubmission(t *testing.T) {
	var received map[string]any
	var idempotencyKey string
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/platform/product-feedbacks" {
			http.NotFound(w, r)
			return
		}
		idempotencyKey = r.Header.Get("Idempotency-Key")
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"feedback-id","created_at":"2026-04-16T12:00:00Z"}`))
	})
	cleanup := setupTestEnv(t, server.URL)
	defer cleanup()

	cmd := newProductFeedbackCmd("1.2.3")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--category", "bug", "  broken output  "})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if idempotencyKey == "" {
		t.Fatal("Idempotency-Key header missing")
	}
	if received["category"] != "BUG" || received["message"] != "broken output" || received["source"] != "LANGSMITH_CLI" {
		t.Fatalf("request = %#v", received)
	}
	if len(received) != 4 {
		t.Fatalf("unexpected feedback payload fields: %#v", received)
	}
	client, ok := received["client"].(map[string]any)
	if !ok || client["version"] != "1.2.3" || client["os"] == "" || client["architecture"] == "" {
		t.Fatalf("client metadata = %#v", received["client"])
	}
	if !strings.Contains(output.String(), "feedback-id") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestProductFeedbackRateLimit(t *testing.T) {
	calls := 0
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"detail":"rate limit exceeded"}`))
	})
	cleanup := setupTestEnv(t, server.URL)
	defer cleanup()

	cmd := newProductFeedbackCmd("1.2.3")
	cmd.SetArgs([]string{"note"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "retry after 3600") {
		t.Fatalf("rate limit error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("requests = %d, want 1", calls)
	}
}

func TestProductFeedbackUnavailable(t *testing.T) {
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	cleanup := setupTestEnv(t, server.URL)
	defer cleanup()

	cmd := newProductFeedbackCmd("1.2.3")
	cmd.SetArgs([]string{"note"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "unavailable on this LangSmith deployment") {
		t.Fatalf("unavailable error = %v", err)
	}
}

func TestFeedbackHint(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	oldFormat := flagOutputFormat
	t.Cleanup(func() { flagOutputFormat = oldFormat })
	root := &cobra.Command{Use: "langsmith"}
	feedback := &cobra.Command{Use: "feedback"}
	auth := &cobra.Command{Use: "auth"}
	login := &cobra.Command{Use: "login"}
	project := &cobra.Command{Use: "project"}
	root.AddCommand(feedback, auth, project)
	auth.AddCommand(login)

	var output bytes.Buffer
	for _, test := range []struct {
		name string
		cmd  *cobra.Command
	}{
		{name: "root", cmd: root},
		{name: "feedback", cmd: feedback},
		{name: "auth", cmd: auth},
		{name: "auth login", cmd: login},
	} {
		t.Run(test.name, func(t *testing.T) {
			output.Reset()
			test.cmd.SetErr(&output)
			showFeedbackHint(test.cmd)
			if output.Len() != 0 {
				t.Fatalf("hint = %q", output.String())
			}
		})
	}

	flagOutputFormat = "json"
	project.SetErr(&output)
	showFeedbackHint(project)
	if output.Len() != 0 {
		t.Fatalf("JSON hint = %q", output.String())
	}

	flagOutputFormat = "pretty"
	showFeedbackHint(project)
	showFeedbackHint(project)
	if got := strings.Count(output.String(), "langsmith feedback"); got != 1 {
		t.Fatalf("feedback hints = %d, output = %q", got, output.String())
	}
}

func TestWriteProductFeedbackResult(t *testing.T) {
	response := &langsmith.ProductFeedbackNewResponse{
		ID:        "feedback-id",
		CreatedAt: time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC),
	}

	var pretty bytes.Buffer
	if err := writeProductFeedbackResult(&pretty, "pretty", response); err != nil {
		t.Fatal(err)
	}
	if got := pretty.String(); !strings.Contains(got, "feedback-id") || !strings.Contains(got, "2026-04-16T12:00:00Z") {
		t.Fatalf("pretty output = %q", got)
	}

	var jsonOutput bytes.Buffer
	if err := writeProductFeedbackResult(&jsonOutput, "json", response); err != nil {
		t.Fatal(err)
	}
	if got := jsonOutput.String(); !strings.Contains(got, `"id":"feedback-id"`) || !strings.Contains(got, `"created_at":"2026-04-16T12:00:00Z"`) {
		t.Fatalf("JSON output = %q", got)
	}
}
