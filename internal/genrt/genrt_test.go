package genrt

import (
	"errors"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/structured"
	"github.com/stretchr/testify/require"
)

func TestAPIErrorRewritesFieldNames(t *testing.T) {
	err := APIError(structured.NewError(structured.CategoryInvalidInput, "session_id or dataset_id is required; tree_filter_x stays"),
		map[string]string{"session_id": "--project", "dataset_id": "--dataset-id"})
	require.Equal(t, "--project or --dataset-id is required; tree_filter_x stays", err.Error())

	err = APIError(errors.New("filter is bad"), map[string]string{"filter": "--filter"})
	require.Equal(t, "--filter is bad", err.Error())
}

func TestReplaceWordBoundaries(t *testing.T) {
	require.Equal(t, "--status and status_code", replaceWord("status and status_code", "status", "--status"))
	require.Equal(t, "query.id.0: bad", replaceWord("query.id.0: bad", "id", "--id"))
}
