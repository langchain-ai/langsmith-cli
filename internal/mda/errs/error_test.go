package errs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/langchain-ai/langsmith-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/mda/proc"
)

func apiError(status int) error {
	request := httptest.NewRequest(http.MethodPost, "https://api.example/v2/deployments", nil)
	return &langsmith.Error{
		StatusCode: status,
		Request:    request,
		Response:   &http.Response{StatusCode: status, Request: request},
	}
}

type hostAPIError struct{ status int }

func (e *hostAPIError) Error() string {
	return "LangSmith Deployment is not enabled for this organization"
}
func (e *hostAPIError) HTTPStatus() int { return e.status }

func TestEveryCategorySerializesToItsDocumentedName(t *testing.T) {
	table := []struct {
		category ErrorCategory
		name     string
	}{
		{Project, "project"},
		{Compilation, "compilation"},
		{Authentication, "authentication"},
		{Permission, "permission"},
		{Remote, "remote"},
		{Network, "network"},
		{Toolchain, "toolchain"},
		{Deployment, "deployment"},
		{UserInput, "user_input"},
		{Unclassified, "unclassified"},
	}

	for _, row := range table {
		encoded, err := json.Marshal(row.category)
		require.NoError(t, err)
		assert.Equal(t, strconv.Quote(row.name), string(encoded))
	}
	assert.Len(t, table, len(categoryNames), "a new category needs a row here")
}

func TestAStatusDecidesTheCategoryAndTheDetailNeverDoes(t *testing.T) {
	cases := []struct {
		status   int
		expected ErrorCategory
	}{
		{http.StatusUnauthorized, Authentication},
		{http.StatusForbidden, Permission},
		{http.StatusTooManyRequests, Remote},
		{http.StatusBadGateway, Remote},
		{http.StatusConflict, Remote},
	}

	for _, c := range cases {
		assert.Equal(t, c.expected, CategoryOf(apiError(c.status)), "SDK %d", c.status)
		assert.Equal(t, c.expected, CategoryOf(&hostAPIError{status: c.status}), "host API %d", c.status)
	}
}

func TestAWrappedAPIErrorIsStillFound(t *testing.T) {
	err := fmt.Errorf("failed to sync Context Hub: %w", apiError(http.StatusUnauthorized))

	assert.Equal(t, Authentication, CategoryOf(err))
}

// The CLI's own client reports a status only through its predicates.
func TestACLIClientStatusDecidesTheCategory(t *testing.T) {
	cases := []struct {
		status   int
		expected ErrorCategory
	}{
		{http.StatusForbidden, Permission},
		{http.StatusNotFound, Remote},
		{http.StatusConflict, Remote},
		{http.StatusBadRequest, Remote},
	}
	for _, c := range cases {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"detail":"no"}`, c.status)
		}))
		err := client.New("test-key", server.URL).RawGet(t.Context(), "/v1/platform/deployments", nil)
		server.Close()

		var sdkErr *langsmith.Error
		require.False(t, errors.As(err, &sdkErr), "expected the client's own error, got %T", err)
		assert.Equal(t, c.expected, CategoryOf(fmt.Errorf("failed to list deployments: %w", err)), "%d", c.status)
	}
}

func TestACategoryBetweenACLIClientErrorAndTheTopWins(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"detail":"no"}`, http.StatusForbidden)
	}))
	defer server.Close()
	forbidden := client.New("test-key", server.URL).RawGet(t.Context(), "/v1/platform/deployments", nil)

	err := fmt.Errorf("outer: %w", WithCategory(Deployment, forbidden))

	assert.Equal(t, Deployment, CategoryOf(err))
}

func TestARefusedConnectionIsANetworkFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	url := "http://" + listener.Addr().String()
	require.NoError(t, listener.Close())
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	require.NoError(t, err)

	_, err = http.DefaultClient.Do(request)

	assert.Equal(t, Network, CategoryOf(fmt.Errorf("failed to reach LangSmith: %w", err)))
}

func TestATimedOutRequestIsANetworkFailure(t *testing.T) {
	err := fmt.Errorf("failed to reach LangSmith: %w", context.DeadlineExceeded)

	assert.Equal(t, Network, CategoryOf(err))
}

func TestAPromptInterruptIsACancellation(t *testing.T) {
	wrapped := fmt.Errorf("failed to read the LangSmith API key: %w", ErrCancelled)

	assert.True(t, IsCancellation(ErrCancelled))
	assert.True(t, IsCancellation(wrapped))
	assert.True(t, IsCancellation(fmt.Errorf("failed to run npm: %w", context.Canceled)))
}

func TestOtherFailuresAreNotCancellations(t *testing.T) {
	assert.False(t, IsCancellation(context.DeadlineExceeded))
	assert.False(t, IsCancellation(apiError(http.StatusUnauthorized)))
	assert.False(t, IsCancellation(errors.New("no agent entry file found")))
}

func TestAnUnrecognizedFailureIsUnclassified(t *testing.T) {
	err := errors.New("no agent entry file found in /private/project")

	assert.Equal(t, Unclassified, CategoryOf(err))
}

func TestACategorizedFailureRendersWhatBailWouldHave(t *testing.T) {
	err := Categorized(Project, ". is not a Managed Deep Agent project")

	assert.EqualError(t, err, ". is not a Managed Deep Agent project")
	assert.Nil(t, errors.Unwrap(err))
}

func failedOutput(stderr string) *proc.Captured {
	return &proc.Captured{Stderr: []byte(stderr)}
}

func TestAFailedChildCarriesItsStderrInTheChain(t *testing.T) {
	err := ChildFailed(failedOutput("Traceback\nImportError: x\n"), "emit failed")

	assert.EqualError(t, err, "emit failed: Traceback\nImportError: x")
	var stderr ChildStderr
	require.ErrorAs(t, err, &stderr)
	assert.Equal(t, ChildStderr("Traceback\nImportError: x"), stderr)
}

func TestASilentFailedChildAddsNoStderrLink(t *testing.T) {
	err := ChildFailed(failedOutput("  \n"), "emit failed")

	assert.EqualError(t, err, "emit failed")
	var stderr ChildStderr
	assert.False(t, errors.As(err, &stderr))
}

func TestARecategorizedErrorKeepsItsTextAndItsCauses(t *testing.T) {
	child := ChildFailed(failedOutput("ImportError: x\n"), "emit failed")
	err := WithCategory(Compilation, fmt.Errorf("compile failed: %w", child))

	assert.EqualError(t, err, "compile failed: emit failed: ImportError: x")
	assert.Equal(t, Compilation, CategoryOf(err))
	var stderr ChildStderr
	assert.ErrorAs(t, err, &stderr)
}

func TestOnlyAnUnclassifiedErrorTakesANewCategory(t *testing.T) {
	unclassified := CategorizeUnclassified(Project, errors.New("x"))
	named := CategorizeUnclassified(Project, Categorized(Authentication, "x"))

	assert.Equal(t, Project, CategoryOf(unclassified))
	assert.Equal(t, Authentication, CategoryOf(named))
}

func TestARecategorizedCancellationIsStillACancellation(t *testing.T) {
	err := WithCategory(Compilation, fmt.Errorf("failed to run npm: %w", ErrCancelled))

	assert.True(t, IsCancellation(err))
}

func TestContextAboveACategorizedFailureKeepsTheTextAndTheCategory(t *testing.T) {
	err := fmt.Errorf("outer: %w", Categorized(Deployment, "inner"))

	assert.EqualError(t, err, "outer: inner")
	assert.Equal(t, Deployment, CategoryOf(err))
}
