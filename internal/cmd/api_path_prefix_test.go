package cmd

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// barePath matches a string literal that looks like a LangSmith API path without
// the /api prefix, e.g. "/v1/platform/issues/%s" or "/runs/rules".
var barePath = regexp.MustCompile(`"(/(?:v1|v2|runs|repos|commits|feedback|sessions|threads|traces)[a-zA-Z0-9/%{}._?=&-]*)"`)

// The generated SDK emits paths under /api, but the CLI also builds paths itself
// for endpoints the SDK does not cover yet and passes them to the Raw* helpers.
// Single-origin self-hosted deployments serve the API under /api and have no
// route for a bare /v1 or /v2, so a hand-written path without the prefix 404s
// there while working fine against cloud — which is how the previous round of
// these went unnoticed.
func TestHandWrittenPathsCarryAPIPrefix(t *testing.T) {
	t.Parallel()

	root := ".." // internal/
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range barePath.FindAllStringSubmatch(string(src), -1) {
			t.Errorf("%s: request path %q is missing the /api prefix; single-origin "+
				"self-hosted serves the API under /api and has no route for the bare form. "+
				"Use \"/api%s\" (or the SDK, if it covers this endpoint).", path, m[1], m[1])
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
}
