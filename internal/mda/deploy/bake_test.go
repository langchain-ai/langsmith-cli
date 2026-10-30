package deploy

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/mda/compile"
	"github.com/langchain-ai/langsmith-cli/internal/mda/deployment"
)

const snapshotsPath = "/api/v2/sandboxes/snapshots"

func noProgress(BakePhase) {}

func TestBakeSkipsWhenRecipeAlreadyReady(t *testing.T) {
	recipe := deployment.RecipeSnapshotName("agent", deployment.RecipeDigest(nil, nil, nil, nil, nil, nil))
	server := serveSequence(t, okReply(`{"snapshots":[{"id":"1","name":"`+recipe+`","status":"ready"}]}`))

	outcome, err := bakeRecipe(t.Context(), hostClient(t, server.URL), "agent", &RecipeSpec{}, map[string]string{}, noProgress)

	require.NoError(t, err)
	assert.Equal(t, BakeOutcome{RecipeName: recipe, RecipeReference: recipe, Skipped: true}, outcome)
}

func TestBakeSkipKeepsTheTagOfADockerBuiltRecipe(t *testing.T) {
	// Every run after the first cold bake takes the skip path, so it has to
	// re-derive the tag or the recipe pointer breaks again.
	digest := deployment.RecipeDigest(nil, nil, nil, ptr("acme/base:0.1.1"), nil, nil)
	recipe := deployment.RecipeSnapshotName("agent", digest)
	server := serveSequence(t, okReply(`{"snapshots":[{"id":"1","name":"`+recipe+`","status":"ready","tags":["0.1.1"]}]}`))

	outcome, err := bakeRecipe(t.Context(), hostClient(t, server.URL), "agent",
		&RecipeSpec{DockerImage: ptr("acme/base:0.1.1")}, map[string]string{}, noProgress)

	require.NoError(t, err)
	assert.True(t, outcome.Skipped)
	assert.Equal(t, recipe+":0.1.1", outcome.RecipeReference)
}

func TestEmptyDockerImageCreatesRecipeDirectly(t *testing.T) {
	digest := deployment.RecipeDigest(nil, nil, nil, ptr("python:3.12-slim"), nil, nil)
	recipe := deployment.RecipeSnapshotName("agent", digest)
	// find (miss) → create docker → wait get. Host tags a Docker-built
	// snapshot with the image's tag, not `latest`.
	server := serveSequence(t,
		okReply(`{"snapshots":[]}`),
		okReply(`{"id":"snap-1","name":"`+recipe+`","status":"building"}`),
		okReply(`{"id":"snap-1","name":"`+recipe+`","status":"ready","tags":["3.12-slim"]}`),
	)
	var phases []BakePhase

	outcome, err := bakeRecipe(t.Context(), hostClient(t, server.URL), "agent",
		&RecipeSpec{DockerImage: ptr("python:3.12-slim")}, map[string]string{},
		func(phase BakePhase) { phases = append(phases, phase) })

	require.NoError(t, err)
	assert.False(t, outcome.Skipped)
	assert.Equal(t, recipe, outcome.RecipeName)
	// The runtime must clone the tag Host assigned; the bare name is resolved
	// as `:latest` and would not exist.
	assert.Equal(t, recipe+":3.12-slim", outcome.RecipeReference)
	assert.Equal(t, []BakePhase{BakePhaseCheckingRecipe, BakePhaseBuildingFromImage}, phases)
	create := server.received()[1]
	assert.Equal(t, "POST "+snapshotsPath, create.line())
	assert.Contains(t, create.body, `"docker_image":"python:3.12-slim"`)
	assert.Contains(t, create.body, `"fs_capacity_bytes":1073741824`)
}

