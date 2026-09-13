package cmd

import (
	"bytes"
	"testing"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitSnapshotRef(t *testing.T) {
	tests := []struct {
		ref      string
		wantName string
		wantTag  string
	}{
		{ref: "my-snap", wantName: "my-snap", wantTag: ""},
		{ref: "my-snap:v2", wantName: "my-snap", wantTag: "v2"},
		{ref: "my-snap:2026081101", wantName: "my-snap", wantTag: "2026081101"},
		{ref: "my-snap:latest", wantName: "my-snap", wantTag: "latest"},
		// A trailing colon carries no tag, so the server applies its default.
		{ref: "my-snap:", wantName: "my-snap", wantTag: ""},
		// Split on the first colon only; the server rejects a colon inside a name.
		{ref: "my-snap:v2:extra", wantName: "my-snap", wantTag: "v2:extra"},
		{ref: "", wantName: "", wantTag: ""},
	}

	for _, tc := range tests {
		t.Run(tc.ref, func(t *testing.T) {
			name, tag := splitSnapshotRef(tc.ref)

			assert.Equal(t, tc.wantName, name)
			assert.Equal(t, tc.wantTag, tag)
		})
	}
}

func TestSnapshotRenderDescription(t *testing.T) {
	tests := []struct {
		name        string
		description string
		want        string
	}{
		{name: "empty is dangling", description: "", want: "Description: -"},
		{name: "set", description: "Python 3.12 with uv", want: "Description: Python 3.12 with uv"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer

			err := snapshotGetCommand.Render.RenderText(&out, langsmith.SnapshotResponse{
				ID:          "snap-1",
				Name:        "my-snap",
				Description: tc.description,
			})

			require.NoError(t, err)
			assert.Contains(t, out.String(), tc.want)
		})
	}
}

func TestJoinOrDashRendersSnapshotTags(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want string
	}{
		{name: "no tags is dangling", tags: nil, want: "Tags:        -"},
		{name: "one tag", tags: []string{"latest"}, want: "Tags:        latest"},
		{name: "many tags", tags: []string{"latest", "v2"}, want: "Tags:        latest, v2"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer

			err := snapshotGetCommand.Render.RenderText(&out, langsmith.SnapshotResponse{
				ID:   "snap-1",
				Name: "my-snap",
				Tags: tc.tags,
			})

			require.NoError(t, err)
			assert.Contains(t, out.String(), tc.want)
		})
	}
}
