package deployment

// Pure helpers for the `deploy` command, ported from the langgraph CLI's
// `deploy-helpers.mts` (`deploy/helpers.rs`). Kept free of I/O so they can be
// unit-tested.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
)

// ValidatedAgentServerURL is the Agent Server URL channel webhooks point at:
// absolute HTTPS without credentials, query, or fragment.
func ValidatedAgentServerURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("Agent Server URL is invalid: %w", err)
	}
	hasCredentials := u.User != nil && (u.User.Username() != "" || hasPassword(u.User))
	hasQuery := u.RawQuery != "" || u.ForceQuery
	hasFragment := strings.Contains(raw, "#")
	if u.Scheme != "https" || u.Host == "" || hasCredentials || hasQuery || hasFragment || u.Opaque != "" {
		return nil, errors.New("Agent Server URL must be absolute HTTPS without credentials, query, or fragment")
	}
	return u, nil
}

func hasPassword(user *url.Userinfo) bool {
	_, set := user.Password()
	return set
}

// DefaultHostURL is the default host backend API URL.
const DefaultHostURL = "https://api.host.langchain.com"

// DefaultLocalHostBackendURL is the local langchainplus host-backend
// (Deployments API `/v2/deployments`).
const DefaultLocalHostBackendURL = "http://localhost:8300"

// LangsmithEndpointEnv is the env var for the LangSmith platform API (Fleet,
// login, secrets).
//
// Also selects the matching host backend for SaaS (`api.smith` ↔ `api.host`)
// and the local nginx→`:8300` split when set to `http://localhost:1980`.
const LangsmithEndpointEnv = "LANGSMITH_ENDPOINT"

// WorkspaceIDEnv is the env var that scopes deploy + runtime LangSmith clients
// to a workspace.
const WorkspaceIDEnv = "LANGSMITH_WORKSPACE_ID"

// MDASandboxAPIEndpointEnv is the deploy-owned full endpoint for the LangSmith
// managed-sandbox API.
const MDASandboxAPIEndpointEnv = "MDA_SANDBOX_API_ENDPOINT"

// RemovedWorkspaceIDEnvs are removed env names that previously meant the same
// id as [WorkspaceIDEnv].
var RemovedWorkspaceIDEnvs = []string{
	"LANGCHAIN_WORKSPACE_ID",
	"LANGSMITH_TENANT_ID",
	"LANGCHAIN_TENANT_ID",
}

// IsTerminalStatus reports whether a revision status ends status polling
// (success, failure, or external interruption).
func IsTerminalStatus(status string) bool {
	switch status {
	case "DEPLOYED", "CREATE_FAILED", "BUILD_FAILED", "DEPLOY_FAILED", "SKIPPED", "INTERRUPTED":
		return true
	}
	return false
}

// IsFailedStatus reports whether a status indicates the deployment failed.
func IsFailedStatus(status string) bool {
	switch status {
	case "CREATE_FAILED", "BUILD_FAILED", "DEPLOY_FAILED":
		return true
	}
	return false
}

// IsInterruptedStatus reports whether a revision was interrupted/cancelled
// from another interface (e.g. the LangSmith UI) rather than building to a
// success or failure here.
func IsInterruptedStatus(status string) bool {
	return status == "INTERRUPTED"
}

