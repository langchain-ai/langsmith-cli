package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
	"github.com/langchain-ai/langsmith-cli/internal/structured"
	"github.com/spf13/cobra"
)

type profileListItem struct {
	Name           string `json:"name"`
	Active         bool   `json:"active"`
	APIURL         string `json:"api_url,omitempty"`
	WorkspaceID    string `json:"workspace_id,omitempty"`
	Auth           string `json:"auth"`
	OAuthExpiresAt string `json:"oauth_expires_at,omitempty"`
}

type profileShowItem struct {
	Name           string `json:"name"`
	Active         bool   `json:"active"`
	APIURL         string `json:"api_url,omitempty"`
	WorkspaceID    string `json:"workspace_id,omitempty"`
	Auth           string `json:"auth"`
	APIKey         string `json:"api_key,omitempty"`
	OAuthExpiresAt string `json:"oauth_expires_at,omitempty"`
}

type profileCreateInput struct {
	workspaceID string
	setCurrent  bool
}

type profileCreateResult struct {
	Status      string `json:"status"`
	Profile     string `json:"profile"`
	APIURL      string `json:"api_url"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Auth        string `json:"auth"`
	Active      bool   `json:"active"`
	Message     string `json:"message"`
}

type profileStatusResult struct {
	Status      string `json:"status"`
	Profile     string `json:"profile"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Message     string `json:"message"`
}

func newProfileCmd() *cobra.Command {
	return structured.Parent{
		Use:   "profile",
		Short: "Manage saved LangSmith profiles",
		Children: []func() *cobra.Command{
			newProfileCreateCmd,
			newProfileListCmd,
			newProfileShowCmd,
			newProfileDeleteCmd,
			newProfileUseCmd,
			newProfileSetWorkspaceCmd,
		},
	}.Cobra()
}

func newProfileCreateCmd() *cobra.Command {
	return structured.Command[*profileCreateInput]{
		Use:   "create NAME",
		Short: "Create an API-key profile",
		Args:  cobra.ExactArgs(1),
		Input: func(cmd *cobra.Command) *profileCreateInput {
			in := &profileCreateInput{}
			cmd.Flags().StringVar(&in.workspaceID, "workspace-id", "", "Default workspace ID to save in the profile")
			cmd.Flags().BoolVar(&in.setCurrent, "set-current", false, "Set the new profile as the current profile")
			return in
		},
		Action: func(ctx context.Context, cmd *cobra.Command, in *profileCreateInput, args []string) (any, error) {
			profileName := args[0]
			workspaceID := in.workspaceID
			if err := validateProfileName(profileName); err != nil {
				return profileCreateResult{}, err
			}
			if workspaceID != "" {
				if err := validateWorkspaceID(workspaceID); err != nil {
					return profileCreateResult{}, err
				}
			}

			cfg, err := lsconfig.Load()
			if err != nil {
				return profileCreateResult{}, err
			}
			if cfg.Profiles == nil {
				cfg.Profiles = make(map[string]lsconfig.Profile)
			}
			if _, exists := cfg.Profiles[profileName]; exists {
				return profileCreateResult{}, fmt.Errorf("profile %q already exists", profileName)
			}

			apiKey := profileCreateAPIKey()
			if apiKey == "" {
				return profileCreateResult{}, fmt.Errorf("api key required; pass --api-key or set LANGSMITH_API_KEY")
			}
			apiURL := profileCreateAPIURL()

			cfg.Profiles[profileName] = lsconfig.Profile{
				APIKey:      apiKey,
				APIURL:      apiURL,
				WorkspaceID: workspaceID,
			}
			if cfg.CurrentProfile == "" || in.setCurrent {
				cfg.CurrentProfile = profileName
			}
			if err := cfg.Save(); err != nil {
				return profileCreateResult{}, err
			}

			active := cfg.CurrentProfile == profileName
			message := fmt.Sprintf("Created profile %q", profileName)
			if active {
				message = fmt.Sprintf("Created and selected profile %q", profileName)
			}
			return profileCreateResult{
				Status:      "created",
				Profile:     profileName,
				APIURL:      apiURL,
				WorkspaceID: workspaceID,
				Auth:        "api_key",
				Active:      active,
				Message:     message,
			}, nil
		},
		Render: structured.Template(`{{.Message}}
`),
	}.Cobra()
}

