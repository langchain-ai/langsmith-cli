package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-cli/internal/client"
	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
	"github.com/langchain-ai/langsmith-cli/internal/structured"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const oauthClientID = "langsmith-cli"
const defaultDeviceCodePollInterval = 5 * time.Second

var openBrowser = openBrowserDefault
var inputIsTerminal = func(r io.Reader) bool {
	f, ok := r.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type oauthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

type oauthErrorResponse struct {
	Code             string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type loginInput struct {
	noBrowser       bool
	timeout         time.Duration
	workspaceID     string
	promptWorkspace bool
}

type loginResult struct {
	Status         string `json:"status"`
	Profile        string `json:"profile"`
	APIURL         string `json:"api_url"`
	WorkspaceID    string `json:"workspace_id,omitempty"`
	OAuthExpiresAt string `json:"oauth_expires_at"`
}

func (e *oauthErrorResponse) Error() string {
	if e.ErrorDescription == "" {
		return e.Code
	}
	return e.Code + ": " + e.ErrorDescription
}

func newLoginCmd() *cobra.Command {
	return structured.Command[*loginInput]{
		Use:   "login",
		Short: "Authenticate with LangSmith using OAuth",
		Long: `Authenticate with LangSmith using OAuth.

The command stores OAuth tokens in ~/.langsmith/config.json under the selected
profile. Select a profile with --profile or LANGSMITH_PROFILE.`,
		Input: func(cmd *cobra.Command) *loginInput {
			in := &loginInput{}
			cmd.Flags().BoolVar(&in.noBrowser, "no-browser", false, "Do not open a browser automatically")
			cmd.Flags().DurationVar(&in.timeout, "timeout", 0, "Maximum time to wait for authorization (default: device-code expiry)")
			cmd.Flags().StringVar(&in.workspaceID, "workspace-id", "", "Workspace ID override to save in the selected profile")
			cmd.Flags().BoolVar(&in.promptWorkspace, "prompt-workspace", false, "Prompt to select and save a workspace override")
			return in
		},
		Action: func(ctx context.Context, cmd *cobra.Command, in *loginInput, args []string) (any, error) {
			cfg, err := lsconfig.Load()
			if err != nil {
				return loginResult{}, err
			}
			workspaceID := strings.TrimSpace(in.workspaceID)
			if workspaceID != "" {
				if err := validateWorkspaceID(workspaceID); err != nil {
					return loginResult{}, err
				}
			}

			profileName := loginProfileName(cfg)
			if err := validateProfileName(profileName); err != nil {
				return loginResult{}, err
			}
			apiURL := loginAPIURL(cfg, profileName)
			if ctx == nil {
				ctx = context.Background()
			}

			device, err := requestDeviceCode(ctx, apiURL)
			if err != nil {
				return loginResult{}, err
			}

			errOut := cmd.ErrOrStderr()
			fmt.Fprintf(errOut, "Open this URL to authorize the LangSmith CLI:\n%s\n\nEnter code: %s\n\n", device.VerificationURI, device.UserCode)
			if !in.noBrowser {
				if err := openBrowser(device.VerificationURI); err != nil {
					fmt.Fprintf(errOut, "Could not open a browser automatically: %v\n\n", err)
				}
			}
			fmt.Fprintln(errOut, "Waiting for authorization...")

			waitFor := in.timeout
			if waitFor <= 0 {
				waitFor = time.Duration(device.ExpiresIn+10) * time.Second
			}
			pollCtx, cancel := context.WithTimeout(ctx, waitFor)
			defer cancel()

			interval := normalizeDeviceCodePollInterval(time.Duration(device.Interval) * time.Second)
			token, err := pollDeviceToken(pollCtx, apiURL, device.DeviceCode, interval)
			if err != nil {
				return loginResult{}, err
			}

			profile := cfg.Profiles[profileName]
			if profile.APIURL == "" || flagAPIURL != "" || strings.TrimSpace(profile.APIURL) != apiURL {
				profile.APIURL = apiURL
			}
			if workspaceID == "" && in.promptWorkspace {
				workspaceID, err = promptWorkspaceSelection(cmd, apiURL, token.AccessToken)
				if err != nil {
					return loginResult{}, err
				}
			}
			if workspaceID != "" {
				profile.WorkspaceID = workspaceID
			}
			applyTokenResponse(&profile, token, time.Now())
			cfg.Profiles[profileName] = profile
			cfg.CurrentProfile = profileName
			if err := cfg.Save(); err != nil {
				return loginResult{}, err
			}

			return loginResult{
				Status:         "logged_in",
				Profile:        profileName,
				APIURL:         apiURL,
				WorkspaceID:    profile.WorkspaceID,
				OAuthExpiresAt: profile.OAuth.ExpiresAt,
			}, nil
		},
		Render: structured.Template(`Logged in to {{.APIURL}} as profile {{printf "%q" .Profile}}`),
	}.Cobra()
}

func loginProfileName(cfg *lsconfig.Config) string {
	if flagProfile != "" {
		return flagProfile
	}
	if envProfile := strings.TrimSpace(getenv("LANGSMITH_PROFILE")); envProfile != "" {
		return envProfile
	}
	if cfg.CurrentProfile != "" {
		return cfg.CurrentProfile
	}
	return "default"
}

func loginAPIURL(cfg *lsconfig.Config, profileName string) string {
	apiURL := lsconfig.DefaultAPIURL
	if profile, ok := cfg.Profiles[profileName]; ok && profile.APIURL != "" {
		apiURL = profile.APIURL
	}
	if envURL := getenv("LANGSMITH_ENDPOINT"); envURL != "" {
		apiURL = envURL
	}
	if flagAPIURL != "" {
		apiURL = flagAPIURL
	}
	return client.NormalizeURL(apiURL)
}

func validateProfileName(name string) error {
	if name == "" || strings.ContainsAny(name, "[]\r\n") {
		return fmt.Errorf("invalid profile name: %q", name)
	}
	return nil
}

func validateWorkspaceID(workspaceID string) error {
	if _, err := uuid.Parse(workspaceID); err != nil {
		return fmt.Errorf("invalid workspace ID %q: expected UUID", workspaceID)
	}
	return nil
}

func promptWorkspaceSelection(cmd *cobra.Command, apiURL, accessToken string) (string, error) {
	in := cmd.InOrStdin()
	if !inputIsTerminal(in) {
		fmt.Fprintln(cmd.ErrOrStderr(), "Skipping workspace prompt because stdin is not interactive.")
		return "", nil
	}
	workspaces, err := listLoginWorkspaces(cmd.Context(), apiURL, accessToken)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Could not list workspaces: %v\n", err)
		return promptWorkspaceID(cmd)
	}
	if len(workspaces) == 1 {
		fmt.Fprintf(cmd.ErrOrStderr(), "Using default workspace %q (%s)\n", workspaceDisplayName(workspaces[0]), workspaces[0].ID)
		return workspaces[0].ID, nil
	}
	if len(workspaces) > 1 {
		fmt.Fprintln(cmd.ErrOrStderr(), "Select a default workspace:")
		for i, workspace := range workspaces {
			fmt.Fprintf(cmd.ErrOrStderr(), "  %d. %s (%s)\n", i+1, workspaceDisplayName(workspace), workspace.ID)
		}
		return promptWorkspaceChoice(cmd, workspaces)
	}
	return promptWorkspaceID(cmd)
}

func workspaceDisplayName(workspace langsmith.WorkspaceListResponse) string {
	if workspace.DisplayName != "" {
		return workspace.DisplayName
	}
	return workspace.ID
}

func listLoginWorkspaces(ctx context.Context, apiURL, accessToken string) ([]langsmith.WorkspaceListResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	c := client.NewWithOptions(client.Options{
		APIURL:           apiURL,
		OAuthAccessToken: accessToken,
	})
	workspaces, err := c.SDK.Workspaces.List(ctx, langsmith.WorkspaceListParams{})
	if err != nil {
		return nil, err
	}
	if workspaces == nil {
		return nil, nil
	}
	return *workspaces, nil
}

func promptWorkspaceChoice(cmd *cobra.Command, workspaces []langsmith.WorkspaceListResponse) (string, error) {
	line, err := promptLine(cmd, "Enter number or workspace ID: ")
	if err != nil {
		return "", err
	}
	choice := strings.TrimSpace(line)
	if choice == "" {
		return "", fmt.Errorf("default workspace required for OAuth login")
	}
	if n, err := strconv.Atoi(choice); err == nil {
		if n < 1 || n > len(workspaces) {
			return "", fmt.Errorf("invalid workspace selection %d", n)
		}
		return workspaces[n-1].ID, nil
	}
	if err := validateWorkspaceID(choice); err != nil {
		return "", err
	}
	return choice, nil
}

func promptWorkspaceID(cmd *cobra.Command) (string, error) {
	line, err := promptLine(cmd, "Default workspace ID: ")
	if err != nil {
		return "", err
	}
	workspaceID := strings.TrimSpace(line)
	if workspaceID == "" {
		return "", fmt.Errorf("default workspace required for OAuth login")
	}
	if err := validateWorkspaceID(workspaceID); err != nil {
		return "", err
	}
	return workspaceID, nil
}

func promptLine(cmd *cobra.Command, prompt string) (string, error) {
	in := cmd.InOrStdin()
	fmt.Fprint(cmd.ErrOrStderr(), prompt)
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("reading input: %w", err)
	}
	return line, nil
}

