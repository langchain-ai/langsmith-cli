package deployment

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
)

func ptr[T any](value T) *T {
	return &value
}

func noEnv(string) (string, bool) {
	return "", false
}

// mapEnv looks values up in envFile after the process env stand-in process.
func mapEnv(envFile map[string]string, process func(string) (string, bool)) envLookup {
	return func(key string) (string, bool) {
		return hostapi.LookupEnv(key, envFile, process)
	}
}

func TestInterruptedStatusIsTerminalButNotFailed(t *testing.T) {
	assert.True(t, IsTerminalStatus("INTERRUPTED"))
	assert.True(t, IsInterruptedStatus("INTERRUPTED"))
	assert.False(t, IsFailedStatus("INTERRUPTED"))
	assert.False(t, IsInterruptedStatus("DEPLOYED"))
	assert.False(t, IsInterruptedStatus("BUILD_FAILED"))
}

func TestSecretsSkipReservedAndEmpty(t *testing.T) {
	env := map[string]string{
		"OPENAI_API_KEY":            "sk-123",
		"LANGSMITH_API_KEY":         "ls-secret",
		"LANGSMITH_GATEWAY_API_KEY": "gateway-secret",
		"EMPTY":                     "",
	}

	var names []string
	for _, secret := range SecretsFromEnv(env) {
		names = append(names, secret.Name)
	}
	assert.Equal(t, []string{"LANGSMITH_GATEWAY_API_KEY", "OPENAI_API_KEY"}, names)
}

func TestGeneratedHandoffSecretIsInjectedUnderDeployOwnedName(t *testing.T) {
	var secrets []hostapi.Secret
	SetMDAHandoffSecret(&secrets, "generated-secret")

	assert.Equal(t, []hostapi.Secret{{Name: MDAHandoffSecretEnv, Value: "generated-secret"}}, secrets)
}

func TestSetWorkspaceIDSecretInjectsWhenMissing(t *testing.T) {
	secrets := []hostapi.Secret{{Name: "OPENAI_API_KEY", Value: "sk"}}
	SetWorkspaceIDSecret(&secrets, "ws-1")
	assert.Equal(t, []hostapi.Secret{
		{Name: "OPENAI_API_KEY", Value: "sk"},
		{Name: WorkspaceIDEnv, Value: "ws-1"},
	}, secrets)
}

func TestSetWorkspaceIDSecretReplacesExisting(t *testing.T) {
	secrets := []hostapi.Secret{{Name: WorkspaceIDEnv, Value: "ws-old"}}
	SetWorkspaceIDSecret(&secrets, "ws-new")
	assert.Equal(t, []hostapi.Secret{{Name: WorkspaceIDEnv, Value: "ws-new"}}, secrets)
}

func TestSetWorkspaceIDSecretSkipsEmpty(t *testing.T) {
	secrets := []hostapi.Secret{{Name: WorkspaceIDEnv, Value: "ws-old"}}
	SetWorkspaceIDSecret(&secrets, "")
	assert.Equal(t, []hostapi.Secret{{Name: WorkspaceIDEnv, Value: "ws-old"}}, secrets)
}

func TestSandboxAPIEndpointUsesPlatformSurfaceAndReplacesStaleValue(t *testing.T) {
	secrets := []hostapi.Secret{{Name: MDASandboxAPIEndpointEnv, Value: "https://stale.example/v2/sandboxes"}}

	EnsureSandboxAPIEndpointSecret(&secrets, "https://self-hosted.example/")

	assert.Equal(t, []hostapi.Secret{{Name: MDASandboxAPIEndpointEnv, Value: "https://self-hosted.example/v2/sandboxes"}}, secrets)
}

func TestEnsureAssistantIDSecretInjectsAndOverwrites(t *testing.T) {
	secrets := []hostapi.Secret{{Name: MDAAssistantIDEnv, Value: "stale"}}
	EnsureAssistantIDSecret(&secrets, "gmail-typescript-example")
	assert.Equal(t, []hostapi.Secret{{Name: MDAAssistantIDEnv, Value: "gmail-typescript-example"}}, secrets)
}

