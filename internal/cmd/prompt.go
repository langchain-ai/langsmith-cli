package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

func parseOwnerRepo(arg string) (owner, repo, commit string, err error) {
	repoCommit := arg
	if idx := strings.Index(arg, ":"); idx != -1 {
		repoCommit = arg[:idx]
		commit = arg[idx+1:]
	}
	parts := strings.SplitN(repoCommit, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", fmt.Errorf("invalid format %q — expected owner/repo or owner/repo:commit", arg)
	}
	return parts[0], parts[1], commit, nil
}

func newPromptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prompt",
		Short: "Manage Prompt Hub repos, commits, and tags",
		Long: `Manage Prompt Hub repos, commits, and tags.

The Prompt Hub lets you version and share prompt templates.
Each prompt is a repo that contains a series of commits (versions)
identified by hash or named tag.

Examples:
  langsmith prompt list
  langsmith prompt list --query "summarize" --public
  langsmith prompt get myorg/my-prompt
  langsmith prompt pull myorg/my-prompt
  langsmith prompt pull myorg/my-prompt:production
  langsmith prompt push myorg/my-prompt --file manifest.json
  langsmith prompt create --name my-prompt --public
  langsmith prompt delete myorg/my-prompt
  langsmith prompt commits myorg/my-prompt`,
	}

	cmd.AddCommand(newPromptListCmd())
	cmd.AddCommand(newPromptGetCmd())
	cmd.AddCommand(newPromptCreateCmd())
	cmd.AddCommand(newPromptDeleteCmd())
	cmd.AddCommand(newPromptPullCmd())
	cmd.AddCommand(newPromptPushCmd())
	cmd.AddCommand(newPromptCommitsCmd())
	cmd.AddCommand(newPromptTagCmd())

	return cmd
}

func newPromptListCmd() *cobra.Command {
	var (
		limit      int
		query      string
		tags       []string
		publicOnly bool
		sortBy     string
		outputFile string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Prompt Hub repos",
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			ctx := context.Background()

			pageSize := int64(20)
			if limit > 0 && int64(limit) < pageSize {
				pageSize = int64(limit)
			}
			params := langsmith.RepoListParams{
				Limit: langsmith.F(pageSize),
			}
			if query != "" {
				params.Query = langsmith.F(query)
			}
			if len(tags) > 0 {
				params.Tags = langsmith.F(tags)
			}
			if cmd.Flags().Changed("public") {
				if publicOnly {
					params.IsPublic = langsmith.F(langsmith.RepoListParamsIsPublicTrue)
				} else {
					params.IsPublic = langsmith.F(langsmith.RepoListParamsIsPublicFalse)
				}
			}
			if sortBy != "" {
				switch sortBy {
				case "likes":
					params.SortField = langsmith.F(langsmith.RepoListParamsSortFieldNumLikes)
				case "downloads":
					params.SortField = langsmith.F(langsmith.RepoListParamsSortFieldNumDownloads)
				case "views":
					params.SortField = langsmith.F(langsmith.RepoListParamsSortFieldNumViews)
				case "updated":
					params.SortField = langsmith.F(langsmith.RepoListParamsSortFieldUpdatedAt)
				default:
					ExitErrorf("unknown --sort-by %q (valid: likes, downloads, views, updated)", sortBy)
				}
			}

			var repos []langsmith.RepoWithLookups
			pager := c.SDK.Repos.ListAutoPaging(ctx, params)
			for pager.Next() {
				repos = append(repos, pager.Current())
				if limit > 0 && len(repos) >= limit {
					break
				}
			}
			if err := pager.Err(); err != nil {
				ExitErrorf("listing prompts: %v", err)
			}

			fmt_ := GetFormat()

			if fmt_ == "pretty" {
				columns := []string{"Full Name", "Public", "Commits", "Likes", "Updated"}
				var rows [][]string
				for _, r := range repos {
					pub := "private"
					if r.IsPublic {
						pub = "public"
					}
					rows = append(rows, []string{
						r.FullName,
						pub,
						fmt.Sprintf("%d", r.NumCommits),
						fmt.Sprintf("%d", r.NumLikes),
						formatTimeShort(r.UpdatedAt),
					})
				}
				output.OutputTable(columns, rows, "Prompts")
			} else {
				var data []map[string]any
				for _, r := range repos {
					data = append(data, repoToMap(r))
				}
				outputJSON(data, outputFile)
			}
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 100, "Maximum number of prompts to return")
	cmd.Flags().StringVar(&query, "query", "", "Filter by name/description substring")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "Filter by tags (comma-separated)")
	cmd.Flags().BoolVar(&publicOnly, "public", false, "Show only public repos")
	cmd.Flags().StringVar(&sortBy, "sort-by", "", "Sort by: likes, downloads, views, updated")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
}