// ReservedEnvVars are env vars that must never be forwarded as deployment
// secrets (auth, runtime wiring, reserved platform configuration). Mirrors
// `RESERVED_ENV_VARS`.
var ReservedEnvVars = []string{
	hostapi.BuildArtifactsTokenEnv,
	"LANGCHAIN_TRACING_V2",
	"LANGSMITH_TRACING_V2",
	"LANGCHAIN_ENDPOINT",
	"LANGCHAIN_PROJECT",
	"LANGSMITH_PROJECT",
	"LANGSMITH_LANGGRAPH_GIT_REPO",
	"LANGGRAPH_GIT_REPO_PATH",
	"LANGCHAIN_API_KEY",
	"LANGCHAIN_WORKSPACE_ID",
	"LANGGRAPH_HOST_API_KEY",
	"LANGSMITH_CONTROL_PLANE_API_KEY",
	"POSTGRES_URI",
	"POSTGRES_PASSWORD",
	"DATABASE_URI",
	"LANGSMITH_LANGGRAPH_GIT_REF",
	"LANGSMITH_LANGGRAPH_GIT_REF_SHA",
	"LANGGRAPH_AUTH_TYPE",
	"LANGSMITH_AUTH_ENDPOINT",
	"LANGSMITH_TENANT_ID",
	"LANGSMITH_AUTH_VERIFY_TENANT_ID",
	"LANGSMITH_HOST_PROJECT_ID",
	"LANGSMITH_HOST_PROJECT_NAME",
	"LANGSMITH_HOST_REVISION_ID",
	"LOG_JSON",
	"LOG_DICT_TRACEBACKS",
	"REDIS_URI",
	"LANGCHAIN_CALLBACKS_BACKGROUND",
	"DD_TRACE_PSYCOPG_ENABLED",
	"DD_TRACE_REDIS_ENABLED",
	"LANGSMITH_DEPLOYMENT_NAME",
	"LANGGRAPH_CLOUD_LICENSE_KEY",
	"LANGSMITH_API_KEY",
	"LANGSMITH_ENDPOINT",
	"POSTGRES_URI_CUSTOM",
	"REDIS_URI_CUSTOM",
	"PATH",
	"PORT",
	"MOUNT_PREFIX",
	"LSD_ENV",
	"LSD_DD_API_KEY",
	"LSD_DD_ENDPOINT",
	"LSD_DEPLOYMENT_TYPE",
	// Deploy-owned, per-revision credential for internal MDA loopback calls.
	"MDA_HANDOFF_SECRET",
	// Deploy-owned: the runtime names its sandboxes after this, and `mda delete`
	// looks them up by the real deployment name.
	"MDA_DEPLOYMENT_NAME",
	// Deploy-owned: baked recipe snapshot name the runtime clones for new threads.
	"MDA_SANDBOX_RECIPE_SNAPSHOT",
	// Deploy-owned: runtime sandbox calls must use the same platform API surface
	// as deploy preflight, rather than deriving one from the tracing endpoint.
	"MDA_SANDBOX_API_ENDPOINT",
	// CLI-only: point channel app setup at another Agent Auth. The key belongs
	// to that LangSmith, never to the deployment.
	"MDA_AGENT_AUTH_URL",
	"MDA_AGENT_AUTH_API_KEY",
	"MDA_AGENT_AUTH_TENANT_ID",
	// CLI-only: Agent Auth's OAuth callback, which a new Linear app lists, when
	// the CLI cannot derive it (the local stack sets it).
	"MDA_AGENT_AUTH_OAUTH_CALLBACK_URL",
	// CLI-only: where a new GitHub App is created, or an existing App with its
	// webhook secret and installation, without a prompt. Agent Auth keeps the
	// App; the deployment never gets them as env vars (`GH_WEBHOOK_SECRET` is
	// the runtime's own override and is uploaded).
	"MDA_GITHUB_APP_OWNER",
	"MDA_GITHUB_APP_ID",
	"MDA_GITHUB_APP_PRIVATE_KEY_FILE",
	"MDA_GITHUB_APP_INSTALLATION_ID",
	"MDA_GITHUB_WEBHOOK_SECRET",
	// CLI-only: a Linear app's credentials instead of the prompts. Agent Auth
	// keeps them; the deployment never gets them as env vars.
	"MDA_LINEAR_CLIENT_ID",
	"MDA_LINEAR_CLIENT_SECRET",
	"MDA_LINEAR_WEBHOOK_SECRET",
	"MDA_LINEAR_WORKSPACE",
	// CLI-only: a brought GitHub App's webhook URL that stays where it is.
	"MDA_GITHUB_WEBHOOK_ELSEWHERE",
	// CLI-only: the user's own Slack app's credentials instead of the prompts.
	// Trigger Server keeps them; the deployment never gets them as env vars.
	"MDA_SLACK_BOT_TOKEN",
	"MDA_SLACK_SIGNING_SECRET",
}

// SecretsFromEnv converts env vars into deployment secrets, dropping reserved
// and empty values. Secrets are ordered by name.
func SecretsFromEnv(envVars map[string]string) []hostapi.Secret {
	var secrets []hostapi.Secret
	for _, name := range sortedKeys(envVars) {
		value := envVars[name]
		if value == "" || slices.Contains(ReservedEnvVars, name) {
			continue
		}
		secrets = append(secrets, hostapi.Secret{Name: name, Value: value})
	}
	return secrets
}

// MDAHandoffSecretEnv is the deploy-owned credential for internal MDA loopback
// calls.
const MDAHandoffSecretEnv = "MDA_HANDOFF_SECRET"

// SetMDAHandoffSecret injects a generated internal handoff credential,
// replacing any stale value.
func SetMDAHandoffSecret(secrets *[]hostapi.Secret, value string) {
	setSecretInPlace(secrets, MDAHandoffSecretEnv, value)
}

// SetWorkspaceIDSecret sets `LANGSMITH_WORKSPACE_ID` on the deployment to the
// resolved workspace id.
//
// The runtime must be scoped to the same workspace the deployment is created
// in, so the resolved id replaces any value carried over from `.env` —
// otherwise `--workspace-id` would move the deployment while leaving runtime
// sandbox / SDK calls pointed at the old workspace.
func SetWorkspaceIDSecret(secrets *[]hostapi.Secret, workspaceID string) {
	if workspaceID == "" {
		return
	}
	setSecretInPlace(secrets, WorkspaceIDEnv, workspaceID)
}

// setSecretInPlace replaces the value of the first secret named name, keeping
// its position, or appends one.
func setSecretInPlace(secrets *[]hostapi.Secret, name, value string) {
	for i := range *secrets {
		if (*secrets)[i].Name == name {
			(*secrets)[i].Value = value
			return
		}
	}
	*secrets = append(*secrets, hostapi.Secret{Name: name, Value: value})
}

// replaceSecret drops every secret named name and appends one with value.
func replaceSecret(secrets *[]hostapi.Secret, name, value string) {
	removeSecret(secrets, name)
	*secrets = append(*secrets, hostapi.Secret{Name: name, Value: value})
}

// removeSecret drops every secret named name.
func removeSecret(secrets *[]hostapi.Secret, name string) {
	*secrets = slices.DeleteFunc(*secrets, func(secret hostapi.Secret) bool {
		return secret.Name == name
	})
}

// hasSecret reports whether a secret named name is present.
func hasSecret(secrets []hostapi.Secret, name string) bool {
	return slices.ContainsFunc(secrets, func(secret hostapi.Secret) bool {
		return secret.Name == name
	})
}

