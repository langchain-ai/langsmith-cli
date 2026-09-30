package structured

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	"github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

// Category classifies a failure so callers can decide what to do next.
type Category string

const (
	CategoryInvalidInput     Category = "invalid_input"
	CategoryAuthentication   Category = "authentication"
	CategoryPermissionDenied Category = "permission_denied"
	CategoryNotFound         Category = "not_found"
	CategoryConflict         Category = "conflict"
	CategoryRateLimited      Category = "rate_limited"
	CategoryServer           Category = "server_error"
	CategoryTimeout          Category = "timeout"
	CategoryNetwork          Category = "network"
	CategoryUnknown          Category = "error"
)

// Action is the suggested next step for an agent or script.
type Action string

const (
	ActionRetry        Action = "retry"
	ActionFixInput     Action = "fix_input"
	ActionAuthenticate Action = "authenticate"
	ActionStop         Action = "stop"
)

type categoryInfo struct {
	exitCode int
	action   Action
}

// Exit codes are part of the CLI contract; do not renumber.
var categories = map[Category]categoryInfo{
	CategoryUnknown:          {1, ActionStop},
	CategoryInvalidInput:     {2, ActionFixInput},
	CategoryAuthentication:   {3, ActionAuthenticate},
	CategoryPermissionDenied: {4, ActionStop},
	CategoryNotFound:         {5, ActionFixInput},
	CategoryConflict:         {6, ActionStop},
	CategoryRateLimited:      {7, ActionRetry},
	CategoryServer:           {8, ActionRetry},
	CategoryTimeout:          {9, ActionRetry},
	CategoryNetwork:          {10, ActionRetry},
}

const maxMessageLen = 500

// Error is the structured failure returned by structured commands.
type Error struct {
	Category  Category `json:"category"`
	Message   string   `json:"message"`
	Action    Action   `json:"action"`
	Retryable bool     `json:"retryable"`
	Status    int      `json:"status,omitempty"`
	RequestID string   `json:"request_id,omitempty"`
	Hint      string   `json:"hint,omitempty"`
	cause     error
}

func NewError(category Category, message string) *Error {
	info, ok := categories[category]
	if !ok {
		category, info = CategoryUnknown, categories[CategoryUnknown]
	}
	return &Error{
		Category:  category,
		Message:   message,
		Action:    info.action,
		Retryable: info.action == ActionRetry,
	}
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.cause }

// ExitCode is the process exit code for this error's category.
func (e *Error) ExitCode() int { return categories[e.Category].exitCode }

// Classify converts SDK, context, and network errors into an *Error.
// Messages come from parsed response fields only, never the raw request.
func Classify(err error) *Error {
	if err == nil {
		return nil
	}
	var se *Error
	if errors.As(err, &se) {
		return se
	}
	var apiErr *langsmith.Error
	if errors.As(err, &apiErr) {
		return fromAPIError(apiErr, err)
	}
	var e *Error
	var netErr net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		e = NewError(CategoryTimeout, "request timed out")
	case errors.Is(err, context.Canceled):
		e = NewError(CategoryUnknown, "request canceled")
	case errors.As(err, &netErr) && netErr.Timeout():
		e = NewError(CategoryTimeout, "request timed out")
	case errors.As(err, &netErr):
		e = NewError(CategoryNetwork, "could not reach the LangSmith API; check --api-url and your network")
	default:
		e = NewError(CategoryUnknown, err.Error())
	}
	e.cause = err
	return e
}

func fromAPIError(apiErr *langsmith.Error, cause error) *Error {
	status := apiErr.StatusCode
	e := NewError(statusCategory(status), "")
	e.Status = status
	e.cause = cause
	if apiErr.Response != nil {
		e.RequestID = apiErr.Response.Header.Get("X-Request-Id")
	}
	message, hint := problemFields(apiErr.JSON.RawJSON())
	if message == "" {
		message = http.StatusText(status)
	}
	if status == http.StatusTooManyRequests || status >= 500 {
		e.Retryable = true
	}
	e.Message = truncate(message)
	e.Hint = truncate(hint)
	if e.Hint == "" {
		switch e.Category {
		case CategoryAuthentication:
			e.Hint = "run 'langsmith auth login' or set LANGSMITH_API_KEY"
		case CategoryPermissionDenied:
			e.Hint = "check that the API key is valid and can access this workspace (--workspace)"
		}
	}
	return e
}