func newPromptGetCmd() *cobra.Command {
	var outputFile string

	cmd := &cobra.Command{
		Use:   "get OWNER/REPO",
		Short: "Get a Prompt Hub repo and its latest manifest",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			owner, repo, _, err := parseOwnerRepo(args[0])
			if err != nil {
				ExitErrorf("%v", err)
			}

			c := MustGetClient()
			ctx := context.Background()

			resp, err := c.SDK.Repos.Get(ctx, owner, repo)
			if err != nil {
				ExitErrorf("getting prompt %s/%s: %v", owner, repo, err)
			}

			r := resp.Repo
			data := repoToMap(r)
			if r.LatestCommitManifest.CommitHash != "" {
				data["latest_commit_hash"] = r.LatestCommitManifest.CommitHash
				data["manifest"] = r.LatestCommitManifest.Manifest
			}

			fmt_ := GetFormat()
			if fmt_ == "pretty" {
				printOutput(data, "pretty", outputFile)
			} else {
				outputJSON(data, outputFile)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	return cmd
}

func newPromptCreateCmd() *cobra.Command {
	var (
		name        string
		description string
		isPublic    bool
		tags        []string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new Prompt Hub repo",
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			ctx := context.Background()

			params := langsmith.RepoNewParams{
				RepoHandle: langsmith.F(name),
				IsPublic:   langsmith.F(isPublic),
			}
			if description != "" {
				params.Description = langsmith.F(description)
			}
			if len(tags) > 0 {
				params.Tags = langsmith.F(tags)
			}

			resp, err := c.SDK.Repos.New(ctx, params)
			if err != nil {
				ExitErrorf("creating prompt: %v", err)
			}

			outputJSON(repoToMap(resp.Repo), "")
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Repo handle / slug (required)")
	cmd.Flags().StringVar(&description, "description", "", "Optional description")
	cmd.Flags().BoolVar(&isPublic, "public", false, "Make the repo public")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "Tags for the repo (comma-separated)")
	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func newPromptDeleteCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete OWNER/REPO",
		Short: "Delete a Prompt Hub repo",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			owner, repo, _, err := parseOwnerRepo(args[0])
			if err != nil {
				ExitErrorf("%v", err)
			}

			if !yes {
				fmt.Fprintf(os.Stderr, "Delete prompt '%s/%s'? [y/N] ", owner, repo)
				var confirm string
				_, _ = fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					ExitError("aborted")
				}
			}

			c := MustGetClient()
			ctx := context.Background()

			_, err = c.SDK.Repos.Delete(ctx, owner, repo)
			if err != nil {
				ExitErrorf("deleting prompt %s/%s: %v", owner, repo, err)
			}

			outputJSON(map[string]any{
				"status": "deleted",
				"owner":  owner,
				"repo":   repo,
			}, "")
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	return cmd
}

func newPromptPullCmd() *cobra.Command {
	var (
		commitRef  string
		outputFile string
	)

	cmd := &cobra.Command{
		Use:   "pull OWNER/REPO[:COMMIT_OR_TAG]",
		Short: "Fetch a prompt's manifest (latest or a specific commit/tag)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			owner, repo, inlineRef, err := parseOwnerRepo(args[0])
			if err != nil {
				ExitErrorf("%v", err)
			}

			ref := "latest"
			if inlineRef != "" {
				ref = inlineRef
			}
			if commitRef != "" {
				ref = commitRef
			}

			c := MustGetClient()
			ctx := context.Background()

			resp, err := c.SDK.Commits.Get(ctx, owner, repo, ref, langsmith.CommitGetParams{})
			if err != nil {
				ExitErrorf("pulling %s/%s@%s: %v", owner, repo, ref, err)
			}

			data := map[string]any{
				"owner":       owner,
				"repo":        repo,
				"commit_hash": resp.CommitHash,
				"description": resp.Description,
				"manifest":    resp.Manifest,
			}

			fmt_ := GetFormat()
			if fmt_ == "pretty" {
				printOutput(data, "pretty", outputFile)
			} else {
				outputJSON(data, outputFile)
			}
		},
	}

	cmd.Flags().StringVar(&commitRef, "commit", "", "Commit hash or tag name (overrides inline :ref)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	return cmd
}

func newPromptPushCmd() *cobra.Command {
	var (
		manifestFile string
		parentCommit string
		description  string
	)

	cmd := &cobra.Command{
		Use:   "push OWNER/REPO",
		Short: "Push a new commit (manifest) to a Prompt Hub repo",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			owner, repo, _, err := parseOwnerRepo(args[0])
			if err != nil {
				ExitErrorf("%v", err)
			}

			var manifestBytes []byte
			if manifestFile != "" {
				manifestBytes, err = os.ReadFile(manifestFile)
				if err != nil {
					ExitErrorf("reading manifest file: %v", err)
				}
			} else {
				manifestBytes, err = io.ReadAll(os.Stdin)
				if err != nil {
					ExitErrorf("reading stdin: %v", err)
				}
			}

			var manifest any
			if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
				ExitErrorf("parsing manifest JSON: %v", err)
			}

			c := MustGetClient()
			ctx := context.Background()

			params := langsmith.CommitNewParams{
				Manifest: langsmith.F(manifest),
			}
			if parentCommit != "" {
				params.ParentCommit = langsmith.F(parentCommit)
			}
			if description != "" {
				params.Description = langsmith.F(description)
			}

			resp, err := c.SDK.Commits.New(ctx, owner, repo, params)
			if err != nil {
				ExitErrorf("pushing commit to %s/%s: %v", owner, repo, err)
			}

			outputJSON(map[string]any{
				"status":      "pushed",
				"owner":       owner,
				"repo":        repo,
				"commit_hash": resp.Commit.CommitHash,
				"commit_id":   resp.Commit.ID,
				"created_at":  formatTimeISO(resp.Commit.CreatedAt),
			}, "")
		},
	}

	cmd.Flags().StringVarP(&manifestFile, "file", "f", "", "Path to manifest JSON file (reads stdin if omitted)")
	cmd.Flags().StringVar(&parentCommit, "parent-commit", "", "Parent commit hash (optional)")
	cmd.Flags().StringVar(&description, "description", "", "Commit description / message (optional)")
	return cmd
}

