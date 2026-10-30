package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/mda/compile"
	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
	"github.com/langchain-ai/langsmith-cli/internal/mda/ui"
)

// newPreflight is a secret preflight for a project at a temporary root that
// must not reach LangSmith unless keys says it may.
func newPreflight(t *testing.T, envVars map[string]string, secrets []hostapi.Secret, dashboardURL string, keys workspaceSecretKeys) *secretPreflight {
	t.Helper()
	unattended(t)
	return &secretPreflight{
		envPath:       filepath.Join(t.TempDir(), ".env"),
		envVars:       envVars,
		secrets:       &secrets,
		client:        unreachableClient(t),
		dashboardURL:  dashboardURL,
		workspaceKeys: keys,
	}
}

func loadedKeys(keys ...string) workspaceSecretKeys {
	return workspaceSecretKeys{state: workspaceSecretKeysLoaded, keys: keys}
}

func failedKeys(message string) workspaceSecretKeys {
	return workspaceSecretKeys{state: workspaceSecretKeysLookupFailed, lookupErr: errors.New(message)}
}

func TestDeploymentStatusPollStopsAfterFiveFailures(t *testing.T) {
	failures := 4

	assert.True(t, pollFailureLimitReached(&failures))
}

func TestDeploymentStatusPollRetriesTheFirstFourFailures(t *testing.T) {
	failures := 0

	for range 4 {
		assert.False(t, pollFailureLimitReached(&failures))
	}
}

func TestExternalImageUpdateRejectsAnInternalSourceDeployment(t *testing.T) {
	err := ensureExternalDeploymentSource("agent", map[string]any{"source": "internal_source"})

	assert.EqualError(t, err, "deployment 'agent' uses source 'internal_source'; deployment source types cannot be changed, so delete it or use a different --name")
}

func TestExternalImagePreflightRejectsAnInternalSourceDeployment(t *testing.T) {
	source, err := selectImageSource(true, ptr("registry.example/agent"))
	require.NoError(t, err)

	err = ensureRequestedDeploymentSource("agent", map[string]any{"source": "internal_source"}, source)

	assert.EqualError(t, err, "deployment 'agent' uses source 'internal_source'; deployment source types cannot be changed, so delete it or use a different --name")
}

func TestExternalImagePreflightAcceptsAnExternalSourceDeployment(t *testing.T) {
	source, err := selectImageSource(true, ptr("registry.example/agent"))
	require.NoError(t, err)

	assert.NoError(t, ensureRequestedDeploymentSource("agent", map[string]any{"source": "external_docker"}, source))
}

func TestInternalSourcePreflightDoesNotRequireAnExternalDeployment(t *testing.T) {
	assert.NoError(t, ensureRequestedDeploymentSource("agent", map[string]any{"source": "internal_source"},
		ImageSource{Kind: ImageSourceInternalSource}))
}

func TestSecretIsClaimedFromTheProjectEnvFile(t *testing.T) {
	assert.True(t, claimSecretFromLocalSources("OPENAI_API_KEY", map[string]string{"OPENAI_API_KEY": "sk-from-dotenv"}, nil))
}

func TestSecretIsClaimedWhenAlreadyStagedForThisDeploy(t *testing.T) {
	secrets := []hostapi.Secret{{Name: "OPENAI_API_KEY", Value: "sk-already-staged"}}

	assert.True(t, claimSecretFromLocalSources("OPENAI_API_KEY", map[string]string{}, secrets))
}

func TestSecretAbsentFromEnvFileIsNotClaimed(t *testing.T) {
	assert.False(t, claimSecretFromLocalSources("OPENAI_API_KEY", map[string]string{}, nil))
}

func TestSecretInTheProcessEnvironmentIsIgnored(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-ambient-from-shell")

	assert.False(t, claimSecretFromLocalSources("OPENAI_API_KEY", map[string]string{}, nil),
		"a key present only in the process environment must not be claimed")
}

func TestUnresolvedGatewayProviderReportsGenericKeyConfigurationError(t *testing.T) {
	_, err := modelAPIKeyRequirementForDeploy("langsmith:unknown/model")

	assert.EqualError(t, err, "The agent model 'langsmith:unknown/model' uses LangSmith Gateway, but its provider "+
		"is not supported. Use a supported `langsmith:<provider>/<model>` identifier, then "+
		"configure that provider under Settings > Integrations > Provider Secrets in "+
		"LangSmith before deploying.")
}

