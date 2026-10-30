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

func TestSetAppendsThenReplaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	require.NoError(t, os.WriteFile(path, []byte("EXISTING=1\n"), 0o644))

	require.NoError(t, SetEnvKey(path, "LANGSMITH_API_KEY", "ls-abc"))
	assert.Equal(t, map[string]string{"EXISTING": "1", "LANGSMITH_API_KEY": "ls-abc"}, ParseEnvFile(path))

	require.NoError(t, SetEnvKey(path, "LANGSMITH_API_KEY", "ls-xyz"))
	assert.Equal(t, map[string]string{"EXISTING": "1", "LANGSMITH_API_KEY": "ls-xyz"}, ParseEnvFile(path))
	// The key was replaced in place, not duplicated.
	assert.Equal(t, "EXISTING=1\nLANGSMITH_API_KEY=ls-xyz\n", readText(t, path))
}

func TestSetEnvKeyReplacesAnExportedKeyAndPreservesOtherLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	require.NoError(t, os.WriteFile(path, []byte("# keep me\r\n  export FOO = old\r\n\r\nOTHER=1"), 0o644))

	require.NoError(t, SetEnvKey(path, "FOO", "new"))

	assert.Equal(t, "# keep me\nFOO=new\n\nOTHER=1\n", readText(t, path))
}

func TestSetEnvKeyCreatesAMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")

	require.NoError(t, SetEnvKey(path, "FOO", "bar"))

	assert.Equal(t, "FOO=bar\n", readText(t, path))
}

func TestSetEnvKeyNoopWhenValueAlreadyMatches(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	require.NoError(t, os.WriteFile(path, []byte("FOO=bar\nOTHER=1\n"), 0o644))
	before, err := os.Stat(path)
	require.NoError(t, err)

	require.NoError(t, SetEnvKey(path, "FOO", "bar"))

	after, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, "FOO=bar\nOTHER=1\n", readText(t, path))
	assert.Equal(t, before.ModTime(), after.ModTime())
}

func TestRemoveEnvKeyDropsMatchingLineAndNoopsWhenAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	require.NoError(t, os.WriteFile(path, []byte("FOO=bar\nOTHER=1\n"), 0o644))

	require.NoError(t, RemoveEnvKey(path, "FOO"))
	assert.Equal(t, "OTHER=1\n", readText(t, path))

	before, err := os.Stat(path)
	require.NoError(t, err)
	require.NoError(t, RemoveEnvKey(path, "FOO"))
	after, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, "OTHER=1\n", readText(t, path))
	assert.Equal(t, before.ModTime(), after.ModTime())
}

func TestRemoveEnvKeyLeavesAnEmptyFileWhenTheLastKeyGoes(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	require.NoError(t, os.WriteFile(path, []byte("export FOO=bar\n"), 0o644))

	require.NoError(t, RemoveEnvKey(path, "FOO"))

	assert.Equal(t, "", readText(t, path))
}

func TestProjectEnvPersistPathPrefersEnvThenEnvLocal(t *testing.T) {
	root := t.TempDir()
	assert.Equal(t, filepath.Join(root, ".env"), ProjectEnvPersistPath(root))

	writeFile(t, root, ".env.local", "")
	assert.Equal(t, filepath.Join(root, ".env.local"), ProjectEnvPersistPath(root))

	writeFile(t, root, ".env", "")
	assert.Equal(t, filepath.Join(root, ".env"), ProjectEnvPersistPath(root))
}

func TestTextLinesSplitsLikeRust(t *testing.T) {
	assert.Empty(t, textLines(""))
	assert.Equal(t, []string{"a", "", "b"}, textLines("a\r\n\nb"))
	assert.Equal(t, []string{"a", ""}, textLines("a\n\n"))
	assert.Equal(t, []string{"a\r"}, textLines("a\r"))
}
