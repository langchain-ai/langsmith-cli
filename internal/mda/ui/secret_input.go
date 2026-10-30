package ui

// Reading a secret in from the outside world, for any command that stores one.
//
// A value only travels one way. Nothing here reads a secret back, and there is
// deliberately no flag that takes one inline, which would leave it in shell
// history and `ps`. Error messages report sizes and counts, never content.

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
)

// maxValueBytes matches the server's cap. A file is checked against its
// metadata and stdin is read through a limit, so an oversized value never
// reaches memory either way.
const maxValueBytes = 1 << 20

// ValueSource says where a secret's value comes from. At most one field is
// set; the zero value is unspecified: a masked prompt on a terminal, stdin
// otherwise.
type ValueSource struct {
	// FromEnv names the environment variable holding the value.
	FromEnv string
	// FromFile is the path of a file holding the value.
	FromFile string
}

// ValueSourceIsExplicit reports whether source names where the value is.
func ValueSourceIsExplicit(source ValueSource) bool {
	return source.FromEnv != "" || source.FromFile != ""
}

// ResolvedValue is a resolved value and where it came from. Source is a label
// for the confirmation line; the value itself never reaches a message.
type ResolvedValue struct {
	Value  string
	Source string
}

// LookupEnv finds the variable name for [ValueSource.FromEnv], from the shell
// or else the project `.env`, reporting which. Callers pass a closure over
// `hostapi.LookupEnvWithOrigin` and the project's `.env`.
type LookupEnv func(name string) (value string, fromEnvFile bool, ok bool)

// ResolveValue resolves source on the default terminal. prompt and stdinHint
// are the two strings that name the caller's own command, so every caller
// shares this without printing another command's example.
func ResolveValue(ctx context.Context, source ValueSource, lookupEnv LookupEnv, prompt, stdinHint string) (ResolvedValue, error) {
	return Default().ResolveValue(ctx, source, lookupEnv, prompt, stdinHint)
}

// ResolveValue resolves source, prompting on t or reading its input when
// source is unspecified.
func (t *Terminal) ResolveValue(ctx context.Context, source ValueSource, lookupEnv LookupEnv, prompt, stdinHint string) (ResolvedValue, error) {
	switch {
	case source.FromEnv != "":
		name := source.FromEnv
		value, fromEnvFile, ok := lookupEnv(name)
		if !ok {
			return ResolvedValue{}, errs.Categorized(errs.UserInput,
				fmt.Sprintf("%s is not set in the environment or the project .env", name))
		}
		origin := "the shell"
		if fromEnvFile {
			origin = ".env"
		}
		return ResolvedValue{Value: value, Source: fmt.Sprintf("%s in %s", name, origin)}, nil
	case source.FromFile == "-":
		return ResolvedValue{}, errs.Categorized(errs.UserInput, fmt.Sprintf(
			"--secret-from-file takes a path. To read the value from stdin, redirect or pipe "+
				"it instead: `%s`.", stdinHint))
	case source.FromFile != "":
		value, err := ValueFromFile(source.FromFile)
		if err != nil {
			return ResolvedValue{}, err
		}
		return ResolvedValue{Value: value, Source: source.FromFile}, nil
	case t.IsInputTerminal():
		value, err := Password(prompt).Mask('•').InteractOn(ctx, t)
		if err != nil {
			return ResolvedValue{}, fmt.Errorf("failed to read the secret value: %w", err)
		}
		if err := rejectEmpty(value, "given"); err != nil {
			return ResolvedValue{}, err
		}
		return ResolvedValue{Value: value}, nil
	default:
		value, err := valueFromReader(t.in)
		if err != nil {
			return ResolvedValue{}, err
		}
		return ResolvedValue{Value: value, Source: "stdin"}, nil
	}
}

// ReadPipedStdinSecret reads a piped secret from the default terminal's input
// without treating a TTY or an empty pipe as a value.
//
// An unspecified [ValueSource] covers both "prompt on a terminal" and "read
// stdin". Call this only after ruling out OAuth flags and explicit value
// sources, so a token on stdin is not discarded by another create path.
func ReadPipedStdinSecret() (string, bool, error) {
	return Default().ReadPipedStdinSecret()
}

// ReadPipedStdinSecret reads a piped secret from t's input.
func (t *Terminal) ReadPipedStdinSecret() (string, bool, error) {
	if t.IsInputTerminal() {
		return "", false, nil
	}
	return optionalSecretFromReader(t.in)
}

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

func valueFromReader(reader io.Reader) (string, error) {
	value, ok, err := optionalSecretFromReader(reader)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", errs.Categorized(errs.UserInput, "no secret value on stdin")
	}
	return value, nil
}

func optionalSecretFromReader(reader io.Reader) (string, bool, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maxValueBytes+1))
	if err != nil {
		return "", false, fmt.Errorf("failed to read the secret value from stdin: %w", err)
	}
	if len(raw) > maxValueBytes {
		return "", false, tooLarge("the value on stdin", int64(len(raw)))
	}
	value := stripOneTrailingNewline(string(raw))
	if strings.TrimSpace(value) == "" {
		return "", false, nil
	}
	if err := rejectDotenvShaped(value, "the input"); err != nil {
		return "", false, err
	}
	return value, true, nil
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
