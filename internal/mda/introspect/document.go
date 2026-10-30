package introspect

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
)

// DocumentVersion is the version of the JSON document the helpers write.
const DocumentVersion = 1

// helperRequest is a request a helper reads on stdin.
type helperRequest interface {
	header() *requestHeader
}

// requestHeader starts every helper request.
type requestHeader struct {
	// Mode is `project`, `lex` (TypeScript only) or `check` (TypeScript only:
	// load the toolchain, to verify an install).
	Mode string `json:"mode"`
	// Output is the file the helper writes its document to. Not stdout: a
	// project module may write to file descriptor 1 directly.
	Output string `json:"output"`
}

func (h *requestHeader) header() *requestHeader {
	return h
}

// projectRequest is what the helpers read on stdin in project mode. Paths are
// relative to Root and use `/`.
type projectRequest struct {
	requestHeader
	Root             string   `json:"root"`
	Agent            string   `json:"agent"`
	Identity         *string  `json:"identity"`
	Memory           *string  `json:"memory"`
	Sandbox          *string  `json:"sandbox"`
	Mcp              *string  `json:"mcp"`
	LegacyConnectors []string `json:"legacy_connectors"`
	Channels         []string `json:"channels"`
	Schedules        []string `json:"schedules"`
}

// document is the one JSON document a helper writes for a project.
//
// Each module the request names has a section: the values read off its exports,
// or why they could not be read. A null section is a module the project does not
// have. The requirement sections are computed by the project's own
// `managed-deepagents` from the collected `tools/mcp` and `channels/` exports,
// and are left out when there is nothing to compute them from.
type document struct {
	Version               int                            `json:"version"`
	Sdk                   sdkFields                      `json:"sdk"`
	Agent                 *section[agentFields]          `json:"agent"`
	Identity              *section[identityFields]       `json:"identity"`
	Memory                *section[memoryFields]         `json:"memory"`
	Sandbox               *section[sandboxFields]        `json:"sandbox"`
	Mcp                   *section[exportFields]         `json:"mcp"`
	LegacyConnectors      []section[exportFields]        `json:"legacy_connectors"`
	Channels              []section[channelFields]       `json:"channels"`
	Schedules             []section[scheduleFields]      `json:"schedules"`
	ConnectorRequirements *section[json.RawMessage]      `json:"connector_requirements,omitempty"`
	McpConnections        *section[[]McpConnectionEntry] `json:"mcp_connections,omitempty"`
	ChannelRequirements   *section[json.RawMessage]      `json:"channel_requirements,omitempty"`
}

// sdkFields is the project's installed `managed-deepagents`, as its modules
// import it.
type sdkFields struct {
	// Version is the installed package's version, when its metadata has one.
	Version *string `json:"version"`
	// NamedFactories reports whether it has named factories:
	// `defineDeepAgent(name, factory)` / `define_deep_agent(name, factory)`.
	NamedFactories bool `json:"named_factories"`
}

// section is what a helper read from one module, or the error it hit. Exactly
// one of Error and Value is non-null.
type section[T any] struct {
	// File is the module, relative to the project root, with `/` separators.
	File  string       `json:"file"`
	Error *helperError `json:"error"`
	Value T            `json:"value"`
}

// get returns the section's value, or its error naming the module.
func (s *section[T]) get() (T, error) {
	if s.Error != nil {
		var zero T
		return zero, s.Error.err(s.label())
	}
	return s.Value, nil
}

// label is the module path as messages show it.
func (s *section[T]) label() string {
	return filepath.FromSlash(s.File)
}

// helperError is a failure a helper reports instead of a value.
type helperError struct {
	// Kind is `syntax` (the module does not parse), `import` (importing it
	// threw), `evaluate` (reading an exported value threw), or `helper` (the
	// helper could not run at all, e.g. `managed-deepagents` is not installed).
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
	// Detail is the Python traceback, when there is one.
	Detail string `json:"detail,omitempty"`
}

// err renders the failure for file, the way the Rust extractors reported a
// file that does not parse.
func (e *helperError) err(file string) error {
	switch e.Kind {
	case "syntax":
		return errs.Categorized(errs.Project,
			fmt.Sprintf("failed to parse %s:%d:%d: %s", file, e.Line, e.Column, e.Message))
	case "helper":
		return errs.Categorized(errs.Toolchain, e.Message)
	}
	verb := "import"
	if e.Kind == "evaluate" {
		verb = "read the exports of"
	}
	message := fmt.Sprintf("failed to %s %s: %s", verb, file, e.Message)
	if e.Detail == "" {
		return errs.Categorized(errs.Project, message)
	}
	return errs.WithCategory(errs.Project, fmt.Errorf("%s: %w", message, errs.ChildStderr(e.Detail)))
}

