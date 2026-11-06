package deploy

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
)

func forbidden(detail string) error {
	return &hostapi.APIError{Status: http.StatusForbidden, Detail: detail}
}

func TestRecognizesTheDeploymentEntitlementGate(t *testing.T) {
	gate, ok := planGate(forbidden("LangSmith Deployment is not enabled for this organization"))
	assert.True(t, ok)
	assert.Equal(t, planGateDeployment, gate)
}

func TestRecognizesTheManagedDeepAgentsEntitlementGate(t *testing.T) {
	gate, ok := planGate(forbidden("Managed Deep Agents is not enabled for this organization"))
	assert.True(t, ok)
	assert.Equal(t, planGateManagedDeepAgents, gate)
}

func TestFindsTheGateUnderAddedContext(t *testing.T) {
	err := fmt.Errorf("POST /v2/deployments request failed: %w",
		forbidden("Managed Deep Agents is not enabled for this organization"))
	gate, ok := planGate(err)
	assert.True(t, ok)
	assert.Equal(t, planGateManagedDeepAgents, gate)
}

func TestOtherForbiddenResponsesAreNotPlanGates(t *testing.T) {
	_, ok := planGate(forbidden("missing permission"))
	assert.False(t, ok)
}

func TestAnUnknownEntitlementIsNotAttributedToAPlan(t *testing.T) {
	_, ok := planGate(forbidden("Dev zero deployments are not enabled for this organization"))
	assert.False(t, ok)
}

func TestANonForbiddenStatusIsNotAPlanGate(t *testing.T) {
	_, ok := planGate(&hostapi.APIError{
		Status: http.StatusTooManyRequests,
		Detail: "LangSmith Deployment is not enabled for this organization",
	})
	assert.False(t, ok)
}

func TestErrorsWithoutAnAPIResponseAreNotPlanGates(t *testing.T) {
	_, ok := planGate(errors.New("could not reach LangSmith"))
	assert.False(t, ok)
}
