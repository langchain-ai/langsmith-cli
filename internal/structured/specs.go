package structured

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"
	"text/template"

	"github.com/langchain-ai/langsmith-cli/internal/output"
)

type Template string

func (s Template) RenderText(w io.Writer, model any) error {
	tmpl, err := template.New("text").Funcs(templateFuncs()).Parse(string(s))
	if err != nil {
		return err
	}
	return tmpl.Execute(w, model)
}

type PropertyList struct {
	Title      string
	Properties []Property
	Caption    string
}

type Property struct {
	Label     string
	Template  string
	OmitEmpty bool
}

func (p PropertyList) RenderText(w io.Writer, model any) error {
	type renderedProperty struct {
		label string
		value string
	}

	props := make([]renderedProperty, 0, len(p.Properties))
	maxLabelWidth := 0
	for _, prop := range p.Properties {
		tmpl, err := template.New(prop.Label).Funcs(templateFuncs()).Parse(prop.Template)
		if err != nil {
			return fmt.Errorf("parsing property %q: %w", prop.Label, err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, model); err != nil {
			return fmt.Errorf("rendering property %q: %w", prop.Label, err)
		}
		value := strings.TrimSpace(buf.String())
		if prop.OmitEmpty && value == "" {
			continue
		}
		props = append(props, renderedProperty{
			label: prop.Label,
			value: value,
		})
		if width := len(prop.Label) + 1; width > maxLabelWidth {
			maxLabelWidth = width
		}
	}

	if p.Title != "" {
		fmt.Fprintln(w, p.Title)
		fmt.Fprintln(w, strings.Repeat("-", len(p.Title)))
	}

	for _, prop := range props {
		lines := strings.Split(prop.value, "\n")
		if len(lines) == 0 {
			lines = []string{""}
		}
		fmt.Fprintf(w, "%-*s  %s\n", maxLabelWidth, prop.label+":", lines[0])
		for _, line := range lines[1:] {
			fmt.Fprintf(w, "%-*s  %s\n", maxLabelWidth, "", line)
		}
	}
	if p.Caption != "" {
		tmpl, err := template.New("caption").Funcs(templateFuncs()).Parse(p.Caption)
		if err != nil {
			return fmt.Errorf("parsing caption: %w", err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, model); err != nil {
			return fmt.Errorf("rendering caption: %w", err)
		}
		text := strings.TrimSpace(buf.String())
		if text != "" {
			fmt.Fprintln(w)
			fmt.Fprintln(w, text)
		}
	}
	return nil
}

type Table struct {
	Title   string
	Rows    string
	Columns []Column
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

	table := output.NewTable(w, headers)

	for _, rowModel := range rows {
		row := make([]string, 0, len(columnTemplates))
		for i, tmpl := range columnTemplates {
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, rowModel); err != nil {
				return fmt.Errorf("rendering column %q: %w", t.Columns[i].Header, err)
			}
			row = append(row, buf.String())
		}
		if err := table.Append(row); err != nil {
			return fmt.Errorf("adding table row: %w", err)
		}
	}

	if err := table.Render(); err != nil {
		return fmt.Errorf("rendering table: %w", err)
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
