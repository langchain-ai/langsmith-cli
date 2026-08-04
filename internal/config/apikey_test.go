package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveAPIKeyValue_Literal(t *testing.T) {
	got, err := ResolveAPIKeyValue("lsv2_pt_literal")
	require.NoError(t, err)
	require.Equal(t, "lsv2_pt_literal", got)

	got, err = ResolveAPIKeyValue("")
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestResolveAPIKeyValue_File(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	require.NoError(t, os.WriteFile(path, []byte("  lsv2_pt_from_file\n"), 0600))

	got, err := ResolveAPIKeyValue("@" + path)
	require.NoError(t, err)
	require.Equal(t, "lsv2_pt_from_file", got)
}

func TestResolveAPIKeyValue_HomeExpansion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	require.NoError(t, os.WriteFile(filepath.Join(home, "key"), []byte("lsv2_pt_home"), 0600))

	got, err := ResolveAPIKeyValue("@~/key")
	require.NoError(t, err)
	require.Equal(t, "lsv2_pt_home", got)
}

func TestProfileResolveAPIKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	require.NoError(t, os.WriteFile(path, []byte("lsv2_pt_profile_file\n"), 0600))

	inline := Profile{APIKey: "lsv2_pt_inline"}
	require.True(t, inline.HasAPIKey())
	key, err := inline.ResolveAPIKey()
	require.NoError(t, err)
	require.Equal(t, "lsv2_pt_inline", key)

	fromFile := Profile{APIKeyFile: path}
	require.True(t, fromFile.HasAPIKey())
	key, err = fromFile.ResolveAPIKey()
	require.NoError(t, err)
	require.Equal(t, "lsv2_pt_profile_file", key)

	var none Profile
	require.False(t, none.HasAPIKey())
	key, err = none.ResolveAPIKey()
	require.NoError(t, err)
	require.Empty(t, key)

	missing := Profile{APIKeyFile: filepath.Join(t.TempDir(), "absent")}
	_, err = missing.ResolveAPIKey()
	require.ErrorContains(t, err, "reading api key file")
}

func TestProfileAPIKeyFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := &Config{Profiles: map[string]Profile{"dev": {APIKeyFile: "~/.langsmith/api-key"}}}
	require.NoError(t, cfg.SaveTo(path))

	loaded, err := LoadFrom(path)
	require.NoError(t, err)
	require.Equal(t, "~/.langsmith/api-key", loaded.Profiles["dev"].APIKeyFile)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), `"api_key_file"`)
}

func TestResolveAPIKeyValue_Errors(t *testing.T) {
	dir := t.TempDir()

	empty := filepath.Join(dir, "empty")
	require.NoError(t, os.WriteFile(empty, []byte("\n \n"), 0600))
	multi := filepath.Join(dir, "multi")
	require.NoError(t, os.WriteFile(multi, []byte("lsv2_pt_a\nlsv2_pt_b\n"), 0600))

	for _, tc := range []struct {
		name  string
		value string
		want  string
	}{
		{"no path", "@", "missing a file path"},
		{"blank path", "@   ", "missing a file path"},
		{"missing file", "@" + filepath.Join(dir, "nope"), "reading api key file"},
		{"empty file", "@" + empty, "is empty"},
		{"multiple values", "@" + multi, "must contain only the key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveAPIKeyValue(tc.value)
			require.ErrorContains(t, err, tc.want)
			require.Empty(t, got)
		})
	}
}