func newPromptCommitsCmd() *cobra.Command {
	var (
		limit      int
		outputFile string
	)

	cmd := &cobra.Command{
		Use:   "commits OWNER/REPO",
		Short: "List commits for a Prompt Hub repo",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			owner, repo, _, err := parseOwnerRepo(args[0])
			if err != nil {
				ExitErrorf("%v", err)
			}

			c := MustGetClient()
			ctx := context.Background()

			pageSize := int64(20)
			if limit > 0 && int64(limit) < pageSize {
				pageSize = int64(limit)
			}

			var commits []langsmith.CommitWithLookups
			pager := c.SDK.Commits.ListAutoPaging(ctx, owner, repo, langsmith.CommitListParams{
				Limit: langsmith.F(pageSize),
			})
			for pager.Next() {
				commits = append(commits, pager.Current())
				if limit > 0 && len(commits) >= limit {
					break
				}
			}
			if err := pager.Err(); err != nil {
				ExitErrorf("listing commits: %v", err)
			}

			fmt_ := GetFormat()

			if fmt_ == "pretty" {
				columns := []string{"Hash", "Author", "Description", "Downloads", "Created"}
				var rows [][]string
				for _, c := range commits {
					hash := c.CommitHash
					if len(hash) > 12 {
						hash = hash[:12]
					}
					rows = append(rows, []string{
						hash,
						c.FullName,
						truncate(c.Description, 40),
						fmt.Sprintf("%d", c.NumDownloads),
						formatTimeShort(c.CreatedAt),
					})
				}
				output.OutputTable(columns, rows, fmt.Sprintf("Commits for %s/%s", owner, repo))
			} else {
				var data []map[string]any
				for _, c := range commits {
					data = append(data, map[string]any{
						"id":                 c.ID,
						"commit_hash":        c.CommitHash,
						"parent_commit_hash": nilStr(c.ParentCommitHash),
						"description":        c.Description,
						"full_name":          c.FullName,
						"num_downloads":      c.NumDownloads,
						"num_views":          c.NumViews,
						"created_at":         formatTimeISO(c.CreatedAt),
					})
				}
				outputJSON(data, outputFile)
			}
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 25, "Maximum number of commits to return")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	return cmd
}