// EnsureSandboxAPIEndpointSecret points runtime sandbox clients at the same
// platform API used by deploy.
func EnsureSandboxAPIEndpointSecret(secrets *[]hostapi.Secret, platformAPIURL string) {
	value := SandboxAPIEndpoint(platformAPIURL)
	if value == "" {
		return
	}
	replaceSecret(secrets, MDASandboxAPIEndpointEnv, value)
}

// SandboxAPIEndpoint builds the full managed-sandbox endpoint from a resolved
// platform API URL, "" when the URL is blank.
func SandboxAPIEndpoint(platformAPIURL string) string {
	platformAPIURL = strings.TrimRight(strings.TrimSpace(platformAPIURL), "/")
	if platformAPIURL == "" {
		return ""
	}
	return platformAPIURL + "/v2/sandboxes"
}

// MDADeploymentNameEnv is the deploy-time deployment name handed to the
// runtime for resource attribution.
const MDADeploymentNameEnv = "MDA_DEPLOYMENT_NAME"

// MDASandboxRecipeSnapshotEnv is the deploy-owned recipe snapshot name the
// runtime clones for new thread sandboxes.
const MDASandboxRecipeSnapshotEnv = "MDA_SANDBOX_RECIPE_SNAPSHOT"

// RecipeSetupSegment is the segment between the deployment name and the recipe
// digest in a baked snapshot name (`{deployment}--setup-{digest}`).
const RecipeSetupSegment = "setup-"

// RecipeDigestLength is the number of hex characters kept from the SHA-256 of
// setup bytes + bake bases.
const RecipeDigestLength = 16

// MDAAssistantIDEnv is the LangGraph graphs / assistant id (`langgraph.json`
// graphs key). Channel ingress (Slack, GitHub) reads this so runs target the
// compiled graph instead of the hardcoded `"agent"` fallback.
const MDAAssistantIDEnv = "MDA_ASSISTANT_ID"

// MDALangsmithURLEnv is the LangSmith UI of the environment the deployment
// runs in, for links that channels post into LangSmith Studio.
const MDALangsmithURLEnv = "MDA_LANGSMITH_URL"

// SandboxNameSeparator is the separator between the deployment name and the
// identity digest in a managed sandbox name.
//
// Doubled so a deployment name can never be a prefix-match for another one:
// `agent--<digest>` is not matched by the `agent-staging--` prefix. Mirrors the
// `--u--` / `--t--` convention used for Context Hub child repos.
const SandboxNameSeparator = "--"

// EnsureDeploymentNameSecret injects `MDA_DEPLOYMENT_NAME` so the runtime can
// name the sandboxes it creates after the deployment that owns them.
//
// The platform reserves `LANGSMITH_DEPLOYMENT_NAME`, but nothing guarantees it
// is present in the container, and it is not forwarded from `.env`. Passing the
// deploy-time name explicitly is what lets `mda delete` find those sandboxes
// again.
//
// Deploy owns this value and overwrites anything already in secrets. A stale
// or copied `.env` entry would otherwise prefix this deployment's sandboxes
// with another deployment's name, putting them out of reach of its own
// `mda delete` and inside the blast radius of the other one's.
func EnsureDeploymentNameSecret(secrets *[]hostapi.Secret, name string) {
	if name == "" {
		return
	}
	replaceSecret(secrets, MDADeploymentNameEnv, name)
}

// EnsureRecipeSnapshotSecret injects [MDASandboxRecipeSnapshotEnv] so the
// runtime clones the baked recipe for new threads.
//
// Same ownership rule as [EnsureDeploymentNameSecret]: deploy/dev owns the
// value and overwrites a stale `.env` copy.
func EnsureRecipeSnapshotSecret(secrets *[]hostapi.Secret, recipeName string) {
	if recipeName == "" {
		return
	}
	replaceSecret(secrets, MDASandboxRecipeSnapshotEnv, recipeName)
}

// ClearRecipeSnapshotSecret drops a previously injected recipe secret when the
// project has no sandbox.
func ClearRecipeSnapshotSecret(secrets *[]hostapi.Secret) {
	removeSecret(secrets, MDASandboxRecipeSnapshotEnv)
}

// HostSandboxNameLimit is Host's max length for sandbox and snapshot names.
const HostSandboxNameLimit = 63

// maxRecipeDeploymentComponentLength is the deployment component length that
// keeps `{component}--setup-{digest}` ≤ 63.
const maxRecipeDeploymentComponentLength = HostSandboxNameLimit -
	len(SandboxNameSeparator) -
	len(RecipeSetupSegment) -
	RecipeDigestLength

// RecipeSnapshotName is the deterministic recipe snapshot name for a
// deployment + digest.
//
// Long deployment names are shortened the same way thread sandbox names are,
// so `{component}--setup-{16}` stays within Host's 63-character limit.
func RecipeSnapshotName(deployment, digest string) string {
	return RecipeSnapshotPrefix(deployment) + digest
}

// RecipeSnapshotPrefix is the prefix shared by every recipe snapshot a
// deployment owns.
func RecipeSnapshotPrefix(deployment string) string {
	return SandboxDeploymentComponent(deployment, maxRecipeDeploymentComponentLength) +
		SandboxNameSeparator + RecipeSetupSegment
}

