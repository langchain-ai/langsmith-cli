package project

// Minimal `.env` reading/writing for resolving and persisting the LangSmith API
// key (and forwarding other vars as deployment secrets) (`deploy/env_file.rs`).

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// ParseEnvFile parses a `.env` file into a map of `KEY=VALUE` pairs.
//
// Uses dotenv semantics (quotes, `export`, full-line and inline `#` comments,
// escapes, and `$VAR` substitution), as the dotenvy crate implements them.
// Missing or unreadable files yield an empty map; malformed lines are skipped.
// A key that appears more than once keeps its last value.
func ParseEnvFile(path string) map[string]string {
	entries := map[string]string{}
	contents, err := os.ReadFile(path)
	if err != nil {
		return entries
	}
	parser := newDotenvParser(string(contents), os.LookupEnv)
	for {
		key, value, err := parser.next()
		if errors.Is(err, io.EOF) {
			return entries
		}
		if err == nil {
			entries[key] = value
		}
	}
}

// SetEnvKey sets (or replaces) `key=value` in the `.env` file at path,
// creating it if necessary and preserving other lines.
//
// No-ops when the key is already present with the same value so watch-mode
// restages do not bump mtime and wake LangGraph's `.env` watcher.
func SetEnvKey(path, key, value string) error {
	existing, _ := readToString(path) // A missing or unreadable file starts empty.
	if current, ok := ParseEnvFile(path)[key]; ok && current == value {
		return nil
	}

	var lines []string
	if existing != "" {
		lines = textLines(existing)
	}

	newLine := key + "=" + value
	replaced := false
	for i, line := range lines {
		if envLineMatchesKey(line, key) {
			lines[i] = newLine
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append(lines, newLine)
	}

	out := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

// ProjectEnvPersistPath is the project env file to update when recovering
// LangSmith credentials.
//
// Prefers an existing `.env`, then `.env.local`, then creates `.env` — the same
// order `mda dev` uses when staging env for local runs.
func ProjectEnvPersistPath(projectRoot string) string {
	env := filepath.Join(projectRoot, ".env")
	if isFile(env) {
		return env
	}
	local := filepath.Join(projectRoot, ".env.local")
	if isFile(local) {
		return local
	}
	return env
}

// RemoveEnvKey removes key from the `.env` file at path when present.
//
// No-ops when the key is absent so watch restages do not bump mtime.
func RemoveEnvKey(path, key string) error {
	existing, _ := readToString(path) // A missing or unreadable file has no key to remove.
	if existing == "" {
		return nil
	}
	lines := textLines(existing)
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if !envLineMatchesKey(line, key) {
			kept = append(kept, line)
		}
	}
	if len(kept) == len(lines) {
		return nil
	}
	out := strings.Join(kept, "\n")
	if out != "" {
		out += "\n"
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

// envLineMatchesKey reports whether line assigns key, with or without an
// `export ` prefix.
func envLineMatchesKey(line, key string) bool {
	trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
	trimmed = strings.TrimPrefix(trimmed, "export ")
	name, _, ok := strings.Cut(trimmed, "=")
	return ok && strings.TrimFunc(name, unicode.IsSpace) == key
}

// textLines splits text into lines like Rust's `str::lines`: at `\n`, dropping
// a `\r` before it, with no empty line after a final newline.
func textLines(text string) []string {
	lines := strings.Split(text, "\n")
	last := lines[len(lines)-1]
	lines = lines[:len(lines)-1]
	for i, line := range lines {
		lines[i] = strings.TrimSuffix(line, "\r")
	}
	if last != "" {
		lines = append(lines, last)
	}
	return lines
}
