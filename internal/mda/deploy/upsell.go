package deploy

// Plan-entitlement upsell for `langsmith deploy` (`deploy/upsell.rs`).
//
// Shipping a Managed Deep Agent needs two LangSmith entitlements, each carried
// by the organization's plan. When either is missing the host backend answers
// `403` with a fixed `detail` (langchainplus `host/api/util.py`):
//
//   - `LangSmith Deployment is not enabled for this organization` — the plan
//     does not include LangSmith Deployment at all (`can_use_langgraph_cloud`).
//   - `Managed Deep Agents is not enabled for this organization` — the plan
//     includes Deployment but not Managed Deep Agents
//     (`managed_deep_agents_enabled`, which the private beta can also grant
//     outside the plan).
//
// Neither is a bug or a misconfiguration the developer can fix in their
// project, so a bare `... (HTTP 403)` line sends them looking in the wrong
// place. Report them as what they are — a billing decision — and link to the
// organization's plans-and-billing page.
//
// Classification reads the typed [hostapi.APIError] rather than the flattened
// message, but the `detail` strings are still a server contract: a reworded
// gate degrades to the plain error, never to a wrong claim about someone's
// plan.

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/mda/deployment"
	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
	"github.com/langchain-ai/langsmith-cli/internal/mda/ui"
)

// pricingURL is the public pricing page. Self-serve upgrades happen on the
// billing page, but Enterprise is a sales conversation, so always offer this
// alongside it.
const pricingURL = "https://www.langchain.com/pricing"

// notEnabled is the shared tail of the host backend's entitlement `detail`
// messages.
const notEnabled = "is not enabled for this organization"

// planGateKind is a LangSmith entitlement the deploying organization's plan
// does not include.
type planGateKind int

const (
	// planGateDeployment: the organization cannot use LangSmith Deployment at
	// all.
	planGateDeployment planGateKind = iota
	// planGateManagedDeepAgents: the organization can deploy, but not Managed
	// Deep Agents.
	planGateManagedDeepAgents
)

// explanation is why the deploy stopped, in the developer's terms rather
// than the API's.
func (g planGateKind) explanation() string {
	if g == planGateManagedDeepAgents {
		return "Your LangSmith organization can deploy, but does not have Managed Deep " +
			"Agents enabled. It is available on the Plus and Enterprise plans, and is " +
			"still in private beta — if upgrading does not enable it, contact LangChain " +
			"to be added."
	}
	return "Your LangSmith organization's plan does not include LangSmith Deployment, " +
		"which `langsmith deploy` needs to host a Managed Deep Agent. It is available on " +
		"the Plus and Enterprise plans."
}

// footer is the cancelled-session footer. Names the missing entitlement so a
// scrolled-back terminal still says which one it was.
func (g planGateKind) footer() string {
	if g == planGateManagedDeepAgents {
		// Not phrased as a plan limit: this entitlement is also gated by the
		// private beta, so an upgraded org can still land here.
		return "Deploy cancelled — Managed Deep Agents is not enabled for this organization."
	}
	return "Deploy cancelled — this plan does not include LangSmith Deployment."
}

// planGate classifies an error as a plan-entitlement gate; false for anything
// else.
//
// Walks the whole chain because deploy steps add context above the transport
// error, and matches case-insensitively so the entitlement is identified by
// the feature it names rather than by exact server casing.
func planGate(err error) (planGateKind, bool) {
	var api *hostapi.APIError
	if !errors.As(err, &api) || api.Status != http.StatusForbidden {
		return 0, false
	}
	detail := strings.ToLower(api.Detail)
	if !strings.Contains(detail, notEnabled) {
		return 0, false
	}
	// Most specific first: the Managed Deep Agents gate is the narrower one.
	switch {
	case strings.Contains(detail, "managed deep agents"):
		return planGateManagedDeepAgents, true
	case strings.Contains(detail, "langsmith deployment"):
		return planGateDeployment, true
	default:
		return 0, false
	}
}

// upgradeURL is a best-effort plans-and-billing link for the organization
// this deploy targets; "" when there is none.
func upgradeURL(ctx context.Context, client *hostapi.Client) string {
	workspaceID, ok := client.WorkspaceIDForLinks(ctx)
	if !ok {
		return ""
	}
	dashboardURL, err := client.DashboardURL(ctx)
	if err != nil {
		slog.Debug("dashboard URL lookup failed", "error", err)
		return ""
	}
	return deployment.PlanUpgradeURL(dashboardURL, workspaceID)
}

// reportPlanGate renders the upsell and closes the session, in place of the
// plain error footer.
//
// upgradeURL is optional: without a resolvable workspace the deep link is
// replaced by directions to the same page, which is still more actionable
// than the raw 403. Each URL sits on its own line, unprefixed, so nothing but
// the note's own border competes with it for terminal width.
func reportPlanGate(gate planGateKind, upgradeURL string) error {
	upgrade := upgradeURL
	if upgrade == "" {
		upgrade = "In LangSmith, open Settings > Billing > Plans and Pricing."
	}
	if err := ui.LogWarning(gate.explanation()); err != nil {
		return err
	}
	if err := ui.Note("Next step",
		"Upgrade this organization\n"+upgrade+"\n\n"+
			"Compare plans, or contact sales for Enterprise\n"+pricingURL); err != nil {
		return err
	}
	return ui.OutroCancel(gate.footer())
}