// repoTag matches the JSON returned by the hub tag endpoints.
type repoTag struct {
	ID         string `json:"id"`
	TagName    string `json:"tag_name"`
	CommitID   string `json:"commit_id"`
	CommitHash string `json:"commit_hash"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

func newPromptTagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "Manage commit tags for a Prompt Hub repo",
		Long: `Manage commit tags for a Prompt Hub repo.

Commit tags are named pointers to specific commits (e.g. "production", "staging").
Each org has a fixed set of tags — use them to promote a commit to an environment
without changing the repo URL that consumers depend on.

Examples:
  langsmith prompt tag list myorg/my-prompt
  langsmith prompt tag create myorg/my-prompt --tag production --commit-id <uuid>
  langsmith prompt tag update myorg/my-prompt production --commit-id <uuid>`,
	}

	cmd.AddCommand(newPromptTagListCmd())
	cmd.AddCommand(newPromptTagCreateCmd())
	cmd.AddCommand(newPromptTagUpdateCmd())
	return cmd
}

func newPromptTagListCmd() *cobra.Command {
	var outputFile string

	cmd := &cobra.Command{
		Use:   "list OWNER/REPO",
		Short: "List all commit tags for a prompt repo",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			owner, repo, _, err := parseOwnerRepo(args[0])
			if err != nil {
				ExitErrorf("%v", err)
			}

			c := MustGetClient()
			ctx := context.Background()

			var tags []repoTag
			path := fmt.Sprintf("/api/v1/repos/%s/%s/tags", owner, repo)
			if err := c.RawGet(ctx, path, &tags); err != nil {
				ExitErrorf("listing tags for %s/%s: %v", owner, repo, err)
			}

			fmt_ := GetFormat()

			if fmt_ == "pretty" {
				columns := []string{"Tag", "Commit ID", "Created"}
				var rows [][]string
				for _, t := range tags {
					rows = append(rows, []string{t.TagName, t.CommitID, t.CreatedAt})
				}
				output.OutputTable(columns, rows, fmt.Sprintf("Tags for %s/%s", owner, repo))
			} else {
				var data []map[string]any
				for _, t := range tags {
					data = append(data, map[string]any{
						"id":          t.ID,
						"tag_name":    t.TagName,
						"commit_id":   t.CommitID,
						"commit_hash": t.CommitHash,
						"created_at":  t.CreatedAt,
						"updated_at":  t.UpdatedAt,
					})
				}
				outputJSON(data, outputFile)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	return cmd
}

func newPromptTagCreateCmd() *cobra.Command {
	var (
		tagName  string
		commitID string
	)

	cmd := &cobra.Command{
		Use:   "create OWNER/REPO",
		Short: "Create a named tag pointing to a specific commit",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			owner, repo, _, err := parseOwnerRepo(args[0])
			if err != nil {
				ExitErrorf("%v", err)
			}

			c := MustGetClient()
			ctx := context.Background()

			body := map[string]any{
				"tag_name":  tagName,
				"commit_id": commitID,
			}
			var tag repoTag
			path := fmt.Sprintf("/api/v1/repos/%s/%s/tags", owner, repo)
			if err := c.RawPost(ctx, path, body, &tag); err != nil {
				ExitErrorf("creating tag %q: %v", tagName, err)
			}

			outputJSON(map[string]any{
				"status":      "created",
				"tag_name":    tag.TagName,
				"commit_id":   tag.CommitID,
				"commit_hash": tag.CommitHash,
			}, "")
		},
	}

	cmd.Flags().StringVar(&tagName, "tag", "", "Tag name (e.g. production, staging) (required)")
	cmd.Flags().StringVar(&commitID, "commit-id", "", "Commit UUID to point this tag at (required)")
	_ = cmd.MarkFlagRequired("tag")
	_ = cmd.MarkFlagRequired("commit-id")
	return cmd
}

func newPromptTagUpdateCmd() *cobra.Command {
	var commitID string

	cmd := &cobra.Command{
		Use:   "update OWNER/REPO TAG_NAME",
		Short: "Move a tag to a different commit",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			owner, repo, _, err := parseOwnerRepo(args[0])
			if err != nil {
				ExitErrorf("%v", err)
			}
			tagName := args[1]

			c := MustGetClient()
			ctx := context.Background()

			body := map[string]any{
				"tag_name":  tagName,
				"commit_id": commitID,
			}
			var tag repoTag
			path := fmt.Sprintf("/api/v1/repos/%s/%s/tags", owner, repo)
			if err := c.RawPost(ctx, path, body, &tag); err != nil {
				ExitErrorf("updating tag %q: %v", tagName, err)
			}

			outputJSON(map[string]any{
				"status":      "updated",
				"tag_name":    tag.TagName,
				"commit_id":   tag.CommitID,
				"commit_hash": tag.CommitHash,
			}, "")
		},
	}

	cmd.Flags().StringVar(&commitID, "commit-id", "", "New commit UUID to point this tag at (required)")
	_ = cmd.MarkFlagRequired("commit-id")
	return cmd
}

func repoToMap(r langsmith.RepoWithLookups) map[string]any {
	return map[string]any{
		"id":               r.ID,
		"full_name":        r.FullName,
		"owner":            nilStr(r.Owner),
		"repo_handle":      r.RepoHandle,
		"description":      nilStr(r.Description),
		"is_public":        r.IsPublic,
		"is_archived":      r.IsArchived,
		"tags":             r.Tags,
		"num_commits":      r.NumCommits,
		"num_likes":        r.NumLikes,
		"num_downloads":    r.NumDownloads,
		"num_views":        r.NumViews,
		"last_commit_hash": nilStr(r.LastCommitHash),
		"created_at":       formatTimeISO(r.CreatedAt),
		"updated_at":       formatTimeISO(r.UpdatedAt),
	}
}
