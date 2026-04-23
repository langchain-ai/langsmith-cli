package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

// hubFileEntry is discriminated by Type: "file" carries Content; "agent" and
// "skill" link to child repos via RepoHandle/Owner/CommitHash.
type hubFileEntry struct {
	Type       string `json:"type"`
	Content    string `json:"content,omitempty"`
	RepoHandle string `json:"repo_handle,omitempty"`
	Owner      string `json:"owner,omitempty"`
	CommitHash string `json:"commit_hash,omitempty"`
}

type hubDirResponse struct {
	CommitID   string                  `json:"commit_id"`
	CommitHash string                  `json:"commit_hash"`
	Files      map[string]hubFileEntry `json:"files"`
}

type hubCommitResponse struct {
	Commit struct {
		ID         string `json:"id"`
		CommitHash string `json:"commit_hash"`
		CreatedAt  string `json:"created_at"`
	} `json:"commit"`
}

type hubRepo struct {
	ID             string   `json:"id"`
	FullName       string   `json:"full_name"`
	Owner          *string  `json:"owner"`
	RepoHandle     string   `json:"repo_handle"`
	RepoType       string   `json:"repo_type"`
	Description    *string  `json:"description"`
	Readme         *string  `json:"readme"`
	IsPublic       bool     `json:"is_public"`
	IsArchived     bool     `json:"is_archived"`
	Tags           []string `json:"tags"`
	NumCommits     int      `json:"num_commits"`
	NumLikes       int      `json:"num_likes"`
	NumDownloads   int      `json:"num_downloads"`
	NumViews       int      `json:"num_views"`
	LastCommitHash *string  `json:"last_commit_hash"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

type hubListResponse struct {
	Repos []hubRepo `json:"repos"`
	Total int       `json:"total"`
}

type hubRepoMeta struct {
	Description *string
	Readme      *string
	Tags        []string
	IsPublic    *bool
}

const (
	hubMaxFileEntries   = 500
	hubMaxFileSizeBytes = 1 << 20
)

var hubExcludedDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
	"dist":         true,
	"build":        true,
}

var hubExcludedFiles = map[string]bool{
	".DS_Store": true,
}

var hubRepoHandlePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

func newHubCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hub",
		Short: "Manage agent and skill repos on the LangSmith Hub",
		Long: `Manage agent and skill repos on the LangSmith Hub.

The hub stores versioned directories of files grouped into "repos" of type
"agent" or "skill". Each push creates a new commit; pull downloads a commit's
files into a local directory.

Prompts are managed via "langsmith prompt" — they use a different endpoint.

Examples:
  langsmith hub init --type skill --dir ./my-skill --name my-skill
  langsmith hub push my-skill --type skill --dir ./my-skill --public
  langsmith hub pull my-skill --dir ./out
  langsmith hub pull myorg/my-skill:production --dir ./out
  langsmith hub list --type agent --query summ
  langsmith hub get my-agent
  langsmith hub delete my-agent --yes`,
	}

	cmd.AddCommand(newHubInitCmd())
	cmd.AddCommand(newHubPushCmd())
	cmd.AddCommand(newHubPullCmd())
	cmd.AddCommand(newHubListCmd())
	cmd.AddCommand(newHubGetCmd())
	cmd.AddCommand(newHubDeleteCmd())
	return cmd
}

func newHubInitCmd() *cobra.Command {
	var (
		repoType    string
		dir         string
		name        string
		description string
		force       bool
	)

	cmd := &cobra.Command{
		Use:   "init --type agent|skill --dir PATH --name NAME",
		Short: "Scaffold a starter agent or skill directory",
		Long: `Scaffold a starter agent or skill directory.

Writes the minimal marker files the hub expects so you can edit them and then
run "langsmith hub push". Refuses to overwrite a non-empty directory unless
--force is passed.

Skill scaffold: SKILL.md (YAML frontmatter with name + description).
Agent scaffold: AGENTS.md, tools.json, config.json.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if repoType != "agent" && repoType != "skill" {
				return fmt.Errorf("--type must be 'agent' or 'skill' (got %q)", repoType)
			}
			if !hubRepoHandlePattern.MatchString(name) {
				return fmt.Errorf("--name %q must match %s", name, hubRepoHandlePattern.String())
			}
			written, err := scaffoldHubDirectory(dir, repoType, name, description, force)
			if err != nil {
				return err
			}
			sort.Strings(written)
			output.OutputJSON(map[string]any{
				"status": "scaffolded",
				"dir":    dir,
				"type":   repoType,
				"name":   name,
				"files":  written,
			}, "")
			return nil
		},
	}

	cmd.Flags().StringVar(&repoType, "type", "", "Repo type: agent or skill (required)")
	cmd.Flags().StringVar(&dir, "dir", "", "Target directory to scaffold into (required)")
	cmd.Flags().StringVar(&name, "name", "", "Handle for the repo; lowercase, a-z0-9-_ (required)")
	cmd.Flags().StringVar(&description, "description", "", "One-line description written into the scaffolded files")
	cmd.Flags().BoolVar(&force, "force", false, "Write even if the target directory is non-empty")
	_ = cmd.MarkFlagRequired("type")
	_ = cmd.MarkFlagRequired("dir")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newHubPushCmd() *cobra.Command {
	var (
		repoType     string
		dir          string
		parentCommit string
		description  string
		readmeFile   string
		tags         []string
		isPublic     bool
	)

	cmd := &cobra.Command{
		Use:   "push [OWNER/]REPO --type agent|skill --dir PATH",
		Short: "Push a local directory as an agent or skill commit",
		Long: `Push a local directory as an agent or skill commit.

Walks the directory, creates the repo if it does not yet exist, and uploads
all files as a single commit. Files under .git/, node_modules/, __pycache__/,
.venv/, venv/, dist/, build/, and .DS_Store are excluded. Symlinks are
skipped. Binary files fail the push with a clear error.

Identifier: "[owner/]repo". If owner is omitted, the current tenant is used.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			owner, name, _, err := parseOwnerRepo(args[0])
			if err != nil {
				return err
			}
			if repoType != "agent" && repoType != "skill" {
				return fmt.Errorf("--type must be 'agent' or 'skill' (got %q)", repoType)
			}

			files, err := readDirectoryAsFiles(dir)
			if err != nil {
				return err
			}
			if len(files) == 0 {
				return fmt.Errorf("no files found under %s (after applying exclusions)", dir)
			}

			var readme *string
			if readmeFile != "" {
				data, err := os.ReadFile(readmeFile)
				if err != nil {
					return fmt.Errorf("reading --readme file %q: %w", readmeFile, err)
				}
				s := string(data)
				readme = &s
			}

			meta := hubRepoMeta{Tags: tags}
			if description != "" {
				meta.Description = &description
			}
			if readme != nil {
				meta.Readme = readme
			}
			if cmd.Flags().Changed("public") {
				p := isPublic
				meta.IsPublic = &p
			}

			c := MustGetClient()
			ctx := context.Background()

			if err := ensureHubRepo(ctx, c, owner, name, repoType, meta); err != nil {
				return err
			}

			body := map[string]any{"files": files}
			if parentCommit != "" {
				body["parent_commit"] = parentCommit
			}
			path := fmt.Sprintf("/v1/platform/hub/repos/%s/%s/directories/commits", owner, name)
			var resp hubCommitResponse
			if err := c.RawPost(ctx, path, body, &resp); err != nil {
				return fmt.Errorf("pushing commit to %s/%s: %w", owner, name, err)
			}

			paths := sortedKeys(files)
			output.OutputJSON(map[string]any{
				"status":      "pushed",
				"owner":       owner,
				"repo":        name,
				"type":        repoType,
				"commit_id":   resp.Commit.ID,
				"commit_hash": resp.Commit.CommitHash,
				"created_at":  resp.Commit.CreatedAt,
				"files":       paths,
			}, "")
			return nil
		},
	}

	cmd.Flags().StringVar(&repoType, "type", "", "Repo type: agent or skill (required)")
	cmd.Flags().StringVar(&dir, "dir", "", "Local directory to upload (required)")
	cmd.Flags().StringVar(&parentCommit, "parent-commit", "", "Parent commit hash (8-64 chars)")
	cmd.Flags().StringVar(&description, "description", "", "Set repo description (updates metadata)")
	cmd.Flags().StringVar(&readmeFile, "readme", "", "Path to a README file (updates metadata)")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "Tag for the repo (repeatable)")
	cmd.Flags().BoolVar(&isPublic, "public", false, "Make the repo public")
	_ = cmd.MarkFlagRequired("type")
	_ = cmd.MarkFlagRequired("dir")
	return cmd
}

func newHubPullCmd() *cobra.Command {
	var (
		dir       string
		commitRef string
	)

	cmd := &cobra.Command{
		Use:   "pull [OWNER/]REPO[:COMMIT_OR_TAG] --dir PATH",
		Short: "Pull an agent or skill commit into a local directory",
		Long: `Pull an agent or skill commit into a local directory.

The destination is wiped before writing so removed upstream files don't
linger. Only file entries are written to disk — nested agent/skill link
entries are reported in the output JSON but not recursively resolved.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			owner, name, inlineRef, err := parseOwnerRepo(args[0])
			if err != nil {
				return err
			}
			ref := inlineRef
			if commitRef != "" {
				ref = commitRef
			}

			path := fmt.Sprintf("/v1/platform/hub/repos/%s/%s/directories", owner, name)
			if ref != "" {
				path += "?commit=" + url.QueryEscape(ref)
			}

			c := MustGetClient()
			ctx := context.Background()

			var resp hubDirResponse
			if err := c.RawGet(ctx, path, &resp); err != nil {
				return fmt.Errorf("pulling %s/%s: %w", owner, name, err)
			}

			written, linked, err := writeFilesToDirectory(dir, resp.Files)
			if err != nil {
				return err
			}
			sort.Strings(written)
			sort.Strings(linked)

			absDir, _ := filepath.Abs(dir)
			out := map[string]any{
				"status":      "pulled",
				"owner":       owner,
				"repo":        name,
				"commit_id":   resp.CommitID,
				"commit_hash": resp.CommitHash,
				"dir":         absDir,
				"files":       written,
			}
			if len(linked) > 0 {
				out["linked_children"] = linked
			}
			output.OutputJSON(out, "")
			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "Destination directory (required; will be wiped before writing)")
	cmd.Flags().StringVar(&commitRef, "commit", "", "Commit hash or tag name (overrides inline :ref)")
	_ = cmd.MarkFlagRequired("dir")
	return cmd
}

