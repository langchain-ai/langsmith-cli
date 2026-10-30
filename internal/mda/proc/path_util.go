package proc

import (
	"os"
	"path/filepath"
	"strings"
)

// ToUnixPath joins path's components with forward slashes (POSIX-style),
// independent of the host OS separator. Used for generated import paths,
// archive entries, and Context Hub keys. Non-UTF8 components are lossily
// preserved (U+FFFD) so path layout is not silently rewritten.
//
// Like Rust's `Path::components`, it drops empty and `.` components after the
// first, but keeps `..`.
func ToUnixPath(path string) string {
	volume := filepath.VolumeName(path)
	rest := path[len(volume):]
	rooted := len(rest) > 0 && os.IsPathSeparator(rest[0])
	pieces := strings.FieldsFunc(rest, func(r rune) bool {
		return r < 0x80 && os.IsPathSeparator(uint8(r))
	})
	kept := pieces[:0]
	for index, piece := range pieces {
		if piece == "." && (index > 0 || rooted) {
			continue
		}
		kept = append(kept, strings.ToValidUTF8(piece, "\uFFFD"))
	}
	joined := strings.Join(kept, "/")
	if rooted {
		joined = "/" + joined
	}
	return volume + joined
}