func newProfileListCmd() *cobra.Command {
	return structured.Command[struct{}]{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List saved profiles",
		Action: func(ctx context.Context, cmd *cobra.Command, in struct{}, args []string) (any, error) {
			cfg, err := lsconfig.Load()
			if err != nil {
				return nil, err
			}

			activeName := cfg.ResolveProfileName(flagProfile, profileEnvName())
			items := make([]profileListItem, 0, len(cfg.Profiles))
			for name, profile := range cfg.Profiles {
				items = append(items, profileListItem{
					Name:           name,
					Active:         name == activeName,
					APIURL:         profile.APIURL,
					WorkspaceID:    profile.WorkspaceID,
					Auth:           profileAuthType(profile),
					OAuthExpiresAt: profile.OAuth.ExpiresAt,
				})
			}
			sort.Slice(items, func(i, j int) bool {
				if items[i].Active != items[j].Active {
					return items[i].Active
				}
				return items[i].Name < items[j].Name
			})
			return items, nil
		},
		Render: structured.Table{
			Rows: ".",
			Columns: []structured.Column{
				{Header: "Active", Template: "{{if .Active}}*{{end}}"},
				{Header: "Name", Template: "{{.Name}}"},
				{Header: "API URL", Template: "{{.APIURL}}"},
				{Header: "Workspace ID", Template: "{{.WorkspaceID}}"},
				{Header: "Auth", Template: "{{.Auth}}"},
				{Header: "Expires At", Template: "{{.OAuthExpiresAt}}"},
			},
		},
	}.Cobra()
}

func newProfileShowCmd() *cobra.Command {
	return structured.Command[struct{}]{
		Use:   "show NAME",
		Short: "Show a saved profile",
		Args:  cobra.ExactArgs(1),
		Action: func(ctx context.Context, cmd *cobra.Command, in struct{}, args []string) (any, error) {
			profileName := args[0]
			cfg, err := lsconfig.Load()
			if err != nil {
				return profileShowItem{}, err
			}
			profile, ok := cfg.Profiles[profileName]
			if !ok {
				return profileShowItem{}, fmt.Errorf("profile %q not found", profileName)
			}

			activeName := cfg.ResolveProfileName(flagProfile, profileEnvName())
			item := profileShowItem{
				Name:           profileName,
				Active:         profileName == activeName,
				APIURL:         profile.APIURL,
				WorkspaceID:    profile.WorkspaceID,
				Auth:           profileAuthType(profile),
				OAuthExpiresAt: profile.OAuth.ExpiresAt,
			}
			if profile.APIKey != "" {
				item.APIKey = lsconfig.MaskSecret(profile.APIKey)
			}
			return item, nil
		},
		Render: structured.Table{
			Rows: ".",
			Columns: []structured.Column{
				{Header: "Active", Template: "{{if .Active}}*{{end}}"},
				{Header: "Name", Template: "{{.Name}}"},
				{Header: "API URL", Template: "{{.APIURL}}"},
				{Header: "Workspace ID", Template: "{{.WorkspaceID}}"},
				{Header: "Auth", Template: "{{.Auth}}"},
				{Header: "API Key", Template: "{{.APIKey}}"},
				{Header: "Expires At", Template: "{{.OAuthExpiresAt}}"},
			},
		},
	}.Cobra()
}

