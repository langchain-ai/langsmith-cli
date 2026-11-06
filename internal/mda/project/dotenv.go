package project

// The `.env` parser of the dotenvy crate 0.15.7 (MIT, Copyright (c) 2014
// Santiago Lapresta and contributors), which the mda CLI reads `.env` files
// with: `iter.rs` joins physical lines into logical ones, `parse.rs` parses a
// logical line into a key and a value.

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

// dotenvParser yields the entries of a `.env` file in order.
type dotenvParser struct {
	lines dotenvLines
	// substitutions holds the values assigned so far, for `$VAR` references.
	substitutions map[string]string
	lookupEnv     func(string) (string, bool)
}

func newDotenvParser(contents string, lookupEnv func(string) (string, bool)) *dotenvParser {
	return &dotenvParser{
		lines:         dotenvLines{rest: contents},
		substitutions: map[string]string{},
		lookupEnv:     lookupEnv,
	}
}

// next returns the next entry, an error for a malformed one (after which the
// parser goes on to the following line), or io.EOF.
func (p *dotenvParser) next() (key, value string, err error) {
	for {
		line, err := p.lines.next()
		if err != nil {
			return "", "", err
		}
		key, value, ok, err := p.parseLine(line)
		if err != nil || ok {
			return key, value, err
		}
	}
}

// dotenvLines splits a `.env` file into logical lines: a quoted value may span
// physical lines, and a comment after whitespace ends the line.
type dotenvLines struct {
	rest string
}

type dotenvLineState int

const (
	lineComplete dotenvLineState = iota
	lineEscape
	lineStrongOpen
	lineStrongOpenEscape
	lineWeakOpen
	lineWeakOpenEscape
	lineComment
	lineWhiteSpace
)

func evalEndState(state dotenvLineState, buf string) (int, dotenvLineState) {
	position := 0
	for index, c := range buf {
		position = index
		switch state {
		case lineWhiteSpace:
			switch c {
			case '#':
				return position, lineComment
			case '\\':
				state = lineEscape
			case '"':
				state = lineWeakOpen
			case '\'':
				state = lineStrongOpen
			default:
				state = lineComplete
			}
		case lineEscape:
			state = lineComplete
		case lineComplete:
			switch {
			case unicode.IsSpace(c) && c != '\n' && c != '\r':
				state = lineWhiteSpace
			case c == '\\':
				state = lineEscape
			case c == '"':
				state = lineWeakOpen
			case c == '\'':
				state = lineStrongOpen
			}
		case lineWeakOpen:
			switch c {
			case '\\':
				state = lineWeakOpenEscape
			case '"':
				state = lineComplete
			}
		case lineWeakOpenEscape:
			state = lineWeakOpen
		case lineStrongOpen:
			switch c {
			case '\\':
				state = lineStrongOpenEscape
			case '\'':
				state = lineComplete
			}
		case lineStrongOpenEscape:
			state = lineStrongOpen
		case lineComment:
			panic("dotenv: a comment ends the line")
		}
	}
	return position, state
}

// readLine reads through the next `\n`, or to the end, like Rust's
// `BufRead::read_line`, which fails on (and consumes) a line that is not
// UTF-8.
func (l *dotenvLines) readLine() (string, error) {
	end := strings.IndexByte(l.rest, '\n') + 1
	if end == 0 {
		end = len(l.rest)
	}
	line := l.rest[:end]
	l.rest = l.rest[end:]
	if !utf8.ValidString(line) {
		return "", errors.New("stream did not contain valid UTF-8")
	}
	return line, nil
}

func (l *dotenvLines) next() (string, error) {
	buf := ""
	state := lineComplete
	for {
		bufPos := len(buf)
		line, err := l.readLine()
		if err != nil {
			return "", err
		}
		if line == "" {
			if state == lineComplete {
				return "", io.EOF
			}
			return "", fmt.Errorf("unterminated .env line %q", buf)
		}
		buf += line
		// Lines that start with a # are skipped before they are evaluated.
		if strings.HasPrefix(strings.TrimLeftFunc(buf, unicode.IsSpace), "#") {
			return "", nil
		}
		var position int
		position, state = evalEndState(state, buf[bufPos:])
		switch state {
		case lineComplete:
			if trimmed, ok := strings.CutSuffix(buf, "\n"); ok {
				buf = strings.TrimSuffix(trimmed, "\r")
			}
			return buf, nil
		case lineComment:
			return buf[:bufPos+position], nil
		}
	}
}

