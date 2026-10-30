package deploy

// Deploy-time Context Hub reconciliation (`deploy/context_hub.rs`).
//
// MDA deploys code, while Context Hub owns deployed context. This file syncs
// local developer-authored context (`instructions.md`, `skills/**`) into a
// Context Hub repo and deliberately preserves agent-authored memory under
// `memories/**`.

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
	"github.com/langchain-ai/langsmith-cli/internal/mda/proc"
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
	"github.com/langchain-ai/langsmith-cli/internal/mda/ui"
)

const deployManifestPath = ".mda/deploy-manifest.json"

// skillsPrefix is the Hub path prefix of deploy-owned skills.
const skillsPrefix = "skills/"

// contextSection is a deploy-owned part of the Hub repo.
type contextSection int

const (
	contextSectionInstructions contextSection = iota
	contextSectionSkills
)

// Label names the section in messages.
func (s contextSection) Label() string {
	if s == contextSectionSkills {
		return "skills/"
	}
	return "instructions.md"
}

// conflictResolution is what a sync does with sections edited in the Hub.
type conflictResolution int

const (
	conflictResolutionOverwrite conflictResolution = iota
	conflictResolutionKeepHub
)

// deployManifest is the content hashes deploy last synced, stored in the repo
// at [deployManifestPath].
type deployManifest struct {
	Instructions *string `json:"instructions"`
	Skills       *string `json:"skills"`
}

func (m deployManifest) equal(other deployManifest) bool {
	return equalOptional(m.Instructions, other.Instructions) && equalOptional(m.Skills, other.Skills)
}

func equalOptional(left, right *string) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

type deployManifestFile struct {
	rawContent string
	parsed     deployManifest
}

type hubContextState struct {
	manifest          *deployManifestFile
	deployOwnedHashes deployManifest
}

func readHubContextState(files map[string]any) (hubContextState, error) {
	manifest, err := readDeployManifest(files)
	if err != nil {
		return hubContextState{}, err
	}
	hashes, err := hashHubContext(files)
	if err != nil {
		return hubContextState{}, err
	}
	return hubContextState{manifest: manifest, deployOwnedHashes: hashes}, nil
}

func (s hubContextState) equal(other hubContextState) bool {
	if (s.manifest == nil) != (other.manifest == nil) {
		return false
	}
	if s.manifest != nil && (s.manifest.rawContent != other.manifest.rawContent || !s.manifest.parsed.equal(other.manifest.parsed)) {
		return false
	}
	return s.deployOwnedHashes.equal(other.deployOwnedHashes)
}

func (s hubContextState) conflictsWithLocal(localHashes deployManifest) []contextSection {
	if s.manifest == nil {
		return nil
	}
	return detectConflicts(s.manifest.parsed, s.deployOwnedHashes, localHashes)
}

// syncPlan is a prepared sync: the local context, the Hub directory it was
// compared against, and the sections both sides changed.
type syncPlan struct {
	repo         string
	local        localContext
	parentCommit string
	existing     map[string]any
	hubState     hubContextState
	localHashes  deployManifest
	conflicts    []contextSection
}

// Conflicts are the sections edited in the Hub since the last sync that also
// differ locally.
func (p *syncPlan) Conflicts() []contextSection {
	return p.conflicts
}

// syncSummary is the result of syncing local context into Context Hub.
type syncSummary struct {
	Repo             string
	HasSkills        bool
	Changed          bool
	HadLocalMemories bool
	// OrphanedHubMemoryCount is how many Hub memory files exist but will not
	// be mounted, because the project declares no memory. Zero when memory is
	// enabled.
	//
	// A count rather than the paths: an agent chooses its own filenames, so a
	// path like `memories/agent/archive/2026-08-05-jane-doe-leave.md` *is*
	// memory content. Keeping paths out of the struct means no later caller
	// can print them by accident.
	OrphanedHubMemoryCount int
}

