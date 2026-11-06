package introspect

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestEscapesRootCountsParentSegmentsFromTheImportingFile(t *testing.T) {
	for _, c := range []struct {
		depth     int
		specifier string
		want      bool
	}{
		{0, "../dep/helper", true},
		{1, "../helper", false},
		{1, "../../dep", true},
		{0, "./local", false},
		{0, "zod", false},
	} {
		if got := escapesRoot(c.depth, c.specifier); got != c.want {
			t.Fatalf("escapesRoot(%d, %q) = %v", c.depth, c.specifier, got)
		}
	}
}

func TestImportsOutsideRootSkipTypeOnlyImports(t *testing.T) {
	source := "import { a } from \"../../shared/a\";\n" +
		"import type { B } from \"../../shared/b\";\n" +
		"export * from \"../../shared/c\";\n" +
		"export type { D } from \"../../shared/d\";\n" +
		"import { e } from \"../e\";\n" +
		"const lazy = await import(\"../../shared/f\");\n"
	got, err := transform(t, t.TempDir(), filepath.Join("tools", "search.ts"), source)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"../../shared/a", "../../shared/c"}; !reflect.DeepEqual(got.OutsideImports, want) {
		t.Fatalf("got %v", got.OutsideImports)
	}
}