func TestDynamicEntryWithoutStaticModelSkipsCredentialPreflight(t *testing.T) {
	preflight := newPreflight(t, map[string]string{}, nil, "https://smith.langchain.com", workspaceSecretKeys{})

	require.NoError(t, preflight.ensureModelAPIKey(t.Context(), nil))

	assert.Equal(t, workspaceSecretKeysUnloaded, preflight.workspaceKeys.state)
}

func TestGatewayModelIsNotSatisfiedByAProjectProviderKey(t *testing.T) {
	preflight := newPreflight(t, map[string]string{"OPENAI_API_KEY": "test-provider-key"}, nil,
		"https://smith.langchain.com", loadedKeys())

	err := preflight.ensureModelAPIKey(t.Context(), ptr("langsmith:openai/gpt-5.5"))

	assert.EqualError(t, err, "The agent model 'langsmith:openai/gpt-5.5' uses LangSmith Gateway and requires "+
		"OPENAI_API_KEY in LangSmith workspace Provider Secrets. Add `OPENAI_API_KEY` under Settings > "+
		"Integrations > Provider Secrets in LangSmith (https://smith.langchain.com), then deploy again.")
}

func TestMissingGatewayKeyGuidanceOmitsAnUnavailableDashboardURL(t *testing.T) {
	preflight := newPreflight(t, map[string]string{}, nil, "", loadedKeys())

	err := preflight.ensureModelAPIKey(t.Context(), ptr("langsmith:openai/gpt-5.5"))

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "https://")
}

func TestGatewayModelIsSatisfiedByAWorkspaceProviderSecret(t *testing.T) {
	preflight := newPreflight(t, map[string]string{}, nil, "https://smith.langchain.com", loadedKeys("OPENAI_API_KEY"))

	assert.NoError(t, preflight.ensureModelAPIKey(t.Context(), ptr("langsmith:openai/gpt-5.5")))
}

func TestGatewayModelFailsWhenWorkspaceProviderSecretsCannotBeChecked(t *testing.T) {
	preflight := newPreflight(t, map[string]string{}, nil, "https://smith.langchain.com", failedKeys("workspace lookup failed"))

	err := preflight.ensureModelAPIKey(t.Context(), ptr("langsmith:openai/gpt-5.5"))

	assert.EqualError(t, err, "The agent model 'langsmith:openai/gpt-5.5' uses LangSmith Gateway and requires "+
		"OPENAI_API_KEY in LangSmith workspace Provider Secrets, but Provider Secrets could not be checked "+
		"(workspace lookup failed).")
}

func TestDirectModelIsSatisfiedByAProjectProviderKey(t *testing.T) {
	preflight := newPreflight(t, map[string]string{"OPENAI_API_KEY": "test-provider-key"}, nil,
		"https://smith.langchain.com", failedKeys("not checked"))

	assert.NoError(t, preflight.ensureModelAPIKey(t.Context(), ptr("openai:gpt-5.5")))
}

func TestDirectModelAcceptsStagedProviderKeyWithoutWorkspaceLookup(t *testing.T) {
	preflight := newPreflight(t, map[string]string{},
		[]hostapi.Secret{{Name: "OPENAI_API_KEY", Value: "test-provider-key"}},
		"https://smith.langchain.com", workspaceSecretKeys{})

	require.NoError(t, preflight.ensureModelAPIKey(t.Context(), ptr("openai:gpt-5.5")))

	assert.Equal(t, workspaceSecretKeysUnloaded, preflight.workspaceKeys.state)
}

