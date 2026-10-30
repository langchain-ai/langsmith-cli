package introspect

import (
	"context"
	"path/filepath"
	"strings"
)

// The source-reading half of `compile/local_packages.rs`: the relative imports
// of a TypeScript file that leave the project root, which the build cannot
// include.

// TypescriptImportsOutsideRoot lists the relative import specifiers of a
// TypeScript file that resolve outside the project root. rel is the file's
// path relative to the root. Type-only imports are skipped because they are
// erased.
//
// Like [InlineTextModules], a source the lexer cannot read has no imports here.
func TypescriptImportsOutsideRoot(ctx context.Context, source, rel string) ([]string, error) {
	lexed, err := lexTypescript(ctx, []string{source})
	if err != nil {
		return nil, err
	}
	return importsOutsideRoot(lexed[0], rel), nil
}

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
