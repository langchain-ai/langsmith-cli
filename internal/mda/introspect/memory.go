package introspect

import (
	"context"
	"fmt"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

// The project's durable-memory declaration and what deploy seeds from it
// (`compile/memory.rs`).
//
// Durable memory is opt-in and owned by one root module: a project with no
// `memory.{ts,tsx,mts,cts,py}` mounts nothing, whatever identity says. This is
// the single place compile and deploy ask "does this project have memory", so
// the generated entry and the Context Hub seed can never disagree.
//
// The helper imports the memory module and classifies the exported `memory`
// value. An unusable value is a hard error rather than an unknown: deploy seeds
// the hot file from this answer, so guessing would either seed a tree the author
// did not ask for or drop the one they did.

// MemorySummary is the root memory declaration of a project.
type MemorySummary struct {
	// Path is relative to the project root (`memory.ts`, `memory.py`, …).
	Path string
	// Scope is the declared durable-memory scope.
	Scope MemoryLayers
}

// MemoryLayers is the durable-memory scope mounted by the root declaration.
type MemoryLayers struct {
	// Agent is deployment-shared memory mounted at `/memories/agent/`.
	Agent bool
	// User is caller-scoped memory mounted at `/memories/user/`.
	User bool
}

// MemoryDecision is what memory this project has, from the one place that
// decides it. A nil Declared means no declaration: no durable memory.
type MemoryDecision struct {
	// Declared is the root declaration, when one was found (it may still
	// resolve to no scope).
	Declared *MemorySummary
}

// AgentEnabled reports whether deploy should seed the shared agent layer.
func (d MemoryDecision) AgentEnabled() bool {
	return d.Declared != nil && d.Declared.Scope.Agent
}

// ResolveMemory resolves the project's memory decision, rejecting the retired
// knobs.
//
// The legacy checks live here rather than at each call site so a project can
// never end up with two answers to "which layers does a run mount": whichever
// asks (compile, deploy) gets the same error.
func ResolveMemory(ctx context.Context, p *project.Project) (MemoryDecision, error) {
	if err := assertNoLegacyMemoryConfig(ctx, p); err != nil {
		return MemoryDecision{}, err
	}
	rel, ok := FindMemoryEntry(p)
	if !ok {
		return MemoryDecision{}, nil
	}
	declared, found, err := extractMemoryLayers(ctx, p, rel)
	if err != nil {
		return MemoryDecision{}, err
	}
	if !found {
		declare := DeclareMemorySnippet(p.Language)
		verb := p.Language.Ident("export", "declare")
		imperative := p.Language.Ident("Export", "Declare")
		return MemoryDecision{}, fmt.Errorf(
			"%s does not %s a static `memory` declaration. %s `memory` from a `%s` call with "+
				"explicit named layers. Deploy seeds durable memory from this value, so it cannot "+
				"be computed at runtime. Delete the file to keep no memory.",
			rel, verb, imperative, declare)
	}
	scope, err := declared.MountedLayers(rel)
	if err != nil {
		return MemoryDecision{}, err
	}
	if scope.User {
		identity, err := ReadIdentitySummary(ctx, p)
		if err != nil {
			return MemoryDecision{}, err
		}
		if identity == nil {
			return MemoryDecision{}, fmt.Errorf(
				"%s enables user memory, but user memory requires identity.%s so the runtime can "+
					"resolve a trusted caller ID", rel, p.Language.Extension())
		}
	}
	return MemoryDecision{Declared: &MemorySummary{Path: rel, Scope: scope}}, nil
}

// assertNoLegacyMemoryConfig fails when a project still configures memory the
// old way.
//
// Both knobs are refused outright instead of being read as "off":
// `disableMemory: false` and `scope.memory: "agent"` used to *enable* memory, so
// honoring either under an opt-in contract would silently invert what the
// author wrote.
func assertNoLegacyMemoryConfig(ctx context.Context, p *project.Project) error {
	language := p.Language
	declare := DeclareMemorySnippet(language)
	verb := language.Ident("export", "declare")
	disables, err := AgentDeclaresDisableMemory(ctx, p)
	if err != nil {
		return err
	}
	if disables {
		return fmt.Errorf(
			"%s sets `%s`, which is no longer supported. Durable memory is opt-in and declared in "+
				"its own module: %s `memory` from memory.%s with `%s` to enable it, or leave that "+
				"file out to keep no memory. Remove the flag from the agent definition.",
			p.AgentFile, language.Ident("disableMemory", "disable_memory"), verb,
			language.Extension(), declare)
	}
	identity, err := ReadIdentitySummary(ctx, p)
	if err != nil {
		return err
	}
	if identity != nil && identity.DeclaresMemory {
		return fmt.Errorf(
			"%s sets `scope.memory`, which is no longer supported. Durable memory has its own root "+
				"declaration: %s `memory` from memory.%s with `%s`, and drop `scope.memory` from "+
				"the identity declaration.",
			identity.Path, verb, language.Extension(), declare)
	}
	return nil
}

// DeclareMemorySnippet is the `memory` constructor call an author of this
// language would write.
func DeclareMemorySnippet(language project.Language) string {
	return language.Ident(
		"defineMemory({ agent: memoryLayer() })",
		"define_memory(agent=MemoryLayer())",
	)
}

// MemoryLayersKind classifies a declared `scope`, before it is reduced to a
// mount decision.
type MemoryLayersKind int

// The classifications of a declared `scope`.
const (
	// MemoryLayersAgent is `scope: [{ type: "agent" }]`.
	MemoryLayersAgent MemoryLayersKind = iota
	// MemoryLayersUser is `scope: [{ type: "user" }]`.
	MemoryLayersUser
	// MemoryLayersAgentAndUser is both layers, normalized agent first.
	MemoryLayersAgentAndUser
	// MemoryLayersNone is `scope: []` / omitted.
	MemoryLayersNone
	// MemoryLayersDuplicate is a layer type listed more than once.
	MemoryLayersDuplicate
	// MemoryLayersShorthand is the retired string shorthand.
	MemoryLayersShorthand
	// MemoryLayersReserved is a value MDA recognizes but does not accept yet
	// (per-layer options).
	MemoryLayersReserved
	// MemoryLayersUnknown is a value outside the contract entirely.
	MemoryLayersUnknown
	// MemoryLayersDynamic is a value that is not layer data at all.
	MemoryLayersDynamic
)

// MemoryLayersValue is a classified `scope` value. Detail renders the offending
// value for the non-mounting kinds, and the layer for a duplicate.
type MemoryLayersValue struct {
	Kind   MemoryLayersKind
	Detail string
}

// MountedLayers reduces the value to the layers it mounts, or the error that
// explains why it mounts none.
func (v MemoryLayersValue) MountedLayers(fileLabel string) (MemoryLayers, error) {
	switch v.Kind {
	case MemoryLayersAgent:
		return MemoryLayers{Agent: true}, nil
	case MemoryLayersUser:
		return MemoryLayers{User: true}, nil
	case MemoryLayersAgentAndUser:
		return MemoryLayers{Agent: true, User: true}, nil
	case MemoryLayersNone:
		return MemoryLayers{}, nil
	case MemoryLayersDuplicate:
		return MemoryLayers{}, fmt.Errorf("%s declares the %s memory layer more than once", fileLabel, v.Detail)
	case MemoryLayersShorthand:
		return MemoryLayers{}, fmt.Errorf(
			"%s declares memory as %s. A layer names whose memory is mounted. Use named `agent` "+
				"and `user` options with layer constructors.", fileLabel, v.Detail)
	case MemoryLayersReserved:
		return MemoryLayers{}, fmt.Errorf(
			"%s declares a memory layer with %s, which is not supported yet. Per-layer mounts, "+
				"access modes, and retention are not implemented. Named layers accept only an "+
				"optional `allow` callback.", fileLabel, v.Detail)
	case MemoryLayersUnknown:
		return MemoryLayers{}, fmt.Errorf(
			"%s declares an unknown memory layer: %s. Use named `agent` and `user` layers.",
			fileLabel, v.Detail)
	default:
		return MemoryLayers{}, fmt.Errorf(
			"%s declares memory that is not a static literal (%s). Deploy seeds durable memory "+
				"from this value, so it has to use explicit named layer constructors.",
			fileLabel, v.Detail)
	}
}

// extractMemoryLayers classifies the exported `memory` of rel; found is false
// when the module exports no memory declaration.
func extractMemoryLayers(ctx context.Context, p *project.Project, rel string) (MemoryLayersValue, bool, error) {
	doc, err := projectDocument(ctx, p)
	if err != nil {
		return MemoryLayersValue{}, false, err
	}
	if doc.Memory == nil {
		return MemoryLayersValue{}, false, fmt.Errorf("%s: %w", rel, errNoHelperValue)
	}
	fields, err := doc.Memory.get()
	if err != nil {
		return MemoryLayersValue{}, false, err
	}
	return parseMemoryLayers(fields, rel)
}

// parseMemoryLayers decodes the helper's `{kind, detail}` classification.
func parseMemoryLayers(fields memoryFields, fileLabel string) (MemoryLayersValue, bool, error) {
	detail := "a computed value"
	if fields.Detail != nil {
		detail = *fields.Detail
	}
	var value MemoryLayersValue
	switch fields.Kind {
	case "absent":
		return MemoryLayersValue{}, false, nil
	case "agent":
		value = MemoryLayersValue{Kind: MemoryLayersAgent}
	case "user":
		value = MemoryLayersValue{Kind: MemoryLayersUser}
	case "agent_user":
		value = MemoryLayersValue{Kind: MemoryLayersAgentAndUser}
	case "none":
		value = MemoryLayersValue{Kind: MemoryLayersNone}
	case "duplicate_agent":
		value = MemoryLayersValue{Kind: MemoryLayersDuplicate, Detail: "agent"}
	case "duplicate_user":
		value = MemoryLayersValue{Kind: MemoryLayersDuplicate, Detail: "user"}
	case "shorthand":
		value = MemoryLayersValue{Kind: MemoryLayersShorthand, Detail: detail}
	case "reserved":
		value = MemoryLayersValue{Kind: MemoryLayersReserved, Detail: detail}
	case "unknown":
		value = MemoryLayersValue{Kind: MemoryLayersUnknown, Detail: detail}
	case "dynamic":
		value = MemoryLayersValue{Kind: MemoryLayersDynamic, Detail: detail}
	default:
		return MemoryLayersValue{}, false, fmt.Errorf("unexpected memory scope kind from %s: %q", fileLabel, fields.Kind)
	}
	return value, true, nil
}