func TestDirectModelRejectsWorkspaceOnlyProviderKey(t *testing.T) {
	for _, tc := range []struct{ model, key string }{
		{"openai:gpt-5.5", "OPENAI_API_KEY"},
		{"gpt-5.5", "OPENAI_API_KEY"},
		{"anthropic:claude-sonnet-4-5", "ANTHROPIC_API_KEY"},
		{"cohere:command-r", "COHERE_API_KEY"},
	} {
		preflight := newPreflight(t, map[string]string{tc.key: ""}, nil, "https://smith.langchain.com", loadedKeys(tc.key))

		err := preflight.ensureModelAPIKey(t.Context(), ptr(tc.model))

		assert.EqualError(t, err, fmt.Sprintf("The agent model '%s' requires %s in %s. Add `%s=...` to %s, then deploy again.",
			tc.model, tc.key, preflight.envPath, tc.key, preflight.envPath))
	}
}

func TestDirectModelRejectsMissingKeyWithoutWorkspaceLookup(t *testing.T) {
	for _, keys := range []workspaceSecretKeys{failedKeys("unavailable"), {}} {
		preflight := newPreflight(t, map[string]string{}, nil, "https://smith.langchain.com", keys)

		err := preflight.ensureModelAPIKey(t.Context(), ptr("openai:gpt-5.5"))

		assert.EqualError(t, err, fmt.Sprintf("The agent model 'openai:gpt-5.5' requires OPENAI_API_KEY in %s. "+
			"Add `OPENAI_API_KEY=...` to %s, then deploy again.", preflight.envPath, preflight.envPath))
		assert.Equal(t, keys.state, preflight.workspaceKeys.state)
	}
}

func TestRevisionStatusDisplayRejectsTerminalControlCharacters(t *testing.T) {
	assert.Equal(t, "UNKNOWN", displayRevisionStatus("BUILDING\nsecret"))
}

func TestRevisionStatusDisplayAcceptsHostStatusTokens(t *testing.T) {
	assert.Equal(t, "DEPLOY_PENDING", displayRevisionStatus("DEPLOY_PENDING"))
}

func TestFirstDeployCollisionsAreEmptyWhenResourcesDoNotExist(t *testing.T) {
	assert.Equal(t, firstDeployCollisions{}, newFirstDeployCollisions(nil, false))
}

func TestFirstDeployCollisionsIncludeAForeignContextHubRepo(t *testing.T) {
	assert.Equal(t, firstDeployCollisions{contextHubRepo: true}, newFirstDeployCollisions(nil, true))
}

func TestFirstDeployCollisionsIncludeExistingLangSmithDeployment(t *testing.T) {
	assert.Equal(t, firstDeployCollisions{langsmithDeployment: true},
		newFirstDeployCollisions(map[string]any{"is_managed_deep_agent": false}, false))
}

func TestFirstDeployCollisionsIncludeBothExistingResources(t *testing.T) {
	assert.Equal(t, firstDeployCollisions{contextHubRepo: true, langsmithDeployment: true},
		newFirstDeployCollisions(map[string]any{}, true))
}

func TestFirstDeployCollisionsAreEmptyForManagedRedeploy(t *testing.T) {
	assert.Equal(t, firstDeployCollisions{},
		newFirstDeployCollisions(map[string]any{"is_managed_deep_agent": true}, true))
}

func TestBakeSpinnerMessagePutsThePhaseOnASecondBarLine(t *testing.T) {
	t.Cleanup(ui.SetDefault(ui.NewTerminal(strings.NewReader(""), &strings.Builder{}, ui.WithColors(true))))

	plain := ansi.Strip(bakeSpinnerMessage(BakePhaseRunningSetup))

	assert.Equal(t, bakeSpinnerTitle+"\n│  running setup.sh", plain)
}

func TestCollisionConfirmationIsNotRequiredWhenResourcesDoNotExist(t *testing.T) {
	for _, tc := range []struct{ yes, interactive bool }{{false, false}, {false, true}, {true, false}} {
		decision, err := decideCollisions("my-agent", firstDeployCollisions{}, tc.yes, tc.interactive)
		require.NoError(t, err)
		assert.Equal(t, collisionDecisionProceed, decision)
	}
}

func TestCollisionConfirmationFailsWithoutInteractiveTerminalOrYes(t *testing.T) {
	_, err := decideCollisions("my-agent", firstDeployCollisions{langsmithDeployment: true}, false, false)

	assert.EqualError(t, err, "Resources for deployment 'my-agent' already exist and require confirmation before they can be overwritten. Re-run with --yes to overwrite them, or pass --name to deploy under a different name.")
	assert.Equal(t, errs.UserInput, errs.CategoryOf(err))
}

