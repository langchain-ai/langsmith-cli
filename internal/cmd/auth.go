package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
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
	APIKeyFile        string `json:"api_key_file,omitempty"`
	OAuthAccessToken  bool   `json:"oauth_access_token,omitempty"`
	OAuthRefreshToken bool   `json:"oauth_refresh_token,omitempty"`
	OAuthExpiresAt    string `json:"oauth_expires_at,omitempty"`
	OAuthExpired      *bool  `json:"oauth_expired,omitempty"`
	ConfigError       string `json:"config_error,omitempty"`
}

var authCommand = structured.Parent{
	Use:   "auth",
	Short: "Manage authentication",
	Children: []func() *cobra.Command{
		newLoginCmd,
		authInfoCommand.Cobra,
		authTokenCommand.Cobra,
	},
}

var authInfoCommand = structured.Command[struct{}]{
	Use:   "info",
	Short: "Show the current authentication state",
	Action: func(ctx context.Context, cmd *cobra.Command, in struct{}, args []string) (any, error) {
		return resolveAuthInfo()
	},
	Render: structured.PropertyList{
		Properties: []structured.Property{
			{Label: "Authenticated", Template: "{{.Authenticated}}"},
			{Label: "Auth", Template: "{{.Auth}}"},
			{Label: "Auth source", Template: "{{.AuthSource}}", OmitEmpty: true},
			{Label: "Auth note", Template: "{{.AuthNote}}", OmitEmpty: true},
			{Label: "API URL", Template: "{{.APIURL}}"},
			{Label: "Workspace ID", Template: "{{.WorkspaceID}}", OmitEmpty: true},
			{Label: "Profile", Template: "{{.Profile}}{{if .ProfileFound}}{{if not .ProfileFound}} (not found){{end}}{{end}}", OmitEmpty: true},
			{Label: "Config file", Template: "{{.ConfigFile}}", OmitEmpty: true},
			{Label: "API key", Template: "{{.APIKey}}", OmitEmpty: true},
			{Label: "API key file", Template: "{{.APIKeyFile}}", OmitEmpty: true},
			{Label: "OAuth access token", Template: "{{if .OAuthAccessToken}}present{{end}}", OmitEmpty: true},
			{Label: "OAuth refresh token", Template: "{{if .OAuthRefreshToken}}present{{end}}", OmitEmpty: true},
			{Label: "OAuth expires at", Template: "{{.OAuthExpiresAt}}{{if .OAuthExpired}}{{if .OAuthExpired}} (expired){{end}}{{end}}", OmitEmpty: true},
			{Label: "Config error", Template: "{{.ConfigError}}", OmitEmpty: true},
		},
	},
}

var authTokenCommand = structured.Command[struct{}]{
	Use:   "token",
	Short: "Print the OAuth access token",
	Action: func(ctx context.Context, cmd *cobra.Command, in struct{}, args []string) (any, error) {
		cfg, err := lsconfig.Load()
		if err != nil {
			return "", err
		}

		envProfile := strings.TrimSpace(os.Getenv("LANGSMITH_PROFILE"))
		profileName, profile, hasProfile := cfg.ResolveProfile(flagProfile, envProfile)
		if (flagProfile != "" || envProfile != "") && !hasProfile {
			return "", fmt.Errorf("profile not found: %s", profileName)
		}
		if !hasProfile || (profile.AccessToken() == "" && profile.OAuth.RefreshToken == "") {
			return "", fmt.Errorf("no OAuth token found; run 'langsmith auth login'")
		}

		apiURL := lsconfig.DefaultAPIURL
		if profile.APIURL != "" {
			apiURL = profile.APIURL
		}
		if envURL := strings.TrimSpace(os.Getenv("LANGSMITH_ENDPOINT")); envURL != "" {
			apiURL = envURL
		}
		if flagAPIURL != "" {
			apiURL = flagAPIURL
		}
		apiURL = client.NormalizeURL(apiURL)

		if profile.OAuth.RefreshToken != "" &&
			(profile.AccessToken() == "" || profile.TokenExpiresSoon(time.Now(), time.Minute)) {
			if ctx == nil {
				ctx = context.Background()
			}
			token, err := refreshProfileToken(ctx, apiURL, profile.OAuth.RefreshToken)
			if err != nil {
				return "", fmt.Errorf("refreshing OAuth token for profile %q: %w; run 'langsmith auth login --profile %s' to reauthenticate", profileName, err, profileName)
			}
			applyTokenResponse(&profile, token, time.Now())
			cfg.Profiles[profileName] = profile
			if err := cfg.Save(); err != nil {
				return "", fmt.Errorf("saving refreshed OAuth token: %w", err)
			}
		}

		return profile.AccessToken(), nil
	},
	Render: structured.Template(`{{.}}
`),
}

func resolveAuthInfo() (authInfoResult, error) {
	apiURL := lsconfig.DefaultAPIURL
	result := authInfoResult{
		Auth:   "none",
		APIURL: apiURL,
	}
	if path, err := lsconfig.DefaultConfigPath(); err == nil {
		result.ConfigFile = path
	}

	cfg, err := lsconfig.Load()
	if err != nil {
		result.ConfigError = err.Error()
		cfg = &lsconfig.Config{Profiles: make(map[string]lsconfig.Profile)}
	}

	envProfile := profileEnvName()
	profileName := cfg.ResolveProfileName(flagProfile, envProfile)
	var profile lsconfig.Profile
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
		key, err := lsconfig.ResolveAPIKeyValue(flagAPIKey)
		if err != nil {
			return result, err
		}
		result.Auth = "api_key"
		result.AuthSource = "flag"
		result.APIKeyFile = lsconfig.APIKeyFilePath(flagAPIKey)
		result.APIKey = lsconfig.MaskSecret(key)
	case os.Getenv(lsconfig.APIKeyEnv) != "":
		result.Auth = "api_key"
		result.AuthSource = "env"
		result.AuthNote = "LANGSMITH_API_KEY is set and takes precedence over saved profile auth."
		result.APIKey = lsconfig.MaskSecret(os.Getenv(lsconfig.APIKeyEnv))
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
	case hasProfile && profile.HasAPIKey():
		key, err := profile.ResolveAPIKey()
		if err != nil {
			return result, fmt.Errorf("profile %q: %w", profileName, err)
		}
		result.Auth = "api_key"
		result.AuthSource = "profile"
		result.APIKeyFile = profile.APIKeyFile
		result.APIKey = lsconfig.MaskSecret(key)
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
