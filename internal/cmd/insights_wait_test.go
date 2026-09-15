package cmd

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	langsmith "github.com/langchain-ai/langsmith-go"
)

func TestInsightsWaitTransitions(t *testing.T) {
	for _, tc := range []struct {
		name    string
		states  []string
		wantErr bool
	}{
		{"completed", []string{"success"}, false},
		{"progress", []string{"queued", "running", "success"}, false},
		{"failed", []string{"error"}, true},
		{"unknown", []string{"unexpected"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, err := waitForInsight(ctx, time.Millisecond, func(context.Context) (*langsmith.SessionInsightGetJobResponse, error) {
				state := tc.states[calls]
				calls++
				return &langsmith.SessionInsightGetJobResponse{Status: state, Error: "test failure"}, nil
			})
			if (err != nil) != tc.wantErr || calls != len(tc.states) {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestInsightsWaitCancellationAndFetchError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	_, err := waitForInsight(ctx, time.Hour, func(context.Context) (*langsmith.SessionInsightGetJobResponse, error) {
		cancel()
		return &langsmith.SessionInsightGetJobResponse{Status: "running"}, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	want := errors.New("fetch failed")
	_, err = waitForInsight(context.Background(), time.Second, func(context.Context) (*langsmith.SessionInsightGetJobResponse, error) { return nil, want })
	if !errors.Is(err, want) {
		t.Fatal(err)
	}
}

func TestInsightsWaitValidation(t *testing.T) {
	for _, flags := range [][]string{{"--timeout", "0s"}, {"--poll-interval", "0s"}} {
		cmd := newInsightsWaitCmd()
		cmd.SetArgs(append([]string{"job"}, flags...))
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--timeout") {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestInsightsWaitRequestAndReport(t *testing.T) {
	calls := 0
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" || !strings.Contains(r.URL.Path, "/sessions/"+deleteTestProjectID+"/insights/job-id") {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("x-tenant-id") != "demo-workspace" {
			t.Error("missing workspace")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"job-id","status":"success","error":null,"report":null,"clusters":[]}`))
	})
	defer setupTestEnv(t, ts.URL)()
	flagWorkspaceID = "demo-workspace"
	previousFormat := flagOutputFormat
	flagOutputFormat = "json"
	t.Cleanup(func() { flagOutputFormat = previousFormat })
	cmd := newInsightsWaitCmd()
	cmd.SetArgs([]string{"job-id", "--project-id", deleteTestProjectID})
	var err error
	stdout := captureStdout(t, func() { err = cmd.Execute() })
	if err != nil || calls != 1 || !strings.Contains(stdout, `"status": "success"`) {
		t.Fatalf("calls=%d err=%v stdout=%s", calls, err, stdout)
	}
}
