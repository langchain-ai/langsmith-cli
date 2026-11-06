package channels

// The icon file a channel names in its options, as a path relative to the
// project's `channels/` directory: the Slack app's and the Linear app's. Each
// channel checks the image itself; this checks where the file is.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// relativeIconPath is the icon as a plain relative path: no root, `.`, `..`,
// `\`, or empty segment. Linear's icon must be one, since the deployment
// serves it by the same path and checks it the same way. label names the
// option in errors. As for a Rust `Path`, a `.` segment after the first is
// ignored.
func relativeIconPath(icon, label string) (string, error) {
	invalid := fmt.Errorf("%s must be a path relative to channels/", label)
	if icon == "" || strings.Contains(icon, `\`) || filepath.VolumeName(icon) != "" {
		return "", invalid
	}
	for i, segment := range strings.Split(icon, "/") {
		if segment == "" || segment == ".." || (segment == "." && i == 0) {
			return "", invalid
		}
	}
	return icon, nil
}

// channelIconFile is the icon's file inside a regular `channels/` directory,
// reached without links, as a regular file of at most maxBytes. Any relative
// path without `..` names it, as Slack's icon always could, such as
// `./assets/icon.png`; a channel that needs a plain path checks
// [relativeIconPath] first.
func channelIconFile(projectRoot, icon, label string, maxBytes int64) (string, error) {
	if icon == "" || filepath.IsAbs(icon) || strings.HasPrefix(icon, "/") || filepath.VolumeName(icon) != "" ||
		hasParentSegment(icon) {
		return "", fmt.Errorf("%s must be a path relative to channels/", label)
	}
	root, err := canonicalize(projectRoot)
	if err != nil {
		return "", fmt.Errorf("failed to resolve project root for %s: %w", label, err)
	}
	channelsPath := filepath.Join(root, "channels")
	channelsInfo, err := os.Lstat(channelsPath)
	if err != nil {
		return "", fmt.Errorf("failed to inspect %s: %w", channelsPath, err)
	}
	if channelsInfo.Mode()&os.ModeSymlink != 0 || !channelsInfo.IsDir() {
		return "", fmt.Errorf("%s directory must be a regular channels/ directory", label)
	}
	channels, err := canonicalize(channelsPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve channels/ for %s: %w", label, err)
	}

	// No link on the way may lead out of channels/, or anywhere else.
	candidate := channels
	for _, part := range strings.FieldsFunc(icon, isPathSeparator) {
		if part == "." {
			continue
		}
		candidate = filepath.Join(candidate, part)
		info, err := os.Lstat(candidate)
		if err != nil {
			return "", fmt.Errorf("failed to inspect %s %s: %w", label, icon, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%s path must not contain symlinks", label)
		}
	}
	resolved, err := canonicalize(candidate)
	if err != nil {
		return "", fmt.Errorf("failed to resolve %s %s: %w", label, icon, err)
	}
	if !within(channels, resolved) {
		return "", fmt.Errorf("%s must remain within channels/", label)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("failed to inspect %s %s: %w", label, icon, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s must be a regular file", label)
	}
	if info.Size() > maxBytes {
		return "", fmt.Errorf("%s must be at most 1 MB", label)
	}
	return resolved, nil
}

func isPathSeparator(r rune) bool {
	return r == '/' || r == filepath.Separator
}

func hasParentSegment(path string) bool {
	for _, part := range strings.FieldsFunc(path, isPathSeparator) {
		if part == ".." {
			return true
		}
	}
	return false
}

// canonicalize is the absolute path with every link resolved, as Rust's
// `Path::canonicalize`; it fails when the path does not exist.
func canonicalize(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absolute)
}

// within reports whether path is base or inside it.
func within(base, path string) bool {
	return path == base || strings.HasPrefix(path, base+string(filepath.Separator))
}
