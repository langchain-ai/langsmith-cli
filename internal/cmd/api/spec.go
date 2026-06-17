package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/cache"
)

const specCacheTTL = 24 * time.Hour

// OpenAPISpec holds the parsed parts of the OpenAPI spec we care about.
type OpenAPISpec struct {
	OpenAPI    string                                `json:"openapi"`
	Info       json.RawMessage                       `json:"info"`
	Paths      map[string]map[string]json.RawMessage `json:"paths"`
	Components json.RawMessage                       `json:"components"`
}

// Endpoint is a single method+path from the spec.
type Endpoint struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Summary string `json:"summary"`
	Tag     string `json:"tag"`
}

// EndpointDetail has full info for a single endpoint.
type EndpointDetail struct {
	Method      string      `json:"method"`
	Path        string      `json:"path"`
	Summary     string      `json:"summary"`
	Tag         string      `json:"tag"`
	Description string      `json:"description,omitempty"`
	Parameters  []Parameter `json:"parameters"`
	RequestBody any         `json:"request_body"`
	Response    any         `json:"response_schema"`
}

// Parameter describes a single API parameter.
type Parameter struct {
	Name        string `json:"name"`
	In          string `json:"in"`
	Required    bool   `json:"required"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
}

// Endpoints returns a sorted list of all endpoints in the spec.
func (s *OpenAPISpec) Endpoints() []Endpoint {
	var endpoints []Endpoint
	for path, methods := range s.Paths {
		for method, raw := range methods {
			m := strings.ToUpper(method)
			if !isHTTPMethod(m) {
				continue // skip "parameters", "summary", etc.
			}
			var detail struct {
				Summary string   `json:"summary"`
				Tags    []string `json:"tags"`
			}
			_ = json.Unmarshal(raw, &detail)
			tag := ""
			if len(detail.Tags) > 0 {
				tag = detail.Tags[0]
			}
			endpoints = append(endpoints, Endpoint{
				Method:  m,
				Path:    path,
				Summary: detail.Summary,
				Tag:     tag,
			})
		}
	}
	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Path != endpoints[j].Path {
			return endpoints[i].Path < endpoints[j].Path
		}
		return endpoints[i].Method < endpoints[j].Method
	})
	return endpoints
}

// resolveShorthand maps a shorthand path to an absolute spec path (prefer /api/v1/, then exact /<path>, then a unique suffix match); "" if ambiguous.
func (s *OpenAPISpec) resolveShorthand(path string) string {
	if candidate := "/api/v1/" + path; s.hasPath(candidate) {
		return candidate
	}
	if candidate := "/" + path; s.hasPath(candidate) {
		return candidate
	}
	suffix := "/" + path
	var match string
	for key := range s.Paths {
		if strings.HasSuffix(key, suffix) {
			if match != "" {
				return "" // ambiguous
			}
			match = key
		}
	}
	return match
}

func (s *OpenAPISpec) hasPath(path string) bool {
	_, ok := s.Paths[path]
	return ok
}

// LookupEndpoint finds an endpoint by method and path, returning full detail.
// The path argument can be shorthand ("sessions") or absolute ("/api/v1/sessions").
func (s *OpenAPISpec) LookupEndpoint(method, path string) (*EndpointDetail, error) {
	normalized := path
	if !strings.HasPrefix(normalized, "/") {
		if resolved := s.resolveShorthand(normalized); resolved != "" {
			normalized = resolved
		} else {
			normalized = "/api/v1/" + normalized
		}
	}
	method = strings.ToUpper(method)

	methods, ok := s.Paths[normalized]
	if !ok {
		return nil, fmt.Errorf("endpoint not found: %s %s", method, normalized)
	}
	raw, ok := methods[strings.ToLower(method)]
	if !ok {
		return nil, fmt.Errorf("endpoint not found: %s %s", method, normalized)
	}

	var parsed struct {
		Summary     string            `json:"summary"`
		Description string            `json:"description"`
		Tags        []string          `json:"tags"`
		Parameters  []json.RawMessage `json:"parameters"`
		RequestBody json.RawMessage   `json:"requestBody"`
		Responses   json.RawMessage   `json:"responses"`
	}
	_ = json.Unmarshal(raw, &parsed)

	tag := ""
	if len(parsed.Tags) > 0 {
		tag = parsed.Tags[0]
	}

	// Parse parameters
	var params []Parameter
	for _, pRaw := range parsed.Parameters {
		var p struct {
			Name        string `json:"name"`
			In          string `json:"in"`
			Required    bool   `json:"required"`
			Description string `json:"description"`
			Schema      struct {
				Type string `json:"type"`
			} `json:"schema"`
		}
		_ = json.Unmarshal(pRaw, &p)
		params = append(params, Parameter{
			Name:        p.Name,
			In:          p.In,
			Required:    p.Required,
			Type:        p.Schema.Type,
			Description: p.Description,
		})
	}

	// Parse request body — resolve $ref one level deep
	var reqBody any
	if parsed.RequestBody != nil {
		reqBody = s.resolveRequestBody(parsed.RequestBody)
	}

	// Parse response — take first 2xx response
	var respSchema any
	if parsed.Responses != nil {
		respSchema = s.resolveResponse(parsed.Responses)
	}

	return &EndpointDetail{
		Method:      method,
		Path:        normalized,
		Summary:     parsed.Summary,
		Tag:         tag,
		Description: parsed.Description,
		Parameters:  params,
		RequestBody: reqBody,
		Response:    respSchema,
	}, nil
}

// resolveRequestBody extracts and resolves the request body schema.
func (s *OpenAPISpec) resolveRequestBody(raw json.RawMessage) any {
	var body struct {
		Required bool `json:"required"`
		Content  map[string]struct {
			Schema json.RawMessage `json:"schema"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil
	}
	for contentType, ct := range body.Content {
		schema := s.resolveRef(ct.Schema)
		return map[string]any{
			"content_type": contentType,
			"required":     body.Required,
			"schema":       schema,
		}
	}
	return nil
}

