package introspect

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
)

// Resolve TypeScript "text module" imports at compile time (`text_module.rs`).
//
// The text modules proposal lets a module import a file's raw contents as a
// string via an import attribute:
//
//	import systemPrompt from "./instructions.md" with { type: "text" };
//
// This syntax is still not supported across every runtime/bundler, so the CLI
// resolves it ahead of time: the TypeScript helper finds the import statements
// of each source with es-module-lexer, and the default imports carrying a
// `type: "text"` attribute are rewritten into an inline `const` bound to a
// template literal:
//
//	const systemPrompt = `# Assistant
//
//	You are a helpful assistant.`;
//
// Only the matched import statements are rewritten; every other byte of the
// source is preserved. Because the inlined content goes into a template
// literal, backticks, `${` interpolation starts, and backslashes are escaped.

// textModuleType is the import-attribute `type` value whose modules are
// inlined as raw text (`type: "text"` only).
const textModuleType = "text"

// lexedImport is one module-level `import` or `export ... from` statement the
// helper found, with UTF-8 byte offsets into its source.
type lexedImport struct {
	// Statement is `import` or `export`.
	Statement string `json:"statement"`
	Specifier string `json:"specifier"`
	TypeOnly  bool   `json:"type_only"`
	// Start and End span the statement, attributes included, without a
	// trailing `;`.
	Start      int        `json:"start"`
	End        int        `json:"end"`
	Attributes [][]string `json:"attributes"`
	// DefaultBinding is the local name of a sole default import.
	DefaultBinding *string `json:"default_binding"`
}

// attribute returns the value of the import attribute key.
func (i lexedImport) attribute(key string) (string, bool) {
	for _, entry := range i.Attributes {
		if len(entry) == 2 && entry[0] == key {
			return entry[1], true
		}
	}
	return "", false
}

// lexedFile is the helper's reading of one source. A source the lexer could
// not read has an Error and no imports.
type lexedFile struct {
	Imports []lexedImport `json:"imports"`
	Error   *string       `json:"error"`
}

// lexRequest asks the TypeScript helper for the import statements of sources.
type lexRequest struct {
	requestHeader
	Files []lexSource `json:"files"`
}

type lexSource struct {
	Source string `json:"source"`
}

// lexTypescript lists the import statements of each source in one helper run.
func lexTypescript(ctx context.Context, sources []string) ([]lexedFile, error) {
	request := &lexRequest{requestHeader: requestHeader{Mode: "lex"}, Files: make([]lexSource, len(sources))}
	for i, source := range sources {
		request.Files[i] = lexSource{Source: source}
	}
	var doc struct {
		Version int         `json:"version"`
		Files   []lexedFile `json:"files"`
	}
	// Lexing imports nothing, so it runs from anywhere.
	err := runTypescriptHelper(ctx, request, "", os.Environ(), "TypeScript sources", ImportOptions{}, &doc)
	if failure, ok := errors.AsType[*helperFailure](err); ok {
		return nil, errs.Categorized(errs.Toolchain, "failed to lex TypeScript sources: "+failure.message)
	}
	if err != nil {
		return nil, err
	}
	if len(doc.Files) != len(sources) {
		return nil, fmt.Errorf("TypeScript sources: %w", errNoHelperValue)
	}
	return doc.Files, nil
}

// InlineTextModules inlines every text-module import found in source.
//
// fileName is used for error messages. resolveDir is the directory the
// importing file lives in, against which relative import specifiers are
// resolved.
//
// Returns the rewritten source, or the original source unchanged when there are
// no text-module imports.
//
// Every TypeScript file in the project passes through here, so syntax errors are
// deliberately not reported: a source the lexer cannot read is copied verbatim
// and the language toolchain that compiles it produces the authoritative
// diagnostic. Imports the lexer never sees stay in the output as written.
func InlineTextModules(ctx context.Context, source, fileName, resolveDir string) (string, error) {
	lexed, err := lexTypescript(ctx, []string{source})
	if err != nil {
		return "", err
	}
	inlined, _, err := inlineTextModules(source, fileName, resolveDir, lexed[0])
	return inlined, err
}

