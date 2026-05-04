package cmd

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/spf13/cobra"
)

var hubRepoHandlePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

type hubListResponse struct {
	Repos []hubRepo `json:"repos"`
	Total int       `json:"total"`
}

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
	"target":       true,
	".next":        true,
	".cache":       true,
}

var hubExcludedFiles = map[string]bool{
	".DS_Store":       true,
	".env":            true,
	".env.local":      true,
	".env.production": true,
	".env.development": true,
	"Thumbs.db":       true,
}

var hubExcludedSuffixes = []string{".pyc", ".pem", ".key", ".pfx", ".p12", ".crt"}

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
	LastCommitHash *string  `json:"last_commit_hash"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

// parseHubOwnerRepo splits "[owner/]repo[:ref]"; missing owner becomes "-" (API "current tenant").
func parseHubOwnerRepo(arg string) (string, string, string, error) {
	if arg == "" {
		return "", "", "", fmt.Errorf("empty repo identifier")
	}

	rest := arg
	ref := ""
	if i := strings.Index(rest, ":"); i >= 0 {
		ref = rest[i+1:]
		rest = rest[:i]
	}

	owner := "-"
	name := rest
	if i := strings.Index(rest, "/"); i >= 0 {
		owner = rest[:i]
		name = rest[i+1:]
	}

	if owner == "" || name == "" {
		return "", "", "", fmt.Errorf("invalid repo identifier %q (expected [OWNER/]REPO[:REF])", arg)
	}
	if strings.Contains(name, "/") {
		return "", "", "", fmt.Errorf("invalid repo identifier %q (too many '/' separators)", arg)
	}
	if owner == "-" && !hubRepoHandlePattern.MatchString(name) {
		return "", "", "", fmt.Errorf("invalid repo handle %q (must match %s)", name, hubRepoHandlePattern.String())
	}

	return owner, name, ref, nil
}

func isHTTP404(err error) bool {
	return err != nil && strings.Contains(err.Error(), "HTTP 404")
}

func isHTTP409(err error) bool {
	return err != nil && strings.Contains(err.Error(), "HTTP 409")
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
  langsmith hub list --type agent --query summ
  langsmith hub get my-agent
  langsmith hub delete my-agent --yes`,
	}
	cmd.AddCommand(newHubInitCmd())
	cmd.AddCommand(newHubGetCmd())
	cmd.AddCommand(newHubListCmd())
	cmd.AddCommand(newHubDeleteCmd())
	cmd.AddCommand(newHubPullCmd())
	cmd.AddCommand(newHubPushCmd())
	return cmd
}