func TestDockerImageBakePassesRegistryID(t *testing.T) {
	registry := compile.SandboxRegistryManifest{URL: "ghcr.io", Username: "octocat", PasswordEnv: "GHCR_TOKEN"}
	registryDigest := registry.DigestInput()
	digest := deployment.RecipeDigest(nil, nil, nil, ptr("ghcr.io/acme/base:1"), &registryDigest, nil)
	recipe := deployment.RecipeSnapshotName("agent", digest)
	// find (miss) → registry miss → create registry → create docker → wait
	server := serveSequence(t,
		okReply(`{"snapshots":[]}`),
		reply{status: http.StatusNotFound, body: `{"detail":"not found"}`},
		okReply(`{"id":"reg-99","name":"ghcr","url":"ghcr.io"}`),
		okReply(`{"id":"snap-1","name":"`+recipe+`","status":"building"}`),
		okReply(`{"id":"snap-1","name":"`+recipe+`","status":"ready"}`),
	)
	password := strings.Join([]string{"fixture", "credential"}, "-")

	outcome, err := bakeRecipe(t.Context(), hostClient(t, server.URL), "agent",
		&RecipeSpec{DockerImage: ptr("ghcr.io/acme/base:1"), Registry: &registry},
		map[string]string{"GHCR_TOKEN": password}, noProgress)

	require.NoError(t, err)
	assert.False(t, outcome.Skipped)
	requests := server.received()
	assert.Equal(t, "GET /api/v2/sandboxes/registries/agent--registry", requests[1].line())
	assert.Equal(t, "POST /api/v2/sandboxes/registries", requests[2].line())
	assert.Contains(t, requests[2].body, `"password":"`+password+`"`)
	assert.Equal(t, "POST "+snapshotsPath, requests[3].line())
	assert.Contains(t, requests[3].body, `"registry_id":"reg-99"`)
}

func TestPrivateImageBakeFailsWhenRegistryPasswordIsMissing(t *testing.T) {
	server := serveSequence(t, okReply(`{"snapshots":[]}`))

	_, err := bakeRecipe(t.Context(), hostClient(t, server.URL), "agent", &RecipeSpec{
		DockerImage: ptr("ghcr.io/acme/base:1"),
		Registry: &compile.SandboxRegistryManifest{
			URL:         "ghcr.io",
			Username:    "octocat",
			PasswordEnv: "MDA_TEST_MISSING_REGISTRY_TOKEN",
		},
	}, map[string]string{}, noProgress)

	assert.EqualError(t, err, "sandbox registry password is missing; set MDA_TEST_MISSING_REGISTRY_TOKEN in the project .env or process environment")
}

func TestDockerBaseBuilderBootsBySnapshotIDOnly(t *testing.T) {
	// The sandbox API rejects a builder create that names the snapshot by
	// both id and name (HTTP 422), so the docker-base path returns only the
	// id.
	server := serveSequence(t,
		okReply(`{"id":"snap-base","name":"agent--dockerbase-abc","status":"building"}`),
		okReply(`{"id":"snap-base","name":"agent--dockerbase-abc","status":"ready"}`),
	)

	name, id, intermediate, err := resolveBuilderBase(t.Context(), hostClient(t, server.URL), "agent", "abc", &RecipeSpec{
		DockerImage:     ptr("python:3.12-slim"),
		SetupScript:     "pip install requests",
		FsCapacityBytes: ptr(uint64(2 << 30)),
	}, "", noProgress)

	require.NoError(t, err)
	assert.Empty(t, name)
	assert.Equal(t, "snap-base", id)
	assert.Contains(t, server.received()[0].body, `"fs_capacity_bytes":2147483648`)
	// The name is still kept so the intermediate base gets cleaned up.
	assert.Equal(t, "agent"+deployment.SandboxNameSeparator+"dockerbase-abc", intermediate)
}

func TestSetupErrorTruncationPreservesUTF8Boundaries(t *testing.T) {
	text := strings.Repeat("a", 1_999) + "éafter"

	assert.Equal(t, strings.Repeat("a", 1_999)+"…", truncateForError(text))
}

func TestBuilderSandboxNameStaysWithinTheHostLimit(t *testing.T) {
	digest := "27f7518029b14543"

	name := builderSandboxName("mda-integ-sandbox-recipe-ts-pr-339-b3f203", digest)

	assert.LessOrEqual(t, len(name), deployment.HostSandboxNameLimit)
	assert.True(t, strings.HasSuffix(name, deployment.SandboxNameSeparator+"bake-"+digest), name)
}

func TestBuilderSandboxNameDistinguishesLongNamesWithASharedPrefix(t *testing.T) {
	digest := "27f7518029b14543"

	first := builderSandboxName("mda-integ-sandbox-recipe-ts-pr-971-ecc3e2", digest)
	second := builderSandboxName("mda-integ-sandbox-recipe-ts-pr-971-ecc3e2-del", digest)

	assert.NotEqual(t, first, second)
	assert.LessOrEqual(t, len(first), deployment.HostSandboxNameLimit)
	assert.LessOrEqual(t, len(second), deployment.HostSandboxNameLimit)
}

