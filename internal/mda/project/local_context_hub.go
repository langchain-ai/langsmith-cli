package project

// Local Context Hub mock for `mda build` / `mda dev` (`local_context_hub.rs`).
//
// Mirrors the deploy-time Hub layout on disk under `.mda/__contexthub__` so the
// managed runtime mounts instructions, skills, and agent memory the same way it
// does against a remote Context Hub repo. Local user memory lives separately
// under `__users__/`, where the runtime opens it by opaque user-repo handle.
//
// The agent tree is seeded only when the project declares the agent slice.

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// LocalContextHub is the resolved local Context Hub mock used by compile/dev
// entries.
type LocalContextHub struct {
	// Root is the absolute path to `.mda/__contexthub__`.
	Root string
	// HasSkills is whether the mock contains deploy-owned skills.
	HasSkills bool
}

// EnsureLocalContextHub ensures `.mda/__contexthub__` exists, syncs harness
// files, and seeds memory.
//
//   - `instructions.md` and `skills/**` are refreshed from the project each
//     call.
//   - When agentMemoryEnabled, `memories/agent/**` is seeded once and then
//     preserved (same as deploy: never overwrite existing hot memory).
//   - Otherwise existing memory is left untouched. Runtime layer selection
//     controls whether preserved memory is mounted.
func EnsureLocalContextHub(p *Project, agentMemoryEnabled bool) (*LocalContextHub, error) {
	root := filepath.Join(p.Root, ".mda", LocalContextHubDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create %s: %w", root, err)
	}

	sourceRoot := ProjectContextRoot(p)
	if err := syncInstructions(sourceRoot, root); err != nil {
		return nil, err
	}
	hasSkills, err := syncSkills(sourceRoot, root)
	if err != nil {
		return nil, err
	}
	if agentMemoryEnabled {
		if err := seedAgentMemory(root); err != nil {
			return nil, err
		}
	}

	absolute, err := canonicalize(root)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %s: %w", root, err)
	}

	return &LocalContextHub{Root: absolute, HasSkills: hasSkills}, nil
}

func syncInstructions(sourceRoot, hubRoot string) error {
	src := filepath.Join(sourceRoot, InstructionsFile)
	dst := filepath.Join(hubRoot, InstructionsFile)
	if isFile(src) {
		contents, err := readToString(src)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", src, err)
		}
		if err := os.WriteFile(dst, []byte(contents), 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", dst, err)
		}
	} else if isFile(dst) {
		if err := os.Remove(dst); err != nil {
			return fmt.Errorf("failed to remove stale %s: %w", dst, err)
		}
	}
	return nil
}

func syncSkills(sourceRoot, hubRoot string) (bool, error) {
	srcSkills := filepath.Join(sourceRoot, SkillsDir)
	dstSkills := filepath.Join(hubRoot, SkillsDir)

	if isDir(dstSkills) {
		if err := os.RemoveAll(dstSkills); err != nil {
			return false, fmt.Errorf("failed to clear %s: %w", dstSkills, err)
		}
	}

	if !isDir(srcSkills) {
		return false, nil
	}

	hasSkills := false
	err := filepath.WalkDir(srcSkills, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk %s: %w", srcSkills, err)
		}
		if IsJunkDir(path) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() || IsJunkFile(path) {
			return nil
		}
		rel, err := filepath.Rel(srcSkills, path)
		if err != nil {
			return fmt.Errorf("failed to relativize %s: %w", path, err)
		}
		dst := filepath.Join(dstSkills, rel)
		parent := filepath.Dir(dst)
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return fmt.Errorf("failed to create %s: %w", parent, err)
		}
		if err := copyFile(path, dst); err != nil {
			return fmt.Errorf("failed to copy %s -> %s: %w", path, dst, err)
		}
		hasSkills = true
		return nil
	})
	return hasSkills, err
}

// copyFile copies src's contents and permission bits to dst, like Rust's
// `fs::copy`.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close() // The copy error is the one to report.
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode().Perm())
}

func seedAgentMemory(hubRoot string) error {
	agent := filepath.Join(hubRoot, AgentMemoryFile)
	var agentContent *string
	if isFile(agent) {
		contents, err := readToString(agent)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", agent, err)
		}
		agentContent = &contents
	}
	flat := filepath.Join(hubRoot, FlatMemoryFile)
	var flatContent *string
	if agentContent == nil && isFile(flat) {
		contents, err := readToString(flat)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", flat, err)
		}
		flatContent = &contents
	}
	seed := ResolveAgentMemorySeed(agentContent, flatContent)
	if !seed.Write {
		return nil
	}
	parent := filepath.Dir(agent)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", parent, err)
	}
	if err := os.WriteFile(agent, []byte(seed.Content), 0o644); err != nil {
		return fmt.Errorf("failed to seed %s: %w", agent, err)
	}
	return nil
}