func TestCollisionConfirmationIsAnsweredByYesWithoutATerminal(t *testing.T) {
	decision, err := decideCollisions("my-agent", firstDeployCollisions{contextHubRepo: true, langsmithDeployment: true}, true, false)

	require.NoError(t, err)
	assert.Equal(t, collisionDecisionProceed, decision)
}

func TestCollisionConfirmationSkipsThePromptWithYesInATerminal(t *testing.T) {
	decision, err := decideCollisions("my-agent", firstDeployCollisions{langsmithDeployment: true}, true, true)

	require.NoError(t, err)
	assert.Equal(t, collisionDecisionProceed, decision)
}

func TestCollisionConfirmationCanPromptInInteractiveTerminal(t *testing.T) {
	decision, err := decideCollisions("my-agent", firstDeployCollisions{contextHubRepo: true}, false, true)

	require.NoError(t, err)
	assert.Equal(t, collisionDecisionPrompt, decision)
}

func TestSchedulesSummaryStaysSilentWhenNothingChanged(t *testing.T) {
	_, ok := schedulesSummary(0, 0)
	assert.False(t, ok)
}

func TestSchedulesSummaryReportsRemovalsAndReconciliations(t *testing.T) {
	removed, ok := schedulesSummary(0, 2)
	assert.True(t, ok)
	assert.Equal(t, "Removed 2 schedule(s)", removed)
	reconciled, ok := schedulesSummary(3, 0)
	assert.True(t, ok)
	assert.Equal(t, "Reconciled 3 schedule(s)", reconciled)
}

func TestScheduleReconcileWarningKeepsTheCauseAndTheNextStep(t *testing.T) {
	err := fmt.Errorf("failed to search existing managed crons: %w", errors.New("operation timed out"))

	assert.Equal(t, "failed to search existing managed crons: operation timed out\n"+
		"The deployment is live, but its schedules may be missing or stale. Re-run `langsmith deploy` to reconcile them.",
		scheduleReconcileWarning(err, 2))
}

func TestScheduleReconcileWarningFlagsStaleCronsWhenNothingIsAuthored(t *testing.T) {
	err := errors.New("operation timed out")

	assert.Contains(t, scheduleReconcileWarning(err, 0), "previous deploy")
	assert.Contains(t, scheduleReconcileWarning(err, 1), "missing or stale")
}

func TestSlackCredentialsAreOptionalButOtherChannelSecretsRemainRequired(t *testing.T) {
	requirements := jsonObject(t, `{
		"slack": {"provider": "slack", "requiredEnv": ["SLACK_SIGNING_SECRET", "SLACK_BOT_TOKEN"]},
		"other": {"provider": "other", "requiredEnv": ["OTHER_TOKEN"]}
	}`)

	required, optionalSlack := channelSecretPolicy(requirements)

	assert.Equal(t, []string{"OTHER_TOKEN"}, required)
	assert.Equal(t, []string{"SLACK_BOT_TOKEN", "SLACK_SIGNING_SECRET"}, optionalSlack)
}

func connectionRow(slug, ownerPolicy string) hostapi.ConnectionSummary {
	return hostapi.ConnectionSummary{
		ConnectionID: "77777777-7777-7777-7777-777777777777",
		Slug:         slug,
		DisplayName:  slug,
		Kind:         "secret",
		OwnerPolicy:  ownerPolicy,
	}
}

func oauthConnectionRow(slug string) hostapi.ConnectionSummary {
	return hostapi.ConnectionSummary{
		ConnectionID:    "77777777-7777-7777-7777-777777777777",
		Slug:            slug,
		DisplayName:     slug,
		Kind:            "oauth2",
		OAuthProviderID: ptr("11111111-1111-1111-1111-111111111111"),
		OAuthAppSlug:    "notion",
	}
}

func mcpConnection(slug, serverName string) compile.McpConnectionEntry {
	return compile.McpConnectionEntry{
		ConnectorName: "mcp",
		ServerName:    serverName,
		URL:           "https://mcp." + serverName + ".com/mcp",
		Slug:          slug,
		OwnerPolicy:   "user",
	}
}