func TestEnsureLangsmithURLSecretInjectsAndOverwrites(t *testing.T) {
	secrets := []hostapi.Secret{{Name: MDALangsmithURLEnv, Value: "https://stale.example"}}
	EnsureLangsmithURLSecret(&secrets, "https://eu.smith.langchain.com")
	assert.Equal(t, []hostapi.Secret{{Name: MDALangsmithURLEnv, Value: "https://eu.smith.langchain.com"}}, secrets)
}

func TestLangsmithEndpointIsNotForwardedFromEnv(t *testing.T) {
	// Host rejects LANGSMITH_ENDPOINT as a user secret (HTTP 400 reserved).
	env := map[string]string{LangsmithEndpointEnv: "https://eu.api.smith.langchain.com"}
	assert.Empty(t, SecretsFromEnv(env))
}

func TestSandboxAPIEndpointIsNotForwardedFromEnv(t *testing.T) {
	env := map[string]string{MDASandboxAPIEndpointEnv: "https://untrusted.example/v2/sandboxes"}
	assert.Empty(t, SecretsFromEnv(env))
}

func TestEnsureDeploymentNameSecretInjectsWhenMissing(t *testing.T) {
	var secrets []hostapi.Secret
	EnsureDeploymentNameSecret(&secrets, "my-agent")
	assert.Equal(t, []hostapi.Secret{{Name: MDADeploymentNameEnv, Value: "my-agent"}}, secrets)
}

func TestEnsureDeploymentNameSecretOverridesAnyEnvValue(t *testing.T) {
	// A stale/copied `.env` value would otherwise name this deployment's
	// sandboxes after a different deployment.
	secrets := []hostapi.Secret{{Name: MDADeploymentNameEnv, Value: "some-other-agent"}}
	EnsureDeploymentNameSecret(&secrets, "my-agent")
	assert.Equal(t, []hostapi.Secret{{Name: MDADeploymentNameEnv, Value: "my-agent"}}, secrets)
}

func TestEnsureDeploymentNameSecretSkipsEmpty(t *testing.T) {
	var secrets []hostapi.Secret
	EnsureDeploymentNameSecret(&secrets, "")
	assert.Empty(t, secrets)
}

func TestRecipeDigestIsStableAndLengthBounded(t *testing.T) {
	setup := []byte("#!/bin/sh\necho hi\n")
	a := RecipeInputs{Setup: setup, SnapshotName: ptr("base")}.Digest()
	b := RecipeInputs{Setup: setup, SnapshotName: ptr("base")}.Digest()
	assert.Equal(t, a, b)
	assert.Len(t, a, RecipeDigestLength)
	assert.True(t, isRecipeDigest(a), a)
	assert.NotEqual(t, a, RecipeInputs{Setup: []byte("#!/bin/sh\necho bye\n"), SnapshotName: ptr("base")}.Digest())
	assert.NotEqual(t, a, RecipeInputs{Setup: setup, SnapshotID: ptr("id-1")}.Digest())
	assert.NotEqual(t,
		RecipeInputs{Setup: setup, DockerImage: ptr("ghcr.io/acme/base:1"), Registry: ptr("ghcr")}.Digest(),
		RecipeInputs{Setup: setup, DockerImage: ptr("ghcr.io/acme/base:1")}.Digest(),
	)
	assert.NotEqual(t,
		RecipeInputs{DockerImage: ptr("python:3.12")}.Digest(),
		RecipeInputs{DockerImage: ptr("python:3.12"), FsCapacityBytes: ptr(uint64(2 << 30))}.Digest(),
	)
}

func TestRecipeDigestMatchesTheRustCLI(t *testing.T) {
	// Recipe snapshots are named by this digest, so it must stay the one mda
	// computed or every existing deployment would bake its recipe again.
	// Expected values: Python's hashlib over the bytes the Rust CLI hashes.
	assert.Equal(t, "1b8723323333a490", RecipeInputs{Setup: []byte("echo ready\n")}.Digest())
	assert.Equal(t, "97b45d43659ad8a1", RecipeInputs{DockerImage: ptr("python:3.12"), Registry: ptr("ghcr"), FsCapacityBytes: ptr(uint64(2 << 30))}.Digest())
}

