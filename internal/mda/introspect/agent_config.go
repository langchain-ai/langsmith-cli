package introspect

import (
	"context"
	"fmt"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

// The authored `defineDeepAgent` / `define_deep_agent` config (`agent_config.rs`).
//
// The agent `name` is required because it becomes the LangGraph graph key /
// assistant id and the default LangSmith deployment name. The Rust CLI read it
// from the source; here the helper imports the agent module and reads it off the
// exported value. A static definition, from `defineAgent` / `define_agent` or
// `defineDeepAgent` / `define_deep_agent`, carries it in its config. A factory
// must be named up front, `defineAgent("name", factory, { asyncSubagents })` /
// `define_agent("name", factory, async_subagents=[...])`, so the name and its
// async subagents are known without running it; a bare factory function is
// refused. A module that does not import is reported instead of defaulted.
//
// Callers that need more than one field should use [ReadAgentStaticConfig]; the
// helper runs once per project either way.

// AgentNameKind says what the authored `name` is.
type AgentNameKind int

// The outcomes of reading the authored `name`.
const (
	// AgentNameAbsent is no `name` on the agent definition.
	AgentNameAbsent AgentNameKind = iota
	// AgentNameStatic is a string name.
	AgentNameStatic
	// AgentNameDynamic is a `name` that is not a string.
	AgentNameDynamic
)

// AgentNameValue is the outcome of reading the authored `name` from
// `defineDeepAgent` / `define_deep_agent`.
//
// The helpers emit it as `{"kind": "STATIC", "value": ...}`.
type AgentNameValue struct {
	Kind AgentNameKind
	// Value is the name when Kind is [AgentNameStatic].
	Value string
}

// UnmarshalJSON decodes `{"kind": "STATIC" | "DYNAMIC" | "ABSENT", "value": ...}`,
// strictly, as [decodeDocument] decodes the rest of the document.
func (v *AgentNameValue) UnmarshalJSON(data []byte) error {
	var raw struct {
		Kind  string  `json:"kind"`
		Value *string `json:"value,omitempty"`
	}
	if err := decodeDocument(data, &raw); err != nil {
		return err
	}
	switch raw.Kind {
	case "STATIC":
		if raw.Value == nil {
			return fmt.Errorf("agent name STATIC is missing its value")
		}
		*v = AgentNameValue{Kind: AgentNameStatic, Value: *raw.Value}
	case "DYNAMIC":
		*v = AgentNameValue{Kind: AgentNameDynamic}
	case "ABSENT":
		*v = AgentNameValue{Kind: AgentNameAbsent}
	default:
		return fmt.Errorf("unknown agent name kind %q", raw.Kind)
	}
	return nil
}

// AgentStaticConfig is the static fields of the authored agent definition.
type AgentStaticConfig struct {
	// AssistantID is the canonical LangGraph graph / assistant id from the
	// authored `name`.
	AssistantID string
	// Model is the static definition's `model` when it is a string.
	Model *string
	// IsFactory reports whether the exported `agent` is a per-run factory.
	IsFactory bool
	// AsyncSubagents are the subagents marked async, each built as its own graph.
	AsyncSubagents []AsyncSubagentGraph
	// UnreadableSubagents counts subagent entries MDA cannot read. The Rust CLI
	// counted entries its parser could not follow; an imported definition is
	// always readable, so this is 0.
	UnreadableSubagents int
}

// GeneralPurposeSubagent is the name of the general-purpose subagent, which MDA
// also runs async on served builds.
const GeneralPurposeSubagent = "general-purpose"

// ServedAsyncSubagents lists the async subagents to build for a served agent:
// the declared ones, plus the general-purpose subagent unless one is declared
// async already.
func (c AgentStaticConfig) ServedAsyncSubagents() []AsyncSubagentGraph {
	graphs := append([]AsyncSubagentGraph(nil), c.AsyncSubagents...)
	graphID := c.AssistantID + "--" + GeneralPurposeSubagent
	// Compare graph ids: a declared `general_purpose` normalizes to the same one.
	for _, graph := range graphs {
		if graph.GraphID == graphID {
			return graphs
		}
	}
	return append(graphs, AsyncSubagentGraph{
		Name:    GeneralPurposeSubagent,
		GraphID: graphID,
		Export:  "agent__general_purpose",
	})
}

// AsyncSubagentGraph is an async subagent and the graph MDA builds for it.
type AsyncSubagentGraph struct {
	// Name is the authored subagent `name`.
	Name string
	// GraphID is the graph key in `langgraph.json`:
	// `<assistant_id>--<normalized name>`.
	GraphID string
	// Export is the export of its graph factory in the generated entry.
	Export string
}

// ReadAgentStaticConfig reads the graph identity and static build metadata of
// the exported agent.
func ReadAgentStaticConfig(ctx context.Context, p *project.Project) (AgentStaticConfig, error) {
	fileLabel := p.AgentFile
	fields, err := readAgentConfigFields(ctx, p)
	if err != nil {
		return AgentStaticConfig{}, err
	}
	assistantID, err := resolveAssistantID(fields.Name, fileLabel, p.Language)
	if err != nil {
		return AgentStaticConfig{}, err
	}
	asyncSubagents, err := resolveAsyncSubagents(fields.AsyncSubagents, assistantID, fileLabel, p.Language)
	if err != nil {
		return AgentStaticConfig{}, err
	}
	return AgentStaticConfig{
		AssistantID:         assistantID,
		Model:               fields.Model,
		IsFactory:           fields.Kind == "factory",
		AsyncSubagents:      asyncSubagents,
		UnreadableSubagents: fields.UnreadableSubagents,
	}, nil
}

func resolveAsyncSubagents(
	values []AgentNameValue,
	assistantID, fileLabel string,
	language project.Language,
) ([]AsyncSubagentGraph, error) {
	var graphs []AsyncSubagentGraph
	for _, value := range values {
		if value.Kind != AgentNameStatic {
			flag := language.Ident("`async: true`", "`\"async\": True`")
			spread := language.Ident("`...spread`", "`**spread`")
			return nil, errs.Categorized(errs.Project, fmt.Sprintf(
				"async subagents in %s must set `name` and %s as literals, after any %s, "+
					"so MDA can build a graph for each one.", fileLabel, flag, spread))
		}
		name := value.Value
		normalized, err := project.NormalizeName(name)
		if err != nil {
			return nil, fmt.Errorf("invalid async subagent name %q in %s: %w", name, fileLabel, err)
		}
		graphID := assistantID + "--" + normalized
		if err := ValidateAssistantID(graphID, fileLabel); err != nil {
			return nil, err
		}
		for _, graph := range graphs {
			if graph.GraphID == graphID {
				return nil, fmt.Errorf("async subagent %q in %s has the same graph id as another "+
					"subagent (%s); names are compared after normalization", name, fileLabel, graphID)
			}
		}
		graphs = append(graphs, AsyncSubagentGraph{
			Name:    name,
			GraphID: graphID,
			Export:  "agent__" + strings.ReplaceAll(normalized, "-", "_"),
		})
	}
	return graphs, nil
}

// AgentDeclaresDisableMemory reports whether the authored agent still sets the
// retired memory opt-out.
func AgentDeclaresDisableMemory(ctx context.Context, p *project.Project) (bool, error) {
	fields, err := readAgentConfigFields(ctx, p)
	if err != nil {
		return false, err
	}
	return fields.DeclaresDisableMemory, nil
}

// AgentAssistantID resolves the LangGraph graph / assistant id from the
// exported agent entry.
//
// Static definitions use their required authored `name`. Factories are named
// by `defineAgent(name, factory)` / `define_agent(name, factory)`.
func AgentAssistantID(ctx context.Context, p *project.Project) (string, error) {
	config, err := ReadAgentStaticConfig(ctx, p)
	if err != nil {
		return "", err
	}
	return config.AssistantID, nil
}

// agentSection is the helper's reading of the agent module, and of the
// `managed-deepagents` it imports.
func agentSection(ctx context.Context, p *project.Project) (agentFields, sdkFields, error) {
	doc, err := projectDocument(ctx, p)
	if err != nil {
		return agentFields{}, sdkFields{}, err
	}
	if doc.Agent == nil {
		return agentFields{}, sdkFields{}, fmt.Errorf("%s: %w", p.AgentFile, errNoHelperValue)
	}
	fields, err := doc.Agent.get()
	return fields, doc.Sdk, err
}

func readAgentConfigFields(ctx context.Context, p *project.Project) (agentFields, error) {
	fields, sdk, err := agentSection(ctx, p)
	if err != nil {
		return agentFields{}, err
	}
	if fields.Kind == "bare_function" {
		return agentFields{}, bareFactoryError(p.AgentFile, p.Language, sdk)
	}
	if err := validateAgentKeys(fields.UnsupportedKeys, p.AgentFile, p.Language); err != nil {
		return agentFields{}, err
	}
	return fields, nil
}

// bareFactoryError is the error for an `agent` export that is a plain function,
// such as a `defineDeepAgent` / `define_deep_agent` factory: its name is in its
// body, which only running it would reveal. sdk is the installed
// `managed-deepagents`, which may predate `defineAgent` / `define_agent`.
func bareFactoryError(fileLabel string, language project.Language, sdk sdkFields) error {
	define := language.Ident("defineAgent", "define_agent")
	if !sdk.NamedFactories {
		installed := "the installed managed-deepagents"
		if sdk.Version != nil {
			installed += " (" + *sdk.Version + ")"
		}
		return errs.Categorized(errs.Project, fmt.Sprintf(
			"%s exports `agent` as a bare factory function, so its name is known only by running it, "+
				"which `langsmith deploy` does not do. It reads a factory's name from %s(name, factory), "+
				"but %s does not have %s. Deploy factory agents with a managed-deepagents release that "+
				"has it, or with the SDK's own `mda deploy`.",
			fileLabel, define, installed, define))
	}
	named := language.Ident(
		"`export const agent = defineAgent(\"research-assistant\", factory, { asyncSubagents: [...] })`",
		"`agent = define_agent(\"research-assistant\", factory, async_subagents=[...])`",
	)
	nameless := language.Ident("defineAgent({ model, ... })", "define_agent(model=..., ...)")
	return errs.Categorized(errs.Project, fmt.Sprintf(
		"%s exports `agent` as a bare factory function, so its name is known only by running it, "+
			"which `langsmith deploy` does not do. Name the factory where it is exported: %s. The name "+
			"becomes the LangGraph assistant id and the default LangSmith deployment name; list every "+
			"async subagent the factory may return. The definition the factory returns may leave the "+
			"name out: %s.",
		fileLabel, named, nameless))
}

func validateAgentKeys(keys []string, fileLabel string, language project.Language) error {
	define := language.Ident("defineDeepAgent", "define_deep_agent")
	for _, key := range keys {
		var hint string
		switch key {
		case "backend":
			hint = "Configure the sandbox in sandbox/."
		case "store":
			hint = "MDA provides the store automatically."
		case "checkpointer":
			hint = "MDA provides the checkpointer automatically."
		case "instructions":
			hint = "Use a factory entrypoint to select instructions per run; static agents use instructions.md."
		case "mcp":
			hint = language.Ident(
				"Use a factory entrypoint to select MCP servers per run; static agents use tools/mcp.ts.",
				"Use a factory entrypoint to select MCP servers per run; static agents use tools/mcp.py.",
			)
		case "memory":
			hint = language.Ident(
				"Use a factory entrypoint to select memory per run; static agents use memory.ts.",
				"Use a factory entrypoint to select memory per run; static agents use memory.py.",
			)
		case "sandbox":
			hint = language.Ident(
				"Use a factory entrypoint to select a sandbox per run; static agents use sandbox/index.ts.",
				"Use a factory entrypoint to select a sandbox per run; static agents use sandbox/__init__.py.",
			)
		case "skills":
			hint = "Use a factory entrypoint to select skills per run; static agents use skills/."
		case "systemPrompt", "system_prompt":
			hint = "Use a factory entrypoint with `instructions` for per-run prompts; static agents use instructions.md."
		case "state_schema":
			hint = "Custom state schemas are not supported in Python."
		default:
			continue
		}
		return errs.Categorized(errs.Project,
			fmt.Sprintf("%s: Remove `%s` from %s(...). %s", fileLabel, key, define, hint))
	}
	return nil
}

func resolveAssistantID(value AgentNameValue, fileLabel string, language project.Language) (string, error) {
	example := language.StringArgument("name", "research-assistant")
	switch value.Kind {
	case AgentNameStatic:
		name, err := project.NormalizeName(value.Value)
		if err != nil {
			return "", fmt.Errorf("invalid agent `name` in %s: %w", fileLabel, err)
		}
		if err := ValidateAssistantID(name, fileLabel); err != nil {
			return "", err
		}
		return name, nil
	case AgentNameDynamic:
		return "", errs.Categorized(errs.Project, fmt.Sprintf(
			"agent `name` in %s must be a static string literal (e.g. %s). Dynamic values cannot be "+
				"used as the LangGraph assistant id.", fileLabel, example))
	default:
		// Only `defineAgent(config)` / `define_agent(**config)` leaves it out: it is
		// for what a named factory returns, not an export.
		named := language.Ident("`defineAgent(\"research-assistant\", { ... })`", "`define_agent(\"research-assistant\", ...)`")
		return "", errs.Categorized(errs.Project, fmt.Sprintf(
			"agent `name` is required in %s. Name the exported agent first, %s; MDA uses it "+
				"as the LangGraph assistant id and the default LangSmith deployment name.",
			fileLabel, named))
	}
}

// IsValidAssistantID reports whether name is a valid assistant / graph id (and
// scaffold agent name).
func IsValidAssistantID(name string) bool {
	return ValidateAssistantID(name, "agent") == nil
}

// ValidateAssistantID validates a candidate assistant / graph id.
func ValidateAssistantID(name, fileLabel string) error {
	if name == "" {
		return fmt.Errorf("agent `name` in %s must be a non-empty string", fileLabel)
	}
	if !isASCIILetter(name[0]) {
		return fmt.Errorf("agent `name` in %s must start with an ASCII letter (got %q)", fileLabel, name)
	}
	for _, c := range []byte(name[1:]) {
		if !isASCIILetter(c) && !('0' <= c && c <= '9') && c != '_' && c != '-' {
			return fmt.Errorf("agent `name` in %s must match [a-zA-Z][a-zA-Z0-9_-]* (got %q)", fileLabel, name)
		}
	}
	return nil
}

func isASCIILetter(c byte) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

// ReadAgentModel reads the `model` configured on the exported agent definition,
// when it is a string (`compile/model.rs`).
//
// The model is read off the imported definition itself, so an unrelated
// `model` elsewhere in the module (a tool config, a subagent) is never mistaken
// for the agent's. A factory picks its model per run, so it has none here.
//
// Unlike [ReadAgentStaticConfig], a missing `name` does not fail this helper —
// model inference is independent of assistant-id validation.
func ReadAgentModel(ctx context.Context, p *project.Project) (string, bool, error) {
	fields, _, err := agentSection(ctx, p)
	if err != nil {
		return "", false, err
	}
	if fields.Model == nil {
		return "", false, nil
	}
	return *fields.Model, true, nil
}
