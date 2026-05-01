package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

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