func newHubListCmd() *cobra.Command {
	var (
		repoType   string
		query      string
		publicOnly bool
		limit      int
		offset     int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List hub repos (optionally filtered by type)",
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{}
			params.Set("limit", fmt.Sprintf("%d", limit))
			params.Set("offset", fmt.Sprintf("%d", offset))
			params.Set("is_archived", "false")
			if repoType != "" {
				if repoType != "agent" && repoType != "skill" {
					return fmt.Errorf("--type must be 'agent' or 'skill' when set (got %q)", repoType)
				}
				params.Set("repo_type", repoType)
			}
			if query != "" {
				params.Set("query", query)
				params.Set("match_prefix", "true")
			}
			if cmd.Flags().Changed("public") {
				if publicOnly {
					params.Set("is_public", "true")
				} else {
					params.Set("is_public", "false")
				}
			}

			c := MustGetClient()
			ctx := context.Background()

			var resp hubListResponse
			if err := c.RawGet(ctx, "/repos/?"+params.Encode(), &resp); err != nil {
				return fmt.Errorf("listing hub repos: %w", err)
			}

			if GetFormat() == "pretty" {
				columns := []string{"Full Name", "Type", "Public", "Commits", "Updated"}
				var rows [][]string
				for _, r := range resp.Repos {
					pub := "private"
					if r.IsPublic {
						pub = "public"
					}
					rows = append(rows, []string{
						r.FullName,
						r.RepoType,
						pub,
						fmt.Sprintf("%d", r.NumCommits),
						r.UpdatedAt,
					})
				}
				output.OutputTable(columns, rows, "Hub repos")
				return nil
			}

			output.OutputJSON(map[string]any{
				"total": resp.Total,
				"repos": resp.Repos,
			}, "")
			return nil
		},
	}

	cmd.Flags().StringVar(&repoType, "type", "", "Filter by repo type: agent or skill")
	cmd.Flags().StringVar(&query, "query", "", "Filter by name substring")
	cmd.Flags().BoolVar(&publicOnly, "public", false, "Filter by public/private")
	cmd.Flags().IntVarP(&limit, "limit", "n", 100, "Maximum number of repos to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "Offset for pagination")
	return cmd
}

func newHubGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [OWNER/]REPO",
		Short: "Get metadata for a hub repo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			owner, name, _, err := parseOwnerRepo(args[0])
			if err != nil {
				return err
			}
			c := MustGetClient()
			ctx := context.Background()

			path := fmt.Sprintf("/repos/%s/%s", owner, name)
			var envelope struct {
				Repo hubRepo `json:"repo"`
			}
			if err := c.RawGet(ctx, path, &envelope); err != nil {
				return fmt.Errorf("getting %s/%s: %w", owner, name, err)
			}
			output.OutputJSON(envelope.Repo, "")
			return nil
		},
	}
	return cmd
}

func newHubDeleteCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete [OWNER/]REPO",
		Short: "Delete an agent or skill repo (and its owned child repos)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			owner, name, _, err := parseOwnerRepo(args[0])
			if err != nil {
				return err
			}
			if !yes {
				fmt.Fprintf(os.Stderr, "Delete hub repo '%s/%s'? [y/N] ", owner, name)
				var confirm string
				_, _ = fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					return errors.New("aborted")
				}
			}
			c := MustGetClient()
			ctx := context.Background()

			path := fmt.Sprintf("/v1/platform/hub/repos/%s/%s/directories", owner, name)
			if err := c.RawDelete(ctx, path, nil); err != nil {
				return fmt.Errorf("deleting %s/%s: %w", owner, name, err)
			}
			output.OutputJSON(map[string]any{
				"status": "deleted",
				"owner":  owner,
				"repo":   name,
			}, "")
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	return cmd
}

