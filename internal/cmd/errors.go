package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	langsmith "github.com/langchain-ai/langsmith-go"
)

type apiErrorBody struct {
	Error            string          `json:"error"`
	Message          string          `json:"message"`
	ErrorDescription string          `json:"error_description"`
	Detail           json.RawMessage `json:"detail"`
}

type apiValidationDetail struct {
	Loc  []any  `json:"loc"`
	Msg  string `json:"msg"`
	Type string `json:"type"`
}

func FormatErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	var apiErr *langsmith.Error
	if !errors.As(err, &apiErr) {
		return err.Error()
	}

	message := formatAPIError(apiErr)
	if message == "" {
		return err.Error()
	}
	if prefix := strings.TrimSuffix(err.Error(), apiErr.Error()); prefix != err.Error() {
		return prefix + message
	}
	return message
}

func formatAPIError(err *langsmith.Error) string {
	statusCode := err.StatusCode
	if statusCode == 0 && err.Response != nil {
		statusCode = err.Response.StatusCode
	}
	message := formatAPIErrorBody([]byte(err.JSON.RawJSON()))
	if statusCode == 0 {
		return message
	}
	status := http.StatusText(statusCode)
	if status == "" {
		status = "HTTP"
	}
	if message == "" {
		return fmt.Sprintf("%d %s", statusCode, status)
	}
	return fmt.Sprintf("%d %s: %s", statusCode, status, message)
}

func formatAPIErrorBody(body []byte) string {
	var parsed apiErrorBody
	if err := json.Unmarshal(body, &parsed); err != nil {
		return strings.TrimSpace(string(body))
	}
	if message := formatAPIErrorDetail(parsed.Detail); message != "" {
		return message
	}
	for _, message := range []string{parsed.Message, parsed.ErrorDescription, parsed.Error} {
		if message = strings.TrimSpace(message); message != "" {
			return message
		}
	}
	return strings.TrimSpace(string(body))
}

func formatAPIErrorDetail(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var message string
	if err := json.Unmarshal(raw, &message); err == nil {
		return strings.TrimSpace(message)
	}
	var details []apiValidationDetail
	if err := json.Unmarshal(raw, &details); err == nil {
		parts := make([]string, 0, len(details))
		for _, detail := range details {
			msg := strings.TrimSpace(detail.Msg)
			if msg == "" {
				continue
			}
			if loc := formatValidationLoc(detail.Loc); loc != "" {
				msg = loc + ": " + msg
			}
			parts = append(parts, msg)
		}
		return strings.Join(parts, "; ")
	}
	return strings.TrimSpace(string(raw))
}

func formatValidationLoc(loc []any) string {
	parts := make([]string, 0, len(loc))
	for _, part := range loc {
		value := strings.TrimSpace(fmt.Sprint(part))
		if value == "" || value == "body" {
			continue
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, ".")
}