// RecipeSnapshotReference is how the runtime must reference the baked recipe,
// given the tags Host assigned.
//
// Host resolves a bare snapshot name as `{name}:latest`. A snapshot built from
// a Docker image inherits the *image's* tag instead (`…--setup-abc` tagged
// `0.1.1`), so staging the bare name makes every thread create fail with
// `snapshot "…:latest" not found`. Pin the assigned tag whenever `latest` is
// absent; a captured snapshot that already carries `latest` keeps the bare
// name.
func RecipeSnapshotReference(name string, tags []string) string {
	if slices.Contains(tags, "latest") {
		return name
	}
	for _, tag := range tags {
		if strings.TrimSpace(tag) != "" {
			return name + ":" + tag
		}
	}
	return name
}

// SandboxRegistryName is the deployment-owned Host registry name for private
// Docker pulls.
func SandboxRegistryName(deployment string) string {
	return deployment + SandboxNameSeparator + "registry"
}

// RecipeDigest is the hex digest (truncated to [RecipeDigestLength]) of setup
// bytes and optional bake bases. Same inputs always produce the same digest.
//
// registry and fsCapacityBytes are only hashed when present so existing
// digests stay stable for projects that do not set them.
func RecipeDigest(setupBytes []byte, snapshotName, snapshotID, dockerImage, registry *string, fsCapacityBytes *uint64) string {
	hasher := sha256.New()
	hasher.Write(setupBytes)
	hasher.Write([]byte{0})
	hasher.Write([]byte(deref(snapshotName)))
	hasher.Write([]byte{0})
	hasher.Write([]byte(deref(snapshotID)))
	hasher.Write([]byte{0})
	hasher.Write([]byte(deref(dockerImage)))
	if registry != nil {
		hasher.Write([]byte{0})
		hasher.Write([]byte(*registry))
	}
	if fsCapacityBytes != nil {
		hasher.Write([]byte("\x00fs_capacity_bytes="))
		hasher.Write([]byte(strconv.FormatUint(*fsCapacityBytes, 10)))
	}
	return hex.EncodeToString(hasher.Sum(nil))[:RecipeDigestLength]
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// IsDeploymentRecipeSnapshot reports whether name is a recipe snapshot owned
// by deployment.
//
// Requires the `{deployment}--setup-` prefix and a non-empty digest suffix so a
// neighbour deployment whose name merely shares a prefix is not swept up.
func IsDeploymentRecipeSnapshot(name, deployment string) bool {
	digest, ok := strings.CutPrefix(name, RecipeSnapshotPrefix(deployment))
	if !ok {
		return false
	}
	return digest != "" && !strings.Contains(digest, SandboxNameSeparator)
}

// IntermediateSnapshotKinds are the temporary base snapshots a bake creates
// before `setup.sh` runs: `dockerbase` (built from `dockerImage`) and
// `sourcebase` (captured from an existing sandbox).
var IntermediateSnapshotKinds = [2]string{"dockerbase", "sourcebase"}

// IntermediateSnapshotName is the name of a temporary bake base snapshot
// (`{deployment}--{kind}-{digest}`).
//
// Long deployment names are shortened like recipe names so the result stays
// within Host's 63-character limit and remains attributable to one deployment.
func IntermediateSnapshotName(deployment, kind, digest string) string {
	return intermediateSnapshotPrefix(deployment, kind) + digest
}

func intermediateSnapshotPrefix(deployment, kind string) string {
	maxComponent := HostSandboxNameLimit -
		len(SandboxNameSeparator) -
		len(kind) -
		1 -
		RecipeDigestLength
	return SandboxDeploymentComponent(deployment, maxComponent) + SandboxNameSeparator + kind + "-"
}

// IsDeploymentIntermediateSnapshot reports whether name is a temporary bake
// base snapshot owned by deployment.
//
// Requires an exact recipe digest after the prefix so a neighbour deployment
// whose name merely shares a prefix is not swept up.
func IsDeploymentIntermediateSnapshot(name, deployment string) bool {
	for _, kind := range IntermediateSnapshotKinds {
		if digest, ok := strings.CutPrefix(name, intermediateSnapshotPrefix(deployment, kind)); ok && isRecipeDigest(digest) {
			return true
		}
	}
	return false
}

func isRecipeDigest(value string) bool {
	if len(value) != RecipeDigestLength {
		return false
	}
	for i := 0; i < len(value); i++ {
		if !isLowerHexByte(value[i]) {
			return false
		}
	}
	return true
}

func isLowerHexByte(b byte) bool {
	return ('0' <= b && b <= '9') || ('a' <= b && b <= 'f')
}

// IsDeploymentBakeSnapshot reports whether name is any bake snapshot (recipe
// or temporary base) owned by deployment.
func IsDeploymentBakeSnapshot(name, deployment string) bool {
	return IsDeploymentRecipeSnapshot(name, deployment) ||
		IsDeploymentIntermediateSnapshot(name, deployment)
}

// BakeSnapshotListPrefix is the prefix that lists every bake snapshot
// deployment owns in one request.
//
// Neighbour deployments can share it; filter results with
// [IsDeploymentBakeSnapshot].
func BakeSnapshotListPrefix(deployment string) string {
	shared := RecipeSnapshotPrefix(deployment)
	for _, kind := range IntermediateSnapshotKinds {
		shared = commonPrefix(shared, intermediateSnapshotPrefix(deployment, kind))
	}
	return shared
}

// commonPrefix is the longest common prefix of left and right, in whole
// characters.
func commonPrefix(left, right string) string {
	end := 0
	for end < len(left) && end < len(right) {
		l, size := utf8.DecodeRuneInString(left[end:])
		r, _ := utf8.DecodeRuneInString(right[end:])
		if l != r {
			break
		}
		end += size
	}
	return left[:end]
}

// EnsureAssistantIDSecret injects [MDAAssistantIDEnv] so channel ingress
// invokes the compiled graph.
//
// Same ownership rule as [EnsureDeploymentNameSecret]: deploy/dev owns the
// value and overwrites a stale `.env` copy.
func EnsureAssistantIDSecret(secrets *[]hostapi.Secret, assistantID string) {
	if assistantID == "" {
		return
	}
	replaceSecret(secrets, MDAAssistantIDEnv, assistantID)
}

// EnsureLangsmithURLSecret injects [MDALangsmithURLEnv] so channels link to the
// deployment's own LangSmith environment, the same one deploy prints as the
// dashboard.
//
// Same ownership rule as [EnsureDeploymentNameSecret]: deploy owns the value
// and overwrites a stale `.env` copy.
func EnsureLangsmithURLSecret(secrets *[]hostapi.Secret, langsmithURL string) {
	if langsmithURL == "" {
		return
	}
	replaceSecret(secrets, MDALangsmithURLEnv, langsmithURL)
}

// SandboxNamePrefix is the prefix shared by every managed sandbox a deployment
// creates.
//
// This is the stable primary-name prefix. Deployments whose full primary name
// fits Host's limit keep this prefix across runtime upgrades.
func SandboxNamePrefix(deploymentName string) string {
	return SandboxDeploymentComponent(deploymentName, maxPrimaryDeploymentComponentLength) + SandboxNameSeparator
}

// SandboxNamePrefixes are the prefixes `mda delete` must query for
// primary and shortened fallback names.
//
// Collision fallback names reserve nine characters for `-<8 hex>`, so their
// deployment component may be shorter than the stable primary component.
func SandboxNamePrefixes(deploymentName string) []string {
	primary := SandboxNamePrefix(deploymentName)
	fallback := SandboxDeploymentComponent(deploymentName, maxFallbackDeploymentComponentLength) + SandboxNameSeparator
	if fallback == primary {
		return []string{primary}
	}
	return []string{primary, fallback}
}

// sandboxDigestLength is the number of hex characters in the identity digest
// the runtimes append to a sandbox name.
const sandboxDigestLength = 12

// fallbackSuffixLength is the number of hex characters in the
// collision-fallback suffix the runtimes add when a deterministic sandbox name
// is already taken.
const fallbackSuffixLength = 8

// Deployment component lengths that keep primary and fallback names within
// Host's 63-character limit.
const (
	maxPrimaryDeploymentComponentLength  = 49
	maxFallbackDeploymentComponentLength = 40
)

// sandboxDeploymentDigestLength is the number of hex characters used to
// distinguish truncated deployment names.
const sandboxDeploymentDigestLength = 8

// SandboxDeploymentComponent is deploymentName when it fits maxLength
// characters, else its first characters, a hyphen and a digest of the whole
// name, maxLength characters in all.
func SandboxDeploymentComponent(deploymentName string, maxLength int) string {
	if utf8.RuneCountInString(deploymentName) <= maxLength {
		return deploymentName
	}
	digest := sha256.Sum256([]byte(deploymentName))
	fullDigest := hex.EncodeToString(digest[:])
	prefixLength := maxLength - sandboxDeploymentDigestLength - 1
	runes := []rune(deploymentName)
	return string(runes[:prefixLength]) + "-" + fullDigest[:sandboxDeploymentDigestLength]
}

// IsDeploymentSandbox reports whether sandboxName is one this deployment's
// runtime created.
//
// The prefix alone is not conclusive: a deployment may itself be named with a
// `--` in it, so `my-agent--` would otherwise sweep up sandboxes belonging to a
// separate `my-agent--foo` deployment. Requiring the remainder to be exactly
// the digest the runtimes generate (optionally plus their collision-fallback
// suffix) keeps teardown from deleting a neighbour's sandbox.
func IsDeploymentSandbox(sandboxName, deploymentName string) bool {
	for _, prefix := range SandboxNamePrefixes(deploymentName) {
		if suffix, ok := strings.CutPrefix(sandboxName, prefix); ok && isManagedSandboxSuffix(suffix) {
			return true
		}
	}
	return false
}

func isManagedSandboxSuffix(suffix string) bool {
	digest, fallback, hasFallback := strings.Cut(suffix, "-")
	if len(digest) != sandboxDigestLength || !isLowercaseHex(digest) {
		return false
	}
	if !hasFallback {
		return true
	}
	return len(fallback) == fallbackSuffixLength && isLowercaseHex(fallback)
}

func isLowercaseHex(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if !isLowerHexByte(value[i]) {
			return false
		}
	}
	return true
}

