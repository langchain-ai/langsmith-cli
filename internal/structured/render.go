package structured

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/itchyny/gojq"
	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

type Spec interface {
	RenderText(io.Writer, any) error
}

func Render(cmd *cobra.Command, model any, spec Spec) error {
	w := cmd.OutOrStdout()
	if expr := cmdutil.ResolveJQ(cmd); expr != "" {
		return renderJQ(w, model, expr)
	}
	if cmdutil.ResolveFormat(cmd) != "pretty" || spec == nil {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(model)
	}
	return spec.RenderText(w, model)
}

func renderJQ(w io.Writer, model any, expr string) error {
	query, err := gojq.Parse(expr)
	if err != nil {
		var e *gojq.ParseError
		if errors.As(err, &e) {
			str, line, column := jqLineColumn(expr, e.Offset-len(e.Token))
			return fmt.Errorf("failed to parse jq expression (line %d, column %d)\n    %s\n    %*c  %w", line, column, str, column, '^', err)
		}
		return err
	}
	code, err := gojq.Compile(query, gojq.WithEnvironLoader(os.Environ))
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

func jqScalar(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case float64:
		if math.Trunc(t) == t {
			return strconv.FormatFloat(t, 'f', 0, 64), true
		}
		return strconv.FormatFloat(t, 'f', 2, 64), true
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
