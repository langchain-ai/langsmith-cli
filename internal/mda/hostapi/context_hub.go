package hostapi

// Context Hub endpoints used during deploy-time context sync.
//
// Calls go to the Context Hub base URL derived from the LangSmith host backend
// URL and use the same credential and optional tenant scope.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"

	"github.com/langchain-ai/langsmith-cli/internal/client"
)

// currentOwner resolves the current tenant/workspace owner.
const currentOwner = "-"

// managedRepoTag is the tag stamped on every repo `langsmith deploy` creates.
//
// This is the only durable record that a Context Hub repo is deploy-owned, and
// createRepo has written it since the first release that created repos at all,
// so every MDA repo in existence carries it and nothing needs backfilling.
// `repo_type: "agent"` is deliberately not used as the signal: the Agent
// Builder UI creates `agent` repos too, and only this tag is unique to deploy.
const managedRepoTag = "managed-deepagents"

// RepoOwnership is what a same-name Context Hub repo is, from a first deploy's
// point of view.
type RepoOwnership int

// The ownerships.
const (
	// RepoOwnershipMissing: no repo with this handle; deploy will create it.
	RepoOwnershipMissing RepoOwnership = iota
	// RepoOwnershipManagedDeepAgent: a repo deploy created. Syncing into it only
	// rewrites content this deploy already owns.
	RepoOwnershipManagedDeepAgent
	// RepoOwnershipForeign: a repo something else created that happens to share
	// the name. Syncing into it would overwrite someone else's work.
	RepoOwnershipForeign
)

// ContextHubClient is the JSON client for the subset of Context Hub needed by
// deploy.
type ContextHubClient struct {
	ls      *client.Client
	baseURL string
	// extraRepoTags are added alongside [managedRepoTag] when this client
	// creates a repo (for example the GitHub-sourced marker the Hub UI reads).
	extraRepoTags []string
}

