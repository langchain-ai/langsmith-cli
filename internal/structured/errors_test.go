package structured

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"testing"

	"github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func apiError(t *testing.T, status int, body string, header http.Header) error {
	t.Helper()
	e := &langsmith.Error{}
	require.NoError(t, e.UnmarshalJSON([]byte(body)))
	e.StatusCode = status
	req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/api/v1/x?api_key=secret", nil)
	req.Header.Set("X-Api-Key", "secret")
	e.Request = req
	e.Response = &http.Response{StatusCode: status, Header: header}
	return fmt.Errorf("listing: %w", e)
}

func TestClassifyAPIErrors(t *testing.T) {
	cases := []struct {
		status   int
		body     string
		category Category
		action   Action
		message  string
		exit     int
	}{
		{400, `{"title":"Bad Request","detail":"page_size must be <= 100","status":400}`, CategoryInvalidInput, ActionFixInput, "page_size must be <= 100", 2},
		{401, `{"detail":"Invalid token"}`, CategoryAuthentication, ActionAuthenticate, "Invalid token", 3},
		{403, `{"error":"forbidden"}`, CategoryPermissionDenied, ActionStop, "forbidden", 4},
		{404, `{"detail":"queue not found"}`, CategoryNotFound, ActionFixInput, "queue not found", 5},
		{409, `{"message":"build in progress"}`, CategoryConflict, ActionStop, "build in progress", 6},
		{422, `{"detail":[{"loc":["body","name"],"msg":"field required"}]}`, CategoryInvalidInput, ActionFixInput, "field required", 2},
		{422, `{"detail":["body: Value error, session_id or dataset_id is required"]}`, CategoryInvalidInput, ActionFixInput, "session_id or dataset_id is required", 2},
		{429, `{}`, CategoryRateLimited, ActionRetry, "Too Many Requests", 7},
		{503, `not json`, CategoryServer, ActionRetry, "Service Unavailable", 8},
		{504, `{}`, CategoryTimeout, ActionRetry, "Gateway Timeout", 9},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			e := Classify(apiError(t, tc.status, tc.body, http.Header{"X-Request-Id": {"req-1"}}))
			require.Equal(t, tc.category, e.Category)
			require.Equal(t, tc.action, e.Action)
			require.Equal(t, tc.message, e.Message)
			require.Equal(t, tc.status, e.Status)
			require.Equal(t, "req-1", e.RequestID)
			require.Equal(t, tc.exit, e.ExitCode())
			require.NotContains(t, e.Message, "secret")
		})
	}
}

func TestClassifyUsesRemedyAsHint(t *testing.T) {
	e := Classify(apiError(t, 400, `{"detail":"bad filter","remedy":"quote string values"}`, http.Header{}))
	require.Equal(t, "quote string values", e.Hint)

	e = Classify(apiError(t, 401, `{}`, http.Header{}))
	require.Contains(t, e.Hint, "langsmith auth login")
}

func TestClassifyTransportErrors(t *testing.T) {
	require.Equal(t, CategoryTimeout, Classify(fmt.Errorf("x: %w", context.DeadlineExceeded)).Category)

	dial := &url.Error{Op: "Get", URL: "https://api.example.com", Err: &net.OpError{Op: "dial", Err: errors.New("refused")}}
	e := Classify(dial)
	require.Equal(t, CategoryNetwork, e.Category)
	require.True(t, e.Retryable)
	require.NotContains(t, e.Message, "api.example.com")

	e = Classify(errors.New("boom"))
	require.Equal(t, CategoryUnknown, e.Category)
	require.Equal(t, 1, e.ExitCode())
	require.Equal(t, "boom", e.Message)
}

func TestClassifyKeepsStructuredErrors(t *testing.T) {
	orig := NewError(CategoryConflict, "already exists")
	require.Same(t, orig, Classify(fmt.Errorf("wrap: %w", orig)))
}

func TestWriteErrorJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := testCmd("json", &stdout)
	err := Classify(apiError(t, 404, `{"detail":"queue not found"}`, http.Header{"X-Request-Id": {"req-1"}}))

	code := WriteError(&stdout, &stderr, cmd, err)

	require.Equal(t, 5, code)
	require.Empty(t, stdout.String())
	var got map[string]map[string]any
	require.NoError(t, json.Unmarshal(stderr.Bytes(), &got))
	require.Equal(t, map[string]any{
		"category":   "not_found",
		"message":    "queue not found",
		"action":     "fix_input",
		"retryable":  false,
		"status":     float64(404),
		"request_id": "req-1",
	}, got["error"])
}

func TestWriteErrorPretty(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := testCmd("pretty", &stdout)
	err := NewError(CategoryInvalidInput, "missing --queue-id")
	err.Hint = "pass the queue UUID"

	require.Equal(t, 2, WriteError(&stdout, &stderr, cmd, err))
	require.Empty(t, stdout.String())
	require.Equal(t, "Error: missing --queue-id\nHint: pass the queue UUID\n", stderr.String())
}

func TestWriteErrorLegacyErrorsUnchanged(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := testCmd("json", &stdout)

	require.Equal(t, 1, WriteError(&stdout, &stderr, cmd, errors.New("old failure")))
	require.Equal(t, "old failure\n", stdout.String())
	require.Empty(t, stderr.String())
}

func TestCommandClassifiesActionAndArgErrors(t *testing.T) {
	newCmd := func() *cobra.Command {
		cmd := Command[struct{}]{
			Use:  "get <id>",
			Args: cobra.ExactArgs(1),
			Action: func(context.Context, *cobra.Command, struct{}, []string) (any, error) {
				return nil, context.DeadlineExceeded
			},
		}.Cobra()
		cmd.PersistentFlags().String("format", "json", "")
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		return cmd
	}

	cmd := newCmd()
	cmd.SetArgs([]string{"abc"})
	require.Equal(t, CategoryTimeout, Classify(cmd.Execute()).Category)

	cmd = newCmd()
	cmd.SetArgs([]string{})
	require.Equal(t, CategoryInvalidInput, Classify(cmd.Execute()).Category)

	cmd = newCmd()
	cmd.SetArgs([]string{"abc", "--nope"})
	require.Equal(t, CategoryInvalidInput, Classify(cmd.Execute()).Category)
}

func TestWantsJSONFromRawArgs(t *testing.T) {
	require.True(t, wantsJSON(nil, []string{"list", "--nope", "--format", "json"}))
	require.True(t, wantsJSON(nil, []string{"list", "--nope", "--format=json"}))
	require.True(t, wantsJSON(nil, []string{"list", "--jq", ".items"}))
	require.False(t, wantsJSON(nil, []string{"list", "--format", "pretty"}))
	require.False(t, wantsJSON(nil, []string{"exec", "--", "--format", "json"}))
}
