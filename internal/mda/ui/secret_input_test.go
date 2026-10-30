package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
)

func TestStdinReaderKeepsPipedAndRedirectedSecrets(t *testing.T) {
	for input, expected := range map[string]string{
		"sk_live_example\n":   "sk_live_example",
		"sk_live_example\r\n": "sk_live_example",
		"line-one\nline-two":  "line-one\nline-two",
	} {
		value, err := valueFromReader(strings.NewReader(input))

		require.NoError(t, err)
		assert.Equal(t, expected, value)
	}
}

func TestOptionalStdinReaderTreatsAnEmptyPipeAsAbsent(t *testing.T) {
	for _, input := range []string{"", "\n"} {
		_, ok, err := optionalSecretFromReader(strings.NewReader(input))

		require.NoError(t, err)
		assert.False(t, ok, "%q", input)
	}
}

func TestOptionalStdinReaderKeepsANonEmptyPipe(t *testing.T) {
	value, ok, err := optionalSecretFromReader(strings.NewReader("sk_live_example\n"))

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "sk_live_example", value)
}

func TestStdinReaderRejectsEmptyAndDotenvShapedInput(t *testing.T) {
	_, empty := valueFromReader(strings.NewReader("\n"))
	_, dotenv := valueFromReader(strings.NewReader("FOO=1\nBAR=2\n"))

	assert.EqualError(t, empty, "no secret value on stdin")
	assert.EqualError(t, dotenv, "the input looks like a .env file: 2 lines assign a variable, and "+
		"all of it would be stored as one secret. Store one value per secret, "+
		"and use --secret-from-env <VAR> to read just one variable.")
	assert.Equal(t, errs.UserInput, errs.CategoryOf(dotenv))
}

func TestAnEnvSourceNamesWhereTheValueCameFrom(t *testing.T) {
	terminal, _ := captured()
	lookup := func(name string) (string, bool, bool) {
		return "from-dotenv", true, name == "TOKEN"
	}

	resolved, err := terminal.ResolveValue(t.Context(), ValueSource{FromEnv: "TOKEN"}, lookup, "Secret", "cat token | langsmith deploy")
	_, missing := terminal.ResolveValue(t.Context(), ValueSource{FromEnv: "OTHER"}, lookup, "Secret", "cat token | langsmith deploy")

	require.NoError(t, err)
	assert.Equal(t, ResolvedValue{Value: "from-dotenv", Source: "TOKEN in .env"}, resolved)
	assert.EqualError(t, missing, "OTHER is not set in the environment or the project .env")
}

func TestAFileSourceOfDashPointsAtStdin(t *testing.T) {
	terminal, _ := captured()

	_, err := terminal.ResolveValue(t.Context(), ValueSource{FromFile: "-"}, nil, "Secret", "cat token | langsmith deploy")

	assert.EqualError(t, err, "--secret-from-file takes a path. To read the value from stdin, redirect or pipe "+
		"it instead: `cat token | langsmith deploy`.")
}

func TestAFileSourceReadsTheFileWithoutItsNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.txt")
	require.NoError(t, os.WriteFile(path, []byte("sk_live_example\n"), 0o600))
	terminal, _ := captured()

	resolved, err := terminal.ResolveValue(t.Context(), ValueSource{FromFile: path}, nil, "Secret", "")

	require.NoError(t, err)
	assert.Equal(t, ResolvedValue{Value: "sk_live_example", Source: path}, resolved)
}

func TestAnUnspecifiedSourceReadsPipedInput(t *testing.T) {
	terminal := NewTerminal(strings.NewReader("sk_live_example\n"), &strings.Builder{}, WithInteractive(false))

	resolved, err := terminal.ResolveValue(t.Context(), ValueSource{}, nil, "Secret", "")

	require.NoError(t, err)
	assert.Equal(t, ResolvedValue{Value: "sk_live_example", Source: "stdin"}, resolved)
}
