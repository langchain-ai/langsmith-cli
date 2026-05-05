package cmd

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	langsmith "github.com/langchain-ai/langsmith-go"
)

func TestNewHubCmd_HasUseAndShort(t *testing.T) {
	cmd := newHubCmd()
	if cmd.Use != "hub" {
		t.Errorf("Use = %q, want %q", cmd.Use, "hub")
	}
	if !strings.Contains(strings.ToLower(cmd.Short), "hub") {
		t.Errorf("Short %q should mention hub", cmd.Short)
	}
}

func TestNewHubCmd_RegisteredOnRoot(t *testing.T) {
	root := NewRootCmd("dev", "dev")
	var found bool
	for _, c := range root.Commands() {
		if c.Use == "hub" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("`hub` not registered on root command")
	}
}

func TestIsHTTP404(t *testing.T) {
	if !isHTTP404(&langsmith.Error{StatusCode: http.StatusNotFound}) {
		t.Error("expected true for typed 404 API error")
	}
	if isHTTP404(&langsmith.Error{StatusCode: http.StatusInternalServerError}) {
		t.Error("expected false for typed 500 API error")
	}
	if isHTTP404(errors.New("HTTP 404: not found")) {
		t.Error("expected false for plain string error")
	}
	if isHTTP404(nil) {
		t.Error("expected false for nil")
	}
}

func TestIsHTTP409(t *testing.T) {
	if !isHTTP409(&langsmith.Error{StatusCode: http.StatusConflict}) {
		t.Error("expected true for typed 409 API error")
	}
	if isHTTP409(&langsmith.Error{StatusCode: http.StatusNotFound}) {
		t.Error("expected false for typed 404 API error")
	}
	if isHTTP409(errors.New("HTTP 409: conflict")) {
		t.Error("expected false for plain string error")
	}
	if isHTTP409(nil) {
		t.Error("expected false for nil")
	}
}

func TestParseHubOwnerRepo(t *testing.T) {
	cases := []struct {
		in        string
		wantOwner string
		wantName  string
		wantRef   string
		wantErr   bool
	}{
		{"my-skill", "-", "my-skill", "", false},
		{"acme/my-skill", "acme", "my-skill", "", false},
		{"acme/my-skill:production", "acme", "my-skill", "production", false},
		{"my-skill:abc123", "-", "my-skill", "abc123", false},
		{"", "", "", "", true},
		{"acme/", "", "", "", true},
		{"/my-skill", "", "", "", true},
		{"acme/x/y", "", "", "", true},
		{"BAD CASE", "", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			owner, name, ref, err := parseHubOwnerRepo(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got owner=%q name=%q ref=%q", tc.in, owner, name, ref)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if owner != tc.wantOwner || name != tc.wantName || ref != tc.wantRef {
				t.Errorf("got (%q,%q,%q), want (%q,%q,%q)", owner, name, ref, tc.wantOwner, tc.wantName, tc.wantRef)
			}
		})
	}
}

func TestHubRepoHandlePattern(t *testing.T) {
	good := []string{"my-skill", "x", "abc_123", "alpha-beta-gamma"}
	bad := []string{"", "1foo", "FOO", "foo bar", "foo/bar", "foo!bar"}
	for _, s := range good {
		if !hubRepoHandlePattern.MatchString(s) {
			t.Errorf("%q should match", s)
		}
	}
	for _, s := range bad {
		if hubRepoHandlePattern.MatchString(s) {
			t.Errorf("%q should NOT match", s)
		}
	}
}
