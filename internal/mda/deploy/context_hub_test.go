package deploy

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

func tsProject(root, agentFile string) *project.Project {
	return testProject(root, agentFile, project.TypeScript)
}

// newLocalContext is an empty local context with memory enabled.
func newLocalContext() localContext {
	return localContext{files: map[string]string{}, memoryEnabled: true}
}

func buildAllCommitFiles(t *testing.T, existing map[string]any, local localContext) map[string]any {
	t.Helper()
	baseline, err := readDeployManifest(existing)
	require.NoError(t, err)
	var parsed *deployManifest
	if baseline != nil {
		parsed = &baseline.parsed
	}
	files, err := buildCommitFiles(existing, local, false, false, parsed, hashLocalContext(local))
	require.NoError(t, err)
	return files
}

func noOpSyncPlan(t *testing.T, existing map[string]any, localHashes deployManifest) *syncPlan {
	t.Helper()
	hubState, err := readHubContextState(existing)
	require.NoError(t, err)
	local := newLocalContext()
	local.memoryEnabled = false
	return &syncPlan{
		repo:         "my-agent",
		local:        local,
		parentCommit: "old",
		existing:     existing,
		hubState:     hubState,
		localHashes:  localHashes,
	}
}

func manifestContent(t *testing.T, manifest deployManifest) string {
	t.Helper()
	files, err := buildCommitFiles(map[string]any{}, localContext{files: map[string]string{}}, false, false, nil, manifest)
	require.NoError(t, err)
	content, ok := entryContent(files[deployManifestPath])
	require.True(t, ok)
	return content
}

// linkRepo runs the link against a server that returns one tagged agent and
// one repo, then answers the tagging request with taggingStatus.
func linkRepo(t *testing.T, taggingStatus int) *stub {
	t.Helper()
	server := serveSequence(t,
		okReply(`{"items":[{"id":"actual-agent","tag_value_id":"agent-tag"}],"next_cursor":null}`),
		okReply(`{"repo":{"id":"repo-id"}}`),
		reply{status: taggingStatus, body: "{}"},
	)
	unattended(t)

	err := linkContextRepoToAgent(t.Context(), hostClient(t, server.URL), contextHubClient(t, server.URL), "deployment-id", "repo-handle")

	require.NoError(t, err)
	return server
}

func TestContextAgentLinkTagsTheRepoWithItsAgent(t *testing.T) {
	requests := linkRepo(t, http.StatusOK).received()

	require.Len(t, requests, 3)
	assert.Equal(t, "GET /api/v1/repos/-/repo-handle", requests[1].line())
	assert.Equal(t, "POST /api/v1/workspaces/current/taggings", requests[2].line())
	assert.Equal(t, jsonValue(t, `{"resource_id":"repo-id","resource_type":"agent","tag_value_id":"agent-tag"}`),
		jsonValue(t, requests[2].body))
}

func TestContextAgentLinkDoesNotFailDeployWhenTaggingIsDenied(t *testing.T) {
	assert.Len(t, linkRepo(t, http.StatusForbidden).received(), 3)
}

func TestCollectLocalContextReadsInstructionsAndSkillsNextToAgent(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/agent.ts", "export const agent = {};\n")
	writeFile(t, root, "src/instructions.md", "Be direct.\n")
	writeFile(t, root, "src/skills/research/SKILL.md", "Research.\n")
	writeFile(t, root, "instructions.md", "ignore root\n")

	local, err := collectLocalContext(tsProject(root, "src/agent.ts"), true)

	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"instructions.md":          "Be direct.\n",
		"skills/research/SKILL.md": "Research.\n",
	}, local.files)
	assert.True(t, local.hasSkills)
}

func TestCollectLocalContextMarksLocalMemoriesWithoutUploadingThem(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "agent.ts", "export const agent = {};\n")
	writeFile(t, root, "memories/agent/AGENTS.md", "local memory\n")

	local, err := collectLocalContext(tsProject(root, "agent.ts"), true)

	require.NoError(t, err)
	assert.True(t, local.hasLocalMemories)
	assert.NotContains(t, local.files, project.AgentMemoryFile)
	assert.NotContains(t, local.files, project.FlatMemoryFile)
}

func TestCollectLocalContextSkipsTheMemorySeedWhenMemoryIsOff(t *testing.T) {
	// A project that declares no memory must not have a hot file seeded for
	// it: deploy and the runtime read the same decision.
	root := t.TempDir()
	writeFile(t, root, "agent.ts", "export const agent = {};\n")

	local, err := collectLocalContext(tsProject(root, "agent.ts"), false)

	require.NoError(t, err)
	assert.False(t, local.memoryEnabled)
	assert.NotContains(t, local.files, project.AgentMemoryFile)
}

