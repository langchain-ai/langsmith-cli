package api

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
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

// LookupEndpoint finds an endpoint by method and path, returning full detail.
// The path argument can be shorthand ("sessions") or absolute ("/api/v1/sessions").
func (s *OpenAPISpec) LookupEndpoint(method, path string) (*EndpointDetail, error) {
	// Normalize: if shorthand, prefix /api/v1/
	normalized := path
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/api/v1/" + normalized
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

// loadSpec loads the OpenAPI spec, using cache if available and not expired.
func loadSpec(apiURL, cacheDir string, forceRefresh bool) (*OpenAPISpec, error) {
	cachePath := specCachePath(cacheDir, apiURL)

	if !forceRefresh {
		if spec, err := loadCachedSpec(cachePath); err == nil {
			return spec, nil
		}
	}

	// Fetch from server
	specURL := apiURL + "/openapi.json"
	resp, err := http.Get(specURL)
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

	// Write cache (best-effort; ignore errors so a read-only cache dir doesn't break the command)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return &spec, nil
	}
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return &spec, nil
	}

	return &spec, nil
}

// loadCachedSpec reads a cached spec if it exists and is within TTL.
func loadCachedSpec(cachePath string) (*OpenAPISpec, error) {
	info, err := os.Stat(cachePath)
	if err != nil {
		return nil, err
	}
	if time.Since(info.ModTime()) > specCacheTTL {
		return nil, fmt.Errorf("cache expired")
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}
	var spec OpenAPISpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

// specCachePath returns the cache file path for a given API URL.
func specCachePath(cacheDir, apiURL string) string {
	h := sha256.Sum256([]byte(apiURL))
	name := fmt.Sprintf("openapi-%x.json", h[:8])
	return filepath.Join(cacheDir, name)
}

// defaultCacheDir returns ~/.langsmith/cache.
func defaultCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "langsmith-cache")
	}
	return filepath.Join(home, ".langsmith", "cache")
}