func statusCategory(status int) Category {
	switch {
	case status == http.StatusBadRequest || status == http.StatusUnprocessableEntity:
		return CategoryInvalidInput
	case status == http.StatusUnauthorized:
		return CategoryAuthentication
	case status == http.StatusForbidden:
		return CategoryPermissionDenied
	case status == http.StatusNotFound:
		return CategoryNotFound
	case status == http.StatusConflict:
		return CategoryConflict
	case status == http.StatusRequestTimeout || status == http.StatusGatewayTimeout:
		return CategoryTimeout
	case status == http.StatusTooManyRequests:
		return CategoryRateLimited
	case status >= 500:
		return CategoryServer
	case status >= 400:
		return CategoryInvalidInput
	default:
		return CategoryUnknown
	}
}

// problemFields reads RFC 7807 bodies and the older {detail}/{error}/{message} shapes.
func problemFields(raw string) (message, hint string) {
	var body map[string]any
	if json.Unmarshal([]byte(raw), &body) != nil {
		return "", ""
	}
	for _, key := range []string{"detail", "title", "error", "message"} {
		if s, ok := body[key].(string); ok && s != "" {
			message = s
			break
		}
	}
	// FastAPI validation errors: {"detail": [{"msg": "..."}]} or {"detail": ["body: ..."]}.
	if message == "" {
		if items, ok := body["detail"].([]any); ok {
			var msgs []string
			for _, item := range items {
				switch v := item.(type) {
				case string:
					msgs = append(msgs, trimValidationPrefix(v))
				case map[string]any:
					if s, ok := v["msg"].(string); ok {
						msgs = append(msgs, trimValidationPrefix(s))
					}
				}
			}
			message = strings.Join(msgs, "; ")
		}
	}
	hint, _ = body["remedy"].(string)
	return message, hint
}

// wantsJSON also scans the raw arguments: when flag parsing fails early, a
// later --format or --jq was never parsed.
func wantsJSON(cmd *cobra.Command, args []string) bool {
	if cmd != nil && (cmdutil.ResolveFormat(cmd) == "json" || cmdutil.ResolveJQ(cmd) != "") {
		return true
	}
	for i, arg := range args {
		switch {
		case arg == "--":
			return false
		case arg == "--format=json", strings.HasPrefix(arg, "--jq="):
			return true
		case (arg == "--format" && i+1 < len(args) && args[i+1] == "json") || arg == "--jq":
			return true
		}
	}
	return false
}

func trimValidationPrefix(s string) string {
	s = strings.TrimPrefix(s, "body: ")
	return strings.TrimPrefix(s, "Value error, ")
}

func truncate(s string) string {
	if len(s) <= maxMessageLen {
		return s
	}
	return s[:maxMessageLen] + "..."
}

// WriteError reports err and returns the exit code. Structured errors go to
// stderr, as JSON when the command was asked for JSON or --jq. Other errors
// keep the CLI's established behavior: the message on stdout, exit code 1.
func WriteError(stdout, stderr io.Writer, cmd *cobra.Command, err error) int {
	var se *Error
	if !errors.As(err, &se) {
		fmt.Fprintln(stdout, err.Error())
		return 1
	}
	if wantsJSON(cmd, os.Args[1:]) {
		enc := json.NewEncoder(stderr)
		_ = enc.Encode(map[string]*Error{"error": se})
		return se.ExitCode()
	}
	fmt.Fprintf(stderr, "Error: %s\n", se.Message)
	if se.Hint != "" {
		fmt.Fprintf(stderr, "Hint: %s\n", se.Hint)
	}
	if se.RequestID != "" {
		fmt.Fprintf(stderr, "Request ID: %s\n", se.RequestID)
	}
	return se.ExitCode()
}