// inlineTextModules rewrites the text-module imports of lexed in source and
// returns the imports it rewrote.
func inlineTextModules(source, fileName, resolveDir string, lexed lexedFile) (string, []lexedImport, error) {
	var edits []edit
	var inlined []lexedImport
	for _, imp := range lexed.Imports {
		if imp.Statement != "import" {
			continue
		}
		typeValue, ok := imp.attribute("type")
		if !ok || typeValue != textModuleType {
			continue
		}
		specifier := imp.Specifier
		if imp.DefaultBinding == nil {
			return "", nil, fmt.Errorf("text-module import of %q in %s must use a single default import, "+
				"e.g. `import name from %q with { type: %q }`", specifier, fileName, specifier, typeValue)
		}
		resolved := specifier
		if !filepath.IsAbs(specifier) {
			resolved = filepath.Join(resolveDir, filepath.FromSlash(specifier))
		}
		contents, err := os.ReadFile(resolved)
		if err == nil && !utf8.Valid(contents) {
			err = fmt.Errorf("stream did not contain valid UTF-8")
		}
		if err != nil {
			return "", nil, fmt.Errorf("failed to inline text module %q imported by %s (resolved to %s): %w",
				specifier, fileName, resolved, err)
		}
		edits = append(edits, edit{
			start: imp.Start,
			end:   consumeTrailingSemicolon(source, imp.End),
			text:  fmt.Sprintf("const %s = `%s`;", *imp.DefaultBinding, escapeTemplateLiteral(string(contents))),
		})
		inlined = append(inlined, imp)
	}
	if len(edits) == 0 {
		return source, nil, nil
	}
	return applyEdits(source, edits), inlined, nil
}

// escapeTemplateLiteral escapes a raw string so it can be embedded inside a
// backtick template literal without changing its meaning.
//
// Backslashes are escaped first (so the escapes added below are not
// double-escaped), then backticks (which would otherwise close the literal) and
// `${` sequences (which would otherwise start an interpolation).
func escapeTemplateLiteral(content string) string {
	content = strings.ReplaceAll(content, `\`, `\\`)
	content = strings.ReplaceAll(content, "`", "\\`")
	return strings.ReplaceAll(content, "${", "\\${")
}

// edit is a single non-overlapping replacement of source[start:end] with text.
type edit struct {
	start, end int
	text       string
}

// applyEdits applies non-overlapping edits, right-to-left so earlier offsets
// stay valid.
func applyEdits(source string, edits []edit) string {
	slices.SortFunc(edits, func(a, b edit) int {
		if a.start != b.start {
			return a.start - b.start
		}
		return a.end - b.end
	})
	out := source
	for i := len(edits) - 1; i >= 0; i-- {
		e := edits[i]
		out = out[:e.start] + e.text + out[e.end:]
	}
	return out
}

// consumeTrailingSemicolon returns the byte offset just past an optional `;`
// following from, skipping only spaces/tabs (never newlines, to avoid
// swallowing the next statement).
func consumeTrailingSemicolon(source string, from int) int {
	i := from
	for i < len(source) && (source[i] == ' ' || source[i] == '\t') {
		i++
	}
	if i < len(source) && source[i] == ';' {
		return i + 1
	}
	return from
}

// TypescriptSource is one TypeScript source compile copies into the build.
type TypescriptSource struct {
	// Source is the file's text.
	Source string
	// Rel is the file's path relative to the project root.
	Rel string
	// ResolveDir is the directory the file lives in.
	ResolveDir string
}

// TypescriptTransform is a [TypescriptSource] as the build gets it.
type TypescriptTransform struct {
	// Source has its text-module imports inlined ([InlineTextModules]).
	Source string
	// OutsideImports are the relative imports of Source that resolve outside
	// the project root ([TypescriptImportsOutsideRoot]).
	OutsideImports []string
}

// TransformTypescriptSources inlines the text modules of every source and lists
// the imports of the result that leave the project root, in one helper run: the
// batch form of [InlineTextModules] followed by [TypescriptImportsOutsideRoot],
// as compile applies them to each file it copies.
func TransformTypescriptSources(ctx context.Context, sources []TypescriptSource) ([]TypescriptTransform, error) {
	if len(sources) == 0 {
		return nil, nil
	}
	texts := make([]string, len(sources))
	for i, source := range sources {
		texts[i] = source.Source
	}
	lexed, err := lexTypescript(ctx, texts)
	if err != nil {
		return nil, err
	}
	transforms := make([]TypescriptTransform, len(sources))
	for i, source := range sources {
		inlined, rewritten, err := inlineTextModules(source.Source, source.Rel, source.ResolveDir, lexed[i])
		if err != nil {
			return nil, err
		}
		// Inlining replaces only the text imports, so the other statements are
		// the transformed source's imports.
		remaining := lexed[i]
		remaining.Imports = slices.DeleteFunc(slices.Clone(remaining.Imports), func(imp lexedImport) bool {
			return slices.ContainsFunc(rewritten, func(done lexedImport) bool { return done.Start == imp.Start })
		})
		transforms[i] = TypescriptTransform{
			Source:         inlined,
			OutsideImports: importsOutsideRoot(remaining, source.Rel),
		}
	}
	return transforms, nil
}