func TestIntermediateSnapshotsMatchOnlyTheirOwnDeployment(t *testing.T) {
	digest := "640ecb01b34ba969"
	for _, kind := range IntermediateSnapshotKinds {
		name := IntermediateSnapshotName("agent", kind, digest)
		assert.Equal(t, fmt.Sprintf("agent--%s-%s", kind, digest), name)
		assert.True(t, IsDeploymentIntermediateSnapshot(name, "agent"))
		// Neighbours that share a prefix with `agent`.
		assert.False(t, IsDeploymentIntermediateSnapshot(name, "agent--x"))
		assert.False(t, IsDeploymentIntermediateSnapshot(IntermediateSnapshotName("agent--x", kind, digest), "agent"))
		assert.False(t, IsDeploymentIntermediateSnapshot(IntermediateSnapshotName("agent-x", kind, digest), "agent"))
		assert.False(t, IsDeploymentIntermediateSnapshot(fmt.Sprintf("agent--%s-%s--setup-%s", kind, digest, digest), "agent"))
	}
	assert.False(t, IsDeploymentIntermediateSnapshot("agent--dockerbase-abc", "agent"))
	assert.False(t, IsDeploymentIntermediateSnapshot("agent--setup-640ecb01b34ba969", "agent"))
}

func TestIntermediateSnapshotNameStaysWithinTheHostLimit(t *testing.T) {
	deployment := "mda-integ-sandbox-recipe-ts-pr-339-abcdef"
	for _, kind := range IntermediateSnapshotKinds {
		name := IntermediateSnapshotName(deployment, kind, "0123456789abcdef")
		assert.LessOrEqual(t, len(name), HostSandboxNameLimit, name)
		assert.True(t, IsDeploymentIntermediateSnapshot(name, deployment))
		assert.False(t, IsDeploymentIntermediateSnapshot(name, "mda-integ-sandbox-recipe-ts-pr-339-abcdeg"))
	}
}

func TestBakeSnapshotListPrefixCoversRecipesAndIntermediates(t *testing.T) {
	for _, deployment := range []string{"agent", "mda-integ-sandbox-recipe-ts-pr-339-abcdef"} {
		prefix := BakeSnapshotListPrefix(deployment)
		assert.NotEmpty(t, prefix)
		assert.True(t, strings.HasPrefix(RecipeSnapshotName(deployment, "0123456789abcdef"), prefix))
		for _, kind := range IntermediateSnapshotKinds {
			assert.True(t, strings.HasPrefix(IntermediateSnapshotName(deployment, kind, "0123456789abcdef"), prefix))
		}
	}
	assert.Equal(t, "agent--", BakeSnapshotListPrefix("agent"))
}

func TestRecipeSnapshotNameUsesSetupSegment(t *testing.T) {
	assert.Equal(t, "my-agent--setup-abcd1234efgh5678", RecipeSnapshotName("my-agent", "abcd1234efgh5678"))
}

func TestRecipeSnapshotNameStaysWithinTheHostLimit(t *testing.T) {
	name := RecipeSnapshotName("mda-integ-sandbox-recipe-ts-pr-339-abcdef", "0123456789abcdef")

	assert.LessOrEqual(t, len(name), HostSandboxNameLimit, name)
	// Same literal as the integration-test helper's mirror of this rule
	// (`integration-tests/src/helpers/snapshots.test.ts`) — the two must agree
	// or the suite looks for a snapshot bake never created.
	assert.Equal(t, "mda-integ-sandbox-recipe-ts-pr-61ebbf7d--setup-0123456789abcdef", name)
}

func TestRecipeReferencePinsTheTagHostAssigned(t *testing.T) {
	// A Docker-built recipe inherits the image tag, so the bare name would
	// resolve as `…:latest` and 404 at thread create.
	assert.Equal(t, "my-agent--setup-abc:0.1.1", RecipeSnapshotReference("my-agent--setup-abc", []string{"0.1.1"}))
}

