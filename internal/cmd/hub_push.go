package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

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
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			owner, name, _, err := parseHubOwnerRepo(args[0])
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

			paths := make([]string, 0, len(files))
			for k := range files {
				paths = append(paths, k)
			}
			sort.Strings(paths)
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
		if isHubExcludedFile(base) {
			return nil
		}
		for _, suf := range hubExcludedSuffixes {
			if strings.HasSuffix(base, suf) {
				return nil
			}
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

func isHubExcludedFile(base string) bool {
	if hubExcludedFiles[base] {
		return true
	}
	// Match README-documented ".env*" exclusion to avoid leaking custom
	// environment files like .env.staging, .env.test, etc.
	return strings.HasPrefix(base, ".env")
}

func isBinary(data []byte) bool {
	n := len(data)
	if n > 512 {
		n = 512
	}
	return bytes.IndexByte(data[:n], 0) != -1
}