// CORSAllowOriginsEnv is the LangGraph Agent Server CORS allowlist (exact
// origins, comma-separated).
const CORSAllowOriginsEnv = "CORS_ALLOW_ORIGINS"

// MDAPublicAppURLEnv is the public browser origin used for Connect redirects
// and CORS derivation.
const MDAPublicAppURLEnv = "MDA_PUBLIC_APP_URL"

// MDAPublicAPIURLEnv is the public Agent Server base URL used for Slack OAuth
// `redirect_uri`.
const MDAPublicAPIURLEnv = "MDA_PUBLIC_API_URL"

// PublicAPIURLKind is the outcome of reading `MDA_PUBLIC_API_URL` for Events /
// Connect URLs.
type PublicAPIURLKind int

// The outcomes.
const (
	// PublicAPIURLHTTPS: an `https://` URL, in [PublicAPIURL.URL].
	PublicAPIURLHTTPS PublicAPIURLKind = iota
	// PublicAPIURLMissing: unset or blank.
	PublicAPIURLMissing
	// PublicAPIURLNotHTTPS: set, but not `https://`; the value is in
	// [PublicAPIURL.URL].
	PublicAPIURLNotHTTPS
)

// PublicAPIURL is the outcome of reading `MDA_PUBLIC_API_URL`.
type PublicAPIURL struct {
	Kind PublicAPIURLKind
	// URL is the trimmed value, without trailing slashes; empty for
	// [PublicAPIURLMissing].
	URL string
}

