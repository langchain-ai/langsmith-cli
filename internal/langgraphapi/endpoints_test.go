package langgraphapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveEndpoints(t *testing.T) {
	tests := []struct {
		name          string
		hostURL       string
		langsmithURL  string
		controlPlane  string
		dashboard     string
		expectIsCloud bool
	}{
		{
			name:          "default is cloud",
			controlPlane:  "https://api.host.langchain.com",
			dashboard:     "https://smith.langchain.com",
			expectIsCloud: true,
		},
		{
			name:          "cloud api url",
			langsmithURL:  "https://api.smith.langchain.com",
			controlPlane:  "https://api.host.langchain.com",
			dashboard:     "https://smith.langchain.com",
			expectIsCloud: true,
		},
		{
			name:          "regional cloud api url with api path",
			langsmithURL:  "https://eu.api.smith.langchain.com/api/v1",
			controlPlane:  "https://eu.api.host.langchain.com",
			dashboard:     "https://eu.smith.langchain.com",
			expectIsCloud: true,
		},
		{
			name:         "self-hosted api url",
			langsmithURL: "https://smith.example.com/api/v1",
			controlPlane: "https://smith.example.com/api-host",
			dashboard:    "https://smith.example.com",
		},
		{
			name:         "self-hosted under a base path",
			langsmithURL: "https://example.com/langsmith/api/",
			controlPlane: "https://example.com/langsmith/api-host",
			dashboard:    "https://example.com/langsmith",
		},
		{
			name:         "explicit local control plane wins",
			hostURL:      "http://localhost:8124/",
			langsmithURL: "https://api.smith.langchain.com",
			controlPlane: "http://localhost:8124",
			dashboard:    "http://localhost:8124",
		},
		{
			name:         "explicit self-hosted control plane",
			hostURL:      "https://ls.corp.example/api-host",
			controlPlane: "https://ls.corp.example/api-host",
			dashboard:    "https://ls.corp.example",
		},
		{
			name:          "explicit regional cloud control plane",
			hostURL:       "https://eu.api.host.langchain.com",
			controlPlane:  "https://eu.api.host.langchain.com",
			dashboard:     "https://eu.smith.langchain.com",
			expectIsCloud: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveEndpoints(tc.hostURL, tc.langsmithURL)
			assert.Equal(t, tc.controlPlane, got.ControlPlaneURL)
			assert.Equal(t, tc.dashboard, got.DashboardURL)
			assert.Equal(t, tc.expectIsCloud, got.IsCloud())
		})
	}
}
