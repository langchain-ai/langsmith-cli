// Loader hooks for the introspection helper: serve text modules.
//
// `import prompt from "./prompt.md" with { type: "text" }` binds the file's
// contents. The build inlines these imports into constants before anything runs
// them; these hooks do the same as each TypeScript module loads, so a module
// that uses text imports imports here exactly as it will run deployed.
//
// - `load` inlines the text imports of a TypeScript module's source before tsx
//   compiles it. They are registered before tsx, so tsx's loader asks them for
//   each module's source.
// - tsx compiles some CommonJS modules from disk itself (a `.cts` module, or one
//   `require`d through its CommonJS loader), dropping the import attributes, so
//   their `require("./prompt.md")` arrives without them. `resolve` recognizes
//   it by lexing the requiring module, and `load` serves the file's text as the
//   module.
//
// `syncHooks` are for `module.registerHooks`, which also sees `require()`; the
// `resolve` and `load` exports are for `module.register` on a Node.js without
// it, where CommonJS text imports are not served.
import { readFileSync } from "node:fs";
import { textImports } from "./lex.mjs";

const TYPESCRIPT = /\.(ts|tsx|mts|cts)$/;

/** URLs of the files a TypeScript module imports as text. */
const textModules = new Set();
/** The text imports of each TypeScript module asked about, by URL and specifier. */
const textImportsByModule = new Map();

export const syncHooks = {
  resolve(specifier, context, nextResolve) {
    return resolved(specifier, context, nextResolve(specifier, context));
  },
  load(url, context, nextLoad) {
    return textModules.has(url) ? textModule(url) : loaded(url, nextLoad(url, context));
  },
};

export async function resolve(specifier, context, nextResolve) {
  return resolved(specifier, context, await nextResolve(specifier, context));
}

export async function load(url, context, nextLoad) {
  return textModules.has(url) ? textModule(url) : loaded(url, await nextLoad(url, context));
}

function resolved(specifier, context, result) {
  const parent = context.parentURL;
  if (parent && isTypescript(parent)) {
    const record = textImportsOf(parent).get(specifier);
    if (record) {
      requireDefaultBinding(record);
      textModules.add(result.url);
    }
  }
  return result;
}

function loaded(url, result) {
  if (!isTypescript(url) || result.source == null) return result;
  const source =
    typeof result.source === "string" ? result.source : Buffer.from(result.source).toString("utf8");
  const imports = lexTextImports(source);
  if (imports.length === 0) return result;
  let inlined = source;
  for (const record of [...imports].reverse()) {
    requireDefaultBinding(record);
    const text = readFileSync(new URL(record.specifier, url), "utf8");
    inlined =
      inlined.slice(0, record.start) +
      `const ${record.defaultBinding} = ${JSON.stringify(text)}` +
      inlined.slice(record.end);
  }
  return { ...result, source: inlined };
}

function textModule(url) {
  const text = readFileSync(new URL(url), "utf8");
  return { format: "commonjs", source: `module.exports = ${JSON.stringify(text)};`, shortCircuit: true };
}

function textImportsOf(url) {
  let imports = textImportsByModule.get(url);
  if (imports === undefined) {
    let source;
    try {
      source = readFileSync(new URL(url), "utf8");
    } catch {
      source = "";
    }
    imports = new Map(lexTextImports(source).map((record) => [record.specifier, record]));
    textImportsByModule.set(url, imports);
  }
  return imports;
}

function lexTextImports(source) {
  try {
    return textImports(source);
  } catch {
    // A module the lexer cannot read is left to tsx, which reports its errors.
    return [];
  }
}

function requireDefaultBinding(record) {
  if (record.defaultBinding === null) {
    throw new Error(
      `text-module import of "${record.specifier}" must use a single default import, ` +
        `e.g. \`import name from "${record.specifier}" with { type: "text" }\``,
    );
  }
}

function isTypescript(url) {
  return url.startsWith("file:") && TYPESCRIPT.test(new URL(url).pathname);
}
