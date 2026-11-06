// Package connection ports what `langsmith deploy` reaches of the mda CLI's
// `connection` module (`connection/mod.rs`): following an Agent Auth OAuth
// authorization session until it ends, and creating a user-owned MCP
// connection through OAuth discovery. The `mda connections` subcommands
// themselves are not ported.
//
// Every request goes to Agent Auth through [hostapi.Client], so the CLI's
// credential (an API key, or an OAuth profile's token) authenticates it. The
// only other request here is the unauthenticated probe of an MCP server that
// failed discovery, sent with plain `net/http`.
package connection

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/mda/deployment"
	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
)

// authorizationCodeGrant is the OAuth grant of a connection a user
// authorizes in the browser (`OAuthGrantType::AuthorizationCode`).
const authorizationCodeGrant = "authorization_code"

// oauthAuthorizationWaitSeconds is how long Agent Auth holds a session status
// request open waiting for the session to change.
const oauthAuthorizationWaitSeconds uint8 = 25

// WaitForOAuthStatus is the session's next status, long-polling Agent Auth.
// Cancelling ctx (Ctrl-C) ends the wait at once.
func WaitForOAuthStatus(ctx context.Context, client *hostapi.Client, sessionID string) (string, error) {
	session, err := WaitForOAuthSession(ctx, client, sessionID)
	if err != nil {
		return "", err
	}
	return session.Status, nil
}

// WaitForOAuthSession is the session once its status changes, e.g. with the
// `ErrorCode` of a failed one, long-polling like [WaitForOAuthStatus].
func WaitForOAuthSession(ctx context.Context, client *hostapi.Client, sessionID string) (hostapi.OAuthAuthorizationSession, error) {
	if err := ctx.Err(); err != nil {
		return hostapi.OAuthAuthorizationSession{}, err
	}
	session, err := client.GetOAuthAuthorizationSession(ctx, sessionID, oauthAuthorizationWaitSeconds)
	if err != nil {
		// Ctrl-C during the long poll is a cancellation, not a failed check.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return hostapi.OAuthAuthorizationSession{}, ctxErr
		}
		return hostapi.OAuthAuthorizationSession{}, fmt.Errorf("failed to check the OAuth authorization session: %w", err)
	}
	return session, nil
}

// OAuthWaitMessages is what a wait on an OAuth authorization session says
// when it ends badly.
type OAuthWaitMessages struct {
	Failed  string
	Expired string
	// Unexpected is followed by the status.
	Unexpected string
}

// PollOAuthSession follows a session from initialStatus until it completes.
func PollOAuthSession(initialStatus string, nextStatus func() (string, error), messages OAuthWaitMessages) error {
	status := initialStatus
	for {
		switch status {
		case "pending":
			next, err := nextStatus()
			if err != nil {
				return err
			}
			status = next
		case "completed":
			return nil
		case "failed":
			return errors.New(messages.Failed)
		case "expired":
			return errors.New(messages.Expired)
		default:
			return fmt.Errorf("%s: %s.", messages.Unexpected, status)
		}
	}
}

// CreateDiscoveredMcpConnection creates a user-owned OAuth connection by
// asking Agent Auth to discover the MCP server's OAuth metadata and
// dynamically register a client.
func CreateDiscoveredMcpConnection(ctx context.Context, client *hostapi.Client, slug, mcpServerURL string) (hostapi.ConnectionSummary, error) {
	created, err := client.CreateOAuthWorkspaceConnection(ctx, hostapi.CreateOAuthConnection{
		Slug:                    slug,
		OwnerPolicy:             hostapi.ConnectionOwnerPolicyUser,
		DisplayName:             slug,
		Scopes:                  []string{},
		AllowedScopes:           []string{},
		TokenEndpointAuthMethod: "none",
		AuthorizationParams:     map[string]any{},
		GrantType:               authorizationCodeGrant,
		TokenParams:             map[string]any{},
		TokenRequestHeaders:     map[string]any{},
		MCPServerURL:            mcpServerURL,
	})
	if err != nil {
		return hostapi.ConnectionSummary{}, explainMcpDcr(ctx, err, slug, mcpServerURL)
	}
	return created, nil
}

