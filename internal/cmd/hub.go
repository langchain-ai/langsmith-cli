package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var hubRepoHandlePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

type hubListResponse struct {
	Repos []hubRepo `json:"repos"`
	Total int       `json:"total"`
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
	return cmd
}