func TestRecipeReferenceStaysBareWhenLatestExistsOrNoTags(t *testing.T) {
	assert.Equal(t, "my-agent--setup-abc", RecipeSnapshotReference("my-agent--setup-abc", []string{"latest"}))
	assert.Equal(t, "my-agent--setup-abc", RecipeSnapshotReference("my-agent--setup-abc", []string{"0.1.1", "latest"}))
	assert.Equal(t, "my-agent--setup-abc", RecipeSnapshotReference("my-agent--setup-abc", nil))
	assert.Equal(t, "my-agent--setup-abc", RecipeSnapshotReference("my-agent--setup-abc", []string{"  "}))
}

func TestRecognizesRecipeSnapshotsThisDeploymentOwns(t *testing.T) {
	assert.True(t, IsDeploymentRecipeSnapshot("my-agent--setup-abcd1234efgh5678", "my-agent"))
	assert.False(t, IsDeploymentRecipeSnapshot("my-agent--setup-", "my-agent"))
	assert.False(t, IsDeploymentRecipeSnapshot("my-agent-staging--setup-abcd1234efgh5678", "my-agent"))
	assert.False(t, IsDeploymentRecipeSnapshot("my-agent--0123456789ab", "my-agent"))
	assert.False(t, IsDeploymentRecipeSnapshot("other--setup-abcd1234efgh5678", "my-agent"))
}

func TestEnsureRecipeSnapshotSecretInjectsAndOverwrites(t *testing.T) {
	secrets := []hostapi.Secret{{Name: MDASandboxRecipeSnapshotEnv, Value: "stale"}}
	EnsureRecipeSnapshotSecret(&secrets, "my-agent--setup-abc")
	assert.Equal(t, []hostapi.Secret{{Name: MDASandboxRecipeSnapshotEnv, Value: "my-agent--setup-abc"}}, secrets)
	ClearRecipeSnapshotSecret(&secrets)
	assert.Empty(t, secrets)
}

func TestRecipeSnapshotEnvIsNotForwardedFromEnv(t *testing.T) {
	env := map[string]string{MDASandboxRecipeSnapshotEnv: "my-agent--setup-abc"}
	assert.Empty(t, SecretsFromEnv(env))
}

func TestDeploymentNameIsNotForwardedFromEnv(t *testing.T) {
	env := map[string]string{MDADeploymentNameEnv: "some-other-agent"}
	assert.Empty(t, SecretsFromEnv(env))
}

func TestResolveCORSAllowOriginsMergesExactOrigins(t *testing.T) {
	env := map[string]string{
		MDAPublicAppURLEnv: "https://app.example.com/",
		MDACORSOriginsEnv:  "http://localhost:4900, https://preview.example.com/",
	}
	assert.Equal(t, []string{
		"http://localhost:4900",
		"https://app.example.com",
		"https://preview.example.com",
	}, ResolveCORSAllowOrigins(env))
}

func TestEnsureCORSAllowOriginsInjectsWhenMissing(t *testing.T) {
	env := map[string]string{MDAPublicAppURLEnv: "https://app.example.com"}
	var secrets []hostapi.Secret
	EnsureCORSAllowOrigins(&secrets, env)
	assert.Equal(t, []hostapi.Secret{{Name: CORSAllowOriginsEnv, Value: "https://app.example.com"}}, secrets)
}

func TestLoginAPIURLDerivesEUAndStagingFromHost(t *testing.T) {
	// Empty lookup, so an ambient `LANGSMITH_ENDPOINT` cannot short-circuit
	// the derivation under test.
	assert.Equal(t, "https://eu.api.smith.langchain.com", hostapi.PlatformAPIURLForHost("https://eu.api.host.langchain.com", noEnv))
	assert.Equal(t, "https://beta.api.smith.langchain.com", hostapi.PlatformAPIURLForHost("https://beta.api.host.langchain.com", noEnv))
}

