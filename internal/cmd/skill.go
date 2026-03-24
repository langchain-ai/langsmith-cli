package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type directoryFileEntry struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
}

type directoryResponse struct {
	CommitID   string                        `json:"commit_id"`
	CommitHash string                        `json:"commit_hash"`
	Files      map[string]directoryFileEntry `json:"files"`
}

type skillPullOutput struct {
	Skill     string   `json:"skill"`
	Canonical string   `json:"canonical_path"`
	LinkedTo  []string `json:"linked_to,omitempty"`
	Files     []string `json:"files"`
}

var agentSkillDirs = map[string]string{
	"claude": ".claude/skills",
	"cursor": ".cursor/skills",
	"codex":  ".codex/skills",
}

func newFleetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fleet",
		Short: "Interact with Fleet",
	}
	cmd.AddCommand(newFleetSkillsCmd())
	return cmd
}

func newFleetSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Manage Fleet skills",
	}
	cmd.AddCommand(newSkillPullCmd())
	return cmd
}

func newSkillPullCmd() *cobra.Command {
	var (
		agents    []string
		global    bool
		forceCopy bool
	)

	cmd := &cobra.Command{
		Use:   "pull <skill-name>",
		Short: "Download a Fleet skill and install it for your coding agent",
		Long: `Download a skill from Fleet and install it locally.

By default, files are saved to ~/.agents/skills/<skill-name>/ and
symlinked into ~/.claude/skills/<skill-name>/.

Use --global=false for project-level install (.agents/ and .claude/).
Use --agent to target other agents (claude, cursor, codex).
Use --copy to copy instead of symlink.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			skillName := args[0]
			c := mustGetClient()

			var canonicalDir string
			if global {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("resolving home directory: %w", err)
				}
				canonicalDir = filepath.Join(home, ".agents", "skills", skillName)
			} else {
				canonicalDir = filepath.Join(".agents", "skills", skillName)
			}

			apiPath := fmt.Sprintf("/v1/platform/hub/repos/-/%s/directories", skillName)
			var resp directoryResponse
			if err := c.RawGet(context.Background(), apiPath, &resp); err != nil {
				return fmt.Errorf("fetching skill %q: %w", skillName, err)
			}

			if _, ok := resp.Files["SKILL.md"]; !ok {
				return fmt.Errorf("%q is not a skill", skillName)
			}

			// Wipe before writing so removed upstream files don't linger.
			if err := os.RemoveAll(canonicalDir); err != nil {
				return fmt.Errorf("cleaning %s: %w", canonicalDir, err)
			}

			var written []string
			for filePath, entry := range resp.Files {
				if entry.Type != "file" {
					continue
				}
				dest := filepath.Join(canonicalDir, filePath)
				if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
					return fmt.Errorf("creating directory for %s: %w", filePath, err)
				}
				if err := os.WriteFile(dest, []byte(entry.Content), 0o644); err != nil {
					return fmt.Errorf("writing %s: %w", filePath, err)
				}
				written = append(written, filePath)
			}
			sort.Strings(written)

			canonicalAbs, err := filepath.Abs(canonicalDir)
			if err != nil {
				return fmt.Errorf("resolving canonical path: %w", err)
			}

			var linked []string
			for _, agent := range agents {
				relDir, ok := agentSkillDirs[agent]
				if !ok {
					return fmt.Errorf("unknown agent %q (valid: claude, cursor, codex)", agent)
				}

				var agentDir string
				if global {
					home, _ := os.UserHomeDir()
					agentDir = filepath.Join(home, relDir)
				} else {
					agentDir = relDir
				}
				linkPath := filepath.Join(agentDir, skillName)

				if err := os.MkdirAll(agentDir, 0o755); err != nil {
					return fmt.Errorf("creating %s skills dir: %w", agent, err)
				}

				// Replace existing symlink; refuse to clobber a real directory.
				if existing, err := os.Lstat(linkPath); err == nil {
					if existing.Mode()&os.ModeSymlink != 0 {
						os.Remove(linkPath)
					} else if existing.IsDir() {
						return fmt.Errorf("%s exists and is not a symlink — remove it first", linkPath)
					}
				}

				if forceCopy {
					if err := copyDir(canonicalAbs, linkPath); err != nil {
						return fmt.Errorf("copying to %s: %w", linkPath, err)
					}
					linked = append(linked, linkPath+" (copied)")
				} else if err := os.Symlink(canonicalAbs, linkPath); err != nil {
					// Symlink failed; fall back to copy.
					if copyErr := copyDir(canonicalAbs, linkPath); copyErr != nil {
						return fmt.Errorf("symlink failed (%w), copy also failed: %w", err, copyErr)
					}
					linked = append(linked, linkPath+" (copied)")
				} else {
					linked = append(linked, linkPath)
				}
			}

			out := skillPullOutput{
				Skill:     skillName,
				Canonical: canonicalAbs,
				LinkedTo:  linked,
				Files:     written,
			}

			if getFormat() == "pretty" {
				fmt.Printf("Installed skill %q to %s\n", skillName, canonicalAbs)
				for _, l := range linked {
					fmt.Printf("  Linked: %s\n", l)
				}
				fmt.Println()
				printFileTree(skillName, written)
				return nil
			}

			data, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling output: %w", err)
			}
			fmt.Println(string(data))
			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&agents, "agent", "a", []string{"claude"}, "Agent targets to link into (claude, cursor, codex)")
	cmd.Flags().BoolVarP(&global, "global", "g", true, "Install globally (~/) vs project-level (./)")
	cmd.Flags().BoolVar(&forceCopy, "copy", false, "Copy files instead of symlinking to agent directories")

	return cmd
}

// printFileTree renders a sorted list of file paths as a tree.
func printFileTree(root string, files []string) {
	type node struct {
		name     string
		children []*node
	}

	top := &node{name: root}
	lookup := map[string]*node{"": top}

	var getOrCreate func(parts []string) *node
	getOrCreate = func(parts []string) *node {
		key := filepath.Join(parts...)
		if n, ok := lookup[key]; ok {
			return n
		}
		parent := getOrCreate(parts[:len(parts)-1])
		n := &node{name: parts[len(parts)-1]}
		parent.children = append(parent.children, n)
		lookup[key] = n
		return n
	}

	for _, f := range files {
		parts := strings.Split(f, "/")
		getOrCreate(parts)
	}

	var walk func(n *node, prefix string)
	walk = func(n *node, prefix string) {
		sort.Slice(n.children, func(i, j int) bool {
			iDir := len(n.children[i].children) > 0
			jDir := len(n.children[j].children) > 0
			if iDir != jDir {
				return jDir
			}
			return n.children[i].name < n.children[j].name
		})
		for i, child := range n.children {
			last := i == len(n.children)-1
			connector := "├── "
			if last {
				connector = "└── "
			}
			suffix := ""
			if len(child.children) > 0 {
				suffix = "/"
			}
			fmt.Printf("%s%s%s%s\n", prefix, connector, child.name, suffix)
			next := prefix + "│   "
			if last {
				next = prefix + "    "
			}
			walk(child, next)
		}
	}

	fmt.Printf("%s/\n", top.name)
	walk(top, "")
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destPath, data, 0o644)
	})
}
