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
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-cli/internal/client"
	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const oauthClientID = "langsmith-cli"

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

func (e *oauthErrorResponse) Error() string {
	if e.ErrorDescription == "" {
		return e.Code
	}
	return e.Code + ": " + e.ErrorDescription
}

func newLoginCmd() *cobra.Command {
	var (
		noBrowser   bool
		timeout     time.Duration
		workspaceID string
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with LangSmith using OAuth",
		Long: `Authenticate with LangSmith using OAuth.

The command stores OAuth tokens in ~/.langsmith/config.json under the selected
profile. Select a profile with --profile or LANGSMITH_PROFILE.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogin(cmd, noBrowser, timeout, workspaceID)
		},
	}
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Do not open a browser automatically")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "Maximum time to wait for authorization (default: device-code expiry)")
	cmd.Flags().StringVar(&workspaceID, "workspace-id", "", "Default workspace ID to save in the selected profile")
	return cmd
}

func runLogin(cmd *cobra.Command, noBrowser bool, timeout time.Duration, workspaceID string) error {
	cfg, err := lsconfig.Load()
	if err != nil {
		return err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID != "" {
		if err := validateWorkspaceID(workspaceID); err != nil {
			return err
		}
	}

	profileName := loginProfileName(cfg)
	if err := validateProfileName(profileName); err != nil {
		return err
	}
	apiURL := loginAPIURL(cfg, profileName)

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	device, err := requestDeviceCode(ctx, apiURL)
	if err != nil {
		return err
	}

	errOut := cmd.ErrOrStderr()
	fmt.Fprintf(errOut, "Open this URL to authorize the LangSmith CLI:\n%s\n\nEnter code: %s\n\n", device.VerificationURI, device.UserCode)
	if !noBrowser {
		if err := openBrowser(device.VerificationURI); err != nil {
			fmt.Fprintf(errOut, "Could not open a browser automatically: %v\n\n", err)
		}
	}
	fmt.Fprintln(errOut, "Waiting for authorization...")

	waitFor := timeout
	if waitFor <= 0 {
		waitFor = time.Duration(device.ExpiresIn+10) * time.Second
	}
	pollCtx, cancel := context.WithTimeout(ctx, waitFor)
	defer cancel()

	interval := time.Duration(device.Interval) * time.Second
	token, err := pollDeviceToken(pollCtx, apiURL, device.DeviceCode, interval)
	if err != nil {
		return err
	}

	profile := cfg.Profiles[profileName]
	if profile.APIURL == "" || flagAPIURL != "" || strings.TrimSpace(profile.APIURL) != apiURL {
		profile.APIURL = apiURL
	}
	if workspaceID == "" && profile.WorkspaceID == "" {
		workspaceID, err = promptWorkspaceID(cmd)
		if err != nil {
			return err
		}
	}
	if workspaceID != "" {
		profile.WorkspaceID = workspaceID
	}
	applyTokenResponse(&profile, token, time.Now())
	cfg.Profiles[profileName] = profile
	cfg.CurrentProfile = profileName
	if err := cfg.Save(); err != nil {
		return err
	}

	result := map[string]string{
		"status":           "logged_in",
		"profile":          profileName,
		"api_url":          apiURL,
		"oauth_expires_at": profile.OAuth.ExpiresAt,
	}
	if profile.WorkspaceID != "" {
		result["workspace_id"] = profile.WorkspaceID
	}
	if GetFormat() == "pretty" {
		fmt.Fprintf(cmd.OutOrStdout(), "Logged in to %s as profile %q\n", apiURL, profileName)
		return nil
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(result)
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

func promptWorkspaceID(cmd *cobra.Command) (string, error) {
	in := cmd.InOrStdin()
	if !inputIsTerminal(in) {
		return "", nil
	}
	fmt.Fprint(cmd.ErrOrStderr(), "Default workspace ID (optional, press Enter to skip): ")
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("reading workspace ID: %w", err)
	}
	workspaceID := strings.TrimSpace(line)
	if workspaceID == "" {
		return "", nil
	}
	if err := validateWorkspaceID(workspaceID); err != nil {
		return "", err
	}
	return workspaceID, nil
}

func requestDeviceCode(ctx context.Context, apiURL string) (*deviceCodeResponse, error) {
	values := url.Values{"client_id": {oauthClientID}}
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
		"refresh_token": {refreshToken},
	}
	var resp oauthTokenResponse
	if err := postOAuthForm(ctx, apiURL, "/oauth/token", values, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func pollDeviceToken(ctx context.Context, apiURL, deviceCode string, interval time.Duration) (*oauthTokenResponse, error) {
	if interval < 0 {
		interval = 0
	}

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

func requestDeviceToken(ctx context.Context, apiURL, deviceCode string) (*oauthTokenResponse, *oauthErrorResponse, error) {
	values := url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"client_id":   {oauthClientID},
		"device_code": {deviceCode},
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
