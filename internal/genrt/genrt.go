// Package genrt is the small runtime that generated commands call into, so
// the generated code stays a thin mapping from flags to SDK calls.
package genrt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/structured"
	"github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

// Operation annotations set on every generated command.
const (
	AnnotationOperationID = "langsmith.operation_id"
	AnnotationRisk        = "langsmith.risk"
)

// Required fails with invalid_input when any named flag was not set.
func Required(cmd *cobra.Command, flags ...string) error {
	var missing []string
	for _, name := range flags {
		if !cmd.Flags().Changed(name) {
			missing = append(missing, "--"+name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return structured.NewError(structured.CategoryInvalidInput, "missing required flag(s): "+strings.Join(missing, ", "))
}

// OneOf validates an enum flag when it was set.
func OneOf(cmd *cobra.Command, flag, value string, allowed ...string) error {
	if !cmd.Flags().Changed(flag) || slices.Contains(allowed, value) {
		return nil
	}
	return structured.NewError(structured.CategoryInvalidInput,
		fmt.Sprintf("invalid --%s %q: must be one of %s", flag, value, strings.Join(allowed, ", ")))
}

// Time parses an RFC 3339 timestamp flag.
func Time(flag, value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, structured.NewError(structured.CategoryInvalidInput,
			fmt.Sprintf("invalid --%s %q: use RFC 3339, e.g. 2026-01-02T15:04:05Z", flag, value))
	}
	return t, nil
}

// JSON reads a JSON flag value: inline JSON, @path, or @- for stdin.
func JSON(flag, value string) (any, error) {
	data := []byte(value)
	if path, ok := strings.CutPrefix(value, "@"); ok {
		var err error
		if path == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(path)
		}
		if err != nil {
			return nil, structured.NewError(structured.CategoryInvalidInput, fmt.Sprintf("reading --%s: %v", flag, err))
		}
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, structured.NewError(structured.CategoryInvalidInput, fmt.Sprintf("invalid JSON in --%s: %v", flag, err))
	}
	return v, nil
}

// ConfirmDestructive requires --yes before a destructive write runs.
func ConfirmDestructive(cmd *cobra.Command) error {
	if yes, _ := cmd.Flags().GetBool("yes"); yes {
		return nil
	}
	return structured.NewError(structured.CategoryInvalidInput,
		fmt.Sprintf("%q is a destructive write; rerun with --yes to confirm", cmd.CommandPath()))
}

// Raw returns the API's JSON for an SDK response, so output matches the wire
// format instead of the SDK struct's zero values. Values without raw JSON
// (maps, slices of scalars) are returned as they are.
func Raw(v any) any {
	if raw, ok := rawJSON(v); ok {
		var decoded any
		if json.Unmarshal([]byte(raw), &decoded) == nil {
			return decoded
		}
	}
	return v
}

// RawItems applies Raw to each page item.
func RawItems[T any](items []T) []any {
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = Raw(item)
	}
	return out
}

func rawJSON(v any) (string, bool) {
	value := reflect.ValueOf(v)
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return "", false
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return "", false
	}
	field := value.FieldByName("JSON")
	if !field.IsValid() {
		return "", false
	}
	method := field.MethodByName("RawJSON")
	if !method.IsValid() {
		return "", false
	}
	out := method.Call(nil)
	if len(out) != 1 {
		return "", false
	}
	raw, ok := out[0].Interface().(string)
	return raw, ok && raw != ""
}

// ProjectID resolves --project (a name) or --project-id (a UUID) to a
// project ID. Generated commands expose every session_id parameter this way.
func ProjectID(ctx context.Context, c *client.Client, name, id string) (string, error) {
	if name != "" && id != "" {
		return "", structured.NewError(structured.CategoryInvalidInput, "pass --project or --project-id, not both")
	}
	if id != "" {
		if _, err := uuid.Parse(id); err != nil {
			return "", structured.NewError(structured.CategoryInvalidInput, fmt.Sprintf("invalid --project-id %q: must be a project UUID", id))
		}
		return id, nil
	}
	// Only an empty result means the name is unknown; network and API errors
	// keep their own category.
	page, err := c.SDK.Sessions.List(ctx, langsmith.SessionListParams{Name: langsmith.F(name), Limit: langsmith.F(int64(1))})
	if err != nil {
		return "", structured.Classify(err)
	}
	if len(page.Items) == 0 {
		return "", structured.NewError(structured.CategoryNotFound, fmt.Sprintf("no project named %q", name))
	}
	return page.Items[0].ID, nil
}

// RequiredOneOf fails with invalid_input when none of the flags was set.
func RequiredOneOf(cmd *cobra.Command, flags ...string) error {
	for _, name := range flags {
		if cmd.Flags().Changed(name) {
			return nil
		}
	}
	return structured.NewError(structured.CategoryInvalidInput, "missing required flag: one of --"+strings.Join(flags, ", --"))
}

// APIError classifies an SDK error and rewrites API field names in the
// server's message to the flags that set them (session_id -> --project).
func APIError(err error, flags map[string]string) error {
	se := structured.Classify(err)
	for wire, flag := range flags {
		se.Message = replaceWord(se.Message, wire, flag)
		se.Hint = replaceWord(se.Hint, wire, flag)
	}
	return se
}

func replaceWord(text, word, with string) string {
	var b strings.Builder
	for {
		i := strings.Index(text, word)
		if i < 0 {
			b.WriteString(text)
			return b.String()
		}
		end := i + len(word)
		boundary := (i == 0 || !isIdent(text[i-1])) && (end == len(text) || !isIdent(text[end]))
		b.WriteString(text[:i])
		if boundary {
			b.WriteString(with)
		} else {
			b.WriteString(word)
		}
		text = text[end:]
	}
}

// isIdent also treats '.' as part of a name, so paths like query.id.0 are left alone.
func isIdent(c byte) bool {
	return c == '_' || c == '-' || c == '.' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
