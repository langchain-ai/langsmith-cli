package hostapi

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func hubClient(t *testing.T, baseURL string) *ContextHubClient {
	t.Helper()
	h, err := NewContextHubClient(apiKeyCLIClient(t, "key", "tenant"), baseURL)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestRepoTagsAlwaysLeadWithTheOwnershipTag(t *testing.T) {
	h := hubClient(t, "http://localhost:1980")
	if got := h.repoTags(); !slices.Equal(got, []string{"managed-deepagents"}) {
		t.Fatalf("repoTags = %v", got)
	}
	tagged := h.WithExtraRepoTags([]string{"managed-deepagents:github", "managed-deepagents"})
	if got := tagged.repoTags(); !slices.Equal(got, []string{"managed-deepagents", "managed-deepagents:github"}) {
		t.Fatalf("repoTags = %v", got)
	}
}

func TestNewContextHubClientRejectsUnusableBaseURLs(t *testing.T) {
	ls := apiKeyCLIClient(t, "key", "")
	for base, want := range map[string]string{
		"":                      "Context Hub base URL is required",
		"ftp://hub.example":     "Context Hub base URL must use http or https",
		"hub.example/no-scheme": "Context Hub base URL is invalid: relative URL without a base",
	} {
		if _, err := NewContextHubClient(ls, base); err == nil || err.Error() != want {
			t.Errorf("NewContextHubClient(%q) = %v, want %q", base, err, want)
		}
	}
}

func TestParsesDirectoryResponses(t *testing.T) {
	directory, err := directorySnapshotFromValue(map[string]any{
		"commit_hash": "abc123",
		"files":       map[string]any{"instructions.md": map[string]any{"type": "file", "content": "hi"}},
	})
	if err != nil || directory.CommitHash != "abc123" || len(directory.Files) != 1 {
		t.Fatalf("directory = %+v, %v", directory, err)
	}
}

func TestParsesAnEmptyDirectory(t *testing.T) {
	directory, err := directorySnapshotFromValue(map[string]any{"files": nil})
	if err != nil || len(directory.Files) != 0 || directory.CommitHash != "" {
		t.Fatalf("directory = %+v, %v", directory, err)
	}
}

func TestRefusesToReadADriftedDirectoryResponseAsEmpty(t *testing.T) {
	// Deploy seeds memory for files it believes are absent, so "empty" has to
	// mean empty, never "the field moved".
	for _, payload := range []string{
		// `files` renamed or re-nested.
		`{}`,
		`{"data":{"files":{"instructions.md":{"content":"hi"}}}}`,
		// `files` became a list of entries.
		`{"commit_hash":"abc123","files":[]}`,
		// Files, but no parent commit to pin the write to.
		`{"files":{"instructions.md":{"content":"hi"}}}`,
	} {
		value, err := decodeAny([]byte(payload))
		if err != nil {
			t.Fatal(err)
		}
		if directory, err := directorySnapshotFromValue(value); err == nil {
			t.Errorf("parsed drift as empty: %s → %+v", payload, directory)
		}
	}
}

func TestGetDirectoryReadsTheHubDirectory(t *testing.T) {
	server := serveOnce(t, 200, `{"commit_hash":"abc","files":{"AGENTS.md":{"type":"file","content":"x"}}}`)
	directory, err := hubClient(t, server.URL).GetDirectory(t.Context(), "my-agent")
	if err != nil || directory.CommitHash != "abc" || len(directory.Files) != 1 {
		t.Fatalf("GetDirectory = %+v, %v", directory, err)
	}
	if line := server.Only(t).Line(); line != "GET /api/v1/platform/hub/repos/-/my-agent/directories" {
		t.Fatalf("request = %s", line)
	}
	missing, err := hubClient(t, serveOnce(t, 404, `{"detail":"nope"}`).URL).GetDirectory(t.Context(), "my-agent")
	if err != nil || len(missing.Files) != 0 || missing.CommitHash != "" {
		t.Fatalf("GetDirectory of a missing repo = %+v, %v", missing, err)
	}
}

func TestCreateDirectoryCommitSendsTheFilesOnTheParent(t *testing.T) {
	server := serveSequence(t, reply{status: 200, body: `{"commit":{"commit_hash":"def"}}`}, reply{status: 409, body: `{"detail":"stale"}`})
	h := hubClient(t, server.URL)
	files := map[string]any{"AGENTS.md": map[string]any{"type": "file", "content": "hi"}, "old.md": nil}

	if err := h.CreateDirectoryCommit(t.Context(), "my-agent", "abc", files); err != nil {
		t.Fatal(err)
	}
	err := h.CreateDirectoryCommit(t.Context(), "my-agent", "", files)
	if err == nil || err.Error() != "Context Hub repo 'my-agent' changed during deploy; rerun `langsmith deploy` to retry" {
		t.Fatalf("conflict = %v", err)
	}

	requests := server.Requests()
	if requests[0].Line() != "POST /api/v1/platform/hub/repos/-/my-agent/directories/commits" {
		t.Errorf("request = %s", requests[0].Line())
	}
	if !jsonEqual(t, requests[0].JSON(t), `{"files":{"AGENTS.md":{"type":"file","content":"hi"},"old.md":null},"parent_commit":"abc"}`) {
		t.Errorf("body = %s", requests[0].Body)
	}
	if !jsonEqual(t, requests[1].JSON(t), `{"files":{"AGENTS.md":{"type":"file","content":"hi"},"old.md":null}}`) {
		t.Errorf("body without a parent = %s", requests[1].Body)
	}
}

func TestTagRepoDoesNotWriteWhenTheRepoIDIsMissing(t *testing.T) {
	server := serveOnce(t, 200, managedRepoBody())
	if err := hubClient(t, server.URL).TagRepo(t.Context(), "my-agent", "agent-tag"); err == nil {
		t.Fatal("tagged a repo without an id")
	}
	server.Only(t)
}

func TestTagRepoTagsTheRepoByID(t *testing.T) {
	server := serveSequence(t, reply{status: 200, body: `{"repo":{"id":"repo-1","tags":[]}}`}, reply{status: 200, body: `{}`})
	if err := hubClient(t, server.URL).TagRepo(t.Context(), "my-agent", "agent-tag"); err != nil {
		t.Fatal(err)
	}
	tagging := server.Requests()[1]
	if tagging.Line() != "POST /api/v1/workspaces/current/taggings" ||
		!jsonEqual(t, tagging.JSON(t), `{"tag_value_id":"agent-tag","resource_type":"agent","resource_id":"repo-1"}`) {
		t.Fatalf("tagging = %s %s", tagging.Line(), tagging.Body)
	}
}

func repoPage(t *testing.T, handles []string) string {
	t.Helper()
	repos := make([]map[string]string, 0, len(handles))
	for _, handle := range handles {
		repos = append(repos, map[string]string{"id": handle, "repo_handle": handle})
	}
	return mustJSON(t, map[string]any{"repos": repos})
}

func managedRepoBody() string {
	return `{"repo":{"repo_handle":"my-agent","tags":["managed-deepagents"]}}`
}

func TestRepoOwnershipTargetsTheCurrentOwnerRepoEndpoint(t *testing.T) {
	server := serveOnce(t, 200, managedRepoBody())
	ownership, err := hubClient(t, server.URL).RepoOwnership(t.Context(), "my-agent")
	if err != nil || ownership != RepoOwnershipManagedDeepAgent {
		t.Fatalf("RepoOwnership = %v, %v", ownership, err)
	}
	request := server.Only(t)
	if request.Line() != "GET /api/v1/repos/-/my-agent" || request.Header.Get("X-Api-Key") != "key" || request.Header.Get("X-Tenant-Id") != "tenant" {
		t.Fatalf("request = %s, headers %v", request.Line(), request.Header)
	}
}

func TestRepoOwnershipReportsMissingForAbsentRepo(t *testing.T) {
	ownership, err := hubClient(t, serveOnce(t, 404, `{"detail":"nope"}`).URL).RepoOwnership(t.Context(), "my-agent")
	if err != nil || ownership != RepoOwnershipMissing {
		t.Fatalf("RepoOwnership = %v, %v", ownership, err)
	}
}

func TestRepoOwnershipReportsForeignForUntaggedOrOtherwiseTaggedRepos(t *testing.T) {
	for _, body := range []string{
		// What the Agent Builder UI creates: `repo_type: "agent"`, no tags.
		`{"repo":{"repo_handle":"my-agent","repo_type":"agent","tags":[]}}`,
		`{"repo":{"repo_handle":"my-agent","tags":["production"]}}`,
		// A description that merely mentions the tag is not ownership.
		`{"repo":{"description":"copied from managed-deepagents","tags":[]}}`,
	} {
		ownership, err := hubClient(t, serveOnce(t, 200, body).URL).RepoOwnership(t.Context(), "my-agent")
		if err != nil || ownership != RepoOwnershipForeign {
			t.Errorf("%s: RepoOwnership = %v, %v; want foreign", body, ownership, err)
		}
	}
}

func TestRepoOwnershipFailsClosedOnAnUnreadableBody(t *testing.T) {
	// Shape drift must never read as "ours": the caller uses this verdict to
	// decide whether overwriting someone else's repo is safe.
	for _, body := range []string{
		"not json",
		"{}",
		`{"repo":{}}`,
		`{"repo":{"tags":"managed-deepagents"}}`,
		`{"repo":{"tags":[{"name":"managed-deepagents"}]}}`,
	} {
		ownership, err := hubClient(t, serveOnce(t, 200, body).URL).RepoOwnership(t.Context(), "my-agent")
		if err != nil || ownership != RepoOwnershipForeign {
			t.Errorf("%s: RepoOwnership = %v, %v; want foreign", body, ownership, err)
		}
	}
}

func TestRepoOwnershipSurfacesOtherErrors(t *testing.T) {
	// A permissions failure must not be flattened into a verdict.
	_, err := hubClient(t, serveOnce(t, 403, `{"detail":"missing permission"}`).URL).RepoOwnership(t.Context(), "my-agent")
	if err == nil || err.Error() != "missing permission (HTTP 403)" {
		t.Fatalf("err = %v", err)
	}
}

func TestEnsureRepoCreatesOnlyWhenTheRepoIsMissing(t *testing.T) {
	server := serveSequence(t, reply{status: 404, body: `{"detail":"nope"}`}, reply{status: 200, body: managedRepoBody()})

	if err := hubClient(t, server.URL).WithExtraRepoTags([]string{"managed-deepagents:github"}).EnsureRepo(t.Context(), "my-agent"); err != nil {
		t.Fatal(err)
	}

	requests := server.Requests()
	if requests[0].Line() != "GET /api/v1/repos/-/my-agent" || requests[1].Line() != "POST /api/v1/repos" {
		t.Fatalf("requests = %s, %s", requests[0].Line(), requests[1].Line())
	}
	want := `{"repo_handle":"my-agent","repo_type":"agent","is_public":false,"description":"Managed Deep Agents deployed context","tags":["managed-deepagents","managed-deepagents:github"]}`
	if !jsonEqual(t, requests[1].JSON(t), want) {
		t.Fatalf("create body = %s", requests[1].Body)
	}
}

func TestEnsureRepoLeavesAnExistingRepoAlone(t *testing.T) {
	server := serveOnce(t, 200, managedRepoBody())
	if err := hubClient(t, server.URL).EnsureRepo(t.Context(), "my-agent"); err != nil {
		t.Fatal(err)
	}
	if line := server.Only(t).Line(); line != "GET /api/v1/repos/-/my-agent" {
		t.Fatalf("request = %s", line)
	}
}

func TestEnsureRepoAcceptsARepoCreatedConcurrently(t *testing.T) {
	server := serveSequence(t, reply{status: 404, body: `{}`}, reply{status: 409, body: `{"detail":"exists"}`})
	if err := hubClient(t, server.URL).EnsureRepo(t.Context(), "my-agent"); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteRepoTargetsTheCurrentOwnerDirectoryEndpoint(t *testing.T) {
	server := serveOnce(t, 200, "{}")
	deleted, err := hubClient(t, server.URL).DeleteRepo(t.Context(), "my-agent")
	if err != nil || !deleted {
		t.Fatalf("DeleteRepo = %v, %v", deleted, err)
	}
	if line := server.Only(t).Line(); line != "DELETE /api/v1/platform/hub/repos/-/my-agent/directories" {
		t.Fatalf("request = %s", line)
	}
}

func TestDeleteRepoTreatsMissingRepoAsAlreadyGone(t *testing.T) {
	deleted, err := hubClient(t, serveOnce(t, 404, `{"detail":"nope"}`).URL).DeleteRepo(t.Context(), "my-agent")
	if err != nil || deleted {
		t.Fatalf("DeleteRepo = %v, %v", deleted, err)
	}
}

func TestDeleteRepoSurfacesOtherErrors(t *testing.T) {
	_, err := hubClient(t, serveOnce(t, 403, `{"detail":"missing permission"}`).URL).DeleteRepo(t.Context(), "my-agent")
	if err == nil || err.Error() != "missing permission (HTTP 403)" {
		t.Fatalf("err = %v", err)
	}
}

func TestListAgentRepoHandlesFiltersToAgentRepos(t *testing.T) {
	server := serveOnce(t, 200, repoPage(t, []string{"my-agent", "other"}))
	handles, err := hubClient(t, server.URL).ListAgentRepoHandles(t.Context())
	if err != nil || !slices.Equal(handles, []string{"my-agent", "other"}) {
		t.Fatalf("handles = %v, %v", handles, err)
	}
	query := server.Only(t).Query(t)
	if strings.Join(query["repo_type"], ",") != "agent" || strings.Join(query["is_archived"], ",") != "false" {
		t.Fatalf("query = %v", query)
	}
}

func TestListAgentRepoHandlesPaginatesUntilAShortPage(t *testing.T) {
	first := make([]string, 100)
	for i := range first {
		first[i] = fmt.Sprintf("repo-%d", i)
	}
	server := serveSequence(t, reply{status: 200, body: repoPage(t, first)}, reply{status: 200, body: repoPage(t, []string{"repo-100"})})

	handles, err := hubClient(t, server.URL).ListAgentRepoHandles(t.Context())

	if err != nil || len(handles) != 101 || handles[100] != "repo-100" {
		t.Fatalf("got %d handles, %v", len(handles), err)
	}
	requests := server.Requests()
	if strings.Join(requests[0].Query(t)["offset"], ",") != "0" || strings.Join(requests[1].Query(t)["offset"], ",") != "100" {
		t.Fatalf("offsets = %v, %v", requests[0].Query(t)["offset"], requests[1].Query(t)["offset"])
	}
}

func TestListAgentRepoHandlesStopsAtTheReportedTotal(t *testing.T) {
	first := make([]string, 100)
	for i := range first {
		first[i] = fmt.Sprintf("repo-%d", i)
	}
	page := strings.TrimSuffix(repoPage(t, first), "}") + `,"total":100}`
	// A full page whose `total` is already reached must not request another.
	handles, err := hubClient(t, serveOnce(t, 200, page).URL).ListAgentRepoHandles(t.Context())
	if err != nil || len(handles) != 100 {
		t.Fatalf("got %d handles, %v", len(handles), err)
	}
}
