package cmd

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// sandboxAuthHeaders returns the auth headers for sandbox dataplane requests.
// If SANDBOX_SERVICE_JWT_SECRET is set, it signs a service JWT for X-Service-Key.
// Otherwise, it falls back to the user's API key via X-Api-Key.
//
// The tenant ID is resolved from: explicit argument > LANGSMITH_WORKSPACE_ID env var.
func sandboxAuthHeaders(tenantID string) map[string]string {
	if tenantID == "" {
		tenantID = os.Getenv("LANGSMITH_WORKSPACE_ID")
	}

	secret := os.Getenv("SANDBOX_SERVICE_JWT_SECRET")
	if secret != "" && tenantID != "" {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub":       "platform-backend",
			"tenant_id": tenantID,
			"exp":       time.Now().Add(5 * time.Minute).Unix(),
		})
		signed, err := token.SignedString([]byte(secret))
		if err == nil {
			return map[string]string{"X-Service-Key": signed}
		}
	}

	if apiKey := getAPIKey(); apiKey != "" {
		return map[string]string{"X-Api-Key": apiKey}
	}

	return nil
}