// parseLine parses a logical line; ok is false for a blank or comment line.
func (p *dotenvParser) parseLine(line string) (key, value string, ok bool, err error) {
	original := line
	invalid := func() error { return fmt.Errorf("invalid .env line %q", original) }

	line = strings.TrimRightFunc(line, unicode.IsSpace)
	line = strings.TrimLeftFunc(line, unicode.IsSpace)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false, nil
	}

	key, line, ok = parseDotenvKey(line)
	if !ok {
		return "", "", false, invalid()
	}
	line = strings.TrimLeftFunc(line, unicode.IsSpace)

	// export can be either an optional prefix or a key itself
	if key == "export" && !strings.HasPrefix(line, "=") {
		key, line, ok = parseDotenvKey(line)
		if !ok {
			return "", "", false, invalid()
		}
		line = strings.TrimLeftFunc(line, unicode.IsSpace)
	}
	line, ok = strings.CutPrefix(line, "=")
	if !ok {
		return "", "", false, invalid()
	}
	line = strings.TrimLeftFunc(line, unicode.IsSpace)

	if line == "" || strings.HasPrefix(line, "#") {
		p.substitutions[key] = ""
		return key, "", true, nil
	}

	value, err = p.parseValue(line)
	if err != nil {
		return "", "", false, err
	}
	p.substitutions[key] = value
	return key, value, true, nil
}

// parseDotenvKey splits a key (an ASCII letter or `_`, then ASCII letters,
// digits, `_` and `.`) off the front of line.
func parseDotenvKey(line string) (key, rest string, ok bool) {
	if line == "" || !(isASCIIAlpha(line[0]) || line[0] == '_') {
		return "", line, false
	}
	end := strings.IndexFunc(line, func(c rune) bool {
		return !(c < 0x80 && (isASCIIAlphanumeric(byte(c)) || c == '_' || c == '.'))
	})
	if end < 0 {
		end = len(line)
	}
	return line[:end], line[end:], true
}

type substitutionMode int

const (
	substitutionNone substitutionMode = iota
	substitutionBlock
	substitutionEscapedBlock
)

func (p *dotenvParser) parseValue(input string) (string, error) {
	invalid := func() error { return fmt.Errorf("invalid .env value %q", input) }
	strongQuote := false // '
	weakQuote := false   // "
	escaped := false
	expectingEnd := false

	var output strings.Builder
	mode := substitutionNone
	var name strings.Builder
	substitute := func() {
		output.WriteString(p.substitution(name.String()))
		name.Reset()
	}

values:
	for _, c := range input {
		// expectingEnd permits `k=v #comment` without affecting `k=v#comment`,
		// and rejects `k=v w`.
		switch {
		case expectingEnd:
			if c == ' ' || c == '\t' {
				continue
			}
			if c == '#' {
				break values
			}
			return "", invalid()
		case escaped:
			switch c {
			case '\\', '\'', '"', '$', ' ':
				output.WriteRune(c)
			case 'n':
				output.WriteByte('\n')
			default:
				return "", invalid()
			}
			escaped = false
		case strongQuote:
			if c == '\'' {
				strongQuote = false
			} else {
				output.WriteRune(c)
			}
		case mode != substitutionNone:
			switch {
			case isRustAlphanumeric(c):
				name.WriteRune(c)
			case mode == substitutionBlock:
				if c == '{' && name.Len() == 0 {
					mode = substitutionEscapedBlock
				} else {
					substitute()
					if c == '$' {
						mode = substitutionBlock
					} else {
						mode = substitutionNone
						output.WriteRune(c)
					}
				}
			default: // substitutionEscapedBlock
				if c == '}' {
					mode = substitutionNone
					substitute()
				} else {
					name.WriteRune(c)
				}
			}
		case c == '$':
			mode = substitutionBlock
		case weakQuote:
			switch c {
			case '"':
				weakQuote = false
			case '\\':
				escaped = true
			default:
				output.WriteRune(c)
			}
		case c == '\'':
			strongQuote = true
		case c == '"':
			weakQuote = true
		case c == '\\':
			escaped = true
		case c == ' ' || c == '\t':
			expectingEnd = true
		default:
			output.WriteRune(c)
		}
	}

	if mode == substitutionEscapedBlock || strongQuote || weakQuote {
		return "", invalid()
	}
	substitute()
	return output.String(), nil
}

// substitution is the value of `$name`: the process environment's, else the
// value assigned earlier in the file, else empty.
func (p *dotenvParser) substitution(name string) string {
	if name == "" {
		return ""
	}
	if value, ok := p.lookupEnv(name); ok {
		return value
	}
	return p.substitutions[name]
}

func isASCIIAlpha(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// isRustAlphanumeric is Rust's `char::is_alphanumeric`: the Unicode Alphabetic
// or Numeric property.
func isRustAlphanumeric(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsNumber(c) ||
		unicode.In(c, unicode.Other_Alphabetic, unicode.Other_Lowercase, unicode.Other_Uppercase)
}
