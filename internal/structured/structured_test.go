package structured

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func testCmd(format string, out *bytes.Buffer) *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().String("format", format, "")
	cmd.Flags().String("jq", "", "")
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

func TestRenderJQFiltersModel(t *testing.T) {
	var out bytes.Buffer
	cmd := testCmd("pretty", &out)
	require.NoError(t, cmd.Flags().Set("jq", ".name"))

	err := Render(cmd, map[string]any{"name": "sandbox"}, Template(`ignored`))

	require.NoError(t, err)
	require.Equal(t, "sandbox\n", out.String())
}

func TestRenderJQEncodesObjects(t *testing.T) {
	var out bytes.Buffer
	cmd := testCmd("pretty", &out)
	require.NoError(t, cmd.Flags().Set("jq", ".items[]"))

	err := Render(cmd, map[string]any{"items": []map[string]any{{"name": "one"}}}, Template(`ignored`))

	require.NoError(t, err)
	require.JSONEq(t, `{"name":"one"}`, strings.TrimSpace(out.String()))
}

func TestCommandCobraAddsJQFlag(t *testing.T) {
	cmd := Command[struct{}]{
		Use: "test",
		Action: func(_ context.Context, _ *cobra.Command, _ struct{}, _ []string) (any, error) {
			return map[string]any{"name": "sandbox"}, nil
		},
	}.Cobra()

	require.NotNil(t, cmd.Flags().Lookup("jq"))
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