func requestDeviceCode(ctx context.Context, apiURL string) (*deviceCodeResponse, error) {
	values := url.Values{
		"client_id": {oauthClientID},
		"resource":  {oauthResource(apiURL)},
	}
	var resp deviceCodeResponse
	if err := postOAuthForm(ctx, apiURL, "/oauth/device/code", values, &resp); err != nil {
		return nil, fmt.Errorf("requesting device code: %w", err)
	}
	if resp.DeviceCode == "" || resp.UserCode == "" || resp.VerificationURI == "" {
		return nil, fmt.Errorf("requesting device code: incomplete response")
	}
	return &resp, nil
}

func refreshProfileToken(ctx context.Context, apiURL, refreshToken string) (*oauthTokenResponse, error) {
	values := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {oauthClientID},
		"resource":      {oauthResource(apiURL)},
		"refresh_token": {refreshToken},
	}
	var resp oauthTokenResponse
	if err := postOAuthForm(ctx, apiURL, "/oauth/token", values, &resp); err != nil {
		return nil, err
	}
	if resp.AccessToken == "" {
		return nil, fmt.Errorf("token response did not include an access token")
	}
	return &resp, nil
}

func pollDeviceToken(ctx context.Context, apiURL, deviceCode string, interval time.Duration) (*oauthTokenResponse, error) {
	interval = normalizeDeviceCodePollInterval(interval)

	for {
		token, oauthErr, err := requestDeviceToken(ctx, apiURL, deviceCode)
		if err != nil {
			return nil, err
		}
		if token != nil {
			return token, nil
		}

		switch oauthErr.Code {
		case "authorization_pending":
		case "slow_down":
			interval += 5 * time.Second
		case "access_denied", "expired_token", "invalid_grant":
			return nil, oauthErr
		default:
			return nil, oauthErr
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("timed out waiting for authorization")
		case <-timer.C:
		}
	}
}

