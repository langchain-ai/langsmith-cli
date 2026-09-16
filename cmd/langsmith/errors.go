package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	langsmith "github.com/langchain-ai/langsmith-go"
)

type commandErrorEnvelope struct {
	Error commandErrorDetail `json:"error"`
}

type commandErrorDetail struct {
	Code       string   `json:"code"`
	Message    string   `json:"message"`
	HTTPStatus *int     `json:"http_status,omitempty"`
	NextSteps  []string `json:"next_steps"`
}

// Cobra can reject an unknown command before parsing persistent flags. Inspect
// only explicit output-format arguments so those failures still honor JSON mode.
func errorOutputFormat(args []string, fallback string) string {
	format := fallback
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if arg == "--format" && i+1 < len(args) {
			i++
			format = args[i]
		} else if value, ok := strings.CutPrefix(arg, "--format="); ok {
			format = value
		}
	}
	return format
}

// JSON diagnostics deliberately exclude upstream error text: it can contain
// request URLs, credentials, user inputs, or server response bodies.
func writeCommandError(w io.Writer, err error, format string) error {
	var diagnostic interface {
		CLIDiagnostic() (string, string, string)
	}
	if format != "json" {
		if errors.As(err, &diagnostic) {
			_, message, next := diagnostic.CLIDiagnostic()
			if next != "" {
				_, writeErr := fmt.Fprintf(w, "%s\nNext: %s\n", message, next)
				return writeErr
			}
			_, writeErr := fmt.Fprintln(w, message)
			return writeErr
		}
		_, writeErr := fmt.Fprintln(w, err.Error())
		return writeErr
	}
	code, message := "command_failed", "The command did not complete successfully."
	next := "Check command --help and supplied arguments. Inspect any partial result and remote state before retrying a write."
	var status *int
	var apiErr *langsmith.Error
	var networkErr net.Error
	switch {
	case errors.As(err, &diagnostic):
		code, message, next = diagnostic.CLIDiagnostic()
	case errors.Is(err, context.Canceled):
		code, message = "canceled", "The command was canceled."
	case errors.Is(err, context.DeadlineExceeded):
		code, message = "timeout", "The command exceeded its deadline."
	case errors.As(err, &apiErr):
		status = &apiErr.StatusCode
		switch apiErr.StatusCode {
		case 400, 422:
			code, message = "invalid_request", "The API rejected the request parameters."
		case 401:
			code, message = "unauthenticated", "Authentication is required or expired."
			next = "Run langsmith auth login or configure an API-key profile, then verify the selected profile."
		case 403:
			code, message = "permission_denied", "The selected identity cannot perform this operation."
			next = "Verify the selected profile, workspace, and resource permissions."
		case 404:
			code, message = "not_found", "The API resource or endpoint was not found."
			next = "Verify resource IDs, workspace, API endpoint, and deployment capabilities."
		case 409:
			code, message = "conflict", "The request conflicts with existing state."
		case 429:
			code, message = "rate_limited", "The API rate limit was exceeded."
		default:
			code, message = "api_error", "The API request failed."
		}
	case errors.As(err, &networkErr):
		code, message = "network_error", "The API connection failed."
	}
	return json.NewEncoder(w).Encode(commandErrorEnvelope{
		Error: commandErrorDetail{
			Code:       code,
			Message:    message,
			HTTPStatus: status,
			NextSteps:  []string{next},
		},
	})
}
