// Package gen holds CLI commands generated from the LangSmith Operation
// Catalog (zz_generated_*.go) and attaches them to the root command.
// This file is handwritten; `make generate` never touches it.
package gen

import (
	"fmt"
	"sort"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/genrt"
	"github.com/spf13/cobra"
)

// Operation is one generated command.
type Operation struct {
	ID   string
	Path []string
	Risk string
	New  func() *cobra.Command
}

// Group is an intermediate command such as `annotation-queue item`.
type Group struct {
	Path  []string
	Short string
}

var (
	operations []Operation
	groups     []Group
)

// Override replaces a generated command. It receives the generated
// constructor so it can wrap it (add flags, resolve names) or ignore it.
type Override func(generated func() *cobra.Command) *cobra.Command

// Operations returns the generated operations, sorted by ID.
func Operations() []Operation {
	out := append([]Operation{}, operations...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Attach adds generated commands under root. An override keyed by operation
// ID wins over the generated command, and a handwritten command already at
// the same path wins over both. It returns the IDs skipped for that reason.
func Attach(root *cobra.Command, overrides map[string]Override) ([]string, error) {
	known := map[string]bool{}
	for _, op := range operations {
		known[op.ID] = true
	}
	for id := range overrides {
		if !known[id] {
			return nil, fmt.Errorf("override for unknown operation %q", id)
		}
	}

	sortedGroups := append([]Group{}, groups...)
	sort.Slice(sortedGroups, func(i, j int) bool { return len(sortedGroups[i].Path) < len(sortedGroups[j].Path) })
	for _, g := range sortedGroups {
		parent := find(root, g.Path[:len(g.Path)-1])
		if parent == nil {
			return nil, fmt.Errorf("missing parent for group %q", strings.Join(g.Path, " "))
		}
		if child(parent, g.Path[len(g.Path)-1]) == nil {
			parent.AddCommand(&cobra.Command{Use: g.Path[len(g.Path)-1], Short: g.Short})
		}
	}

	var skipped []string
	for _, op := range Operations() {
		parent := find(root, op.Path[:len(op.Path)-1])
		name := op.Path[len(op.Path)-1]
		if child(parent, name) != nil {
			skipped = append(skipped, op.ID)
			continue
		}
		var cmd *cobra.Command
		if override, ok := overrides[op.ID]; ok {
			cmd = override(op.New)
			if cmd.Name() != name {
				return nil, fmt.Errorf("override for %s must be named %q, got %q", op.ID, name, cmd.Name())
			}
		} else {
			cmd = op.New()
		}
		if cmd.Annotations == nil {
			cmd.Annotations = map[string]string{}
		}
		cmd.Annotations[genrt.AnnotationOperationID] = op.ID
		cmd.Annotations[genrt.AnnotationRisk] = op.Risk
		parent.AddCommand(cmd)
	}
	return skipped, nil
}

func find(root *cobra.Command, path []string) *cobra.Command {
	cmd := root
	for _, name := range path {
		if cmd = child(cmd, name); cmd == nil {
			return nil
		}
	}
	return cmd
}

func child(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}