func TestPlatformAPIURLPrefersProcessEnvOverDotenv(t *testing.T) {
	dotenv := map[string]string{LangsmithEndpointEnv: "https://eu.api.smith.langchain.com"}
	blankDotenv := map[string]string{LangsmithEndpointEnv: "   "}
	process := func(key string) (string, bool) {
		if key == LangsmithEndpointEnv {
			return "https://from-process-env.example.com", true
		}
		return "", false
	}

	// The process env wins over `.env` for this CLI setting (unlike deployment
	// secrets, which only ever come from `.env`).
	assert.Equal(t, "https://from-process-env.example.com", resolvePlatformAPIURLWith(DefaultHostURL, mapEnv(dotenv, process)))
	assert.Equal(t, "https://from-process-env.example.com", resolvePlatformAPIURLWith(DefaultHostURL, mapEnv(blankDotenv, process)))
	assert.Equal(t, "https://eu.api.smith.langchain.com", resolvePlatformAPIURLWith(DefaultHostURL, mapEnv(dotenv, noEnv)))
}

// apiBases is [resolveLangsmithAPIBasesWith] over envFile alone.
func apiBases(endpoint string) [2]string {
	host, platform := resolveLangsmithAPIBasesWith(mapEnv(map[string]string{LangsmithEndpointEnv: endpoint}, noEnv))
	return [2]string{host, platform}
}

func TestResolveLangsmithAPIBasesSplitsLocalNginxAndHostBackend(t *testing.T) {
	assert.Equal(t, [2]string{DefaultLocalHostBackendURL, "http://localhost:1980"}, apiBases("http://localhost:1980/"))
}

func TestResolveLangsmithAPIBasesDerivesHostFromPlatformEndpoint(t *testing.T) {
	assert.Equal(t,
		[2]string{"https://beta.api.host.langchain.com", "https://beta.api.smith.langchain.com"},
		apiBases("https://beta.api.smith.langchain.com"))
}

func TestResolveLangsmithAPIBasesDerivesSelfHostedDeploymentPath(t *testing.T) {
	assert.Equal(t,
		[2]string{"http://langsmith.localhost/api-host", "http://langsmith.localhost"},
		apiBases("http://langsmith.localhost/"))
}

func TestResolveLangsmithAPIBasesPutsLocalStackHostBackendBelowAPIHost(t *testing.T) {
	assert.Equal(t, "http://localhost:21980/api-host", apiBases("http://localhost:21980")[0])
}

func TestResolveLangsmithAPIBasesStripsSDKRestPathFromSelfHostedEndpoint(t *testing.T) {
	assert.Equal(t, "https://langsmith.example.com/api-host", apiBases("https://langsmith.example.com/api/v1/")[0])
}

func TestResolveLangsmithAPIBasesDefaultsToSaaS(t *testing.T) {
	host, platform := resolveLangsmithAPIBasesWith(noEnv)
	assert.Equal(t, [2]string{DefaultHostURL, "https://api.smith.langchain.com"}, [2]string{host, platform})
}

func TestEnsurePublicAPIURLInjectsNonLoopback(t *testing.T) {
	var secrets []hostapi.Secret
	EnsurePublicAPIURL(&secrets, "https://slack-abc.us.langgraph.app/")
	assert.Equal(t, []hostapi.Secret{{Name: MDAPublicAPIURLEnv, Value: "https://slack-abc.us.langgraph.app"}}, secrets)
}

func TestEnsurePublicAPIURLSkipsLoopbackAndExisting(t *testing.T) {
	var secrets []hostapi.Secret
	EnsurePublicAPIURL(&secrets, "http://localhost:5557")
	assert.Empty(t, secrets)
	secrets = append(secrets, hostapi.Secret{Name: MDAPublicAPIURLEnv, Value: "https://already.set"})
	EnsurePublicAPIURL(&secrets, "https://other.example.com")
	assert.Equal(t, []hostapi.Secret{{Name: MDAPublicAPIURLEnv, Value: "https://already.set"}}, secrets)
}

