// Command gen generates CLI commands from the LangSmith Operation Catalog.
//
//	go run ./cmd/gen -catalog path/to/operations.json
//
// It writes internal/gen/zz_generated_*.go and a snapshot of the exposed
// operations to internal/gen/catalog.json. Without -catalog it regenerates
// from that snapshot.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/codegen"
)

func main() {
	catalogPath := flag.String("catalog", "", "path to operations.json (default: the committed snapshot)")
	outDir := flag.String("out", "internal/gen", "output directory")
	check := flag.Bool("check", false, "fail if generated files are stale instead of writing them")
	skip := flag.Bool("skip-unsupported", false, "skip operations the generator cannot map (coverage survey)")
	flag.Parse()

	if err := run(*catalogPath, *outDir, *check, *skip); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(catalogPath, outDir string, check, skip bool) error {
	snapshotPath := filepath.Join(outDir, "catalog.json")
	if catalogPath == "" {
		catalogPath = snapshotPath
	}
	data, err := os.ReadFile(catalogPath)
	if err != nil {
		return err
	}
	catalog, err := codegen.ParseCatalog(data)
	if err != nil {
		return err
	}
	files, skipped, err := codegen.GenerateWith(catalog, codegen.Options{SkipUnsupported: skip})
	if err != nil {
		return err
	}
	for _, s := range skipped {
		fmt.Fprintln(os.Stderr, "skipped:", s)
	}
	snapshot, err := json.MarshalIndent(catalog.Subset(), "", "  ")
	if err != nil {
		return err
	}
	files["catalog.json"] = append(snapshot, '\n')

	existing, _ := filepath.Glob(filepath.Join(outDir, "zz_generated_*.go"))
	var stale []string
	for _, path := range existing {
		if _, ok := files[filepath.Base(path)]; !ok {
			stale = append(stale, path)
		}
	}
	for name, content := range files {
		path := filepath.Join(outDir, name)
		if check {
			if current, err := os.ReadFile(path); err != nil || !bytes.Equal(current, content) {
				stale = append(stale, path)
			}
			continue
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return err
		}
	}
	if check {
		if len(stale) > 0 {
			return fmt.Errorf("generated files are stale; run `make generate`:\n  %s", strings.Join(stale, "\n  "))
		}
		return nil
	}
	for _, path := range stale {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	fmt.Printf("generated %d operations into %s\n", len(catalog.Exposed())-len(skipped), outDir)
	return nil
}