func normalizeDeviceCodePollInterval(interval time.Duration) time.Duration {
	if interval < defaultDeviceCodePollInterval {
		return defaultDeviceCodePollInterval
	}
	return interval
}

func requestDeviceToken(ctx context.Context, apiURL, deviceCode string) (*oauthTokenResponse, *oauthErrorResponse, error) {
	values := url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"client_id":   {oauthClientID},
		"device_code": {deviceCode},
		"resource":    {oauthResource(apiURL)},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, oauthURL(apiURL, "/oauth/token"), strings.NewReader(values.Encode()))
	if err != nil {
		return nil, nil, fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("exchanging device code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("reading token response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		var token oauthTokenResponse
		if err := json.Unmarshal(body, &token); err != nil {
			return nil, nil, fmt.Errorf("decoding token response: %w", err)
		}
		if token.AccessToken == "" {
			return nil, nil, fmt.Errorf("token response did not include an access token")
		}
		return &token, nil, nil
	}

	oauthErr := decodeOAuthError(body, resp.StatusCode)
	return nil, oauthErr, nil
}

func postOAuthForm(ctx context.Context, apiURL, path string, values url.Values, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, oauthURL(apiURL, path), strings.NewReader(values.Encode()))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeOAuthError(body, resp.StatusCode)
	}
	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

func decodeOAuthError(body []byte, statusCode int) *oauthErrorResponse {
	var oauthErr oauthErrorResponse
	if err := json.Unmarshal(body, &oauthErr); err != nil || oauthErr.Code == "" {
		oauthErr = oauthErrorResponse{
			Code:             fmt.Sprintf("http_%d", statusCode),
			ErrorDescription: strings.TrimSpace(string(body)),
		}
	}
	return &oauthErr
}

func applyTokenResponse(profile *lsconfig.Profile, token *oauthTokenResponse, now time.Time) {
	profile.OAuth.AccessToken = token.AccessToken
	if token.RefreshToken != "" {
		profile.OAuth.RefreshToken = token.RefreshToken
	}
	if token.ExpiresIn > 0 {
		profile.OAuth.ExpiresAt = now.Add(time.Duration(token.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
	}
}

func oauthURL(apiURL, path string) string {
	return strings.TrimRight(client.NormalizeURL(apiURL), "/") + path
}

// oauthResource is the API origin expected by the OAuth server; it must not
// include the /api/v1 suffix accepted by LANGSMITH_ENDPOINT.
func oauthResource(apiURL string) string {
	return strings.TrimRight(client.NormalizeURL(apiURL), "/")
}

func openBrowserDefault(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}

func getenv(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}
