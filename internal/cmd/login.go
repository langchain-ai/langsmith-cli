package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
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
	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
	"github.com/langchain-ai/langsmith-cli/internal/mda/ui"
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
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// instructions are what to open and what to do with the code there. When the
// server sends verification_uri_complete (RFC 8628 §3.3.1), the page opens with
// the code filled in, so the person only checks it matches.
func (d *deviceCodeResponse) instructions() (string, string) {
	if d.VerificationURIComplete != "" {
		return d.VerificationURIComplete, "Check that the page shows this code, or enter it"
	}
	return d.VerificationURI, "Enter code"
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

// activationURL tags the activation page with signupSource=cli, so an account
// created there is attributed to the CLI. Current servers already tag the
// CLI's link; this covers older ones and the plain verification_uri. Signup
// also opens the page on its create-account form; a person already logged in
// goes straight to authorizing.
func activationURL(rawURL string, signup bool) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	q.Set("signupSource", "cli")
	if signup {
		q.Set("mode", "sign_up")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func newLoginCmd() *cobra.Command {
	return newDeviceAuthCmd(false)
}

func newSignupCmd() *cobra.Command {
	return newDeviceAuthCmd(true)
}

// newDeviceAuthCmd builds login and signup. Both run the same device-code
// flow; signup opens the page on the create-account form, and the new account
// is authorized for the CLI when the person finishes there.
func newDeviceAuthCmd(signup bool) *cobra.Command {
	var (
		noBrowser       bool
		timeout         time.Duration
		promptWorkspace bool
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with LangSmith using OAuth",
		Long: `Authenticate with LangSmith using OAuth.

The command stores OAuth tokens in ~/.langsmith/config.json under the selected
profile. Select a profile with --profile or LANGSMITH_PROFILE.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogin(cmd, noBrowser, timeout, flagWorkspaceID, promptWorkspace, signup)
		},
	}
	if signup {
		cmd.Use = "signup"
		cmd.Short = "Create a LangSmith account and log the CLI in to it"
		cmd.Long = `Create a LangSmith account in the browser and log the CLI in to it.

The browser opens on the sign-up form. After the account is created, the CLI is
authorized for it, and OAuth tokens are stored in ~/.langsmith/config.json under
the selected profile, as with "langsmith auth login".`
	}
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Do not open a browser automatically")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "Maximum time to wait for authorization (default: device-code expiry)")
	cmd.Flags().BoolVar(&promptWorkspace, "prompt-workspace", false, "Prompt to select and save a workspace override")
	return cmd
}

func runLogin(cmd *cobra.Command, noBrowser bool, timeout time.Duration, workspaceID string, promptWorkspace bool, signup bool) error {
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

	authorization := deviceAuthorization{
		apiURL:    apiURL,
		noBrowser: noBrowser,
		timeout:   timeout,
		signup:    signup,
		out:       cmd.ErrOrStderr(),
	}
	oauthMeta, token, err := authorization.run(ctx)
	if err != nil {
		return err
	}

	if workspaceID == "" && promptWorkspace {
		workspaceID, err = promptWorkspaceSelection(cmd, apiURL, token.AccessToken)
		if err != nil {
			return err
		}
	}
	cfg.CurrentProfile = profileName
	profile, err := saveOAuthLogin(cfg, profileName, apiURL, workspaceID, oauthMeta, token)
	if err != nil {
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
		writeLoginSuccess(cmd.OutOrStdout(), apiURL, profileName)
		return nil
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// deviceAuthorization is one run of the OAuth device authorization flow
// against a LangSmith API: it asks for a device code, tells the person where
// to approve it, and waits for the tokens.
type deviceAuthorization struct {
	apiURL string
	// noBrowser leaves opening the approval page to the person.
	noBrowser bool
	// timeout bounds the wait; zero waits until the device code expires.
	timeout time.Duration
	// signup opens the page on the create-account form.
	signup bool
	// out receives the instructions.
	out io.Writer
}

// run signs in and returns the server's OAuth metadata and the tokens.
func (d deviceAuthorization) run(ctx context.Context) (*client.OAuthMetadata, *oauthTokenResponse, error) {
	oauthMeta, err := client.ResolveOAuth(ctx, d.apiURL)
	if err != nil {
		return nil, nil, err
	}

	device, err := requestDeviceCode(ctx, oauthMeta)
	if err != nil {
		return nil, nil, err
	}

	verificationURL, codeHint := device.instructions()
	verificationURL = activationURL(verificationURL, d.signup)
	if d.signup {
		fmt.Fprintf(d.out, "Open this URL to create your LangSmith account and authorize the CLI:\n%s\n\n%s: %s\n\n", verificationURL, codeHint, device.UserCode)
	} else {
		fmt.Fprintf(d.out, "Open this URL to authorize the LangSmith CLI:\n%s\n\n%s: %s\n\nNo LangSmith account yet? You can sign up on that page.\n\n", verificationURL, codeHint, device.UserCode)
	}
	if !d.noBrowser {
		if err := openBrowser(verificationURL); err != nil {
			fmt.Fprintf(d.out, "Could not open a browser automatically: %v\n\n", err)
		}
	}
	fmt.Fprintln(d.out, "Waiting for authorization...")

	waitFor := d.timeout
	if waitFor <= 0 {
		waitFor = time.Duration(device.ExpiresIn+10) * time.Second
	}
	pollCtx, cancel := context.WithTimeout(ctx, waitFor)
	defer cancel()

	interval := normalizeDeviceCodePollInterval(time.Duration(device.Interval) * time.Second)
	token, err := pollDeviceToken(pollCtx, oauthMeta, device.DeviceCode, interval)
	if err != nil {
		// The caller stopping the wait is not a timeout.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, nil, ctxErr
		}
		return nil, nil, err
	}
	return oauthMeta, token, nil
}

// saveOAuthLogin stores a device login's tokens in profileName, pointed at
// apiURL and, when workspaceID is set, that workspace. A workspace saved for
// another LangSmith is dropped: it belongs to that installation.
func saveOAuthLogin(cfg *lsconfig.Config, profileName, apiURL, workspaceID string, oauthMeta *client.OAuthMetadata, token *oauthTokenResponse) (lsconfig.Profile, error) {
	profile := cfg.Profiles[profileName]
	previousURL := strings.TrimSpace(profile.APIURL)
	if previousURL == "" {
		previousURL = lsconfig.DefaultAPIURL
	}
	if !client.SameOrigin(previousURL, apiURL) {
		profile.WorkspaceID = ""
	}
	profile.APIURL = apiURL
	if workspaceID != "" {
		profile.WorkspaceID = workspaceID
	}
	profile.OAuth.Issuer = oauthMeta.Issuer
	applyTokenResponse(&profile, token, time.Now())
	cfg.Profiles[profileName] = profile
	if err := cfg.Save(); err != nil {
		return lsconfig.Profile{}, err
	}
	return profile, nil
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

func requestDeviceCode(ctx context.Context, meta *client.OAuthMetadata) (*deviceCodeResponse, error) {
	values := url.Values{
		"client_id": {oauthClientID},
		"resource":  {meta.Resource},
	}
	var resp deviceCodeResponse
	if err := postOAuthForm(ctx, meta.DeviceAuthorizationEndpoint, values, &resp); err != nil {
		return nil, fmt.Errorf("requesting device code: %w", err)
	}
	if resp.DeviceCode == "" || resp.UserCode == "" || resp.VerificationURI == "" {
		return nil, fmt.Errorf("requesting device code: incomplete response")
	}
	return &resp, nil
}

func pollDeviceToken(ctx context.Context, meta *client.OAuthMetadata, deviceCode string, interval time.Duration) (*oauthTokenResponse, error) {
	interval = normalizeDeviceCodePollInterval(interval)

	for {
		token, oauthErr, err := requestDeviceToken(ctx, meta, deviceCode)
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

func requestDeviceToken(ctx context.Context, meta *client.OAuthMetadata, deviceCode string) (*oauthTokenResponse, *oauthErrorResponse, error) {
	values := url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"client_id":   {oauthClientID},
		"device_code": {deviceCode},
		"resource":    {meta.Resource},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, meta.TokenEndpoint, strings.NewReader(values.Encode()))
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

func postOAuthForm(ctx context.Context, endpointURL string, values url.Values, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, strings.NewReader(values.Encode()))
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

// loginAt signs in with the device flow at apiURL, the way `langsmith auth
// login` does, writing the instructions to out, and saves the tokens to the
// selected profile; the returned options authenticate with it. The profile
// becomes the current one only when there is none.
//
// apiURL can come from the project being deployed rather than from the CLI's
// own settings. Signing in anywhere but where `langsmith auth login` would
// opens a page that server names and repoints the profile there, so it is
// confirmed first, naming both; without a terminal to ask on, it fails
// with the `langsmith auth login` command that does it.
func loginAt(ctx context.Context, apiURL string, out io.Writer) (client.Options, error) {
	cfg, err := lsconfig.Load()
	if err != nil {
		return client.Options{}, err
	}
	profileName := loginProfileName(cfg)
	if err := validateProfileName(profileName); err != nil {
		return client.Options{}, err
	}
	apiURL = client.NormalizeURL(apiURL)
	if configured := loginAPIURL(cfg, profileName); !client.SameOrigin(configured, apiURL) {
		if err := confirmLoginElsewhere(ctx, profileName, configured, apiURL); err != nil {
			return client.Options{}, err
		}
	}
	fmt.Fprintf(out, "Signing in to LangSmith at %s\n\n", apiURL)
	oauthMeta, token, err := deviceAuthorization{apiURL: apiURL, out: out}.run(ctx)
	if err != nil {
		return client.Options{}, err
	}
	if cfg.CurrentProfile == "" {
		cfg.CurrentProfile = profileName
	}
	profile, err := saveOAuthLogin(cfg, profileName, apiURL, "", oauthMeta, token)
	if err != nil {
		return client.Options{}, err
	}
	return client.Options{APIURL: apiURL, ProfileName: profileName, WorkspaceID: profile.WorkspaceID}, nil
}

// confirmLoginElsewhere asks before signing profileName in at apiURL instead
// of configured, the LangSmith it is set up for. Declining cancels.
func confirmLoginElsewhere(ctx context.Context, profileName, configured, apiURL string) error {
	confirmed, err := ui.Confirm(fmt.Sprintf(
		"Sign in to LangSmith at %s? Profile '%s' is set up for %s and will point at %s instead.",
		apiURL, profileName, configured, apiURL)).
		InitialValue(false).
		Interact(ctx)
	switch {
	case errors.Is(err, ui.ErrNotInteractive):
		return fmt.Errorf("profile '%s' is set up for %s, not %s; run `langsmith auth login --api-url %s` to sign in there",
			profileName, configured, apiURL, apiURL)
	case err != nil:
		return err
	case !confirmed:
		return errs.ErrCancelled
	default:
		return nil
	}
}
