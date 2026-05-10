package structured

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/stretchr/testify/require"
)

func testAPIError(t *testing.T, statusCode int, body string) *langsmith.Error {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, "https://api.example.com/v2/sandboxes/boxes", nil)
	require.NoError(t, err)

	apiErr := &langsmith.Error{
		StatusCode: statusCode,
		Request:    req,
		Response: &http.Response{
			StatusCode: statusCode,
		},
	}
	require.NoError(t, json.Unmarshal([]byte(body), apiErr))
	return apiErr
}

func TestFormatErrorSimplifiesAPIValidationDetail(t *testing.T) {
	apiErr := testAPIError(t, http.StatusUnprocessableEntity, `{
		"detail": [{
			"loc": ["body", "snapshot_id"],
			"msg": "field required",
			"type": "value_error.missing"
		}, {
			"loc": ["body", "snapshot_name"],
			"msg": "field required",
			"type": "value_error.missing"
		}]
	}`)

	err := FormatError(fmt.Errorf("creating sandbox: %w", apiErr))

	require.Equal(t, "creating sandbox: 422 Unprocessable Entity: snapshot_id: field required; snapshot_name: field required", err.Error())
	require.NotContains(t, err.Error(), "POST")
	require.NotContains(t, err.Error(), `"detail"`)
}

func TestFormatErrorSimplifiesBodyLevelAPIValidationDetail(t *testing.T) {
	apiErr := testAPIError(t, http.StatusUnprocessableEntity, `{
		"detail": [{
			"loc": ["body"],
			"msg": "one of snapshot_id or snapshot_name is required",
			"type": "value_error"
		}]
	}`)

	err := FormatError(fmt.Errorf("creating sandbox: %w", apiErr))

	require.Equal(t, "creating sandbox: 422 Unprocessable Entity: one of snapshot_id or snapshot_name is required", err.Error())
}

func TestFormatErrorUsesMessageFields(t *testing.T) {
	apiErr := testAPIError(t, http.StatusForbidden, `{
		"error": "Forbidden",
		"message": "workspace access required"
	}`)

	err := FormatError(apiErr)

	require.Equal(t, "403 Forbidden: workspace access required", err.Error())
}

func TestFormatErrorReturnsOriginalNonAPIError(t *testing.T) {
	original := errors.New("plain error")

	err := FormatError(original)

	require.Same(t, original, err)
}