// agentFields is the helper's reading of the exported `agent`.
type agentFields struct {
	// Kind is `static` (a definition), `factory` (a named factory), `bare_function`
	// (a factory without a name), `missing` (no `agent` export), or `other`.
	Kind                  string           `json:"kind"`
	Name                  AgentNameValue   `json:"name"`
	Model                 *string          `json:"model"`
	DeclaresDisableMemory bool             `json:"declares_disable_memory"`
	UnsupportedKeys       []string         `json:"unsupported_keys"`
	AsyncSubagents        []AgentNameValue `json:"async_subagents"`
	UnreadableSubagents   int              `json:"unreadable_subagents"`
}

// identityFields is the helper's reading of the exported `identity`.
type identityFields struct {
	Declared       bool               `json:"declared"`
	Auth           *string            `json:"auth"`
	AuthOmitted    bool               `json:"auth_omitted"`
	Scope          *IdentityScopeAxes `json:"scope"`
	DeclaresMemory bool               `json:"declares_memory"`
}

// memoryFields classifies the exported `memory` the way `memory_layers.py` did.
type memoryFields struct {
	Kind   string  `json:"kind"`
	Detail *string `json:"detail"`
}

// sandboxFields is the helper's reading of the exported `sandbox`: the bake
// keys it sets, as plain JSON, and the ones that are not plain JSON.
type sandboxFields struct {
	Declared  bool                       `json:"declared"`
	IsSandbox bool                       `json:"is_sandbox"`
	Options   map[string]json.RawMessage `json:"options"`
	Errors    map[string]string          `json:"errors"`
}

// exportFields says whether a module has a named export (and a default one).
type exportFields struct {
	HasExport  bool `json:"has_export"`
	HasDefault bool `json:"has_default"`
}

// channelFields is [exportFields] for `channel`, with the ingress provider of
// the exported channel.
type channelFields struct {
	exportFields
	Ingress *string `json:"ingress"`
}

// scheduleFields is the helper's reading of the exported `schedule`.
type scheduleFields struct {
	Declared   bool            `json:"declared"`
	IsSchedule bool            `json:"is_schedule"`
	Config     json.RawMessage `json:"config"`
	Error      *string         `json:"error"`
}

// decodeDocument decodes a helper's document into out strictly, so the two
// sides of the document cannot drift apart unnoticed: a field out does not
// declare is an error, and so is a field it declares that the document lacks,
// unless its tag says `omitempty`. Numbers keep their precision.
func decodeDocument(data []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if decoder.More() {
		return errors.New("trailing data after the document")
	}
	var raw any
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return err
	}
	return requireFields(reflect.TypeOf(out), raw, "document")
}

var (
	unmarshalerType = reflect.TypeFor[json.Unmarshaler]()
	rawMessageType  = reflect.TypeFor[json.RawMessage]()
)

// requireFields fails when a struct field of t that is not `omitempty` is
// missing from the JSON value raw decoded into it. path names raw in errors.
func requireFields(t reflect.Type, raw any, path string) error {
	if raw == nil || t == rawMessageType || reflect.PointerTo(t).Implements(unmarshalerType) {
		return nil
	}
	switch t.Kind() {
	case reflect.Pointer:
		return requireFields(t.Elem(), raw, path)
	case reflect.Slice, reflect.Array:
		items, _ := raw.([]any)
		for i, item := range items {
			if err := requireFields(t.Elem(), item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		entries, _ := raw.(map[string]any)
		for key, item := range entries {
			if err := requireFields(t.Elem(), item, path+"."+key); err != nil {
				return err
			}
		}
	case reflect.Struct:
		object, _ := raw.(map[string]any)
		for i := range t.NumField() {
			field := t.Field(i)
			if field.Anonymous {
				if err := requireFields(field.Type, raw, path); err != nil {
					return err
				}
				continue
			}
			name, options, _ := strings.Cut(field.Tag.Get("json"), ",")
			if !field.IsExported() || name == "-" {
				continue
			}
			value, ok := object[name]
			if !ok {
				if options == "omitempty" {
					continue
				}
				return fmt.Errorf("%s has no field %q", path, name)
			}
			if err := requireFields(field.Type, value, path+"."+name); err != nil {
				return err
			}
		}
	}
	return nil
}