// resolveResponse extracts the first 2xx response schema.
func (s *OpenAPISpec) resolveResponse(raw json.RawMessage) any {
	var responses map[string]struct {
		Content map[string]struct {
			Schema json.RawMessage `json:"schema"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &responses); err != nil {
		return nil
	}
	for _, code := range []string{"200", "201", "202", "204"} {
		resp, ok := responses[code]
		if !ok {
			continue
		}
		for _, ct := range resp.Content {
			return s.resolveRef(ct.Schema)
		}
	}
	return nil
}

// resolveRef resolves a JSON schema, inlining one level of $ref from components.
func (s *OpenAPISpec) resolveRef(raw json.RawMessage) any {
	if raw == nil {
		return nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		var result any
		_ = json.Unmarshal(raw, &result)
		return result
	}

	// Check for $ref
	if refRaw, ok := obj["$ref"]; ok {
		var ref string
		_ = json.Unmarshal(refRaw, &ref)
		resolved := s.resolveComponentRef(ref)
		if resolved != nil {
			return resolved
		}
	}

	// Otherwise return as generic map
	var result any
	_ = json.Unmarshal(raw, &result)
	return result
}

// resolveComponentRef resolves a $ref like "#/components/schemas/Foo" one level deep.
func (s *OpenAPISpec) resolveComponentRef(ref string) any {
	if s.Components == nil {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(ref, "#/"), "/")
	if len(parts) < 3 || parts[0] != "components" || parts[1] != "schemas" {
		return map[string]any{"$ref": ref}
	}
	schemaName := parts[2]

	var components struct {
		Schemas map[string]json.RawMessage `json:"schemas"`
	}
	if err := json.Unmarshal(s.Components, &components); err != nil {
		return map[string]any{"$ref": ref}
	}
	schemaRaw, ok := components.Schemas[schemaName]
	if !ok {
		return map[string]any{"$ref": ref}
	}

	// Parse one level — don't recurse into nested $refs
	var schema any
	_ = json.Unmarshal(schemaRaw, &schema)
	return schema
}

// cachedSpec returns the cached spec (nil on miss); it never fetches, so it is safe on the request hot path.
func cachedSpec(apiURL, cacheDir string) *OpenAPISpec {
	data, err := cache.ReadIfFresh(cache.PathForKey(cacheDir, "openapi", apiURL), specCacheTTL)
	if err != nil {
		return nil
	}
	var spec OpenAPISpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil
	}
	return &spec
}

// loadSpec loads the OpenAPI spec, using cache if available and not expired.
func loadSpec(apiURL, cacheDir string, forceRefresh bool) (*OpenAPISpec, error) {
	cachePath := cache.PathForKey(cacheDir, "openapi", apiURL)

	if !forceRefresh {
		if data, err := cache.ReadIfFresh(cachePath, specCacheTTL); err == nil {
			var spec OpenAPISpec
			if err := json.Unmarshal(data, &spec); err == nil {
				return &spec, nil
			}
		}
	}

	// Fetch from server
	specURL := apiURL + "/openapi.json"
	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Get(specURL)
	if err != nil {
		return nil, fmt.Errorf("fetching OpenAPI spec from %s: %w", specURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetching OpenAPI spec: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading OpenAPI spec: %w", err)
	}

	var spec OpenAPISpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parsing OpenAPI spec: %w", err)
	}

	// Write cache (best-effort)
	_ = cache.Write(cachePath, data)

	return &spec, nil
}
