package codegen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNaming(t *testing.T) {
	require.Equal(t, "annotation_queue", singular("annotation_queues"))
	require.Equal(t, "box", singular("boxes"))
	require.Equal(t, "registry", singular("registries"))
	require.Equal(t, "feedback", singular("feedback"))
	require.Equal(t, "get-count", verb("retrieve_count"))
	require.Equal(t, "get", verb("retrieve"))
	require.Equal(t, "delete-all", verb("delete_all"))
	require.Equal(t, "QueueItemID", pascal("queue_item_id"))
}

func testCatalog(op Operation) *Catalog {
	return &Catalog{SchemaVersion: SchemaVersion, Operations: []Operation{op}, Schemas: map[string]*Schema{
		"Widget":     {Type: "object", Properties: map[string]*Schema{"id": {Type: "string"}, "name": {Type: "string"}}},
		"WidgetPage": {Type: "object", Properties: map[string]*Schema{"items": {Type: "array", Items: &Schema{Ref: "#/components/schemas/Widget"}}}},
	}}
}

func widgetOp() Operation {
	params := "WidgetListParams"
	contentType := "application/json"
	items := "items"
	op := Operation{ID: "widgets.list", Resource: []string{"widgets"}, Action: "list", Risk: "READ", Summary: "List widgets"}
	op.HTTP.Method, op.HTTP.Path = "get", "/api/v1/widgets"
	op.Exposure.CLI = true
	op.SDK.Go = &GoSDK{Service: "Widgets", Method: "List", ParamsType: &params, Fields: map[string]GoField{
		"cursor": {Name: "Cursor"}, "page_size": {Name: "PageSize"}, "kind": {Name: "Kind", EnumType: "WidgetListParamsKind"},
	}}
	op.Parameters.Query = []Param{
		{Name: "kind", Required: true, Schema: &Schema{Type: "string", Enum: []any{"a", "b"}}},
		{Name: "cursor", Schema: &Schema{Type: "string"}},
		{Name: "page_size", Schema: &Schema{Type: "integer"}},
	}
	op.Response = &Response{Status: 200, ContentType: &contentType, Schema: &Schema{Ref: "#/components/schemas/WidgetPage"}}
	op.Pagination = &Pagination{Scheme: "items_cursor_get_pagination", Type: "cursor", ParamLocation: "query"}
	op.Pagination.Request.CursorParam = "cursor"
	op.Pagination.Request.LimitParam = "page_size"
	op.Pagination.Response.ItemsField = &items
	op.Pagination.Response.NextCursorField = "next_cursor"
	return op
}

func TestGenerateListCommand(t *testing.T) {
	files, err := Generate(testCatalog(widgetOp()))
	require.NoError(t, err)
	src := string(files["zz_generated_widgets.go"])

	require.Contains(t, src, `Path: []string{"widget", "list"}, Risk: "READ"`)
	require.Contains(t, src, `cmd.Flags().StringVar(&in.Cursor, "cursor"`)
	require.Contains(t, src, `cmd.Flags().Int64Var(&in.PageSize, "limit"`)
	require.Contains(t, src, `genrt.Required(cmd, "kind")`)
	require.Contains(t, src, `genrt.OneOf(cmd, "kind", in.Kind, "a", "b")`)
	require.Contains(t, src, `params.Kind = langsmith.F(langsmith.WidgetListParamsKind(in.Kind))`)
	require.Contains(t, src, `res, err := c.SDK.Widgets.List(ctx, params, opts...)`)
	require.Contains(t, src, `structured.NewCursorPage(genrt.RawItems(res.Items), res.NextCursor)`)
	require.Contains(t, src, `{Header: "name", Template: "{{field . \"name\"}}"}`)
}

func TestGenerateDestructiveWriteRequiresYes(t *testing.T) {
	op := widgetOp()
	op.ID, op.Action, op.Risk, op.Pagination = "widgets.delete", "delete", "DESTRUCTIVE_WRITE", nil
	op.HTTP.Method = "delete"
	op.Parameters.Query = nil
	op.Summary = "Delete widgets"
	op.SDK.Go.Method, op.SDK.Go.ParamsType = "Delete", nil
	op.Response = &Response{Status: 204}

	files, err := Generate(testCatalog(op))
	require.NoError(t, err)
	src := string(files["zz_generated_widgets.go"])
	require.Contains(t, src, `cmd.Flags().Bool("yes", false`)
	require.Contains(t, src, `genrt.ConfirmDestructive(cmd)`)
	require.Contains(t, src, `err = c.SDK.Widgets.Delete(ctx, opts...)`)
}

func TestGenerateRejectsUnsupportedShapes(t *testing.T) {
	collides := widgetOp()
	collides.Parameters.Query = append(collides.Parameters.Query, Param{Name: "format", Schema: &Schema{Type: "string"}})
	collides.SDK.Go.Fields["format"] = GoField{Name: "Format"}
	_, err := Generate(testCatalog(collides))
	require.ErrorContains(t, err, `collides with the global --format flag`)

	structuredQuery := widgetOp()
	structuredQuery.Parameters.Query = append(structuredQuery.Parameters.Query, Param{Name: "filter", Schema: &Schema{Type: "object"}})
	structuredQuery.SDK.Go.Fields["filter"] = GoField{Name: "Filter"}
	_, err = Generate(testCatalog(structuredQuery))
	require.ErrorContains(t, err, `query parameter "filter" has a structured type`)

	files, skipped, err := GenerateWith(testCatalog(structuredQuery), Options{SkipUnsupported: true})
	require.NoError(t, err)
	require.Len(t, skipped, 1)
	require.Empty(t, files)
}

// TestGeneratedCodeIsFresh fails when internal/gen no longer matches what
// the committed catalog snapshot generates. Fix with `make generate`.
func TestGeneratedCodeIsFresh(t *testing.T) {
	dir := filepath.Join("..", "gen")
	data, err := os.ReadFile(filepath.Join(dir, "catalog.json"))
	require.NoError(t, err)
	catalog, err := ParseCatalog(data)
	require.NoError(t, err)
	files, err := Generate(catalog)
	require.NoError(t, err)

	existing, _ := filepath.Glob(filepath.Join(dir, "zz_generated_*.go"))
	require.Len(t, existing, len(files), "unexpected generated files; run `make generate`")
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err, "run `make generate`")
		require.Equal(t, string(want), string(got), "%s is stale; run `make generate`", name)
	}
}