func TestDeploySelectsAUniqueMissingUserOwnedMcpConnectionForDCR(t *testing.T) {
	requirements := jsonObject(t, `{"mcp": {"userConnections": ["notion"]}}`)
	entries := []compile.McpConnectionEntry{mcpConnection("notion", "notion")}

	selected, err := missingUserOwnedMcpConnections(requirements, map[string]string{}, map[string]hostapi.ConnectionSummary{}, entries)

	require.NoError(t, err)
	assert.Equal(t, entries, selected)
}

func TestDeployLeavesAnExistingMcpConnectionUnchanged(t *testing.T) {
	requirements := jsonObject(t, `{"mcp": {"userConnections": ["notion"]}}`)
	existing := map[string]hostapi.ConnectionSummary{"notion": oauthConnectionRow("notion")}

	selected, err := missingUserOwnedMcpConnections(requirements, map[string]string{}, existing,
		[]compile.McpConnectionEntry{mcpConnection("notion", "notion")})

	require.NoError(t, err)
	assert.Empty(t, selected)
}

func TestDeployDoesNotGuessBetweenMultipleMcpServersForOneSlug(t *testing.T) {
	requirements := jsonObject(t, `{"mcp": {"userConnections": ["shared"]}}`)
	entries := []compile.McpConnectionEntry{mcpConnection("shared", "notion"), mcpConnection("shared", "linear")}

	_, err := missingUserOwnedMcpConnections(requirements, map[string]string{}, map[string]hostapi.ConnectionSummary{}, entries)

	assert.EqualError(t, err, "connection 'shared' is used by multiple user-owned MCP servers in this "+
		"project: mcp::notion, mcp::linear. Keep the slug unique per MCP server.")
}

func TestDeployLeavesAMissingNonMcpConnectionForNormalValidation(t *testing.T) {
	requirements := jsonObject(t, `{"other": {"userConnections": ["acme"]}}`)

	selected, err := missingUserOwnedMcpConnections(requirements, map[string]string{}, map[string]hostapi.ConnectionSummary{}, nil)

	require.NoError(t, err)
	assert.Empty(t, selected)
}

func TestConnectionValidationAcceptsOAuthRowsWithoutOwnerPolicy(t *testing.T) {
	requirements := jsonObject(t, `{"mcp": {"userConnections": ["notion"]}}`)
	existing := map[string]hostapi.ConnectionSummary{"notion": oauthConnectionRow("notion")}

	assert.NoError(t, validateWorkspaceConnectionsForManifest(requirements, map[string]string{}, existing))
}

func TestConnectionValidationAcceptsExistingWorkspaceRows(t *testing.T) {
	requirements := jsonObject(t, `{"mcp": {"userConnections": ["notion"], "agentConnections": ["docs-token"]}}`)
	existing := map[string]hostapi.ConnectionSummary{
		"docs-token": connectionRow("docs-token", "agent"),
		"notion":     connectionRow("notion", "user"),
	}

	assert.NoError(t, validateWorkspaceConnectionsForManifest(requirements, map[string]string{}, existing))
}

func TestConnectionValidationRejectsMissingWorkspaceRows(t *testing.T) {
	requirements := jsonObject(t, `{"mcp": {"userConnections": ["notion"], "agentConnections": ["tavily", "docs-token"]}}`)
	existing := map[string]hostapi.ConnectionSummary{
		"notion":     connectionRow("notion", "user"),
		"docs-token": connectionRow("docs-token", "agent"),
	}

	err := validateWorkspaceConnectionsForManifest(requirements, map[string]string{}, existing)

	assert.EqualError(t, err, "connections declared by this project are not in the workspace\n\n"+
		"  'tavily'  agent-owned\n\nCreate each with `mda connections create`.")
}

func TestConnectionValidationListsEachMissingConnectionWithItsDeclaredOwner(t *testing.T) {
	requirements := jsonObject(t, `{"mcp": {"userConnections": ["notion-docs"], "agentConnections": ["tavily"]}}`)

	err := validateWorkspaceConnectionsForManifest(requirements, map[string]string{}, map[string]hostapi.ConnectionSummary{})

	assert.EqualError(t, err, strings.Join([]string{
		"connections declared by this project are not in the workspace",
		"",
		"  'tavily'       agent-owned",
		"  'notion-docs'  user-owned",
		"",
		"Create each with `mda connections create`.",
	}, "\n"))
}