func TestCountHubMemoryFilesCountsMemoryFilesOnly(t *testing.T) {
	files := map[string]any{
		"instructions.md":          fileEntry("prompt\n"),
		"memories/agent/AGENTS.md": fileEntry("learned\n"),
		"memories/agent/notes.md":  fileEntry("more\n"),
		"skills/x/SKILL.md":        fileEntry("skill\n"),
	}

	// A count, never the paths: agent-chosen filenames are memory content.
	assert.Equal(t, 2, countHubMemoryFiles(files))
}

func TestCollectLocalContextErrorsOnNonUTF8Skills(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "agent.ts", "export const agent = {};\n")
	skill := filepath.Join(root, "skills", "binary", "SKILL.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(skill), 0o755))
	require.NoError(t, os.WriteFile(skill, []byte{0xff, 0xfe, 0xfd}, 0o644))

	_, err := collectLocalContext(tsProject(root, "agent.ts"), true)

	assert.EqualError(t, err, "Context Hub only accepts UTF-8 text files; failed to read "+skill+": stream did not contain valid UTF-8")
}

func TestCollectLocalContextSkipsOSJunkInSkills(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "agent.ts", "export const agent = {};\n")
	writeFile(t, root, "skills/research/SKILL.md", "Research skill.\n")
	require.NoError(t, os.WriteFile(filepath.Join(root, "skills", "research", ".DS_Store"), []byte{0x00, 0x01, 0x02, 0xff}, 0o644))

	local, err := collectLocalContext(tsProject(root, "agent.ts"), true)

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"skills/research/SKILL.md": "Research skill.\n"}, local.files)
}

func TestCollectLocalContextKeepsSkillSubtreesNamedLikeProjectDirs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "agent.ts", "export const agent = {};\n")
	writeFile(t, root, "skills/release/build/steps.md", "Build steps.\n")
	writeFile(t, root, "skills/release/dist/notes.md", "Dist notes.\n")
	writeFile(t, root, "skills/release/evals/rubric.md", "Rubric.\n")
	writeFile(t, root, "skills/release/memories/hints.md", "Hints.\n")

	local, err := collectLocalContext(tsProject(root, "agent.ts"), true)

	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"skills/release/build/steps.md":    "Build steps.\n",
		"skills/release/dist/notes.md":     "Dist notes.\n",
		"skills/release/evals/rubric.md":   "Rubric.\n",
		"skills/release/memories/hints.md": "Hints.\n",
	}, local.files)
}

func TestBuildCommitFilesUpdatesLocalContextAndPreservesMemories(t *testing.T) {
	existing := map[string]any{
		"instructions.md":       fileEntry("old\n"),
		"skills/stale/SKILL.md": fileEntry("stale\n"),
		project.AgentMemoryFile: fileEntry("remember\n"),
		"memories/notes.md":     fileEntry("keep\n"),
	}
	local := newLocalContext()
	local.files["instructions.md"] = "new\n"
	local.files["skills/new/SKILL.md"] = "new skill\n"

	files := buildAllCommitFiles(t, existing, local)

	assert.Equal(t, fileEntry("new\n"), files["instructions.md"])
	assert.Equal(t, fileEntry("new skill\n"), files["skills/new/SKILL.md"])
	assert.Contains(t, files, "skills/stale/SKILL.md")
	assert.Nil(t, files["skills/stale/SKILL.md"])
	assert.NotContains(t, files, project.AgentMemoryFile)
	assert.NotContains(t, files, "memories/notes.md")
}

func TestBuildCommitFilesCreatesAnEmptyMemoryFileWhenAbsent(t *testing.T) {
	files := buildAllCommitFiles(t, map[string]any{}, newLocalContext())

	// The file exists so the Hub shows where memory lives and `edit_file`
	// works on the first save, but it holds no platform prose — the
	// instructions are injected by the runtime.
	assert.Equal(t, fileEntry(""), files[project.AgentMemoryFile])
	assert.Empty(t, project.DefaultMemoryContent)
}

func TestBuildCommitFilesMigratesFlatMemoryIntoAgentSlice(t *testing.T) {
	existing := map[string]any{project.FlatMemoryFile: fileEntry("legacy\n")}

	files := buildAllCommitFiles(t, existing, newLocalContext())

	assert.Equal(t, fileEntry("legacy\n"), files[project.AgentMemoryFile])
	// Copied, not moved: nothing here deletes memory.
	assert.NotContains(t, files, project.FlatMemoryFile)
}

func TestBuildCommitFilesLeavesAgentWrittenMemoryUntouched(t *testing.T) {
	// The steady state: no managed block, so a redeploy must not rewrite
	// durable memory at all.
	existing := map[string]any{project.AgentMemoryFile: fileEntry("- favorite color: blue\n")}

	files := buildAllCommitFiles(t, existing, newLocalContext())

	assert.NotContains(t, files, project.AgentMemoryFile)
}