func readDirectoryAsFiles(root string) (map[string]hubFileEntry, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	files := make(map[string]hubFileEntry)
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		base := d.Name()
		if d.IsDir() {
			if hubExcludedDirs[base] {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if hubExcludedFiles[base] {
			return nil
		}
		if strings.HasSuffix(base, ".pyc") {
			return nil
		}

		rel = filepath.ToSlash(rel)

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", rel, err)
		}
		if len(data) > hubMaxFileSizeBytes {
			return fmt.Errorf("file %s is %d bytes; exceeds %d-byte limit", rel, len(data), hubMaxFileSizeBytes)
		}
		if isBinary(data) {
			return fmt.Errorf("file %s appears to be binary; binary files are not supported (exclude it or encode manually)", rel)
		}

		files[rel] = hubFileEntry{Type: "file", Content: string(data)}
		if len(files) > hubMaxFileEntries {
			return fmt.Errorf("directory contains more than %d files; reduce before pushing", hubMaxFileEntries)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func isBinary(data []byte) bool {
	n := len(data)
	if n > 512 {
		n = 512
	}
	return bytes.IndexByte(data[:n], 0) != -1
}

func writeFilesToDirectory(dest string, files map[string]hubFileEntry) (written []string, linked []string, err error) {
	if err := os.RemoveAll(dest); err != nil {
		return nil, nil, fmt.Errorf("cleaning %s: %w", dest, err)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, nil, fmt.Errorf("creating %s: %w", dest, err)
	}
	for path, entry := range files {
		if entry.Type != "file" {
			linked = append(linked, fmt.Sprintf("%s (%s)", path, entry.Type))
			continue
		}
		out := filepath.Join(dest, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return nil, nil, fmt.Errorf("creating directory for %s: %w", path, err)
		}
		if err := os.WriteFile(out, []byte(entry.Content), 0o644); err != nil {
			return nil, nil, fmt.Errorf("writing %s: %w", path, err)
		}
		written = append(written, path)
	}
	return written, linked, nil
}

func ensureHubRepo(ctx context.Context, c *client.Client, owner, name, repoType string, meta hubRepoMeta) error {
	getPath := fmt.Sprintf("/repos/%s/%s", owner, name)
	var existing map[string]any
	err := c.RawGet(ctx, getPath, &existing)
	if err == nil {
		if meta.Description != nil || meta.Readme != nil || meta.Tags != nil || meta.IsPublic != nil {
			body := map[string]any{}
			if meta.Description != nil {
				body["description"] = *meta.Description
			}
			if meta.Readme != nil {
				body["readme"] = *meta.Readme
			}
			if meta.Tags != nil {
				body["tags"] = meta.Tags
			}
			if meta.IsPublic != nil {
				body["is_public"] = *meta.IsPublic
			}
			if err := c.RawPatch(ctx, getPath, body, nil); err != nil {
				return fmt.Errorf("updating metadata for %s/%s: %w", owner, name, err)
			}
		}
		return nil
	}
	if !isHTTP404(err) {
		return fmt.Errorf("checking %s/%s: %w", owner, name, err)
	}

	if !hubRepoHandlePattern.MatchString(name) {
		return fmt.Errorf("repo handle %q must match %s", name, hubRepoHandlePattern.String())
	}
	create := map[string]any{
		"repo_handle": name,
		"repo_type":   repoType,
		"is_public":   false,
	}
	if meta.IsPublic != nil {
		create["is_public"] = *meta.IsPublic
	}
	if meta.Description != nil {
		create["description"] = *meta.Description
	}
	if meta.Readme != nil {
		create["readme"] = *meta.Readme
	}
	if meta.Tags != nil {
		create["tags"] = meta.Tags
	}
	if err := c.RawPost(ctx, "/repos/", create, nil); err != nil {
		if isHTTP409(err) {
			return nil
		}
		return fmt.Errorf("creating %s/%s: %w", owner, name, err)
	}
	return nil
}

// The raw client wraps non-2xx responses as fmt.Errorf("HTTP %d: %s", ...),
// so status detection is prefix-matching on the error string.
func isHTTP404(err error) bool {
	return err != nil && strings.Contains(err.Error(), "HTTP 404")
}

func isHTTP409(err error) bool {
	return err != nil && strings.Contains(err.Error(), "HTTP 409")
}

func scaffoldHubDirectory(dir, repoType, name, description string, force bool) ([]string, error) {
	if repoType != "agent" && repoType != "skill" {
		return nil, fmt.Errorf("repoType must be 'agent' or 'skill' (got %q)", repoType)
	}
	if !hubRepoHandlePattern.MatchString(name) {
		return nil, fmt.Errorf("name %q must match %s", name, hubRepoHandlePattern.String())
	}

	if info, err := os.Stat(dir); err == nil {
		if !info.IsDir() {
			return nil, fmt.Errorf("%s exists and is not a directory", dir)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", dir, err)
		}
		if len(entries) > 0 && !force {
			return nil, fmt.Errorf("%s is not empty; pass --force to write anyway", dir)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat %s: %w", dir, err)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating %s: %w", dir, err)
	}

	desc := description
	if desc == "" {
		desc = "TODO: one-sentence description of what this " + repoType + " does."
	}

	var templates map[string]string
	switch repoType {
	case "skill":
		templates = map[string]string{
			"SKILL.md": fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n# %s\n\n<!-- Describe when and how this skill should be used. -->\n",
				name, escapeYAMLScalar(desc), name),
		}
	case "agent":
		templates = map[string]string{
			"AGENTS.md":   fmt.Sprintf("# %s\n\n%s\n\n<!-- Usage rules, tool policies, and operational guardrails for this agent go here. -->\n", name, desc),
			"tools.json":  "{\n  \"tools\": [],\n  \"interrupt_config\": {}\n}\n",
			"config.json": "{}\n",
		}
	}

	var written []string
	for rel, content := range templates {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return nil, fmt.Errorf("creating directory for %s: %w", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", rel, err)
		}
		written = append(written, rel)
	}
	return written, nil
}

// Scaffolded descriptions are one-liners, so a full YAML serializer is
// overkill — quote only when the scalar would otherwise misparse.
func escapeYAMLScalar(s string) string {
	if s == "" {
		return "\"\""
	}
	if strings.ContainsAny(s, ":#\n\"'") || strings.HasPrefix(s, "-") || strings.HasPrefix(s, " ") {
		b, _ := json.Marshal(s)
		return string(b)
	}
	return s
}

func sortedKeys(m map[string]hubFileEntry) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