// NewContextHubClient creates a Context Hub client for baseURL that sends
// every request through ls, which carries the credential and workspace.
func NewContextHubClient(ls *client.Client, baseURL string) (*ContextHubClient, error) {
	if baseURL == "" {
		return nil, errors.New("Context Hub base URL is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" {
		if err == nil {
			err = errors.New("relative URL without a base")
		}
		return nil, fmt.Errorf("Context Hub base URL is invalid: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("Context Hub base URL must use http or https")
	}
	return &ContextHubClient{ls: ls, baseURL: strings.TrimRight(baseURL, "/")}, nil
}

// WithExtraRepoTags returns a copy of the client that tags repos it creates
// with tags in addition to the ownership tag. Ownership detection only ever
// looks for [managedRepoTag], so extra tags never change what RepoOwnership
// reports.
func (h *ContextHubClient) WithExtraRepoTags(tags []string) *ContextHubClient {
	copied := *h
	copied.extraRepoTags = append([]string(nil), tags...)
	return &copied
}

func (h *ContextHubClient) repoTags() []string {
	tags := []string{managedRepoTag}
	for _, tag := range h.extraRepoTags {
		if tag != managedRepoTag {
			tags = append(tags, tag)
		}
	}
	return tags
}

// EnsureRepo ensures a Context Hub repo exists for this deployment.
func (h *ContextHubClient) EnsureRepo(ctx context.Context, repo string) error {
	ownership, err := h.RepoOwnership(ctx, repo)
	if err != nil {
		return err
	}
	if ownership == RepoOwnershipMissing {
		return h.createRepo(ctx, repo)
	}
	return nil
}

// TagRepo tags repo with the Agent tag value tagValueID, which lists the repo
// under that agent in Context Hub. Tagging twice is a no-op.
func (h *ContextHubClient) TagRepo(ctx context.Context, repo, tagValueID string) error {
	status, text, err := h.getRepo(ctx, repo)
	if err != nil {
		return err
	}
	if !isSuccess(status) {
		return apiErrorFromResponse(status, text)
	}
	raw, err := decodeJSONBody(text, "Context Hub repo")
	if err != nil {
		return err
	}
	payload, err := decodeAny(raw)
	if err != nil {
		return err
	}
	repoID, err := RequireStr(field(payload, "repo"), "id", "Context Hub repo")
	if err != nil {
		return err
	}
	path := "/api/v1/workspaces/current/taggings"
	status, text, err = h.send(ctx, http.MethodPost, path, nil, map[string]any{
		"tag_value_id":  tagValueID,
		"resource_type": "agent",
		"resource_id":   repoID,
	})
	if err != nil {
		return err
	}
	if !isSuccess(status) {
		return apiErrorFromResponse(status, text)
	}
	return nil
}

// RepoOwnership classifies a same-name Context Hub repo as missing,
// deploy-owned, or someone else's.
//
// Fails closed: only a repo whose `tags` array literally contains
// [managedRepoTag] is reported as [RepoOwnershipManagedDeepAgent]. A body we
// cannot parse, a repo with no tags, or tags of an unexpected shape all read
// as [RepoOwnershipForeign], because the caller uses this to decide whether
// overwriting is safe. Non-404 failures surface as errors rather than being
// flattened into a verdict: a 403 must never be mistaken for "nothing is
// there".
func (h *ContextHubClient) RepoOwnership(ctx context.Context, repo string) (RepoOwnership, error) {
	status, text, err := h.getRepo(ctx, repo)
	if err != nil {
		return 0, err
	}
	if status == http.StatusNotFound {
		return RepoOwnershipMissing, nil
	}
	if !isSuccess(status) {
		return 0, apiErrorFromResponse(status, text)
	}
	if repoHasManagedTag(text) {
		return RepoOwnershipManagedDeepAgent, nil
	}
	return RepoOwnershipForeign, nil
}

// getRepo is `GET /api/v1/repos/-/{repo}`.
func (h *ContextHubClient) getRepo(ctx context.Context, repo string) (int, []byte, error) {
	return h.sdk(ctx, http.MethodGet, "/api/v1/repos/"+currentOwner+"/"+repo,
		func(ctx context.Context, opts ...option.RequestOption) error {
			_, err := h.ls.SDK.Repos.Get(ctx, currentOwner, repo, opts...)
			return err
		})
}

// GetDirectory reads the repo's directory, empty when the repo has none.
func (h *ContextHubClient) GetDirectory(ctx context.Context, repo string) (DirectorySnapshot, error) {
	status, text, err := h.sdk(ctx, http.MethodGet, directoriesPath(repo),
		func(ctx context.Context, opts ...option.RequestOption) error {
			_, err := h.ls.SDK.Repos.Directories.List(ctx, currentOwner, repo, langsmith.RepoDirectoryListParams{}, opts...)
			return err
		})
	if err != nil {
		return DirectorySnapshot{}, err
	}
	if status == http.StatusNotFound {
		return DirectorySnapshot{Files: map[string]any{}}, nil
	}
	if !isSuccess(status) {
		return DirectorySnapshot{}, apiErrorFromResponse(status, text)
	}
	raw, err := decodeJSONBody(text, "Context Hub directory")
	if err != nil {
		return DirectorySnapshot{}, err
	}
	value, err := decodeAny(raw)
	if err != nil {
		return DirectorySnapshot{}, err
	}
	return directorySnapshotFromValue(value)
}

// CreateDirectoryCommit commits files to the repo's directory on top of
// parentCommit ("" for none).
func (h *ContextHubClient) CreateDirectoryCommit(ctx context.Context, repo, parentCommit string, files map[string]any) error {
	var params langsmith.RepoDirectoryCommitParams
	if parentCommit != "" {
		params.ParentCommit = langsmith.F(parentCommit)
	}
	status, text, err := h.sdk(ctx, http.MethodPost, directoriesPath(repo)+"/commits",
		func(ctx context.Context, opts ...option.RequestOption) error {
			// The files are the raw entries deploy builds, null deleting a
			// path, which the SDK's typed field would drop.
			opts = append(opts, option.WithJSONSet("files", objectOrEmpty(files)))
			_, err := h.ls.SDK.Repos.Directories.Commit(ctx, currentOwner, repo, params, opts...)
			return err
		})
	if err != nil {
		return err
	}
	if status == http.StatusConflict {
		return fmt.Errorf("Context Hub repo '%s' changed during deploy; rerun `langsmith deploy` to retry", repo)
	}
	if !isSuccess(status) {
		return apiErrorFromResponse(status, text)
	}
	_, err = decodeJSONBody(text, "Context Hub commit")
	return err
}

func (h *ContextHubClient) createRepo(ctx context.Context, repo string) error {
	status, text, err := h.sdk(ctx, http.MethodPost, "/api/v1/repos",
		func(ctx context.Context, opts ...option.RequestOption) error {
			_, err := h.ls.SDK.Repos.New(ctx, langsmith.RepoNewParams{
				RepoHandle:  langsmith.F(repo),
				RepoType:    langsmith.F(langsmith.RepoNewParamsRepoTypeAgent),
				IsPublic:    langsmith.F(false),
				Description: langsmith.F("Managed Deep Agents deployed context"),
				// Read back by RepoOwnership to tell our repos from same-name
				// ones we must not overwrite. Keep the two in step.
				Tags: langsmith.F(h.repoTags()),
			}, opts...)
			return err
		})
	if err != nil {
		return err
	}
	if isSuccess(status) || status == http.StatusConflict {
		return nil
	}
	return apiErrorFromResponse(status, text)
}

// DeleteRepo deletes a Context Hub repo and everything committed to it.
//
// Returns false when the repo is already gone (HTTP 404) so teardown is
// idempotent.
func (h *ContextHubClient) DeleteRepo(ctx context.Context, repo string) (bool, error) {
	status, text, err := h.sdk(ctx, http.MethodDelete, directoriesPath(repo),
		func(ctx context.Context, opts ...option.RequestOption) error {
			return h.ls.SDK.Repos.Directories.Delete(ctx, currentOwner, repo, langsmith.RepoDirectoryDeleteParams{}, opts...)
		})
	if err != nil {
		return false, err
	}
	if status == http.StatusNotFound {
		return false, nil
	}
	if !isSuccess(status) {
		return false, apiErrorFromResponse(status, text)
	}
	return true, nil
}

// ListAgentRepoHandles lists the handles of every non-archived agent repo in
// the workspace.
//
// Paginates to exhaustion. Used by teardown to find a deployment's repo plus
// any legacy per-user or org memory children under it, which are not
// discoverable by name alone.
func (h *ContextHubClient) ListAgentRepoHandles(ctx context.Context) ([]string, error) {
	const pageSize = 100
	handles := []string{}
	offset := 0

	for {
		status, text, err := h.sdk(ctx, http.MethodGet, "/api/v1/repos",
			func(ctx context.Context, opts ...option.RequestOption) error {
				_, err := h.ls.SDK.Repos.List(ctx, langsmith.RepoListParams{
					SingleRepoType: langsmith.F(langsmith.RepoListParamsRepoTypeAgent),
					IsArchived:     langsmith.F(langsmith.RepoListParamsIsArchivedFalse),
					Limit:          langsmith.F(int64(pageSize)),
					Offset:         langsmith.F(int64(offset)),
				}, opts...)
				return err
			})
		if err != nil {
			return nil, err
		}
		if !isSuccess(status) {
			return nil, apiErrorFromResponse(status, text)
		}
		raw, err := decodeJSONBody(text, "Context Hub repos")
		if err != nil {
			return nil, err
		}
		payload, err := decodeAny(raw)
		if err != nil {
			return nil, err
		}
		page, _ := field(payload, "repos").([]any)
		for _, repo := range page {
			if handle, ok := field(repo, "repo_handle").(string); ok {
				handles = append(handles, handle)
			}
		}

		if len(page) < pageSize {
			break
		}
		offset += len(page)
		if total, ok := field(payload, "total").(float64); ok && total >= 0 && float64(offset) >= total {
			break
		}
	}

	return handles, nil
}

// send is one raw Context Hub request at path.
func (h *ContextHubClient) send(ctx context.Context, method, path string, params []queryParam, payload any) (int, []byte, error) {
	status, text, err := sendJSONRequest(ctx, h.ls, method, h.baseURL+path, params, payload, requestOptions{timeout: RequestTimeout})
	if err != nil {
		return 0, nil, requestFailed(method, path, err)
	}
	return status, text, nil
}

// sdk is one typed SDK call against Context Hub; method and path name it in
// errors.
func (h *ContextHubClient) sdk(ctx context.Context, method, path string, call sdkCall) (int, []byte, error) {
	status, text, err := sendSDKRequest(ctx, h.baseURL, requestOptions{timeout: RequestTimeout}, call)
	if err != nil {
		return 0, nil, requestFailed(method, path, err)
	}
	return status, text, nil
}

func directoriesPath(repo string) string {
	return "/api/v1/platform/hub/repos/" + currentOwner + "/" + repo + "/directories"
}

// repoHasManagedTag reports whether a get-repo response body carries
// [managedRepoTag].
//
// Deliberately narrow: the tag must appear as an exact string element of
// `repo.tags`. Matching on the raw body instead would let a repo that merely
// mentions the tag in its description or readme pass as deploy-owned. Anything
// unexpected (unparseable JSON, no `repo`, no `tags`, `tags` that is not an
// array) answers false, so the caller keeps its confirmation prompt.
func repoHasManagedTag(body []byte) bool {
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return false
	}
	tags, _ := field(field(value, "repo"), "tags").([]any)
	for _, tag := range tags {
		if tag == managedRepoTag {
			return true
		}
	}
	return false
}

// DirectorySnapshot is a repo's directory: its commit and its files as raw
// JSON entries.
type DirectorySnapshot struct {
	// CommitHash is the directory's commit, "" for none.
	CommitHash string
	Files      map[string]any
}

// directorySnapshotFromValue parses a directories response, refusing to read
// shape drift as "empty".
//
// Strict on purpose. Deploy treats an absent file as "nothing deployed yet"
// and seeds memory, and it passes CommitHash as the commit's parent so the Hub
// can reject a racing write. Quietly defaulting both turns a renamed or
// re-nested field into a deploy that seeds an empty memory file over the
// agent's real one and waives the conflict check that would have caught it.
// An error here costs a failed deploy; the silent version costs someone's
// memory.
func directorySnapshotFromValue(value any) (DirectorySnapshot, error) {
	commitHash, _ := field(value, "commit_hash").(string)
	object, _ := value.(map[string]any)
	filesValue, present := object["files"]
	var files map[string]any
	switch filesValue := filesValue.(type) {
	case map[string]any:
		files = filesValue
	case nil:
		if !present {
			return DirectorySnapshot{}, errors.New("Context Hub directory response has no `files` field")
		}
		// An existing directory with nothing in it. A repo with no directory
		// at all answers 404, which GetDirectory handles before parsing, so a
		// missing field here is drift, not emptiness.
		files = map[string]any{}
	default:
		return DirectorySnapshot{}, errors.New("Context Hub directory response has a non-object `files` field")
	}
	if len(files) > 0 && commitHash == "" {
		return DirectorySnapshot{}, errors.New("Context Hub directory response lists files but no `commit_hash`")
	}
	return DirectorySnapshot{CommitHash: commitHash, Files: files}, nil
}