func TestBuildCommitFilesSkipsMemoryFileWhenMemoryIsDisabled(t *testing.T) {
	local := newLocalContext()
	local.memoryEnabled = false

	files := buildAllCommitFiles(t, map[string]any{}, local)

	assert.NotContains(t, files, project.AgentMemoryFile)
}

func TestBuildCommitFilesDeletesRemoteInstructionsWhenLocalIsAbsent(t *testing.T) {
	existing := map[string]any{
		"instructions.md":       fileEntry("old\n"),
		project.AgentMemoryFile: fileEntry(""),
	}

	files := buildAllCommitFiles(t, existing, newLocalContext())

	assert.Contains(t, files, "instructions.md")
	assert.Nil(t, files["instructions.md"])
}

func TestBuildCommitFilesIsEmptyWhenContextIsCurrent(t *testing.T) {
	local := newLocalContext()
	local.files["instructions.md"] = "same\n"
	local.files["skills/research/SKILL.md"] = "same skill\n"
	existing := map[string]any{
		"instructions.md":          fileEntry("same\n"),
		"skills/research/SKILL.md": fileEntry("same skill\n"),
		project.AgentMemoryFile:    fileEntry(""),
		deployManifestPath:         fileEntry(manifestContent(t, hashLocalContext(local))),
	}

	assert.Empty(t, buildAllCommitFiles(t, existing, local))
}

func TestDetectsHubEditsButNotLocalOnlyChanges(t *testing.T) {
	old := hashFiles([]hubFile{{project.InstructionsFile, "old\n"}})
	edited := hashFiles([]hubFile{{project.InstructionsFile, "edited in Hub\n"}})
	local := hashFiles([]hubFile{{project.InstructionsFile, "new locally\n"}})
	baseline := deployManifest{Instructions: &old}

	assert.Equal(t, []contextSection{contextSectionInstructions},
		detectConflicts(baseline, deployManifest{Instructions: &edited}, deployManifest{Instructions: &local}))
	assert.Empty(t, detectConflicts(baseline, deployManifest{Instructions: &old}, deployManifest{Instructions: &local}))
}

func TestHashFilesMatchesTheMdaCLI(t *testing.T) {
	// Deploy manifests written by the mda CLI must keep comparing equal.
	assert.Equal(t, "fnv1a128:24d58209fbfb732d8959b6d8fee62084",
		hashFiles([]hubFile{{project.InstructionsFile, "new\n"}}))
}

func TestSyncContextBootstrapsRepoWithoutManifest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "agent.ts", "export const agent = {};\n")
	writeFile(t, root, "instructions.md", "new\n")
	server := serveSequence(t,
		okReply(`{"repo":{"repo_handle":"my-agent"}}`),
		okReply(`{"commit_hash":"old","files":{"instructions.md":{"type":"file","content":"old\n"}}}`),
		okReply(`{"commit":{"commit_hash":"new"}}`),
	)
	client := contextHubClient(t, server.URL)

	plan, err := prepareContextSync(t.Context(), client, tsProject(root, "agent.ts"), "my-agent", false)
	require.NoError(t, err)
	assert.Empty(t, plan.Conflicts())
	_, err = applyContextSync(t.Context(), client, plan, conflictResolutionOverwrite)
	require.NoError(t, err)

	instructions := hashFiles([]hubFile{{project.InstructionsFile, "new\n"}})
	manifest := manifestContent(t, deployManifest{Instructions: &instructions})
	assert.Equal(t, map[string]any{
		"files": map[string]any{
			deployManifestPath:       map[string]any{"type": "file", "content": manifest},
			project.InstructionsFile: map[string]any{"type": "file", "content": "new\n"},
		},
		"parent_commit": "old",
	}, jsonValue(t, server.received()[2].body))
}

func TestKeepingHubSkillsDoesNotUpdateOrDeleteThem(t *testing.T) {
	existing := map[string]any{
		"skills/hub/SKILL.md":   fileEntry("edited in Hub\n"),
		project.AgentMemoryFile: fileEntry("remember\n"),
	}
	oldSkills := hashFiles([]hubFile{{"skills/hub/SKILL.md", "old\n"}})
	baseline := deployManifest{Skills: &oldSkills}
	local := newLocalContext()
	local.files["skills/local/SKILL.md"] = "local\n"

	files, err := buildCommitFiles(existing, local, false, true, &baseline, hashLocalContext(local))

	require.NoError(t, err)
	assert.NotContains(t, files, "skills/hub/SKILL.md")
	assert.NotContains(t, files, "skills/local/SKILL.md")
	assert.NotContains(t, files, project.AgentMemoryFile)
}

func emptyManifestHub(t *testing.T) map[string]any {
	t.Helper()
	return map[string]any{deployManifestPath: fileEntry(manifestContent(t, deployManifest{}))}
}

