// Above this size we skip formatting to avoid event-loop stalls.
const MAX_INPUT_LENGTH = 50_000;

// Error payloads are often re-`repr()`'d a few times.
const MAX_UNESCAPE_PASSES = 6;

const OPEN_BRACKETS = new Set(['{', '[', '(']);
const CLOSE_BRACKETS = new Set(['}', ']', ')']);
const STRUCTURAL = /["'{[(]/;

const ESCAPE_REPLACEMENTS: Record<string, string> = {
  n: '\n',
  r: '\r',
  t: '\t',
  '\\': '\\',
  "'": "'",
  '"': '"',
};

// CPython opens every formatted traceback with this exact line.
const TRACEBACK_HEADER = 'Traceback (most recent call last):';
const ESCAPED_NEWLINE = /\\n/g;

const isWhitespace = (ch: string): boolean =>
  ch === ' ' || ch === '\t' || ch === '\n' || ch === '\r';

/**
 * Reads the quoted literal starting at `i`, returning it verbatim (delimiters
 * and backslash escapes included) and the index just past the closing quote.
 * An unterminated literal reads to end-of-input; characters are never dropped.
 */
const readStringLiteral = (
  input: string,
  i: number
): { literal: string; next: number } => {
  const quote = input[i];
  let literal = quote;
  let j = i + 1;
  while (j < input.length) {
    const ch = input[j];
    if (ch === '\\') {
      literal += ch + (input[j + 1] ?? '');
      j += 2;
      continue;
    }
    literal += ch;
    j += 1;
    if (ch === quote) break;
  }
  return { literal, next: j };
};

/**
 * Maps each opening-bracket index to whether it should expand onto multiple
 * lines. A bracket expands only when its contents contain a string literal or
 * a nested bracket, so casual prose parentheses ("failed (status 500)") stay
 * inline.
 */
const computeExpandMap = (input: string): Map<number, boolean> => {
  const expand = new Map<number, boolean>();
  const stack: { index: number; hasSignal: boolean }[] = [];
  let i = 0;
  while (i < input.length) {
    const ch = input[i];
    if (ch === '"' || ch === "'") {
      if (stack.length > 0) stack[stack.length - 1].hasSignal = true;
      i = readStringLiteral(input, i).next;
      continue;
    }
    if (OPEN_BRACKETS.has(ch)) {
      if (stack.length > 0) stack[stack.length - 1].hasSignal = true;
      stack.push({ index: i, hasSignal: false });
      i += 1;
      continue;
    }
    if (CLOSE_BRACKETS.has(ch)) {
      const frame = stack.pop();
      if (frame !== undefined) expand.set(frame.index, frame.hasSignal);
      i += 1;
      continue;
    }
    i += 1;
  }
  return expand;
};

/**
 * Decodes `\n`, `\r`, `\t`, `\\`, `\'`, and `\"` so escaped payloads render
 * readably, repeating while the result still looks structured. Unknown escapes
 * (e.g. `\x41`) are left intact.
 *
 * `preserveQuote` leaves that one quote escaped, so unwrapping a literal's body
 * cannot produce a doubled delimiter (`''hi''` instead of `'hi'`).
 */
const decodeEscapedBody = (text: string, preserveQuote?: string): string => {
  let prev = text;
  for (let pass = 0; pass < MAX_UNESCAPE_PASSES; pass += 1) {
    const next = prev.replace(/\\(.)/g, (match, ch: string) =>
      ch === preserveQuote ? match : (ESCAPE_REPLACEMENTS[ch] ?? match)
    );
    if (next === prev) break;
    prev = next;
    if (!STRUCTURAL.test(prev)) break;
  }
  return prev;
};

const decodeStringLiteral = (literal: string): string => {
  if (!literal.includes('\\')) return literal;
  const quote = literal[0];
  const hasClosingQuote = literal.length > 1 && literal.endsWith(quote);
  const bodyEnd = hasClosingQuote ? literal.length - 1 : literal.length;
  const body = literal.slice(1, bodyEnd);
  return (
    quote + decodeEscapedBody(body, quote) + (hasClosingQuote ? quote : '')
  );
};

/**
 * True when an entire multi-line traceback arrived as one escaped line, rather
 * than as structured data that merely contains escaped newlines in its own
 * string values (those go through the per-literal decode below).
 */
const looksFullyEscaped = (input: string): boolean => {
  if (input.includes('\n')) return false;
  if (!input.includes(TRACEBACK_HEADER)) return false;
  const matches = input.match(ESCAPED_NEWLINE);
  return matches != null && matches.length >= 2;
};

/**
 * Re-indents data structures embedded in an error string so dense single-line
 * dumps (Python `repr()`, JSON, stringified dicts) become readable, decoding
 * backslash escapes inside string literals along the way. Plain prose and
 * inputs with nothing structural are returned unchanged.
 */
export const prettifyErrorPayload = (
  input: string,
  indentUnit = '  '
): string => {
  if (input.length > MAX_INPUT_LENGTH) return input;

  if (looksFullyEscaped(input)) {
    // Decoding restores the traceback's own line breaks.
    const decoded = decodeEscapedBody(input);
    if (decoded !== input) return decoded;
  }

  if (!STRUCTURAL.test(input)) return input;

  const expand = computeExpandMap(input);
  if (expand.size === 0) return input;

  let out = '';
  let depth = 0;
  let i = 0;
  const expanded: boolean[] = [];

  const newline = () => {
    // Drop the trailing whitespace run only, to stay O(1) amortized.
    let end = out.length;
    while (end > 0 && (out[end - 1] === ' ' || out[end - 1] === '\t')) end -= 1;
    out = out.slice(0, end) + '\n' + indentUnit.repeat(depth);
  };

  const skipWhitespace = () => {
    while (i < input.length && isWhitespace(input[i])) i += 1;
  };

  while (i < input.length) {
    const ch = input[i];

    if (ch === '"' || ch === "'") {
      const { literal, next } = readStringLiteral(input, i);
      out += decodeStringLiteral(literal);
      i = next;
    } else if (OPEN_BRACKETS.has(ch)) {
      const doExpand = expand.get(i) ?? false;
      expanded.push(doExpand);
      out += ch;
      i += 1;
      if (doExpand) {
        depth += 1;
        newline();
        skipWhitespace();
      }
    } else if (CLOSE_BRACKETS.has(ch)) {
      if (expanded.pop() ?? false) {
        depth -= 1;
        newline();
      }
      out += ch;
      i += 1;
    } else if (ch === ',' && (expanded[expanded.length - 1] ?? false)) {
      out += ch;
      i += 1;
      newline();
      skipWhitespace();
    } else {
      out += ch;
      i += 1;
    }
  }

  return out;
};
