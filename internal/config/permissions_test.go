package config

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSaveToWritesOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits are not meaningful on windows")
	}
	dir := filepath.Join(t.TempDir(), "langsmith")
	path := filepath.Join(dir, "config.json")
	cfg := &Config{Profiles: map[string]Profile{"dev": {APIKey: "lsv2_pt_secret"}}}
	require.NoError(t, cfg.SaveTo(path))

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())

	dirInfo, err := os.Stat(dir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0700), dirInfo.Mode().Perm())
}

func TestLoadFromWarnsWhenCredentialsAreReadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits are not meaningful on windows")
	}

	write := func(t *testing.T, name, body string, perm os.FileMode) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), name)
		require.NoError(t, os.WriteFile(path, []byte(body), perm))
		return path
	}

	const withKey = `{"profiles":{"dev":{"api_key":"lsv2_pt_secret"}}}`
	const withToken = `{"profiles":{"dev":{"oauth":{"refresh_token":"rt"}}}}`

	t.Run("api key in a readable config", func(t *testing.T) {
		path := write(t, "config.json", withKey, 0644)
		stderr := captureStderr(t, func() {
			_, err := LoadFrom(path)
			require.NoError(t, err)
			// The warning is emitted once per path.
			_, err = LoadFrom(path)
			require.NoError(t, err)
		})
		require.Equal(t, 1, strings.Count(stderr, "warning:"))
		require.Contains(t, stderr, path)
		require.Contains(t, stderr, "chmod 600")
		require.NotContains(t, stderr, "lsv2_pt_secret")
	})

	t.Run("oauth token in a readable config", func(t *testing.T) {
		path := write(t, "config.json", withToken, 0640)
		stderr := captureStderr(t, func() {
			_, err := LoadFrom(path)
			require.NoError(t, err)
		})
		require.Contains(t, stderr, "warning:")
	})

	t.Run("owner-only config", func(t *testing.T) {
		path := write(t, "config.json", withKey, 0600)
		stderr := captureStderr(t, func() {
			_, err := LoadFrom(path)
			require.NoError(t, err)
		})
		require.Empty(t, stderr)
	})

	t.Run("missing config", func(t *testing.T) {
		stderr := captureStderr(t, func() {
			_, err := LoadFrom(filepath.Join(t.TempDir(), "absent.json"))
			require.NoError(t, err)
		})
		require.Empty(t, stderr)
	})
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stderr
	os.Stderr = w
	fn()
	os.Stderr = orig
	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}
