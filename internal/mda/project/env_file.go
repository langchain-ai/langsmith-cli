package project

// Minimal `.env` reading for resolving the LangSmith API key and forwarding
// other vars as deployment secrets (`deploy/env_file.rs`).

import (
	"errors"
	"io"
	"os"
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
