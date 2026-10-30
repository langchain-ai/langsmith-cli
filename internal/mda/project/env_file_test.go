package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseHandlesCommentsQuotesAndExport(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	require.NoError(t, os.WriteFile(path, []byte("# comment\nexport FOO=bar\nBAZ=\"quoted value\"\nEMPTY=\n"), 0o644))

	assert.Equal(t, map[string]string{"FOO": "bar", "BAZ": "quoted value", "EMPTY": ""}, ParseEnvFile(path))
}

func TestParseStripsInlineCommentsFromUnquotedValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	require.NoError(t, os.WriteFile(path, []byte("TOKEN=abc123 # prod key\nHASHY=abc#123\n"+
		"QUOTED=\"abc123 # keep\"\nQUOTED_TRAIL=\"abc123\" # drop\nSINGLE='x # y'\n"), 0o644))

	assert.Equal(t, map[string]string{
		"TOKEN":        "abc123",
		"HASHY":        "abc#123",
		"QUOTED":       "abc123 # keep",
		"QUOTED_TRAIL": "abc123",
		"SINGLE":       "x # y",
	}, ParseEnvFile(path))
}

func TestParseMissingFileIsEmpty(t *testing.T) {
	assert.Empty(t, ParseEnvFile(filepath.Join(t.TempDir(), ".env")))
}
