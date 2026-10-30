package introspect

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeText(t *testing.T, dir, name, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func inline(t *testing.T, dir, source string) string {
	t.Helper()
	out, err := InlineTextModules(t.Context(), source, "agent.ts", dir)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestEscapesBackticksDollarsAndBackslashes(t *testing.T) {
	for in, want := range map[string]string{
		"plain":  "plain",
		"a`b":    "a\\`b",
		"${x}":   "\\${x}",
		`a\b`:    `a\\b`,
		`\${x}`:  `\\\${x}`,
		"`${a}`": "\\`\\${a}\\`",
	} {
		if got := escapeTemplateLiteral(in); got != want {
			t.Fatalf("%q: got %q, want %q", in, got, want)
		}
	}
}

func TestInlinesATextImportAsATemplateLiteral(t *testing.T) {
	dir := t.TempDir()
	writeText(t, dir, "instructions.md", "# Assistant\n\nBe helpful.\n")
	out := inline(t, dir, "import systemPrompt from \"./instructions.md\" with { type: \"text\" };\nexport const x = systemPrompt;\n")
	if want := "const systemPrompt = `# Assistant\n\nBe helpful.\n`;\nexport const x = systemPrompt;\n"; out != want {
		t.Fatalf("got %q", out)
	}
}

func TestLeavesOtherImportsUntouched(t *testing.T) {
	dir := t.TempDir()
	writeText(t, dir, "instructions.md", "hello")
	writeText(t, dir, "data.json", "{}")
	for _, source := range []string{
		"import sp from \"./instructions.md\" with { type: \"markdown\" };\n",
		"import data from \"./data.json\" with { type: \"json\" };\n",
		"import { foo } from \"./foo\";\nimport bar from \"./bar\";\n",
	} {
		if out := inline(t, dir, source); out != source {
			t.Fatalf("got %q", out)
		}
	}
}

func TestEscapesMarkdownContainingBackticksAndInterpolation(t *testing.T) {
	dir := t.TempDir()
	writeText(t, dir, "instructions.md", "Use `code` and ${vars} and a path C:\\tmp")
	out := inline(t, dir, "import sp from \"./instructions.md\" with { type: \"text\" };\n")
	if want := "const sp = `Use \\`code\\` and \\${vars} and a path C:\\\\tmp`;\n"; out != want {
		t.Fatalf("got %q", out)
	}
}

func TestRejectsNamedTextImports(t *testing.T) {
	dir := t.TempDir()
	writeText(t, dir, "instructions.md", "hi")
	_, err := InlineTextModules(t.Context(),
		"import { systemPrompt } from \"./instructions.md\" with { type: \"text\" };\n", "agent.ts", dir)
	requireErrorContains(t, err, "single default import")
}

func TestErrorsWhenTheReferencedFileIsMissing(t *testing.T) {
	_, err := InlineTextModules(t.Context(),
		"import sp from \"./missing.md\" with { type: \"text\" };\n", "agent.ts", t.TempDir())
	requireErrorContains(t, err, "failed to inline text module \"./missing.md\" imported by agent.ts")
}

func TestResolvesRelativePathsInSubdirectories(t *testing.T) {
	dir := t.TempDir()
	writeText(t, dir, "prompts/system.md", "nested prompt")
	out := inline(t, dir, "import sp from \"./prompts/system.md\" with { type: \"text\" };\n")
	if out != "const sp = `nested prompt`;\n" {
		t.Fatalf("got %q", out)
	}
}

func TestInlinesMultipleImportsAndPreservesSurroundingCode(t *testing.T) {
	dir := t.TempDir()
	writeText(t, dir, "a.md", "AAA")
	writeText(t, dir, "b.md", "BBB — ünïcödé")
	out := inline(t, dir, "import { defineDeepAgent } from \"managed-deepagents\";\n"+
		"// ünïcödé before the imports shifts byte offsets\n"+
		"import a from \"./a.md\" with { type: \"text\" };\n"+
		"import b from \"./b.md\" with { type: \"text\" };\n"+
		"export const agent = defineDeepAgent({ a, b });\n")
	want := "import { defineDeepAgent } from \"managed-deepagents\";\n" +
		"// ünïcödé before the imports shifts byte offsets\n" +
		"const a = `AAA`;\n" +
		"const b = `BBB — ünïcödé`;\n" +
		"export const agent = defineDeepAgent({ a, b });\n"
	if out != want {
		t.Fatalf("got %q", out)
	}
}

func TestHandlesTSXSources(t *testing.T) {
	dir := t.TempDir()
	writeText(t, dir, "prompt.md", "tsx prompt")
	out, err := InlineTextModules(t.Context(),
		"import sp from \"./prompt.md\" with { type: \"text\" };\nconst el = <div>{sp}</div>;\n", "view.tsx", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "const sp = `tsx prompt`;\n") {
		t.Fatalf("got %q", out)
	}
}

func TestSupportsTheLegacyAssertKeyword(t *testing.T) {
	dir := t.TempDir()
	writeText(t, dir, "instructions.md", "asserted")
	if out := inline(t, dir, "import sp from \"./instructions.md\" assert { type: \"text\" };\n"); out != "const sp = `asserted`;\n" {
		t.Fatalf("got %q", out)
	}
}

func TestSourcesTheLexerCannotReadAreCopiedVerbatim(t *testing.T) {
	source := "export const agent = defineDeepAgent({{ disableMemory: true });\n"
	if out := inline(t, t.TempDir(), source); out != source {
		t.Fatalf("got %q", out)
	}
}

func TestTransformTypescriptSourcesInBatch(t *testing.T) {
	dir := t.TempDir()
	writeText(t, dir, "tools/prompt.md", "P")
	transforms, err := TransformTypescriptSources(t.Context(), []TypescriptSource{
		{
			Source:     "import p from \"./prompt.md\" with { type: \"text\" };\nimport { a } from \"../../shared/a\";\n",
			Rel:        filepath.Join("tools", "search.ts"),
			ResolveDir: filepath.Join(dir, "tools"),
		},
		{Source: "export * from \"../outside\";\n", Rel: "agent.ts", ResolveDir: dir},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []TypescriptTransform{
		{Source: "const p = `P`;\nimport { a } from \"../../shared/a\";\n", OutsideImports: []string{"../../shared/a"}},
		{Source: "export * from \"../outside\";\n", OutsideImports: []string{"../outside"}},
	}
	if !reflect.DeepEqual(transforms, want) {
		t.Fatalf("got %#v", transforms)
	}
}
