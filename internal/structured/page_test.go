package structured

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

type item struct {
	ID string `json:"id"`
}

func TestCursorPageJSON(t *testing.T) {
	more, _ := json.Marshal(NewCursorPage([]item{{ID: "a"}}, "c1"))
	require.JSONEq(t, `{"items":[{"id":"a"}],"next_cursor":"c1","has_more":true}`, string(more))

	done, _ := json.Marshal(NewCursorPage[item](nil, ""))
	require.JSONEq(t, `{"items":[],"next_cursor":null,"has_more":false}`, string(done))
}

func TestOffsetPage(t *testing.T) {
	total := int64(5)
	p := NewOffsetPage([]item{{ID: "a"}, {ID: "b"}}, 2, 2, &total)
	require.True(t, p.HasMore)
	require.Equal(t, "4", *p.NextCursor)

	p = NewOffsetPage([]item{{ID: "e"}}, 4, 2, &total)
	require.False(t, p.HasMore)
	require.Nil(t, p.NextCursor)

	// Without a total, a full page means there may be more.
	p = NewOffsetPage([]item{{ID: "a"}, {ID: "b"}}, 0, 2, nil)
	require.True(t, p.HasMore)
	require.Equal(t, "2", *p.NextCursor)
	p = NewOffsetPage([]item{{ID: "a"}}, 0, 2, nil)
	require.False(t, p.HasMore)
}

func TestParseOffsetCursor(t *testing.T) {
	offset, err := ParseOffsetCursor("40")
	require.NoError(t, err)
	require.Equal(t, int64(40), offset)

	offset, err = ParseOffsetCursor("")
	require.NoError(t, err)
	require.Zero(t, offset)

	_, err = ParseOffsetCursor("abc")
	require.Equal(t, CategoryInvalidInput, Classify(err).Category)
	_, err = ParseOffsetCursor("-1")
	require.Error(t, err)
}

func TestPageTableShowsContinuation(t *testing.T) {
	var out bytes.Buffer
	cmd := testCmd("pretty", &out)
	spec := PageTable{Columns: []Column{{Header: "ID", Template: "{{.ID}}"}}}

	require.NoError(t, Render(cmd, NewCursorPage([]item{{ID: "a"}}, "c1"), spec))
	require.Contains(t, out.String(), "a")
	require.Contains(t, out.String(), "More results: rerun with --cursor c1")

	out.Reset()
	require.NoError(t, Render(cmd, NewCursorPage([]item{{ID: "a"}}, ""), spec))
	require.NotContains(t, out.String(), "More results")
}

func TestPageJQSeesEnvelope(t *testing.T) {
	var out bytes.Buffer
	cmd := testCmd("pretty", &out)
	require.NoError(t, cmd.Flags().Set("jq", ".has_more, .next_cursor, (.items | length)"))

	require.NoError(t, Render(cmd, NewCursorPage([]item{{ID: "a"}}, "c1"), nil))
	require.Equal(t, "true\nc1\n1\n", out.String())
}
