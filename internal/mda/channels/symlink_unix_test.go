//go:build unix

package channels

// Links are refused on the way to an icon. Creating one needs privileges on
// Windows, so these run where links are ordinary files.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadSlackIconRejectsASymlink(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "channels"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "outside.png"), []byte("outside"), 0o644))
	require.NoError(t, os.Symlink(filepath.Join(root, "outside.png"), filepath.Join(root, "channels", "icon.png")))

	_, err := readSlackIcon(root, "icon.png")

	require.EqualError(t, err, "Slack channel icon path must not contain symlinks")
}

func TestALinearIconLinkIsRefused(t *testing.T) {
	project := t.TempDir()
	channels := filepath.Join(project, "channels")
	require.NoError(t, os.MkdirAll(filepath.Join(channels, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(channels, "assets", "icon.png"), []byte("\x89PNG\r\n\x1a\nrest"), 0o644))
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "icon.png"), []byte("\x89PNG\r\n\x1a\n"), 0o644))
	// A link, whether it leads out of channels/ or not; the project loader
	// leaves both out of the build.
	require.NoError(t, os.Symlink(filepath.Join(channels, "assets", "icon.png"), filepath.Join(channels, "alias.png")))
	require.NoError(t, os.Symlink(filepath.Join(outside, "icon.png"), filepath.Join(channels, "linked.png")))
	built := []string{filepath.Join("channels", "assets", "icon.png")}

	for _, icon := range []string{"alias.png", "linked.png"} {
		_, err := validateIconFile(project, built, icon)

		require.EqualError(t, err, "Linear channel app.icon "+icon+" isn't part of the build: use a regular file, not a link, outside folders the build leaves out such as build/ or dist/.")
	}

	// Even one the build lists is checked on disk.
	_, err := validateIconFile(project, append(built, filepath.Join("channels", "linked.png")), "linked.png")
	require.EqualError(t, err, "Linear channel app.icon path must not contain symlinks")
}
