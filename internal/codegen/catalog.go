// Package codegen turns the LangSmith Operation Catalog (operations.json, built
// from the public OpenAPI spec and Stainless config) into Cobra commands that call the
// generated langsmith-go SDK.
package codegen

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// SchemaVersion is the operations.json version this generator understands.
const SchemaVersion = 1

type Catalog struct {
	SchemaVersion int                `json:"schema_version"`
	Operations    []Operation        `json:"operations"`
	Schemas       map[string]*Schema `json:"schemas"`
}

type Operation struct {
	ID       string   `json:"id"`
	Resource []string `json:"resource"`
	Action   string   `json:"action"`
	HTTP     struct {
		Method string `json:"method"`
		Path   string `json:"path"`
	} `json:"http"`
	Summary     string  `json:"summary"`
	Description string  `json:"description"`
	Deprecated  *string `json:"deprecated"`
	SDK         struct {
		Go *GoSDK `json:"go"`
	} `json:"sdk"`
	Parameters struct {
		Path  []Param `json:"path"`
		Query []Param `json:"query"`
	} `json:"parameters"`
	RequestBody *Body       `json:"request_body"`
	Response    *Response   `json:"response"`
	Pagination  *Pagination `json:"pagination"`
	Risk        string      `json:"risk"`
	Exposure    struct {
		CLI bool `json:"cli"`
		MCP any  `json:"mcp"`
	} `json:"exposure"`
	Override *string `json:"override"`
}

type GoSDK struct {
	Service    string             `json:"service"`
	Method     string             `json:"method"`
	PathArgs   []string           `json:"path_args"`
	ParamsType *string            `json:"params_type"`
	Fields     map[string]GoField `json:"fields"`
}

type GoField struct {
	Name     string `json:"name"`
	EnumType string `json:"enum_type,omitempty"`
}

type Param struct {
	Name        string  `json:"name"`
	Required    bool    `json:"required"`
	Schema      *Schema `json:"schema"`
	Description string  `json:"description"`
}

type Body struct {
	Required bool    `json:"required"`
	Schema   *Schema `json:"schema"`
}

type Response struct {
	Status      int     `json:"status"`
	ContentType *string `json:"content_type"`
	Schema      *Schema `json:"schema"`
}

type Pagination struct {
	Scheme        string `json:"scheme"`
	Type          string `json:"type"`
	ParamLocation string `json:"param_location"`
	Request       struct {
		CursorParam  string `json:"cursor_param"`
		OffsetParam  string `json:"offset_param"`
		LimitParam   string `json:"limit_param"`
		LimitDefault *int64 `json:"limit_default"`
		LimitMax     *int64 `json:"limit_max"`
	} `json:"request"`
	Response struct {
		ItemsField      *string `json:"items_field"`
		NextCursorField string  `json:"next_cursor_field"`
		TotalField      string  `json:"total_field"`
	} `json:"response"`
}

// Schema is the subset of JSON Schema the generator reads.
type Schema struct {
	Ref         string             `json:"$ref,omitempty"`
	Type        any                `json:"type,omitempty"`
	Format      string             `json:"format,omitempty"`
	Enum        []any              `json:"enum,omitempty"`
	Items       *Schema            `json:"items,omitempty"`
	Properties  map[string]*Schema `json:"properties,omitempty"`
	Required    []string           `json:"required,omitempty"`
	AnyOf       []*Schema          `json:"anyOf,omitempty"`
	OneOf       []*Schema          `json:"oneOf,omitempty"`
	AllOf       []*Schema          `json:"allOf,omitempty"`
	Description string             `json:"description,omitempty"`
}

func ParseCatalog(data []byte) (*Catalog, error) {
	var c Catalog
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parsing catalog: %w", err)
	}
	if c.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("catalog schema_version %d, generator supports %d", c.SchemaVersion, SchemaVersion)
	}
	return &c, nil
}

// Exposed returns the CLI-exposed operations, sorted by ID.
func (c *Catalog) Exposed() []Operation {
	var ops []Operation
	for _, op := range c.Operations {
		if op.Exposure.CLI {
			ops = append(ops, op)
		}
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].ID < ops[j].ID })
	return ops
}

// resolve follows $refs, a single allOf, and the null branch of anyOf/oneOf.
func (c *Catalog) resolve(s *Schema) *Schema {
	for i := 0; s != nil && i < 8; i++ {
		switch {
		case s.Ref != "":
			s = c.Schemas[strings.TrimPrefix(s.Ref, "#/components/schemas/")]
		case len(s.AllOf) == 1 && len(s.Properties) == 0:
			s = s.AllOf[0]
		case len(nonNull(s.AnyOf)) == 1 && len(s.Properties) == 0:
			s = nonNull(s.AnyOf)[0]
		case len(nonNull(s.OneOf)) == 1 && len(s.Properties) == 0:
			s = nonNull(s.OneOf)[0]
		default:
			return s
		}
	}
	return s
}

func nonNull(variants []*Schema) []*Schema {
	var out []*Schema
	for _, v := range variants {
		if v != nil && v.Type != "null" {
			out = append(out, v)
		}
	}
	return out
}

func (s *Schema) typeName() string {
	switch t := s.Type.(type) {
	case string:
		return t
	case []any:
		for _, v := range t {
			if name, ok := v.(string); ok && name != "null" {
				return name
			}
		}
	}
	return ""
}

// Subset returns a catalog with only exposed operations and the schemas they
// reach, used as the committed CLI contract snapshot.
func (c *Catalog) Subset() *Catalog {
	return &Catalog{SchemaVersion: c.SchemaVersion, Operations: c.Exposed(), Schemas: c.Schemas}
}