func TestStaleRecipeCleanupPreservesOnlyTheActiveRecipe(t *testing.T) {
	server := serveSequence(t,
		okReply(`{"snapshots":[
			{"id":"active-id","name":"agent--setup-active","status":"ready"},
			{"id":"stale-id","name":"agent--setup-stale","status":"ready"},
			{"id":"neighbor-id","name":"agent--setup-stale--extra","status":"ready"}]}`),
		reply{status: http.StatusNoContent},
	)

	cleanup, err := deleteStaleRecipeSnapshots(t.Context(), hostClient(t, server.URL), "agent", "agent--setup-active")

	require.NoError(t, err)
	assert.Empty(t, cleanup.InUse)
	assert.Equal(t, "DELETE "+snapshotsPath+"/stale-id", server.lines()[1])
	assert.Len(t, server.received(), 2)
}

func TestStaleRecipeCleanupReportsDeleteFailures(t *testing.T) {
	server := serveSequence(t,
		okReply(`{"snapshots":[{"id":"stale-id","name":"agent--setup-stale","status":"ready"}]}`),
		reply{status: http.StatusInternalServerError, body: `{"detail":"delete failed"}`},
	)

	_, err := deleteStaleRecipeSnapshots(t.Context(), hostClient(t, server.URL), "agent", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete 1 stale snapshot(s): 'agent--setup-stale': ")
	assert.Len(t, server.received(), 2)
}

const snapshotInUse = `{"error":"Conflict","message":"snapshot is in use by one or more sandboxes; release them before deleting"}`

func TestStaleRecipeCleanupSkipsSnapshotsStillInUse(t *testing.T) {
	server := serveSequence(t,
		okReply(`{"snapshots":[{"id":"stale-id","name":"agent--setup-stale","status":"ready"}]}`),
		reply{status: http.StatusConflict, body: snapshotInUse},
	)

	cleanup, err := deleteStaleRecipeSnapshots(t.Context(), hostClient(t, server.URL), "agent", "")

	require.NoError(t, err)
	assert.Len(t, server.received(), 2)
	assert.Equal(t, []string{"agent--setup-stale"}, cleanup.InUse)
	note, ok := cleanup.InUseNote()
	assert.True(t, ok)
	assert.Equal(t, "Left 1 previous recipe snapshot in place because live sandboxes still use it ('agent--setup-stale'). It will be removed on a later deploy once those sandboxes are gone.", note)
}

func TestStaleRecipeCleanupRemovesLeakedIntermediateBases(t *testing.T) {
	server := serveSequence(t,
		okReply(`{"snapshots":[
			{"id":"active-id","name":"agent--setup-active","status":"ready"},
			{"id":"docker-id","name":"agent--dockerbase-640ecb01b34ba969","status":"ready"},
			{"id":"source-id","name":"agent--sourcebase-0123456789abcdef","status":"ready"},
			{"id":"neighbor-docker-id","name":"agent--x--dockerbase-640ecb01b34ba969","status":"ready"},
			{"id":"neighbor-source-id","name":"agent--sourcebase-0123456789abcdef--dockerbase-640ecb01b34ba969","status":"ready"},
			{"id":"short-digest-id","name":"agent--dockerbase-abc","status":"ready"}]}`),
		reply{status: http.StatusNoContent},
		reply{status: http.StatusNoContent},
	)

	cleanup, err := deleteStaleRecipeSnapshots(t.Context(), hostClient(t, server.URL), "agent", "agent--setup-active")

	require.NoError(t, err)
	assert.Equal(t, StaleRecipeCleanup{}, cleanup)
	requests := server.received()
	list, err := url.ParseRequestURI(requests[0].uri)
	require.NoError(t, err)
	assert.Equal(t, "agent--", list.Query().Get("name_contains"))
	// Neighbour snapshots are kept.
	assert.Equal(t, []string{
		"DELETE " + snapshotsPath + "/docker-id",
		"DELETE " + snapshotsPath + "/source-id",
	}, server.lines()[1:])
}

func TestStaleRecipeCleanupLeavesInUseIntermediateBasesQuietly(t *testing.T) {
	server := serveSequence(t,
		okReply(`{"snapshots":[
			{"id":"docker-id","name":"agent--dockerbase-640ecb01b34ba969","status":"ready"},
			{"id":"source-id","name":"agent--sourcebase-0123456789abcdef","status":"ready"}]}`),
		reply{status: http.StatusConflict, body: snapshotInUse},
		reply{status: http.StatusConflict, body: snapshotInUse},
	)

	cleanup, err := deleteStaleRecipeSnapshots(t.Context(), hostClient(t, server.URL), "agent", "")

	require.NoError(t, err)
	assert.Equal(t, []string{
		"DELETE " + snapshotsPath + "/docker-id",
		"DELETE " + snapshotsPath + "/source-id",
	}, server.lines()[1:])
	assert.Empty(t, cleanup.InUse)
	_, ok := cleanup.InUseNote()
	assert.False(t, ok)
}