func TestEnsureCORSAllowOriginsPreservesExplicit(t *testing.T) {
	env := map[string]string{MDAPublicAppURLEnv: "https://app.example.com"}
	secrets := []hostapi.Secret{{Name: CORSAllowOriginsEnv, Value: "https://explicit.example.com"}}
	EnsureCORSAllowOrigins(&secrets, env)
	assert.Equal(t, []hostapi.Secret{{Name: CORSAllowOriginsEnv, Value: "https://explicit.example.com"}}, secrets)
}

// timeoutError is a net.Error that timed out, as a client deadline reports.
type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestRequestTimeoutDetectsTimeouts(t *testing.T) {
	assert.True(t, IsRequestTimeout(errors.New("POST /v2/deployments request failed: operation timed out")))
	assert.True(t, IsRequestTimeout(fmt.Errorf("POST /v2/deployments: %w", timeoutError{})))
	assert.False(t, IsRequestTimeout(errors.New("deployment not found")))
}

func TestFindDeploymentByNameMatchesExactName(t *testing.T) {
	response := map[string]any{"resources": []any{
		map[string]any{"id": "dep-1", "name": "my-agent-copy"},
		map[string]any{"id": "dep-2", "name": "my-agent"},
	}}

	assert.Equal(t, "dep-2", FindDeploymentByName(response, "my-agent")["id"])
}

func TestFindDeploymentIDReturnsNoneWhenNameIsMissing(t *testing.T) {
	response := map[string]any{"resources": []any{map[string]any{"id": "dep-1", "name": "other"}}}

	assert.Equal(t, "", FindDeploymentIDByName(response, "missing"))
}

func TestFindAgentDeploymentSkipsPreviews(t *testing.T) {
	response := map[string]any{"resources": []any{
		map[string]any{"id": "preview", "is_preview": true},
		map[string]any{"id": "deployment", "is_preview": false},
	}}

	assert.Equal(t, "deployment", FindAgentDeployment(response)["id"])
	assert.Nil(t, FindAgentDeployment(map[string]any{"resources": []any{}}))
}

func TestDeploymentIsManagedDeepAgentRequiresTrueServerMarker(t *testing.T) {
	assert.True(t, DeploymentIsManagedDeepAgent(map[string]any{"is_managed_deep_agent": true}))
}

func TestDeploymentIsManagedDeepAgentRejectsMissingServerMarker(t *testing.T) {
	assert.False(t, DeploymentIsManagedDeepAgent(map[string]any{}))
}

func TestDeploymentIsManagedDeepAgentRejectsFalseServerMarker(t *testing.T) {
	assert.False(t, DeploymentIsManagedDeepAgent(map[string]any{"is_managed_deep_agent": false}))
}

func TestDeploymentIsManagedDeepAgentAcceptsTheRequestSideSpelling(t *testing.T) {
	assert.True(t, DeploymentIsManagedDeepAgent(map[string]any{"managed_deep_agent": true}))
}

// marker is [DeploymentManagedDeepAgentMarker] as a nil, false or true pointer.
func marker(deployment map[string]any) *bool {
	value, ok := DeploymentManagedDeepAgentMarker(deployment)
	if !ok {
		return nil
	}
	return &value
}

func TestManagedDeepAgentMarkerDistinguishesAbsentFromFalse(t *testing.T) {
	assert.Nil(t, marker(map[string]any{}))
	assert.Equal(t, ptr(false), marker(map[string]any{"is_managed_deep_agent": false}))
	assert.Equal(t, ptr(true), marker(map[string]any{"is_managed_deep_agent": true}))
}

func TestManagedDeepAgentMarkerIgnoresNonBooleanValues(t *testing.T) {
	assert.Nil(t, marker(map[string]any{"is_managed_deep_agent": "true"}))
}

func TestStatusURLUsesTheAuthoritativeBYOCDashboardOrigin(t *testing.T) {
	deployment := map[string]any{"tenant_id": "workspace-1"}

	assert.Equal(t,
		"https://aws.smith.langchain.com/o/workspace-1/host/deployments/deployment-1",
		DeploymentStatusURL(deployment, "deployment-1", "https://aws.smith.langchain.com"))
}

