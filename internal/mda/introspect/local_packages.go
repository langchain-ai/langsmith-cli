package introspect

import (
	"path/filepath"
	"strings"
)

// The source-reading half of `compile/local_packages.rs`: the relative imports
// of a TypeScript file that leave the project root, which the build cannot
// include ([TransformTypescriptSources]).

// importsOutsideRoot lists the relative import specifiers of lexed, the file at
// rel relative to the project root, that resolve outside the root. Type-only
// imports are skipped because they are erased.
func importsOutsideRoot(lexed lexedFile, rel string) []string {
	depth := 0
	if parent := filepath.Dir(rel); parent != "." {
		depth = len(strings.Split(filepath.ToSlash(parent), "/"))
	}
	var outside []string
	for _, imp := range lexed.Imports {
		if imp.TypeOnly {
			continue
		}
		if escapesRoot(depth, imp.Specifier) {
			outside = append(outside, imp.Specifier)
		}
	}
	return outside
}

// escapesRoot reports whether specifier, imported from a file depth
// directories below the root, resolves above the root.
func escapesRoot(depth int, specifier string) bool {
	if !strings.HasPrefix(specifier, "../") {
		return false
	}
	for _, component := range strings.Split(specifier, "/") {
		switch component {
		case "", ".":
		case "..":
			depth--
		default:
			depth++
		}
		if depth < 0 {
			return true
		}
	}
	return false
}
