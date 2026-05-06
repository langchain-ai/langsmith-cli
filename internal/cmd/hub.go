package cmd

import (
	"github.com/spf13/cobra"
)

type hubFileEntry struct {
	Type       string `json:"type"`
	Content    string `json:"content,omitempty"`
	RepoHandle string `json:"repo_handle,omitempty"`
	Owner      string `json:"owner,omitempty"`
	CommitHash string `json:"commit_hash,omitempty"`
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
	".DS_Store":        true,
	".env":             true,
	".env.local":       true,
	".env.production":  true,
	".env.development": true,
	"Thumbs.db":        true,
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
