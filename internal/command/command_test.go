package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func testCmd(format string, out *bytes.Buffer) *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().String("format", format, "")
	cmd.SetOut(out)
	return cmd
}

func TestRenderJSONIgnoresTextSpec(t *testing.T) {
	var out bytes.Buffer
	cmd := testCmd("json", &out)

	err := Render(cmd, map[string]any{"name": "sandbox"}, Template(`ignored`))

	require.NoError(t, err)
	require.JSONEq(t, `{"name":"sandbox"}`, out.String())
}

func TestTemplateRenderText(t *testing.T) {
	var out bytes.Buffer
	cmd := testCmd("pretty", &out)

	err := Render(cmd, struct{ Name string }{Name: "sandbox"}, Template(`Name: {{.Name}}`))

	require.NoError(t, err)
	require.Equal(t, "Name: sandbox", out.String())
}

func TestTableRenderText(t *testing.T) {
	var out bytes.Buffer
	cmd := testCmd("pretty", &out)
	model := struct {
		Rows []struct {
			Name       string
			SnapshotID string
		}
	}{
		Rows: []struct {
			Name       string
			SnapshotID string
		}{
			{Name: "a", SnapshotID: "1234567890abcdef"},
		},
	}

	err := Render(cmd, model, Table{
		Title: "Rows",
		Rows:  ".Rows",
		Columns: []Column{
			{Header: "Name", Template: "{{.Name}}"},
			{Header: "Snapshot", Template: "{{shortID .SnapshotID}}"},
		},
	})

	require.NoError(t, err)
	got := out.String()
	require.Contains(t, got, "Rows")
	require.Contains(t, got, "NAME")
	require.Contains(t, got, "SNAPSHOT")
	require.Contains(t, got, "a")
	require.Contains(t, got, "12345678...")
	require.False(t, strings.Contains(got, "{{"))
}

func TestTableRenderTextMissingRowsPath(t *testing.T) {
	var out bytes.Buffer
	cmd := testCmd("pretty", &out)

	err := Render(cmd, struct{}{}, Table{
		Rows: ".Missing",
		Columns: []Column{
			{Header: "Name", Template: "{{.Name}}"},
		},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), `rows path ".Missing"`)
}