func newProfileDeleteCmd() *cobra.Command {
	return structured.Command[struct{}]{
		Use:   "delete NAME",
		Short: "Delete a saved profile",
		Args:  cobra.ExactArgs(1),
		Action: func(ctx context.Context, cmd *cobra.Command, in struct{}, args []string) (any, error) {
			profileName := args[0]
			cfg, err := lsconfig.Load()
			if err != nil {
				return profileStatusResult{}, err
			}
			if _, ok := cfg.Profiles[profileName]; !ok {
				return profileStatusResult{}, fmt.Errorf("profile %q not found", profileName)
			}
			delete(cfg.Profiles, profileName)
			if cfg.CurrentProfile == profileName {
				cfg.CurrentProfile = ""
			}
			if err := cfg.Save(); err != nil {
				return profileStatusResult{}, err
			}

			return profileStatusResult{
				Status:  "deleted",
				Profile: profileName,
				Message: fmt.Sprintf("Deleted profile %q", profileName),
			}, nil
		},
		Render: structured.Template(`{{.Message}}
`),
	}.Cobra()
}

func newProfileUseCmd() *cobra.Command {
	return structured.Command[struct{}]{
		Use:   "use NAME",
		Short: "Set the current profile",
		Args:  cobra.ExactArgs(1),
		Action: func(ctx context.Context, cmd *cobra.Command, in struct{}, args []string) (any, error) {
			profileName := args[0]
			cfg, err := lsconfig.Load()
			if err != nil {
				return profileStatusResult{}, err
			}
			if _, ok := cfg.Profiles[profileName]; !ok {
				return profileStatusResult{}, fmt.Errorf("profile %q not found", profileName)
			}
			cfg.CurrentProfile = profileName
			if err := cfg.Save(); err != nil {
				return profileStatusResult{}, err
			}

			return profileStatusResult{
				Status:  "switched",
				Profile: profileName,
				Message: fmt.Sprintf("Using profile %q", profileName),
			}, nil
		},
		Render: structured.Template(`{{.Message}}
`),
	}.Cobra()
}

func newProfileSetWorkspaceCmd() *cobra.Command {
	return structured.Command[struct{}]{
		Use:   "set-workspace WORKSPACE_ID",
		Short: "Set the default workspace for a profile",
		Args:  cobra.ExactArgs(1),
		Action: func(ctx context.Context, cmd *cobra.Command, in struct{}, args []string) (any, error) {
			workspaceID := args[0]
			if err := validateWorkspaceID(workspaceID); err != nil {
				return profileStatusResult{}, err
			}
			cfg, err := lsconfig.Load()
			if err != nil {
				return profileStatusResult{}, err
			}
			if cfg.Profiles == nil {
				cfg.Profiles = make(map[string]lsconfig.Profile)
			}

			profileName := loginProfileName(cfg)
			if err := validateProfileName(profileName); err != nil {
				return profileStatusResult{}, err
			}
			profile := cfg.Profiles[profileName]
			profile.WorkspaceID = workspaceID
			cfg.Profiles[profileName] = profile
			if cfg.CurrentProfile == "" {
				cfg.CurrentProfile = profileName
			}
			if err := cfg.Save(); err != nil {
				return profileStatusResult{}, err
			}

			return profileStatusResult{
				Status:      "workspace_set",
				Profile:     profileName,
				WorkspaceID: workspaceID,
				Message:     fmt.Sprintf("Set default workspace for profile %q", profileName),
			}, nil
		},
		Render: structured.Template(`{{.Message}}
`),
	}.Cobra()
}

func profileCreateAPIKey() string {
	if flagAPIKey != "" {
		return flagAPIKey
	}
	return os.Getenv("LANGSMITH_API_KEY")
}

func profileCreateAPIURL() string {
	apiURL := lsconfig.DefaultAPIURL
	if envURL := os.Getenv("LANGSMITH_ENDPOINT"); envURL != "" {
		apiURL = envURL
	}
	if flagAPIURL != "" {
		apiURL = flagAPIURL
	}
	return client.NormalizeURL(apiURL)
}

func profileAuthType(profile lsconfig.Profile) string {
	switch {
	case profile.AccessToken() != "" || profile.OAuth.RefreshToken != "":
		return "oauth"
	case profile.APIKey != "":
		return "api_key"
	default:
		return "none"
	}
}

func profileEnvName() string {
	return strings.TrimSpace(os.Getenv("LANGSMITH_PROFILE"))
}