// mcpProbe is how a direct request to the MCP URL answered.
//
// Agent Auth reports a failed discovery the same way whether the host does
// not exist or simply registers no client, so a typo reads as missing CIMD or
// DCR. The CLI asks the server itself to tell the two apart.
type mcpProbe int

const (
	// mcpProbeUnreachable: no HTTP response at all: unknown host, refused
	// connection, or timeout.
	mcpProbeUnreachable mcpProbe = iota
	// mcpProbeNotFound: the host answered, but nothing is served at that path.
	mcpProbeNotFound
	// mcpProbeResponded: the host answered. An MCP server rejects an
	// unauthenticated probe, so any other status still means discovery had
	// something to talk to.
	mcpProbeResponded
)

const mcpProbeTimeout = 8 * time.Second

// probeMcpServer asks the MCP server itself, without any credential.
func probeMcpServer(ctx context.Context, url string) mcpProbe {
	ctx, cancel := context.WithTimeout(ctx, mcpProbeTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return mcpProbeUnreachable
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return classifyMcpProbe(0)
	}
	// Only the status matters.
	_ = response.Body.Close()
	return classifyMcpProbe(response.StatusCode)
}

// classifyMcpProbe reads the probe's status; 0 is no response.
func classifyMcpProbe(status int) mcpProbe {
	switch status {
	case 0:
		return mcpProbeUnreachable
	case http.StatusNotFound:
		return mcpProbeNotFound
	default:
		return mcpProbeResponded
	}
}

func explainMcpDcr(ctx context.Context, err error, slug, mcpURL string) error {
	err = explain(err, slug)
	var api *hostapi.APIError
	if !errors.As(err, &api) {
		return err
	}
	if !mcpDcrUnsupported(api) {
		return err
	}
	return mcpDiscoveryError(probeMcpServer(ctx, mcpURL), slug, mcpURL)
}

func mcpDiscoveryError(probe mcpProbe, slug, mcpURL string) error {
	switch probe {
	case mcpProbeUnreachable:
		return errs.Categorized(errs.Network, fmt.Sprintf(
			"'%s' did not answer when langsmith checked it from this machine, so it is "+
				"not possible to tell whether it supports CIMD or OAuth Dynamic Client "+
				"Registration. Check the host for a typo.", mcpURL))
	case mcpProbeNotFound:
		return errs.Categorized(errs.UserInput, fmt.Sprintf(
			"'%s' answered 404, so that host serves no MCP endpoint at that path. "+
				"Check the path for a typo.", mcpURL))
	default:
		return errs.Categorized(errs.Remote, fmt.Sprintf(
			"'%s' does not support CIMD or OAuth Dynamic Client Registration, so "+
				"langsmith cannot register a client automatically. Create the connection with "+
				"`mda connections create %s --oauth <service>` when the provider is in "+
				"`mda connections catalog`, or pass --authorize-url, --token-url, and "+
				"--client-id.", mcpURL, slug))
	}
}

func mcpDcrUnsupported(api *hostapi.APIError) bool {
	switch api.Status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusNotFound:
		return true
	}
	return dcrUnsupportedDetail(api.Detail)
}

func dcrUnsupportedDetail(detail string) bool {
	detail = strings.ToLower(detail)
	return strings.Contains(detail, "registration_endpoint") ||
		strings.Contains(detail, "dynamic client registration") ||
		strings.Contains(detail, "client id metadata") ||
		strings.Contains(detail, "cimd") ||
		strings.Contains(detail, "oauth discovery") ||
		(strings.Contains(detail, "dcr") && strings.Contains(detail, "not support"))
}

func explain(err error, slug string) error {
	var api *hostapi.APIError
	if !errors.As(err, &api) {
		return err
	}
	if api.Status == http.StatusConflict && slug != "" {
		return errs.Categorized(errs.Remote, fmt.Sprintf(
			"A connection named '%s' already exists in this workspace. "+
				"Delete it with `mda connections delete %s` and create again, or pick a "+
				"different slug.", slug, slug))
	}
	if api.Status == http.StatusForbidden &&
		strings.Contains(strings.ToLower(api.Detail), "org_scoped_key_requires_workspace") {
		return errs.Categorized(errs.Permission, fmt.Sprintf(
			"This API key is not scoped to a workspace, so LangSmith cannot tell "+
				"which one to use. Pass --workspace-id, or set %s.", deployment.WorkspaceIDEnv))
	}
	return err
}
