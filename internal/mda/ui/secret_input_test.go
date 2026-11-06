package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
)

func writeSecretFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secret.txt")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	return path
}

func TestAFileValueDropsOneTrailingNewline(t *testing.T) {
	for contents, expected := range map[string]string{
		"sk_live_example\n":   "sk_live_example",
		"sk_live_example\r\n": "sk_live_example",
		"line-one\nline-two":  "line-one\nline-two",
	} {
		value, err := ValueFromFile(writeSecretFile(t, contents))

		require.NoError(t, err)
		assert.Equal(t, expected, value)
	}
}

func TestAFileValueRejectsEmptyAndDotenvShapedFiles(t *testing.T) {
	emptyPath := writeSecretFile(t, "\n")
	dotenvPath := writeSecretFile(t, "FOO=1\nBAR=2\n")

	_, empty := ValueFromFile(emptyPath)
	_, dotenv := ValueFromFile(dotenvPath)

	assert.EqualError(t, empty, "no secret value in "+emptyPath)
	assert.EqualError(t, dotenv, dotenvPath+" looks like a .env file: 2 lines assign a variable, and "+
		"all of it would be stored as one secret. Store one value per secret, "+
		"and use --secret-from-env <VAR> to read just one variable.")
	assert.Equal(t, errs.UserInput, errs.CategoryOf(dotenv))
}
