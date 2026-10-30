// Module-syntax lexing shared by the introspection helper and its loader hook.
//
// es-module-lexer finds import and export statements without parsing the rest
// of a module. It lexes JavaScript and erasable TypeScript, but not JSX: a TSX
// file fails where its markup starts. The imports of a module come before its
// markup in practice, so a failed lex is retried on the text before the line it
// failed on, and the imports found there are kept, the way the Rust CLI kept
// what its recovering parser saw.
import { init, parse } from "es-module-lexer";

await init();

/** How many shorter prefixes a failed lex is retried on before giving up. */
const MAX_RETRIES = 64;

const ASSERT_CLAUSE = /^[ \t\r\n]*assert[ \t\r\n]*\{([^}]*)\}/;
const ATTRIBUTE_ENTRY = /(["']?)([A-Za-z_$][\w$-]*)\1[ \t\r\n]*:[ \t\r\n]*(["'])((?:(?!\3)[^\\]|\\.)*)\3/g;
const IDENTIFIER = /^[A-Za-z_$][\w$]*$/;

/**
 * The module-level `import` / `export ... from` statements of `source`, with
 * UTF-16 offsets into it.
 *
 * Each record: `statement` (`"import"` or `"export"`), `specifier`, `typeOnly`,
 * `start` / `end` (the statement, attributes included, without a trailing
 * `;`), `attributes` (`[key, value]` pairs or `null`), and `defaultBinding`
 * (the local name of a sole default import, or `null`).
 */
export function lexImports(source) {
  return lexStatic(source).map((record) => describe(source, record));
}

/** The text-module imports (`with { type: "text" }`) of `source`. */
export function textImports(source) {
  return lexImports(source).filter(
    (record) =>
      record.statement === "import" &&
      record.attributes?.some(([key, value]) => key === "type" && value === "text"),
  );
}

/** UTF-8 byte offset of each UTF-16 offset in `offsets` (ascending or not). */
export function byteOffsets(source, offsets) {
  return offsets.map((offset) => Buffer.byteLength(source.slice(0, offset), "utf8"));
}

function lexStatic(source) {
  let text = source;
  for (let attempt = 0; ; attempt++) {
    try {
      const [imports] = parse(text);
      return imports.filter(
        (record) => record.type === "static" || record.type === "reexport-star",
      );
    } catch (error) {
      const failedAt = typeof error?.idx === "number" ? error.idx : -1;
      const lineStart = failedAt > 0 ? text.lastIndexOf("\n", failedAt - 1) + 1 : 0;
      if (lineStart <= 0 || attempt >= MAX_RETRIES) throw error;
      text = text.slice(0, lineStart);
    }
  }
}

function describe(source, record) {
  const statement = source.startsWith("import", record.importStart) ? "import" : "export";
  let end = record.importEnd;
  let attributes = record.attributes ? record.attributes.map(([k, v]) => [k, v]) : null;
  if (attributes === null && statement === "import") {
    // The legacy `assert { ... }` spelling, which the lexer leaves outside the
    // statement.
    const legacy = ASSERT_CLAUSE.exec(source.slice(end));
    if (legacy) {
      attributes = [...legacy[1].matchAll(ATTRIBUTE_ENTRY)].map((entry) => [entry[2], entry[4]]);
      end += legacy[0].length;
    }
  }
  return {
    statement,
    specifier: record.specifier,
    typeOnly: record.typeOnly === true,
    start: record.importStart,
    end,
    attributes,
    defaultBinding: statement === "import" ? defaultBinding(source, record) : null,
  };
}

/** The local name of `import name from "..."`, or `null` for any other clause. */
function defaultBinding(source, record) {
  // The specifier starts after its opening quote.
  const clause = stripComments(source.slice(record.importStart + "import".length, record.start - 1))
    .trim()
    .replace(/(^|\s)from$/, "")
    .trim();
  return IDENTIFIER.test(clause) && clause !== "type" ? clause : null;
}

function stripComments(text) {
  return text.replace(/\/\*[\s\S]*?\*\//g, " ").replace(/\/\/[^\n]*/g, " ");
}