func TestNoOpSyncIgnoresMemoryOnlyChanges(t *testing.T) {
	existing := emptyManifestHub(t)
	existing[project.AgentMemoryFile] = fileEntry("old memory\n")
	plan := noOpSyncPlan(t, existing, deployManifest{})
	server := serveSequence(t, okReply(`{"commit_hash":"new","files":{".mda/deploy-manifest.json":{"type":"file","content":"{\"instructions\":null,\"skills\":null}\n"},"memories/agent/AGENTS.md":{"type":"file","content":"new memory\n"}}}`))

	summary, err := applyContextSync(t.Context(), contextHubClient(t, server.URL), plan, conflictResolutionKeepHub)

	require.NoError(t, err)
	assert.False(t, summary.Changed)
}

func TestNoOpSyncRejectsDeployOwnedChanges(t *testing.T) {
	plan := noOpSyncPlan(t, emptyManifestHub(t), deployManifest{})
	server := serveSequence(t, okReply(`{"commit_hash":"new","files":{".mda/deploy-manifest.json":{"type":"file","content":"{\"instructions\":null,\"skills\":null}\n"},"instructions.md":{"type":"file","content":"edited in Hub\n"}}}`))

	_, err := applyContextSync(t.Context(), contextHubClient(t, server.URL), plan, conflictResolutionKeepHub)

	assert.EqualError(t, err, "Context Hub repo 'my-agent' changed during deploy; rerun `langsmith deploy` to retry")
}

func TestNoOpSyncRejectsManifestTextChanges(t *testing.T) {
	plan := noOpSyncPlan(t, emptyManifestHub(t), deployManifest{})
	server := serveSequence(t, okReply(`{"commit_hash":"new","files":{".mda/deploy-manifest.json":{"type":"file","content":"{\"instructions\":null,\"skills\":null,\"note\":\"edited\"}\n"}}}`))

	_, err := applyContextSync(t.Context(), contextHubClient(t, server.URL), plan, conflictResolutionKeepHub)

	assert.EqualError(t, err, "Context Hub repo 'my-agent' changed during deploy; rerun `langsmith deploy` to retry")
}

func TestSyncContextCreatesRepoWhenGetRepoReturns404(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "agent.ts", "export const agent = {};\n")
	writeFile(t, root, "instructions.md", "Use local.\n")
	server := serveSequence(t,
		reply{status: http.StatusNotFound, body: `{"detail":"missing"}`},
		okReply(`{"repo":{"repo_handle":"my-agent"}}`),
		reply{status: http.StatusNotFound, body: `{"detail":"missing"}`},
		okReply(`{"commit":{"commit_hash":"new-commit"}}`),
	)
	client := contextHubClient(t, server.URL)

	plan, err := prepareContextSync(t.Context(), client, tsProject(root, "agent.ts"), "my-agent", true)
	require.NoError(t, err)
	summary, err := applyContextSync(t.Context(), client, plan, conflictResolutionOverwrite)
	require.NoError(t, err)

	lines := server.lines()
	require.Len(t, lines, 4)
	assert.Equal(t, "GET /api/v1/repos/-/my-agent", lines[0])
	assert.Equal(t, "POST /api/v1/repos", lines[1])
	assert.True(t, strings.HasPrefix(lines[2], "GET /api/v1/platform/hub/repos/-/my-agent/directories"), lines[2])
	assert.Equal(t, "POST /api/v1/platform/hub/repos/-/my-agent/directories/commits", lines[3])
	assert.True(t, summary.Changed)
	assert.Equal(t, "my-agent", summary.Repo)
}

func TestSyncContextReportsDirectoryCommitConflicts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "agent.ts", "export const agent = {};\n")
	writeFile(t, root, "instructions.md", "new\n")
	server := serveSequence(t,
		okReply(`{"repo":{"repo_handle":"my-agent"}}`),
		okReply(`{"commit_hash":"old","files":{"instructions.md":{"type":"file","content":"old\n"},"memories/agent/AGENTS.md":{"type":"file","content":""}}}`),
		reply{status: http.StatusConflict, body: `{"detail":"stale parent"}`},
	)
	client := contextHubClient(t, server.URL)

	plan, err := prepareContextSync(t.Context(), client, tsProject(root, "agent.ts"), "my-agent", true)
	require.NoError(t, err)
	_, err = applyContextSync(t.Context(), client, plan, conflictResolutionOverwrite)

	assert.EqualError(t, err, "Context Hub repo 'my-agent' changed during deploy; rerun `langsmith deploy` to retry")
}

func TestANullDeployManifestIsInvalid(t *testing.T) {
	_, err := readDeployManifest(map[string]any{deployManifestPath: fileEntry("null\n")})

	assert.EqualError(t, err, "Context Hub deploy manifest is invalid: expected an object, found null")
}
