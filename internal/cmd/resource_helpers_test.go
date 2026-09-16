package cmd

import (
	"errors"
	"testing"
)

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