// prepareContextSync inspects local and Hub context without changing
// directory contents.
//
// memoryEnabled comes from the project's memory declaration
// ([introspect.ResolveMemory]) — the single answer compile and deploy both
// read, so the seeded tree always matches what the runtime mounts.
func prepareContextSync(ctx context.Context, client *hostapi.ContextHubClient, p *project.Project, repo string, memoryEnabled bool) (*syncPlan, error) {
	if err := client.EnsureRepo(ctx, repo); err != nil {
		return nil, err
	}

	local, err := collectLocalContext(p, memoryEnabled)
	if err != nil {
		return nil, err
	}
	snapshot, err := client.GetDirectory(ctx, repo)
	if err != nil {
		return nil, err
	}
	localHashes := hashLocalContext(local)
	hubState, err := readHubContextState(snapshot.Files)
	if err != nil {
		return nil, err
	}
	conflicts := hubState.conflictsWithLocal(localHashes)

	return &syncPlan{
		repo:         repo,
		local:        local,
		parentCommit: snapshot.CommitHash,
		existing:     snapshot.Files,
		hubState:     hubState,
		localHashes:  localHashes,
		conflicts:    conflicts,
	}, nil
}

// applyContextSync applies a prepared context sync while preserving all
// Hub-owned memory files.
func applyContextSync(ctx context.Context, client *hostapi.ContextHubClient, plan *syncPlan, resolution conflictResolution) (syncSummary, error) {
	preserveInstructions := resolution == conflictResolutionKeepHub &&
		slices.Contains(plan.conflicts, contextSectionInstructions)
	preserveSkills := resolution == conflictResolutionKeepHub &&
		slices.Contains(plan.conflicts, contextSectionSkills)
	var baseline *deployManifest
	if plan.hubState.manifest != nil {
		baseline = &plan.hubState.manifest.parsed
	}
	files, err := buildCommitFiles(plan.existing, plan.local, preserveInstructions, preserveSkills, baseline, plan.localHashes)
	if err != nil {
		return syncSummary{}, err
	}

	changed := len(files) > 0
	if changed {
		if err := client.CreateDirectoryCommit(ctx, plan.repo, plan.parentCommit, files); err != nil {
			return syncSummary{}, err
		}
	} else {
		current, err := client.GetDirectory(ctx, plan.repo)
		if err != nil {
			return syncSummary{}, err
		}
		state, err := readHubContextState(current.Files)
		if err != nil {
			return syncSummary{}, err
		}
		if !state.equal(plan.hubState) {
			return syncSummary{}, fmt.Errorf("Context Hub repo '%s' changed during deploy; rerun `langsmith deploy` to retry", plan.repo)
		}
	}

	hasSkills := plan.local.hasSkills
	if preserveSkills {
		hasSkills = slices.ContainsFunc(slices.Collect(maps.Keys(plan.existing)), isSkillPath)
	}
	orphaned := 0
	if !plan.local.memoryEnabled {
		orphaned = countHubMemoryFiles(plan.existing)
	}
	return syncSummary{
		Repo:                   plan.repo,
		HasSkills:              hasSkills,
		Changed:                changed,
		HadLocalMemories:       plan.local.hasLocalMemories,
		OrphanedHubMemoryCount: orphaned,
	}, nil
}

// linkContextRepoToAgent tags repo with the agent bound to deploymentID, so
// the repo shows in that agent's Context Hub list. Call after the deployment
// exists. Failures print a warning and do not stop the deploy.
func linkContextRepoToAgent(ctx context.Context, client *hostapi.Client, contextClient *hostapi.ContextHubClient, deploymentID, repo string) error {
	err := func() error {
		tag, ok, err := client.DeploymentAgentTagValueID(ctx, deploymentID)
		if err != nil || !ok {
			return err
		}
		return contextClient.TagRepo(ctx, repo, tag)
	}()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return ui.LogWarning(fmt.Sprintf(
			"Could not link Context Hub repo '%s' to its LangSmith agent: %v. Deploy will continue.", repo, err))
	}
	return nil
}

// countHubMemoryFiles is how many `memories/**` files the Hub snapshot
// already has.
//
// Deliberately returns a count, not the paths — see
// [syncSummary.OrphanedHubMemoryCount].
func countHubMemoryFiles(files map[string]any) int {
	prefix := project.MemoriesDir + "/"
	count := 0
	for path := range files {
		if strings.HasPrefix(path, prefix) {
			count++
		}
	}
	return count
}

// localContext is the deploy-owned context of the project.
type localContext struct {
	files            map[string]string
	hasSkills        bool
	hasLocalMemories bool
	memoryEnabled    bool
}

func isSkillPath(path string) bool {
	return strings.HasPrefix(path, skillsPrefix)
}