// ResolveHTTPSPublicAPIURL reads and classifies `MDA_PUBLIC_API_URL` from a
// merged env map.
func ResolveHTTPSPublicAPIURL(envVars map[string]string) PublicAPIURL {
	raw := strings.TrimRight(strings.TrimSpace(envVars[MDAPublicAPIURLEnv]), "/")
	if raw == "" {
		return PublicAPIURL{Kind: PublicAPIURLMissing}
	}
	if strings.HasPrefix(raw, "https://") {
		return PublicAPIURL{Kind: PublicAPIURLHTTPS, URL: raw}
	}
	return PublicAPIURL{Kind: PublicAPIURLNotHTTPS, URL: raw}
}

// envLookup finds a non-empty value for an env var.
type envLookup func(key string) (string, bool)

// processEnvLookup is [hostapi.LookupEnv] over envVars with the process env
// first.
func processEnvLookup(envVars map[string]string) envLookup {
	return func(key string) (string, bool) {
		return hostapi.LookupEnv(key, envVars, os.LookupEnv)
	}
}

// ResolveLangsmithAPIBases resolves host-backend and platform (Fleet) API
// bases from `LANGSMITH_ENDPOINT`, read from the process env, then envVars.
//
// One env var is enough for every SaaS environment: set the platform API
// (`https://eu.api.smith.langchain.com`, `https://beta.api.smith.langchain.com`,
// …) and the matching host backend is derived (`…api.host…`).
//
// Local LangSmith serves Deployments on host-backend `:8300` and Fleet on
// nginx `:1980`. `LANGSMITH_ENDPOINT=http://localhost:1980` therefore pairs
// with [DefaultLocalHostBackendURL].
//
// Self-hosted LangSmith serves the platform API at the configured origin and
// the Deployment API below `/api-host` on that origin.
func ResolveLangsmithAPIBases(envVars map[string]string) (hostURL, platformAPIURL string) {
	return resolveLangsmithAPIBasesWith(processEnvLookup(envVars))
}

func resolveLangsmithAPIBasesWith(env envLookup) (hostURL, platformAPIURL string) {
	endpoint, ok := env(LangsmithEndpointEnv)
	if !ok {
		host := DefaultHostURL
		platform, derived := hostapi.LangsmithAPIBaseURL(host)
		if !derived {
			platform = host
		}
		return host, platform
	}
	endpoint = strings.TrimRight(endpoint, "/")
	switch {
	case isLocalLangsmithNginx(endpoint):
		return DefaultLocalHostBackendURL, endpoint
	case strings.Contains(endpoint, "host.langchain.com"):
		// Allow pointing at the host backend directly; derive platform.
		platform, derived := hostapi.LangsmithAPIBaseURL(endpoint)
		if !derived {
			platform = endpoint
		}
		return endpoint, platform
	default:
		host, derived := hostapi.LangsmithHostBackendURL(endpoint)
		if !derived {
			host, derived = hostapi.SelfHostedHostBackendURL(endpoint)
		}
		if !derived {
			host = endpoint
		}
		return host, endpoint
	}
}

// ResolvePlatformAPIURL is the platform API origin for device login and Fleet.
//
// Prefers process env then project `.env` for `LANGSMITH_ENDPOINT`, then
// derives from the host backend (`*.api.host.langchain.com` →
// `*.api.smith.langchain.com`).
func ResolvePlatformAPIURL(hostURL string, envVars map[string]string) string {
	return resolvePlatformAPIURLWith(hostURL, processEnvLookup(envVars))
}

func resolvePlatformAPIURLWith(hostURL string, env envLookup) string {
	return hostapi.ResolveLoginAPIURL(hostURL, env)
}

// isLocalLangsmithNginx reports whether endpoint is the local `poe run-auth`
// nginx that fronts Fleet but not `/v2/deployments`.
func isLocalLangsmithNginx(endpoint string) bool {
	parts := hostapi.ParseHostURL(endpoint)
	switch parts.Hostname {
	case "localhost", "127.0.0.1", "::1", "[::1]":
	default:
		return false
	}
	// ParseHostURL strips the port from hostname; recover it from the authority.
	authority := strings.TrimPrefix(parts.Trimmed, parts.Scheme+"://")
	authority, _, _ = strings.Cut(authority, "/")
	return strings.HasSuffix(authority, ":1980")
}

// MDACORSOriginsEnv holds optional comma-separated extra browser origins for
// CORS.
const MDACORSOriginsEnv = "MDA_CORS_ORIGINS"

