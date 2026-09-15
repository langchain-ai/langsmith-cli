package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	langsmith "github.com/langchain-ai/langsmith-go"
)

func TestCommandJSONErrors(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code string
	}{
		{errors.New("secret-token https://user:password@example.org customer-input"), "command_failed"},
		{context.Canceled, "canceled"},
		{context.DeadlineExceeded, "timeout"},
		{&langsmith.Error{StatusCode: 401}, "unauthenticated"},
		{&langsmith.Error{StatusCode: 403}, "permission_denied"},
		{&langsmith.Error{StatusCode: 404}, "not_found"},
		{&langsmith.Error{StatusCode: 409}, "conflict"},
		{&langsmith.Error{StatusCode: 422}, "invalid_request"},
		{&langsmith.Error{StatusCode: 429}, "rate_limited"},
		{&langsmith.Error{StatusCode: 500}, "api_error"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeCommandError(&out, fmt.Errorf("secret-token: %w", tc.err), "json"); err != nil {
				t.Fatal(err)
			}
			var result struct {
				Error struct {
					Code string   `json:"code"`
					Next []string `json:"next_steps"`
				} `json:"error"`
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Error.Code != tc.code || len(result.Error.Next) == 0 {
				t.Fatalf("invalid diagnostic: %s", out.String())
			}
			for _, secret := range []string{"secret-token", "password", "customer-input"} {
				if strings.Contains(out.String(), secret) {
					t.Fatal("sensitive error text leaked")
				}
			}
		})
	}
}

func TestCommandPrettyError(t *testing.T) {
	var out bytes.Buffer
	if err := writeCommandError(&out, errors.New("missing --dataset"), "pretty"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "missing --dataset\n" {
		t.Fatal(out.String())
	}
}

type testDiagnostic struct{}

func TestErrorOutputFormatBeforeFlagParsing(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--format", "json", "unknown-command"}, "json"},
		{[]string{"unknown-command", "--format=json"}, "json"},
		{[]string{"--", "--format=json"}, "pretty"},
		{[]string{"--format=json", "--format=pretty"}, "pretty"},
	} {
		if got := errorOutputFormat(tc.args, "pretty"); got != tc.want {
			t.Fatalf("%v got %s want %s", tc.args, got, tc.want)
		}
	}
}

func (testDiagnostic) Error() string { return "secret-value" }
func (testDiagnostic) CLIDiagnostic() (string, string, string) {
	return "invalid_resource_id", "Invalid resource ID.", "Use the resource UUID."
}

func TestCommandTypedDiagnostic(t *testing.T) {
	var out bytes.Buffer
	if err := writeCommandError(&out, fmt.Errorf("sensitive wrapper: %w", testDiagnostic{}), "json"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"code":"invalid_resource_id"`) || strings.Contains(out.String(), "secret-value") || strings.Contains(out.String(), "sensitive wrapper") {
		t.Fatalf("bad diagnostic: %s", out.String())
	}
}
