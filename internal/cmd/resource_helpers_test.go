package cmd

import (
	"errors"
	"strings"
	"testing"
)

func TestSameResourceID(t *testing.T) {
	const id = "abcdef12-1234-4567-89ab-123456789abc"
	for _, tc := range []struct {
		left, right string
		equal       bool
	}{
		{id, id, true},
		{id, strings.ToUpper(id), true},
		{id, "abcdef12-1234-4567-89ab-123456789abd", false},
		{"", "", false},
		{id, "invalid", false},
	} {
		if got := sameResourceID(tc.left, tc.right); got != tc.equal {
			t.Errorf("sameResourceID(%q, %q) = %v; want %v", tc.left, tc.right, got, tc.equal)
		}
	}
}

func TestResourceDiagnostics(t *testing.T) {
	_, objectErr := resourceObject(`{"secret":`)
	for _, tc := range []struct {
		err  error
		code string
	}{
		{resourceUUID("secret-value"), "invalid_resource_id"},
		{objectErr, "invalid_json_object"},
	} {
		var diagnostic interface {
			CLIDiagnostic() (string, string, string)
		}
		if !errors.As(tc.err, &diagnostic) {
			t.Fatalf("missing typed diagnostic: %v", tc.err)
		}
		code, message, next := diagnostic.CLIDiagnostic()
		if code != tc.code || message == "" || next == "" {
			t.Fatalf("invalid diagnostic %q %q %q", code, message, next)
		}
	}
}