func collectLocalContext(p *project.Project, memoryEnabled bool) (localContext, error) {
	root := project.ProjectContextRoot(p)
	files := map[string]string{}

	instructions := filepath.Join(root, project.InstructionsFile)
	if isRegularFile(instructions) {
		content, err := readUTF8ContextFile(instructions)
		if err != nil {
			return localContext{}, err
		}
		files[project.InstructionsFile] = content
	}

	skills := filepath.Join(root, project.SkillsDir)
	if isDirectory(skills) {
		if err := collectSkillFiles(root, skills, files); err != nil {
			return localContext{}, err
		}
	}

	return localContext{
		files:            files,
		hasSkills:        slices.ContainsFunc(slices.Collect(maps.Keys(files)), isSkillPath),
		hasLocalMemories: isDirectory(filepath.Join(root, project.MemoriesDir)),
		memoryEnabled:    memoryEnabled,
	}, nil
}

// collectSkillFiles reads the files under skillsDir into files, keyed by
// their path from contextRoot. A symlinked skillsDir is followed, as walkdir
// follows a root link; links beneath it are not.
func collectSkillFiles(contextRoot, skillsDir string, files map[string]string) error {
	target, err := filepath.EvalSymlinks(skillsDir)
	if err != nil {
		return err
	}
	return filepath.WalkDir(target, func(walked string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		inside, err := filepath.Rel(target, walked)
		if err != nil {
			return err
		}
		path := filepath.Join(skillsDir, inside)
		if project.IsJunkDir(path) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() || project.IsJunkFile(path) {
			return nil
		}
		rel, err := filepath.Rel(contextRoot, path)
		if err != nil {
			return err
		}
		content, err := readUTF8ContextFile(path)
		if err != nil {
			return err
		}
		files[proc.ToUnixPath(rel)] = content
		return nil
	})
}

func readUTF8ContextFile(path string) (string, error) {
	contents, err := os.ReadFile(path)
	if err == nil && !utf8.Valid(contents) {
		err = errors.New("stream did not contain valid UTF-8")
	}
	if err != nil {
		return "", fmt.Errorf("Context Hub only accepts UTF-8 text files; failed to read %s: %w", path, err)
	}
	return string(contents), nil
}

func readDeployManifest(existing map[string]any) (*deployManifestFile, error) {
	entry, ok := existing[deployManifestPath]
	if !ok {
		return nil, nil
	}
	content, ok := entryContent(entry)
	if !ok {
		return nil, errors.New("Context Hub deploy manifest is not a text file")
	}
	var parsed *deployManifest
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, fmt.Errorf("Context Hub deploy manifest is invalid: %w", err)
	}
	if parsed == nil {
		return nil, errors.New("Context Hub deploy manifest is invalid: expected an object, found null")
	}
	return &deployManifestFile{rawContent: content, parsed: *parsed}, nil
}

// hubFile is one file of a context tree, for hashing.
type hubFile struct {
	path    string
	content string
}

func hashLocalContext(local localContext) deployManifest {
	var manifest deployManifest
	if content, ok := local.files[project.InstructionsFile]; ok {
		hash := hashFiles([]hubFile{{project.InstructionsFile, content}})
		manifest.Instructions = &hash
	}
	var skills []hubFile
	for path, content := range local.files {
		if isSkillPath(path) {
			skills = append(skills, hubFile{path, content})
		}
	}
	manifest.Skills = hashFilesIfAny(skills)
	return manifest
}

func hashHubContext(existing map[string]any) (deployManifest, error) {
	var manifest deployManifest
	if entry, ok := existing[project.InstructionsFile]; ok {
		content, ok := entryContent(entry)
		if !ok {
			return deployManifest{}, errors.New("Context Hub instructions.md is not a text file")
		}
		hash := hashFiles([]hubFile{{project.InstructionsFile, content}})
		manifest.Instructions = &hash
	}
	var skills []hubFile
	for _, path := range slices.Sorted(maps.Keys(existing)) {
		if !isSkillPath(path) {
			continue
		}
		content, ok := entryContent(existing[path])
		if !ok {
			return deployManifest{}, fmt.Errorf("Context Hub skill '%s' is not a text file", path)
		}
		skills = append(skills, hubFile{path, content})
	}
	manifest.Skills = hashFilesIfAny(skills)
	return manifest, nil
}

func hashFilesIfAny(files []hubFile) *string {
	if len(files) == 0 {
		return nil
	}
	hash := hashFiles(files)
	return &hash
}

