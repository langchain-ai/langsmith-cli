package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestConfirmDelete(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "yes", input: "yes\n"},
		{name: "short yes", input: "y\n"},
		{name: "empty response", input: "\n", wantErr: true},
		{name: "EOF", input: "", wantErr: true},
		{name: "yes followed by EOF", input: "yes", wantErr: true},
		{name: "explicit no", input: "n\n", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			var stderr bytes.Buffer
			cmd.SetIn(strings.NewReader(tt.input))
			cmd.SetErr(&stderr)

			err := confirmDelete(cmd, deleteConfirmation{
				target:   "a resource and its data",
				identity: `Resource: "production"`,
			})
			if (err != nil) != tt.wantErr {
				t.Fatalf("confirmDelete() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !strings.Contains(stderr.String(), "Continue? [y/N]") {
				t.Fatalf("confirmation prompt missing: %s", stderr.String())
			}
		})
	}
}
