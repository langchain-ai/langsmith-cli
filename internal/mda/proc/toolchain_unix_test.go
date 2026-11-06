//go:build unix

package proc

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeUsablePythonStub(t *testing.T, python string) {
	t.Helper()
	require.NoError(t, os.WriteFile(python, []byte("#!/bin/sh\necho Python 3.11.0\n"), 0o755))
}