func TestStatusURLRequiresTenant(t *testing.T) {
	assert.Equal(t,
		"https://smith.langchain.com/o/t-1/host/deployments/dep-1",
		DeploymentStatusURL(map[string]any{"tenant_id": "t-1"}, "dep-1", "https://smith.langchain.com"))
	assert.Equal(t, "", DeploymentStatusURL(map[string]any{}, "dep-1", "https://smith.langchain.com"))
	assert.Equal(t, "", DeploymentStatusURL(map[string]any{"tenant_id": "  "}, "dep-1", "https://smith.langchain.com"))
}

func TestPlanUpgradeURLPointsAtTheWorkspaceBillingPage(t *testing.T) {
	assert.Equal(t,
		"https://smith.langchain.com/o/7109fb64-c8b8-48d5-a107-4ab03748e980/settings/payments",
		PlanUpgradeURL("https://smith.langchain.com", "7109fb64-c8b8-48d5-a107-4ab03748e980"))
}

func TestPlanUpgradeURLRejectsAWorkspaceIDThatIsNotAUUID(t *testing.T) {
	// A configured `LANGSMITH_WORKSPACE_ID` reaches this unvalidated, and a link
	// is only worth printing when the id cannot reshape the path.
	for _, workspaceID := range []string{
		"",
		"not-a-uuid",
		"../../settings",
		"7109fb64-c8b8-48d5-a107-4ab03748e980/x",
		"7109fb64c8b848d5a1074ab03748e980",
		"7109fb64-c8b8-48d5-a107-4ab03748e98g",
	} {
		assert.Equal(t, "", PlanUpgradeURL("https://smith.langchain.com", workspaceID), "expected no link for %q", workspaceID)
	}
}

func TestDeploymentRuntimeURLPrefersKnownRuntimeFields(t *testing.T) {
	deployment := map[string]any{
		"deployment_url": "https://runtime.example.com/",
		"url":            "https://dashboard.example.com",
	}
	assert.Equal(t, "https://runtime.example.com", DeploymentRuntimeURL(deployment))
}

func TestDeploymentRuntimeURLReadsNestedCustomURL(t *testing.T) {
	deployment := map[string]any{"source_config": map[string]any{"custom_url": "https://custom.example.com/"}}
	assert.Equal(t, "https://custom.example.com", DeploymentRuntimeURL(deployment))
}

func TestContextHubURLMapsFromHostBackendURL(t *testing.T) {
	for host, want := range map[string]string{
		"https://api.host.langchain.com":    "https://api.smith.langchain.com",
		"https://eu.api.host.langchain.com": "https://eu.api.smith.langchain.com",
		"http://localhost:8123":             "http://localhost:8123",
		"http://localhost:8300":             "http://localhost:1980",
		"http://127.0.0.1:8300":             "http://127.0.0.1:1980",
		"https://custom.example.com":        "https://custom.example.com",
	} {
		assert.Equal(t, want, ContextHubBaseURL(host), host)
	}
}

func TestContextHubURLUsesPlatformForSelfHostedDeploymentPath(t *testing.T) {
	assert.Equal(t, "http://langsmith.localhost",
		ResolveContextHubBaseURL("http://langsmith.localhost/api-host", "http://langsmith.localhost"))
}

func TestValidatedAgentServerURLAcceptsAbsoluteHTTPS(t *testing.T) {
	u, err := ValidatedAgentServerURL("https://agent.example.com/base")
	require.NoError(t, err)
	assert.Equal(t, "https://agent.example.com/base", u.String())
}

func TestValidatedAgentServerURLRejectsCredentialsQueryFragmentAndHTTP(t *testing.T) {
	for _, raw := range []string{
		"http://agent.example.com",
		"https://user@agent.example.com",
		"https://:secret@agent.example.com",
		"https://agent.example.com?x=1",
		"https://agent.example.com?",
		"https://agent.example.com#frag",
		"https://agent.example.com#",
		"https:opaque",
		"/relative",
	} {
		_, err := ValidatedAgentServerURL(raw)
		assert.EqualError(t, err, "Agent Server URL must be absolute HTTPS without credentials, query, or fragment", raw)
	}
}
