package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/itchyny/gojq"
)

// activeJQ is the --jq expression of the running command. The root command
// sets it before each run, so every JSON writer in this package applies it.
var activeJQ string

// SetJQ sets the jq expression applied to JSON written by this package.
func SetJQ(expr string) { activeJQ = expr }

// JQ returns the jq expression applied to JSON written by this package.
func JQ() string { return activeJQ }

// CompileJQ parses and compiles a jq expression. Parse errors point at the
// offending column so a typo is easy to spot.
func CompileJQ(expr string) (*gojq.Code, error) {
	query, err := gojq.Parse(expr)
	if err != nil {
		var e *gojq.ParseError
		if errors.As(err, &e) {
			str, line, column := jqLineColumn(expr, e.Offset-len(e.Token))
			return nil, fmt.Errorf("failed to parse jq expression (line %d, column %d)\n    %s\n    %*c  %w", line, column, str, column, '^', err)
		}
		return nil, err
	}
	return gojq.Compile(query, gojq.WithEnvironLoader(os.Environ))
}

// RenderJQ runs expr over the JSON encoding of model and writes each result
// on its own line. Strings, numbers, booleans and null print bare, like
// `jq -r`; arrays and objects print as compact JSON.
func RenderJQ(w io.Writer, model any, expr string) error {
	code, err := CompileJQ(expr)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(model); err != nil {
		return err
	}
	var data any
	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		return err
	}

	iter := code.Run(data)
	for {
		v, ok := iter.Next()
		if !ok {
			return nil
		}
		if err, ok := v.(error); ok {
			var halt *gojq.HaltError
			if errors.As(err, &halt) && halt.Value() == nil {
				return nil
			}
			return err
		}
		if text, ok := jqScalar(v); ok {
			if _, err := fmt.Fprintln(w, text); err != nil {
				return err
			}
			continue
		}
		enc := json.NewEncoder(w)
		if err := enc.Encode(v); err != nil {
			return err
		}
	}
}

// WriteJSON writes v as indented JSON, or through the active --jq expression
// when one is set.
func WriteJSON(w io.Writer, v any) error {
	if activeJQ != "" {
		return RenderJQ(w, v, activeJQ)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func jqScalar(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case float64:
		// Same text the JSON encoder writes, so a bare number keeps its full
		// precision and matches the value in the unfiltered JSON output. NaN
		// and infinities have no JSON form and fall back to Go's shortest text.
		b, err := json.Marshal(t)
		if err != nil {
			return strconv.FormatFloat(t, 'g', -1, 64), true
		}
		return string(b), true
	case nil:
		return "", true
	case bool:
		return fmt.Sprintf("%v", t), true
	default:
		return "", false
	}
}

func jqLineColumn(expr string, offset int) (string, int, int) {
	for line := 1; ; line++ {
		index := strings.Index(expr, "\n")
		if index < 0 {
			return expr, line, offset + 1
		}
		if index >= offset {
			return expr[:index], line, offset + 1
		}
		expr = expr[index+1:]
		offset -= index + 1
	}
}

// WriteJSONLine writes v as one line of compact JSON, or through the active
// --jq expression when one is set.
func WriteJSONLine(w io.Writer, v any) error {
	if activeJQ != "" {
		return RenderJQ(w, v, activeJQ)
	}
	return json.NewEncoder(w).Encode(v)
}