// hashFiles is the 128-bit FNV-1a hash of the files, sorted by path, each
// path and content prefixed with its big-endian 64-bit length.
func hashFiles(files []hubFile) string {
	sorted := slices.Clone(files)
	slices.SortFunc(sorted, func(left, right hubFile) int {
		return strings.Compare(left.path, right.path)
	})
	hash := fnv.New128a()
	for _, file := range sorted {
		for _, part := range []string{file.path, file.content} {
			var length [8]byte
			binary.BigEndian.PutUint64(length[:], uint64(len(part)))
			// hash.Hash writes never fail.
			_, _ = hash.Write(length[:])
			_, _ = hash.Write([]byte(part))
		}
	}
	return "fnv1a128:" + hex.EncodeToString(hash.Sum(nil))
}

func detectConflicts(baseline, hub, local deployManifest) []contextSection {
	var conflicts []contextSection
	if !equalOptional(hub.Instructions, baseline.Instructions) && !equalOptional(hub.Instructions, local.Instructions) {
		conflicts = append(conflicts, contextSectionInstructions)
	}
	if !equalOptional(hub.Skills, baseline.Skills) && !equalOptional(hub.Skills, local.Skills) {
		conflicts = append(conflicts, contextSectionSkills)
	}
	return conflicts
}

func buildCommitFiles(
	existing map[string]any,
	local localContext,
	preserveInstructions, preserveSkills bool,
	baseline *deployManifest,
	localHashes deployManifest,
) (map[string]any, error) {
	files := map[string]any{}

	if !preserveInstructions {
		upsertOrDeleteInstructions(existing, local, files)
	}
	if !preserveSkills {
		upsertLocalFiles(existing, local, files)
		deleteStaleSkills(existing, local, files)
	}
	if local.memoryEnabled {
		ensureMemorySeed(existing, files)
	}

	manifest := deployManifest{
		Instructions: localHashes.Instructions,
		Skills:       localHashes.Skills,
	}
	if preserveInstructions {
		manifest.Instructions = nil
		if baseline != nil {
			manifest.Instructions = baseline.Instructions
		}
	}
	if preserveSkills {
		manifest.Skills = nil
		if baseline != nil {
			manifest.Skills = baseline.Skills
		}
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	upsertFileIfChanged(existing, files, deployManifestPath, string(encoded)+"\n")

	return files, nil
}

func upsertOrDeleteInstructions(existing map[string]any, local localContext, files map[string]any) {
	if content, ok := local.files[project.InstructionsFile]; ok {
		upsertFileIfChanged(existing, files, project.InstructionsFile, content)
		return
	}
	if _, ok := existing[project.InstructionsFile]; ok {
		files[project.InstructionsFile] = nil
	}
}

func upsertLocalFiles(existing map[string]any, local localContext, files map[string]any) {
	for path, content := range local.files {
		if path == project.InstructionsFile {
			continue
		}
		upsertFileIfChanged(existing, files, path, content)
	}
}

func deleteStaleSkills(existing map[string]any, local localContext, files map[string]any) {
	for path := range existing {
		if _, ok := local.files[path]; isSkillPath(path) && !ok {
			files[path] = nil
		}
	}
}

func ensureMemorySeed(existing map[string]any, files map[string]any) {
	// Prefer the agent-scoped slice. When only a legacy flat
	// `memories/AGENTS.md` exists, migrate its contents into
	// `memories/agent/AGENTS.md` so identity scope `memory: "agent"` (and the
	// local mock) still remount hot memory. An existing agent-scoped file is
	// never rewritten.
	agent := optionalEntryContent(existing, project.AgentMemoryFile)
	flat := optionalEntryContent(existing, project.FlatMemoryFile)
	if seed := project.ResolveAgentMemorySeed(agent, flat); seed.Write {
		files[project.AgentMemoryFile] = fileEntry(seed.Content)
	}
}

func optionalEntryContent(existing map[string]any, path string) *string {
	content, ok := entryContent(existing[path])
	if !ok {
		return nil
	}
	return &content
}

func upsertFileIfChanged(existing map[string]any, files map[string]any, path, content string) {
	if current, ok := entryContent(existing[path]); !ok || current != content {
		files[path] = fileEntry(content)
	}
}

func fileEntry(content string) map[string]any {
	return map[string]any{"type": "file", "content": content}
}

func entryContent(entry any) (string, bool) {
	object, _ := entry.(map[string]any)
	content, ok := object["content"].(string)
	return content, ok
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