// EnsurePublicAPIURL injects `MDA_PUBLIC_API_URL` from the known Agent Server
// runtime URL.
//
// Slack OAuth `redirect_uri` must be the public deployment URL; the ASGI
// request origin on LangGraph Host is often an internal loopback. Skips when
// the secret is already present or runtimeURL is empty/loopback.
func EnsurePublicAPIURL(secrets *[]hostapi.Secret, runtimeURL string) {
	if hasSecret(*secrets, MDAPublicAPIURLEnv) {
		return
	}
	trimmed := strings.TrimRight(strings.TrimSpace(runtimeURL), "/")
	if trimmed == "" || isLoopbackHTTPURL(trimmed) {
		return
	}
	*secrets = append(*secrets, hostapi.Secret{Name: MDAPublicAPIURLEnv, Value: trimmed})
}

func isLoopbackHTTPURL(raw string) bool {
	rest, ok := strings.CutPrefix(raw, "https://")
	if !ok {
		rest = strings.TrimPrefix(raw, "http://")
	}
	host, _, _ := strings.Cut(rest, "/")
	hostname, _, _ := strings.Cut(host, ":")
	switch hostname {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

// EnsureCORSAllowOrigins injects `CORS_ALLOW_ORIGINS` from
// `MDA_PUBLIC_APP_URL` / `MDA_CORS_ORIGINS`.
//
// LangGraph owns browser CORS for the whole Agent Server (including `/ok` and
// custom `http.app` routes). The managed HTTP app must not add a second
// `Access-Control-Allow-Origin`. Does nothing when `CORS_ALLOW_ORIGINS` is
// already present or no public origins are configured.
func EnsureCORSAllowOrigins(secrets *[]hostapi.Secret, envVars map[string]string) {
	if hasSecret(*secrets, CORSAllowOriginsEnv) {
		return
	}
	origins := ResolveCORSAllowOrigins(envVars)
	if len(origins) == 0 {
		return
	}
	*secrets = append(*secrets, hostapi.Secret{Name: CORSAllowOriginsEnv, Value: strings.Join(origins, ",")})
}

// ResolveCORSAllowOrigins builds the exact-origin CORS allowlist for LangGraph
// `CORS_ALLOW_ORIGINS`, sorted and without duplicates.
//
// Origins are taken verbatim from `MDA_PUBLIC_APP_URL` and `MDA_CORS_ORIGINS`
// (trailing slashes stripped). Callers must list every browser origin that
// should be allowed — no host-specific expansion.
func ResolveCORSAllowOrigins(envVars map[string]string) []string {
	var origins []string
	add := func(value string) {
		origin := strings.TrimRight(strings.TrimSpace(value), "/")
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	if publicApp, ok := envVars[MDAPublicAppURLEnv]; ok {
		add(publicApp)
	}
	if extra, ok := envVars[MDACORSOriginsEnv]; ok {
		for part := range strings.SplitSeq(extra, ",") {
			add(part)
		}
	}
	slices.Sort(origins)
	return slices.Compact(origins)
}

// IsRequestTimeout reports whether an error chain represents an HTTP client
// timeout.
func IsRequestTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return strings.Contains(err.Error(), "operation timed out")
}

// FindDeploymentByName finds a deployment by exact name match within a
// list-deployments response; nil when none matches.
func FindDeploymentByName(response map[string]any, name string) map[string]any {
	for _, deployment := range resources(response) {
		if deploymentName, ok := deployment["name"].(string); ok && deploymentName == name {
			return deployment
		}
	}
	return nil
}

// resources are the objects in a list response's `resources` array.
func resources(response map[string]any) []map[string]any {
	items, _ := response["resources"].([]any)
	objects := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if object, ok := item.(map[string]any); ok {
			objects = append(objects, object)
		}
	}
	return objects
}

// DeploymentManagedDeepAgentMarker reads the server-owned Managed Deep Agent
// marker off a deployment object; ok is false when the object carries none.
//
// An absent marker is not the same as "not a Managed Deep Agent": list
// responses are trimmed, so a marker absent here may still be present on the
// full object from `GET /v2/deployments/{id}`. Callers that use the marker to
// decide whether overwriting is safe must treat an absent marker as unknown
// and re-read the full object rather than assuming false.
//
// Both spellings are accepted: `create_deployment` sends the flag as
// `managed_deep_agent`, and the host backend is expected to surface the
// persisted marker as `is_managed_deep_agent`.
func DeploymentManagedDeepAgentMarker(deployment map[string]any) (marker, ok bool) {
	for _, key := range []string{"is_managed_deep_agent", "managed_deep_agent"} {
		if value, isBool := deployment[key].(bool); isBool {
			return value, true
		}
	}
	return false, false
}

// DeploymentIsManagedDeepAgent reports whether a deployment is a known Managed
// Deep Agent. An absent marker is not a claim of ownership, so it reads as
// false; see [DeploymentManagedDeepAgentMarker] when the distinction matters.
func DeploymentIsManagedDeepAgent(deployment map[string]any) bool {
	marker, ok := DeploymentManagedDeepAgentMarker(deployment)
	return ok && marker
}

// FindAgentDeployment is the agent's deployment within a
// `ListAgentDeployments` response, skipping preview deployments; nil when
// there is none.
func FindAgentDeployment(response map[string]any) map[string]any {
	for _, deployment := range resources(response) {
		if preview, ok := deployment["is_preview"].(bool); !ok || !preview {
			return deployment
		}
	}
	return nil
}

// FindDeploymentIDByName finds a deployment ID by exact name match within a
// list-deployments response; "" when none matches.
func FindDeploymentIDByName(response map[string]any, name string) string {
	id, _ := FindDeploymentByName(response, name)["id"].(string)
	return id
}

// ContextHubBaseURL derives the Context Hub API base URL from the host backend
// API URL.
func ContextHubBaseURL(hostURL string) string {
	parts := hostapi.ParseHostURL(hostURL)
	const apiHost = "api.host.langchain.com"
	// Local langchainplus splits host-backend (:8300) from the LangSmith API /
	// Context Hub surface (nginx :1980). SaaS maps api.host → api.smith; mirror
	// that split when deploying against a local host-backend.
	if parts.Hostname == "localhost" || parts.Hostname == "127.0.0.1" {
		if port, ok := localHostBackendPort(parts.Trimmed); ok && port == 8300 {
			return parts.Scheme + "://" + parts.Hostname + ":1980"
		}
		return parts.Trimmed
	}
	if parts.Hostname == apiHost {
		return "https://api.smith.langchain.com"
	}
	if prefix, ok := strings.CutSuffix(parts.Hostname, "."+apiHost); ok {
		return "https://" + prefix + ".api.smith.langchain.com"
	}
	return parts.Trimmed
}

// ResolveContextHubBaseURL resolves Context Hub independently from the
// Deployment API.
//
// Known SaaS and local-development host mappings stay authoritative. A custom
// host has no sibling-service mapping, so Context Hub uses the configured
// platform API instead of inheriting a path such as `/api-host`.
func ResolveContextHubBaseURL(hostURL, platformAPIURL string) string {
	derived := ContextHubBaseURL(hostURL)
	if derived == strings.TrimRight(hostURL, "/") {
		return strings.TrimRight(platformAPIURL, "/")
	}
	return derived
}

// localHostBackendPort is the port of a `scheme://host:port` URL, if present.
func localHostBackendPort(hostURL string) (uint16, bool) {
	_, afterScheme, ok := strings.Cut(hostURL, "://")
	if !ok {
		return 0, false
	}
	hostPort, _, _ := strings.Cut(afterScheme, "/")
	_, port, ok := strings.Cut(hostPort, ":")
	if !ok {
		return 0, false
	}
	parsed, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return 0, false
	}
	return uint16(parsed), true
}

