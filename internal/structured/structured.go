package structured

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/itchyny/gojq"
	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

type Spec interface {
	RenderText(io.Writer, any) error
}

type Command[I any] struct {
	Use    string
	Short  string
	Long   string
	Args   cobra.PositionalArgs
	Input  func(*cobra.Command) I
	Action func(context.Context, *cobra.Command, I, []string) (any, error)
	Render Spec
}

type Result struct {
	Model          any
	TextModel      any
	ErrAfterRender error
}

func (c Command[I]) Cobra() *cobra.Command {
	var input I
	cmd := &cobra.Command{
		Use:   c.Use,
		Short: c.Short,
		Long:  c.Long,
		Args:  c.Args,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := c.Action(cmd.Context(), cmd, input, args)
			if err != nil {
				return err
			}
			return Render(cmd, result, c.Render)
		},
	}
	if c.Input != nil {
		input = c.Input(cmd)
	}
	cmd.Flags().String("jq", "", "Filter JSON output using a jq expression")
	return cmd
}

type Parent struct {
	Use      string
	Short    string
	Long     string
	Children []func() *cobra.Command
}

func (p Parent) Cobra() *cobra.Command {
	cmd := &cobra.Command{
		Use:   p.Use,
		Short: p.Short,
		Long:  p.Long,
	}
	for _, child := range p.Children {
		cmd.AddCommand(child())
	}
	return cmd
}

func Render(cmd *cobra.Command, model any, spec Spec) error {
	errAfterRender := error(nil)
	textModel := model
	if result, ok := model.(Result); ok {
		model = result.Model
		textModel = result.TextModel
		if textModel == nil {
			textModel = model
		}
		errAfterRender = result.ErrAfterRender
	}

	w := cmd.OutOrStdout()
	var err error
	if expr := cmdutil.ResolveJQ(cmd); expr != "" {
		err = renderJQ(w, model, expr)
	} else if cmdutil.ResolveFormat(cmd) != "pretty" || spec == nil {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		err = enc.Encode(model)
	} else {
		err = spec.RenderText(w, textModel)
	}
	if err != nil {
		return err
	}
	return errAfterRender
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

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"dash":              dash,
		"formatBytes":       formatBytes,
		"formatBytesOrDash": formatBytesOrDash,
		"formatCount":       formatCount,
		"formatTime":        formatTime,
		"jsonIndent":        jsonIndent,
		"shortID":           shortID,
	}
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func formatBytes(b int64) string {
	const gb = 1024 * 1024 * 1024
	const mb = 1024 * 1024
	if b >= gb {
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	}
	return fmt.Sprintf("%.0f MB", float64(b)/float64(mb))
}

func formatBytesOrDash(b int64) string {
	if b <= 0 {
		return "-"
	}
	return formatBytes(b)
}

func formatCount(n int64) string {
	if n <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d", n)
}

func formatTime(ts string) string {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return ts
	}
	return t.Local().Format("2006-01-02 15:04")
}

func shortID(id string) string {
	if id == "" {
		return "-"
	}
	if len(id) > 8 {
		return id[:8] + "..."
	}
	return id
}

func jsonIndent(v any, prefix, indent string) (string, error) {
	b, err := json.MarshalIndent(v, prefix, indent)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type Template string

func (s Template) RenderText(w io.Writer, model any) error {
	tmpl, err := template.New("text").Funcs(templateFuncs()).Parse(string(s))
	if err != nil {
		return err
	}
	return tmpl.Execute(w, model)
}

type Table struct {
	Title   string
	Rows    string
	Columns []Column
	Footer  Template
}

type Column struct {
	Header   string
	Template string
}

func (t Table) RenderText(w io.Writer, model any) error {
	rows, err := resolveRows(model, t.Rows)
	if err != nil {
		return err
	}

	columnTemplates := make([]*template.Template, 0, len(t.Columns))
	headers := make([]string, 0, len(t.Columns))
	for _, col := range t.Columns {
		headers = append(headers, col.Header)
		tmpl, err := template.New(col.Header).Funcs(templateFuncs()).Parse(col.Template)
		if err != nil {
			return fmt.Errorf("parsing column %q: %w", col.Header, err)
		}
		columnTemplates = append(columnTemplates, tmpl)
	}

	if t.Title != "" {
		fmt.Fprintln(w, t.Title)
		fmt.Fprintln(w, strings.Repeat("-", len(t.Title)))
	}

	table := tablewriter.NewWriter(w)
	table.SetHeader(headers)
	table.SetBorder(false)
	table.SetColumnSeparator("  ")
	table.SetHeaderLine(true)
	table.SetAutoWrapText(false)

	for _, rowModel := range rows {
		row := make([]string, 0, len(columnTemplates))
		for i, tmpl := range columnTemplates {
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, rowModel); err != nil {
				return fmt.Errorf("rendering column %q: %w", t.Columns[i].Header, err)
			}
			row = append(row, buf.String())
		}
		table.Append(row)
	}

	table.Render()
	if t.Footer != "" {
		if err := t.Footer.RenderText(w, model); err != nil {
			return err
		}
	}
	return nil
}

func resolveRows(model any, path string) ([]any, error) {
	if path == "" {
		path = "."
	}

	value := reflect.ValueOf(model)
	if path != "." {
		parts := strings.Split(strings.TrimPrefix(path, "."), ".")
		for _, part := range parts {
			if part == "" {
				continue
			}
			value = dereference(value)
			if !value.IsValid() {
				return nil, fmt.Errorf("rows path %q resolved to nil", path)
			}
			switch value.Kind() {
			case reflect.Struct:
				value = value.FieldByName(part)
			case reflect.Map:
				value = value.MapIndex(reflect.ValueOf(part))
			default:
				return nil, fmt.Errorf("rows path %q cannot select %q from %s", path, part, value.Kind())
			}
			if !value.IsValid() {
				return nil, fmt.Errorf("rows path %q could not resolve %q", path, part)
			}
		}
	}

	value = dereference(value)
	if !value.IsValid() {
		return nil, nil
	}
	if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
		return nil, fmt.Errorf("rows path %q resolved to %s, want slice or array", path, value.Kind())
	}

	rows := make([]any, 0, value.Len())
	for i := 0; i < value.Len(); i++ {
		rows = append(rows, value.Index(i).Interface())
	}
	return rows, nil
}

func dereference(value reflect.Value) reflect.Value {
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	return value
}
