package cmd

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

func TestFormatErrorMessageSimplifiesAPIValidationDetail(t *testing.T) {
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

	got := FormatErrorMessage(fmt.Errorf("creating sandbox: %w", apiErr))

	require.Equal(t, "creating sandbox: 422 Unprocessable Entity: snapshot_id: field required; snapshot_name: field required", got)
	require.NotContains(t, got, "POST")
	require.NotContains(t, got, `"detail"`)
}

func TestFormatErrorMessageSimplifiesBodyLevelAPIValidationDetail(t *testing.T) {
	apiErr := testAPIError(t, http.StatusUnprocessableEntity, `{
		"detail": [{
			"loc": ["body"],
			"msg": "one of snapshot_id or snapshot_name is required",
			"type": "value_error"
		}]
	}`)

	got := FormatErrorMessage(fmt.Errorf("creating sandbox: %w", apiErr))

	require.Equal(t, "creating sandbox: 422 Unprocessable Entity: one of snapshot_id or snapshot_name is required", got)
}

func TestFormatErrorMessageUsesMessageFields(t *testing.T) {
	apiErr := testAPIError(t, http.StatusForbidden, `{
		"error": "Forbidden",
		"message": "workspace access required"
	}`)

	got := FormatErrorMessage(apiErr)

	require.Equal(t, "403 Forbidden: workspace access required", got)
}

func TestFormatErrorMessageReturnsNonAPIErrorString(t *testing.T) {
	err := errors.New("plain error")

	got := FormatErrorMessage(err)

	require.Equal(t, "plain error", got)
}
