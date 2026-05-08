package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/config"
	"github.com/langchain-ai/langsmith-cli/internal/structured"
	"github.com/spf13/cobra"
)

type authInfoResult struct {
	Authenticated     bool   `json:"authenticated"`
	Auth              string `json:"auth"`
	AuthSource        string `json:"auth_source,omitempty"`
	AuthNote          string `json:"auth_note,omitempty"`
	APIURL            string `json:"api_url"`
	WorkspaceID       string `json:"workspace_id,omitempty"`
	Profile           string `json:"profile,omitempty"`
	ProfileFound      *bool  `json:"profile_found,omitempty"`
	ConfigFile        string `json:"config_file,omitempty"`
	APIKey            string `json:"api_key,omitempty"`
	OAuthAccessToken  bool   `json:"oauth_access_token,omitempty"`
	OAuthRefreshToken bool   `json:"oauth_refresh_token,omitempty"`
	OAuthExpiresAt    string `json:"oauth_expires_at,omitempty"`
	OAuthExpired      *bool  `json:"oauth_expired,omitempty"`
	ConfigError       string `json:"config_error,omitempty"`
}

func newAuthCmd() *cobra.Command {
	return structured.Parent{
		Use:   "auth",
		Short: "Manage LangSmith authentication",
		Children: []func() *cobra.Command{
			newLoginCmd,
			newAuthInfoCmd,
		},
	}.Cobra()
}

func newAuthInfoCmd() *cobra.Command {
	return structured.Command[struct{}]{
		Use:   "info",
		Short: "Show the current authentication state",
		Action: func(ctx context.Context, cmd *cobra.Command, input struct{}, args []string) (any, error) {
			return resolveAuthInfo()
		},
		Render: structured.Template(`Authenticated: {{.Authenticated}}
Auth: {{.Auth}}
{{- if .AuthSource}}
Auth source: {{.AuthSource}}
{{- end}}
{{- if .AuthNote}}
Auth note: {{.AuthNote}}
{{- end}}
API URL: {{.APIURL}}
{{- if .WorkspaceID}}
Workspace ID: {{.WorkspaceID}}
{{- end}}
{{- if .Profile}}
Profile: {{.Profile}}{{if .ProfileFound}}{{if not .ProfileFound}} (not found){{end}}{{end}}
{{- end}}
{{- if .ConfigFile}}
Config file: {{.ConfigFile}}
{{- end}}
{{- if .APIKey}}
API key: {{.APIKey}}
{{- end}}
{{- if .OAuthAccessToken}}
OAuth access token: present
{{- end}}
{{- if .OAuthRefreshToken}}
OAuth refresh token: present
{{- end}}
{{- if .OAuthExpiresAt}}
OAuth expires at: {{.OAuthExpiresAt}}{{if .OAuthExpired}}{{if .OAuthExpired}} (expired){{end}}{{end}}
{{- end}}
{{- if .ConfigError}}
Config error: {{.ConfigError}}
{{- end}}
`),
	}.Cobra()
}

func resolveAuthInfo() (authInfoResult, error) {
	apiURL := config.DefaultAPIURL
	result := authInfoResult{
		Auth:   "none",
		APIURL: apiURL,
	}
	if path, err := config.DefaultConfigPath(); err == nil {
		result.ConfigFile = path
	}

	cfg, err := config.Load()
	if err != nil {
		result.ConfigError = err.Error()
		cfg = &config.Config{Profiles: make(map[string]config.Profile)}
	}

	envProfile := profileEnvName()
	profileName := cfg.ResolveProfileName(flagProfile, envProfile)
	var profile config.Profile
	hasProfile := false
	if profileName != "" {
		profile, hasProfile = cfg.Profiles[profileName]
		result.Profile = profileName
		found := hasProfile
		result.ProfileFound = &found
	}
	if hasProfile && profile.APIURL != "" {
		apiURL = profile.APIURL
	}
	if envURL := strings.TrimSpace(os.Getenv("LANGSMITH_ENDPOINT")); envURL != "" {
		apiURL = envURL
	}
	if flagAPIURL != "" {
		apiURL = flagAPIURL
	}
	result.APIURL = client.NormalizeURL(apiURL)

	if hasProfile {
		result.WorkspaceID = profile.WorkspaceID
	}
	if v := strings.TrimSpace(os.Getenv("LANGSMITH_TENANT_ID")); v != "" {
		result.WorkspaceID = v
	}
	if v := strings.TrimSpace(os.Getenv("LANGSMITH_WORKSPACE_ID")); v != "" {
		result.WorkspaceID = v
	}

	switch {
	case flagAPIKey != "":
		result.Auth = "api_key"
		result.AuthSource = "flag"
		result.APIKey = config.MaskSecret(flagAPIKey)
	case os.Getenv("LANGSMITH_API_KEY") != "":
		result.Auth = "api_key"
		result.AuthSource = "env"
		result.AuthNote = "LANGSMITH_API_KEY is set and takes precedence over saved profile auth."
		result.APIKey = config.MaskSecret(os.Getenv("LANGSMITH_API_KEY"))
	case hasProfile && (profile.AccessToken() != "" || profile.OAuth.RefreshToken != ""):
		result.Auth = "oauth"
		result.AuthSource = "profile"
		result.OAuthAccessToken = profile.AccessToken() != ""
		result.OAuthRefreshToken = profile.OAuth.RefreshToken != ""
		result.OAuthExpiresAt = profile.OAuth.ExpiresAt
		if expiresAt, ok := profile.TokenExpiresAtTime(); ok {
			expired := !expiresAt.After(time.Now())
			result.OAuthExpired = &expired
		}
	case hasProfile && profile.APIKey != "":
		result.Auth = "api_key"
		result.AuthSource = "profile"
		result.APIKey = config.MaskSecret(profile.APIKey)
	}
	result.Authenticated = result.Auth != "none"

	if result.ConfigError != "" && !result.Authenticated {
		return result, fmt.Errorf("loading config: %s", result.ConfigError)
	}
	if (flagProfile != "" || envProfile != "") && !hasProfile && result.AuthSource != "flag" && result.AuthSource != "env" {
		return result, fmt.Errorf("profile not found: %s", profileName)
	}
	return result, nil
}
