//go:build !windows

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSandboxConsoleEnvFrom_AllowsTerminalEnvOnly(t *testing.T) {
	env, err := sandboxConsoleEnvFrom([]string{
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"LANG=en_US.UTF-8",
		"LC_ALL=en_US.UTF-8",
		"CLICOLOR=1",
		"CLICOLOR_FORCE=1",
		"FORCE_COLOR=1",
		"NO_COLOR=1",
		"PATH=/usr/bin",
		"LANGSMITH_API_KEY=secret",
		"EMPTY=",
		"malformed",
	}, nil)

	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"TERM":           "xterm-256color",
		"COLORTERM":      "truecolor",
		"LANG":           "en_US.UTF-8",
		"LC_ALL":         "en_US.UTF-8",
		"CLICOLOR":       "1",
		"CLICOLOR_FORCE": "1",
		"FORCE_COLOR":    "1",
		"NO_COLOR":       "1",
	}, env)
}

func TestSandboxConsoleEnvFrom_AddsExplicitEnv(t *testing.T) {
	env, err := sandboxConsoleEnvFrom([]string{
		"TERM=xterm",
		"PATH=/usr/bin",
		"LOCAL_ONLY=from-local",
	}, []string{
		"TERM=xterm-256color",
		"PATH",
		"EMPTY=",
	})

	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"TERM":  "xterm-256color",
		"PATH":  "/usr/bin",
		"EMPTY": "",
	}, env)
}

func TestSandboxConsoleEnvFrom_RejectsMissingForwardedEnv(t *testing.T) {
	_, err := sandboxConsoleEnvFrom(nil, []string{"MISSING"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--env MISSING is not set")
}