// PlanUpgradeURL computes the LangSmith plans-and-billing URL for a
// workspace's organization (Settings > Billing > Plans and Pricing).
//
// The dashboard's `/o/{id}` segment is a *workspace* id despite its name, and
// LangSmith resolves the organization from it, so any workspace in the
// organization reaches the same billing page.
//
// Returns "" unless the id is a hyphenated UUID. The id can come from
// `LANGSMITH_WORKSPACE_ID` or `--workspace-id`, and a link is only worth
// printing when it is well-formed — a value carrying path or whitespace
// characters would render as a misleading URL.
func PlanUpgradeURL(dashboardURL, workspaceID string) string {
	if !isHyphenatedUUID(workspaceID) {
		return ""
	}
	return dashboardURL + "/o/" + workspaceID + "/settings/payments"
}

// isHyphenatedUUID reports whether value is a UUID in its canonical
// hyphenated form (hex digits of either case).
func isHyphenatedUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i := 0; i < len(value); i++ {
		switch i {
		case 8, 13, 18, 23:
			if value[i] != '-' {
				return false
			}
		default:
			b := value[i]
			if !isLowerHexByte(b) && !('A' <= b && b <= 'F') {
				return false
			}
		}
	}
	return true
}

// DeploymentStatusURL computes the LangSmith dashboard URL for a deployment;
// "" when no tenant is known.
func DeploymentStatusURL(deployment map[string]any, deploymentID, dashboardURL string) string {
	tenantID := DeploymentTenantID(deployment)
	if tenantID == "" {
		return ""
	}
	return dashboardURL + "/o/" + tenantID + "/host/deployments/" + deploymentID
}

// DeploymentTenantID reads the workspace ("tenant") that owns a deployment off
// a host-backend deployment object; "" when absent. Present on list, create,
// get, and patch responses alike.
func DeploymentTenantID(deployment map[string]any) string {
	tenantID, _ := deployment["tenant_id"].(string)
	return strings.TrimSpace(tenantID)
}

// DeploymentRuntimeURL extracts the deployed Agent Server URL from host-backend
// deployment payloads; "" when there is none.
func DeploymentRuntimeURL(deployment map[string]any) string {
	topLevelKeys := []string{
		"deployment_url",
		"api_url",
		"runtime_url",
		"host_url",
		"app_url",
		"url",
	}

	for _, key := range topLevelKeys {
		if found := httpURL(deployment[key]); found != "" {
			return found
		}
	}

	for _, nestedKey := range []string{"source_config", "active_revision", "latest_revision"} {
		object, ok := deployment[nestedKey].(map[string]any)
		if !ok {
			continue
		}
		for _, key := range append(slices.Clone(topLevelKeys), "custom_url") {
			if found := httpURL(object[key]); found != "" {
				return found
			}
		}
	}

	return ""
}

func httpURL(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	trimmed := strings.TrimRight(strings.TrimSpace(text), "/")
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	return ""
}

// sortedKeys are the keys of values in order, as a BTreeMap iterates them.
func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
