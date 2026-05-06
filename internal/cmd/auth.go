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

var authCommand = structured.Parent{
	Use:   "auth",
	Short: "Manage authentication",
	Children: []func() *cobra.Command{
		authTokenCommand.Cobra,
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
			return "", fmt.Errorf("no OAuth token found; run 'langsmith login'")
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
				return "", fmt.Errorf("refreshing OAuth token for profile %q: %w; run 'langsmith login --profile %s' to reauthenticate", profileName, err, profileName)
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
