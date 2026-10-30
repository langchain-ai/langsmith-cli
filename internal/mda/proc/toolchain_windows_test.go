//go:build windows

package proc

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeUsablePythonStub copies the host interpreter, since Windows runs no
// script as an executable.
func writeUsablePythonStub(t *testing.T, python string) {
	t.Helper()
	out, err := exec.Command(Python3Program(), "-c", "import sys; print(sys.executable)").Output()
	require.NoError(t, err, "host python executable lookup failed")
	host, err := os.ReadFile(strings.TrimSpace(string(out)))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(python, host, 0o755))
}
