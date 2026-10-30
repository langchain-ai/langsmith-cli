package ui

// Reading a secret in from a file, for any command that stores one.
//
// A value only travels one way. Nothing here reads a secret back, and there is
// deliberately no flag that takes one inline, which would leave it in shell
// history and `ps`. Error messages report sizes and counts, never content.

import (
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
)

// maxValueBytes matches the server's cap. A file is checked against its
// metadata, so an oversized value never reaches memory.
const maxValueBytes = 1 << 20

// ValueFromFile reads a secret's value from the file at path.
func ValueFromFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", path, err)
	}
	if size := info.Size(); size > maxValueBytes {
		return "", tooLarge(path, size)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", path, err)
	}
	value := stripOneTrailingNewline(string(raw))
	if err := rejectEmpty(value, "in "+path); err != nil {
		return "", err
	}
	if err := rejectDotenvShaped(value, path); err != nil {
		return "", err
	}
	return value, nil
}

func tooLarge(what string, size int64) error {
	return errs.Categorized(errs.UserInput,
		fmt.Sprintf("%s is over the %d byte limit (%d bytes or more)", what, maxValueBytes, size))
}

func rejectEmpty(value, what string) error {
	if strings.TrimSpace(value) == "" {
		return errs.Categorized(errs.UserInput, "no secret value "+what)
	}
	return nil
}

func rejectDotenvShaped(value, what string) error {
	assignments := 0
	for _, line := range lines(value) {
		if isAssignment(line) {
			assignments++
		}
	}
	if assignments < 2 {
		return nil
	}
	return errs.Categorized(errs.UserInput, fmt.Sprintf(
		"%s looks like a .env file: %d lines assign a variable, and "+
			"all of it would be stored as one secret. Store one value per secret, "+
			"and use --secret-from-env <VAR> to read just one variable.", what, assignments))
}

func isAssignment(line string) bool {
	line = strings.TrimLeftFunc(line, unicode.IsSpace)
	line = strings.TrimLeftFunc(strings.TrimPrefix(line, "export "), unicode.IsSpace)
	name, _, found := strings.Cut(line, "=")
	if !found || name == "" {
		return false
	}
	for index, r := range name {
		letter := r == '_' || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z')
		digit := '0' <= r && r <= '9'
		if !letter && (index == 0 || !digit) {
			return false
		}
	}
	return true
}

func stripOneTrailingNewline(raw string) string {
	if trimmed, ok := strings.CutSuffix(raw, "\n"); ok {
		return strings.TrimSuffix(trimmed, "\r")
	}
	return raw
}