func TestConnectionValidationNamesTheChannelThatProvidesAReservedConnection(t *testing.T) {
	requirements := jsonObject(t, `{"mcp": {"userConnections": ["mda/github-token"]}}`)
	provided := map[string]string{"mda/github-token": "github"}

	err := validateWorkspaceConnectionsForManifest(requirements, provided, map[string]hostapi.ConnectionSummary{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "the github channel's app")
}

func TestConnectionValidationRejectsAmbiguousManifestOwnership(t *testing.T) {
	requirements := jsonObject(t, `{"mcp": {"userConnections": ["shared"], "agentConnections": ["shared"]}}`)

	err := validateWorkspaceConnectionsForManifest(requirements, map[string]string{}, map[string]hostapi.ConnectionSummary{})

	assert.EqualError(t, err, "connection 'shared' is declared as both agent-owned and user-owned in this project. "+
		"Use one owner per slug in connectors.")
}

func TestHTTPChannelLinksIncludeReceiveOnlyChannelsAndUseDeclaredPaths(t *testing.T) {
	requirements := jsonObject(t, `{
		"orders": {"provider": "slack", "autoReply": false, "eventsPath": "/channels/orders/events"},
		"alerts": {"provider": "custom", "autoReply": true, "eventsPath": "/channels/alerts/events"},
		"slack": {"transport": "trigger_server"},
		"sender": {"provider": "custom", "autoReply": true}
	}`)

	assert.Equal(t, []ui.Link{
		{Label: "alerts", URL: "https://agent.example/base/channels/alerts/events"},
		{Label: "orders", URL: "https://agent.example/base/channels/orders/events"},
	}, httpChannelLinks("https://agent.example/base/", requirements))
}

func TestHTTPChannelLinksAreEmptyWithoutHTTPChannels(t *testing.T) {
	for _, requirements := range []string{
		`{}`,
		`{"slack": {"transport": "trigger_server"}}`,
		`{"github": {"provider": "github", "eventsPath": "/channels/github/events"}}`,
	} {
		assert.Empty(t, httpChannelLinks("https://agent.example", jsonObject(t, requirements)), requirements)
	}
}

func TestChannelSetupRequiresALiveURL(t *testing.T) {
	assert.EqualError(t, requireTriggerRegistrationURL("", true), "Deployment started, but channel setup could not complete because no revision was available to track; retry `langsmith deploy` once the deployment is live.")
	assert.NoError(t, requireTriggerRegistrationURL("https://agent.example", true))
}

func TestMissingRevisionRemainsNonFatalWithoutATriggerChannel(t *testing.T) {
	assert.NoError(t, requireTriggerRegistrationURL("", false))
}

func TestDeploymentLinksListsAgentServerAndDashboard(t *testing.T) {
	assert.Equal(t, []ui.Link{
		{Label: "Agent Server", URL: "https://agent.us.langgraph.app"},
		{Label: "Dashboard", URL: "https://smith.langchain.com/o/t-1/host/deployments/dep-1"},
	}, deploymentLinks("https://agent.us.langgraph.app", "https://smith.langchain.com/o/t-1/host/deployments/dep-1"))
}

func TestDeploymentLinksOmitsDashboardWhenTenantIsUnknown(t *testing.T) {
	assert.Equal(t, []ui.Link{{Label: "Agent Server", URL: "https://agent.us.langgraph.app"}},
		deploymentLinks("https://agent.us.langgraph.app", ""))
}

func TestDeploymentIDStatusRecordCanonicalizesAUUID(t *testing.T) {
	assert.Equal(t, "Deployment ID: 7ad71a02-1c0e-4477-a0e4-d27e108e5ab6",
		deploymentIDStatusRecord("7AD71A02-1C0E-4477-A0E4-D27E108E5AB6"))
}

func TestDeploymentIDStatusRecordRejectsUntrustedText(t *testing.T) {
	assert.Empty(t, deploymentIDStatusRecord("id\nDeployment status: DEPLOYED"))
}

func TestScheduleCronRequestMapsGraphToAssistantID(t *testing.T) {
	threadID, payload, err := scheduleCronRequest("daily", jsonObject(t, `{
		"graph": "agent",
		"schedule": "0 9 * * *",
		"input": {"messages": [{"role": "user", "content": "Hi"}]},
		"metadata": {"mda_managed": true, "mda_schedule": "daily"},
		"on_run_completed": "delete"
	}`))

	require.NoError(t, err)
	assert.Empty(t, threadID)
	assert.Equal(t, "agent", payload["assistant_id"])
	assert.NotContains(t, payload, "graph")
	assert.Equal(t, "delete", payload["on_run_completed"])
}

func TestScheduleCronRequestExtractsThreadID(t *testing.T) {
	threadID, payload, err := scheduleCronRequest("nightly", jsonObject(t, `{
		"graph": "agent",
		"thread_id": "nightly-thread",
		"schedule": "30 2 * * *",
		"input": {"messages": [{"role": "user", "content": "Sweep"}]},
		"metadata": {"mda_managed": true, "mda_schedule": "nightly"}
	}`))

	require.NoError(t, err)
	assert.Equal(t, "nightly-thread", threadID)
	assert.Equal(t, "agent", payload["assistant_id"])
	assert.NotContains(t, payload, "thread_id")
}

func TestManagedCronIDsReadsBareSearchArray(t *testing.T) {
	response := jsonValue(t, `[
		{"cron_id": "cron-1", "metadata": {"mda_managed": true, "mda_schedule": "daily"}},
		{"cron_id": "cron-2", "metadata": {"mda_managed": true, "mda_managed_by": "runtime"}},
		{"id": "not-a-cron-id"}
	]`)

	ids, err := managedCronIDs(response, false)

	require.NoError(t, err)
	assert.Equal(t, []string{"cron-1", "cron-2"}, ids)
}

func TestManagedCronIDsRejectsUnexpectedSearchShape(t *testing.T) {
	_, err := managedCronIDs(jsonValue(t, `{"crons": [{"cron_id": "cron-1"}]}`), false)

	assert.EqualError(t, err, "unexpected cron search response shape: expected a JSON array")
}

func TestManagedCronIDsPreservesRuntimeAndUnmanagedRecordsOnDeploy(t *testing.T) {
	response := jsonValue(t, `[
		{"cron_id": "legacy", "metadata": {"mda_managed": true, "mda_schedule": "daily"}},
		{"cron_id": "deploy", "metadata": {"mda_managed": true, "mda_managed_by": "deploy"}},
		{"cron_id": "runtime", "metadata": {"mda_managed": true, "mda_schedule": "daily", "mda_managed_by": "runtime"}},
		{"cron_id": "unknown", "metadata": {"mda_managed": true, "mda_managed_by": "unknown"}},
		{"cron_id": "unmarked", "metadata": {"mda_managed": true}},
		{"cron_id": "foreign", "metadata": {"mda_schedule": "daily"}}
	]`)

	deployOwned, err := managedCronIDs(response, true)
	require.NoError(t, err)
	assert.Equal(t, []string{"legacy", "deploy"}, deployOwned)
	all, err := managedCronIDs(response, false)
	require.NoError(t, err)
	assert.Equal(t, []string{"legacy", "deploy", "runtime", "unknown", "unmarked"}, all)
}

// captureStderr sends what the process writes to stderr into a pipe for the
// rest of the test, and returns a function that ends the capture and returns
// what was written.
func captureStderr(t *testing.T) func() string {
	t.Helper()
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	previous := os.Stderr
	os.Stderr = writer
	captured := make(chan string)
	go func() {
		data, err := io.ReadAll(reader)
		assert.NoError(t, err)
		captured <- string(data)
	}()
	done := false
	finish := func() string {
		if done {
			return ""
		}
		done = true
		os.Stderr = previous
		require.NoError(t, writer.Close())
		return <-captured
	}
	t.Cleanup(func() { finish() })
	return finish
}

func TestWithoutATerminalEachDeploymentStatusIsPrintedOnce(t *testing.T) {
	session := unattended(t)
	stderr := captureStderr(t)
	server := serveSequence(t,
		okReply(`{"resources":[{"id":"revision-1"}]}`),
		okReply(`{"id":"revision-1","status":"BUILD_FAILED"}`),
	)

	_, err := pollAndReport(t.Context(), hostClient(t, server.URL), "dep-1", "agent", "", nil, time.Minute)

	assert.EqualError(t, err, "deployment of 'agent' failed with status BUILD_FAILED")
	assert.Empty(t, stderr())
	assert.Equal(t, 1, strings.Count(session.String(), "BUILD_FAILED…"), session.String())
}

func TestWithoutATerminalEachBakePhaseIsPrintedOnce(t *testing.T) {
	session := unattended(t)
	stderr := captureStderr(t)
	sp := ui.Spinner()
	sp.Start(bakeSpinnerTitle)
	progress := bakeProgress(sp)

	progress(BakePhaseCheckingRecipe)
	progress(BakePhaseStartingBuilder)
	sp.Stop("Baked sandbox recipe snapshot")

	assert.Empty(t, stderr())
	for _, phase := range []BakePhase{BakePhaseCheckingRecipe, BakePhaseStartingBuilder} {
		assert.Equal(t, 1, strings.Count(session.String(), phase.Label()), session.String())
	}
}

// inputOnlyTerminal makes the default terminal one that can be typed on but
// shows prompts nowhere, as when stderr is redirected to a file.
func inputOnlyTerminal(t *testing.T) {
	t.Helper()
	t.Cleanup(ui.SetDefault(ui.NewTerminal(strings.NewReader(""), &strings.Builder{}, ui.WithTerminals(true, false), ui.WithColors(false))))
}

func TestContextConflictsAskForAStrategyWhenPromptsCannotShow(t *testing.T) {
	inputOnlyTerminal(t)

	_, err := resolveContextConflicts(t.Context(), []contextSection{contextSectionInstructions}, nil)

	assert.EqualError(t, err, "Context Hub instructions.md changed since the last MDA sync. Re-run with "+
		"--context-strategy overwrite or --context-strategy keep-hub.")
}

func TestCollisionsAskForYesWhenPromptsCannotShow(t *testing.T) {
	inputOnlyTerminal(t)
	server := serveSequence(t,
		okReply(`{"resources":[{"name":"my-agent","id":"dep-1","source":"internal_source"}]}`),
		okReply(`{"name":"my-agent","id":"dep-1","source":"internal_source"}`),
		reply{status: http.StatusNotFound, body: `{"detail":"not found"}`},
	)

	_, err := confirmFirstDeployCollisions(t.Context(), hostClient(t, server.URL), contextHubClient(t, server.URL),
		"my-agent", nil, ImageSource{Kind: ImageSourceInternalSource}, false)

	assert.EqualError(t, err, "Resources for deployment 'my-agent' already exist and require confirmation before "+
		"they can be overwritten. Re-run with --yes to overwrite them, or pass --name to deploy under a different name.")
}

func TestAStoppedWorkspaceSecretLookupIsACancellationNotAMissingSecret(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	slack := jsonObject(t, `{"slack": {"provider": "slack", "requiredEnv": ["SLACK_SIGNING_SECRET", "SLACK_BOT_TOKEN"]}}`)

	err := newPreflight(t, map[string]string{}, nil, "", workspaceSecretKeys{}).ensureChannelRequirementsSecrets(ctx, slack)
	assert.ErrorIs(t, err, context.Canceled)

	err = newPreflight(t, map[string]string{}, nil, "", workspaceSecretKeys{}).ensureRequiredSecret(ctx, "OTHER_TOKEN", "Needed")
	assert.ErrorIs(t, err, context.Canceled)
}

func TestSourceUploadErrorsLeaveOutTheSignedQuery(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	base := server.URL
	server.Close()

	err := uploadToGCS(t.Context(), base+"/upload/source.tar.gz?X-Goog-Signature=secret-signature", []byte("archive"))

	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), `source upload failed: Put "`+base+`/upload/source.tar.gz": `), err.Error())
	assert.NotContains(t, err.Error(), "secret-signature")
}
