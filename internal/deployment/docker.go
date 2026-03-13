package deployment

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const DefaultPostgresURI = "postgres://postgres:postgres@langgraph-postgres:5432/postgres?sslmode=disable"

// Version represents a semantic version.
type Version struct {
	Major int
	Minor int
	Patch int
}

// Less returns true if v is less than other.
func (v Version) Less(other Version) bool {
	if v.Major != other.Major {
		return v.Major < other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor < other.Minor
	}
	return v.Patch < other.Patch
}

// GreaterOrEqual returns true if v >= other.
func (v Version) GreaterOrEqual(other Version) bool {
	return !v.Less(other)
}

// DockerComposeType is either "plugin" or "standalone".
type DockerComposeType string

const (
	ComposePlugin     DockerComposeType = "plugin"
	ComposeStandalone DockerComposeType = "standalone"
)

// DockerCapabilities holds detected Docker capabilities.
type DockerCapabilities struct {
	VersionDocker              Version
	VersionCompose             Version
	HealthcheckStartInterval   bool
	ComposeType                DockerComposeType
}

// ParseVersion parses a version string into a Version.
func ParseVersion(version string) Version {
	version = strings.TrimPrefix(version, "v")
	parts := strings.SplitN(version, ".", 3)

	parsePart := func(s string) int {
		// Remove anything after - or +
		s = strings.Split(s, "-")[0]
		s = strings.Split(s, "+")[0]
		n, _ := strconv.Atoi(s)
		return n
	}

	switch len(parts) {
	case 1:
		return Version{Major: parsePart(parts[0])}
	case 2:
		return Version{Major: parsePart(parts[0]), Minor: parsePart(parts[1])}
	default:
		return Version{Major: parsePart(parts[0]), Minor: parsePart(parts[1]), Patch: parsePart(parts[2])}
	}
}

// CheckCapabilities detects Docker and Docker Compose capabilities.
func CheckCapabilities() (*DockerCapabilities, error) {
	// Check docker available
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, fmt.Errorf("docker not installed")
	}

	stdout, _, err := RunCommand("docker", "info", "-f", "{{json .}}")
	if err != nil {
		return nil, fmt.Errorf("docker not installed or not running")
	}

	var info map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &info); err != nil {
		return nil, fmt.Errorf("docker not installed or not running")
	}

	serverVersion, _ := info["ServerVersion"].(string)
	if serverVersion == "" {
		return nil, fmt.Errorf("docker not running")
	}

	var composeVersionStr string
	var composeType DockerComposeType

	// Try plugin first
	if clientInfo, ok := info["ClientInfo"].(map[string]interface{}); ok {
		if plugins, ok := clientInfo["Plugins"].([]interface{}); ok {
			for _, p := range plugins {
				if plugin, ok := p.(map[string]interface{}); ok {
					if plugin["Name"] == "compose" {
						if v, ok := plugin["Version"].(string); ok {
							composeVersionStr = v
							composeType = ComposePlugin
						}
					}
				}
			}
		}
	}

	if composeVersionStr == "" {
		// Try standalone
		if _, err := exec.LookPath("docker-compose"); err != nil {
			return nil, fmt.Errorf("docker compose not installed")
		}
		composeVersionStr, _, err = RunCommand("docker-compose", "--version", "--short")
		if err != nil {
			return nil, fmt.Errorf("docker compose not installed")
		}
		composeType = ComposeStandalone
	}

	dockerVersion := ParseVersion(serverVersion)
	composeVersion := ParseVersion(composeVersionStr)

	return &DockerCapabilities{
		VersionDocker:            dockerVersion,
		VersionCompose:           composeVersion,
		HealthcheckStartInterval: dockerVersion.GreaterOrEqual(Version{25, 0, 0}),
		ComposeType:              composeType,
	}, nil
}

// ComposeOptions holds options for generating a docker-compose file.
type ComposeOptions struct {
	Port               int
	DebuggerPort       int
	DebuggerBaseURL    string
	PostgresURI        string
	Image              string
	BaseImage          string
	APIVersion         string
	EngineRuntimeMode  string
}

// ComposeAsDict creates a docker compose file as nested ordered maps.
func ComposeAsDict(caps *DockerCapabilities, opts ComposeOptions) map[string]interface{} {
	if opts.EngineRuntimeMode == "" {
		opts.EngineRuntimeMode = "combined_queue_worker"
	}

	includeDB := opts.PostgresURI == ""
	postgresURI := opts.PostgresURI
	if postgresURI == "" {
		postgresURI = DefaultPostgresURI
	}

	services := newOrderedMap()

	// Redis service
	redisHealthcheck := newOrderedMap()
	redisHealthcheck.Set("test", "redis-cli ping")
	redisHealthcheck.Set("interval", "5s")
	redisHealthcheck.Set("timeout", "1s")
	redisHealthcheck.Set("retries", 5)

	redis := newOrderedMap()
	redis.Set("image", "redis:6")
	redis.Set("healthcheck", redisHealthcheck)
	services.Set("langgraph-redis", redis)

	// Postgres service
	if includeDB {
		pgEnv := newOrderedMap()
		pgEnv.Set("POSTGRES_DB", "postgres")
		pgEnv.Set("POSTGRES_USER", "postgres")
		pgEnv.Set("POSTGRES_PASSWORD", "postgres")

		pgHealthcheck := newOrderedMap()
		pgHealthcheck.Set("test", "pg_isready -U postgres")
		pgHealthcheck.Set("start_period", "10s")
		pgHealthcheck.Set("timeout", "1s")
		pgHealthcheck.Set("retries", 5)
		if caps.HealthcheckStartInterval {
			pgHealthcheck.Set("interval", "60s")
			pgHealthcheck.Set("start_interval", "1s")
		} else {
			pgHealthcheck.Set("interval", "5s")
		}

		postgres := newOrderedMap()
		postgres.Set("image", "pgvector/pgvector:pg16")
		postgres.Set("ports", []string{`"5433:5432"`})
		postgres.Set("environment", pgEnv)
		postgres.Set("command", []string{"postgres", "-c", "shared_preload_libraries=vector"})
		postgres.Set("volumes", []string{"langgraph-data:/var/lib/postgresql/data"})
		postgres.Set("healthcheck", pgHealthcheck)
		services.Set("langgraph-postgres", postgres)
	}

	// Debugger service
	if opts.DebuggerPort > 0 {
		debugger := newOrderedMap()
		debugger.Set("image", "langchain/langgraph-debugger")
		debugger.Set("restart", "on-failure")

		if includeDB {
			debuggerDeps := newOrderedMap()
			pgDep := newOrderedMap()
			pgDep.Set("condition", "service_healthy")
			debuggerDeps.Set("langgraph-postgres", pgDep)
			debugger.Set("depends_on", debuggerDeps)
		}

		debugger.Set("ports", []string{fmt.Sprintf(`"%d:3968"`, opts.DebuggerPort)})

		if opts.DebuggerBaseURL != "" {
			debuggerEnv := newOrderedMap()
			debuggerEnv.Set("VITE_STUDIO_LOCAL_GRAPH_URL", opts.DebuggerBaseURL)
			debugger.Set("environment", debuggerEnv)
		}

		services.Set("langgraph-debugger", debugger)
	}

	// API service
	apiEnv := newOrderedMap()
	apiEnv.Set("REDIS_URI", "redis://langgraph-redis:6379")
	apiEnv.Set("POSTGRES_URI", postgresURI)
	if opts.EngineRuntimeMode == "distributed" {
		apiEnv.Set("N_JOBS_PER_WORKER", `"0"`)
	}

	apiDeps := newOrderedMap()
	redisDep := newOrderedMap()
	redisDep.Set("condition", "service_healthy")
	apiDeps.Set("langgraph-redis", redisDep)

	if includeDB {
		pgDep := newOrderedMap()
		pgDep.Set("condition", "service_healthy")
		apiDeps.Set("langgraph-postgres", pgDep)
	}

	api := newOrderedMap()
	api.Set("ports", []string{fmt.Sprintf(`"%d:8000"`, opts.Port)})
	api.Set("depends_on", apiDeps)
	api.Set("environment", apiEnv)

	if opts.Image != "" {
		api.Set("image", opts.Image)
	}

	if caps.HealthcheckStartInterval {
		apiHealthcheck := newOrderedMap()
		apiHealthcheck.Set("test", "python /api/healthcheck.py")
		apiHealthcheck.Set("interval", "60s")
		apiHealthcheck.Set("start_interval", "1s")
		apiHealthcheck.Set("start_period", "10s")
		api.Set("healthcheck", apiHealthcheck)
	}

	services.Set("langgraph-api", api)

	// Build compose dict
	compose := newOrderedMap()
	if includeDB {
		volumes := newOrderedMap()
		volData := newOrderedMap()
		volData.Set("driver", "local")
		volumes.Set("langgraph-data", volData)
		compose.Set("volumes", volumes)
	}
	compose.Set("services", services)

	return compose.ToMap()
}

// Compose creates a docker compose file as a YAML string.
func Compose(caps *DockerCapabilities, opts ComposeOptions) string {
	d := ComposeAsDict(caps, opts)
	return DictToYAML(d, 0, false)
}

// DictToYAML converts a map to a YAML string.
// It handles orderedMap and regular maps, lists, and scalar values.
func DictToYAML(d interface{}, indent int, isTopLevel ...bool) string {
	topLevel := len(isTopLevel) == 0 || isTopLevel[0]
	space := strings.Repeat("    ", indent)
	var result strings.Builder

	switch v := d.(type) {
	case map[string]interface{}:
		// Use ordered keys from _order if present
		keys := getOrderedKeys(v)
		for idx, key := range keys {
			if idx >= 1 && indent < 2 && topLevel {
				result.WriteString("\n")
			}
			val := v[key]
			switch child := val.(type) {
			case map[string]interface{}:
				result.WriteString(fmt.Sprintf("%s%s:\n", space, key))
				result.WriteString(DictToYAML(child, indent+1, false))
			case []string:
				result.WriteString(fmt.Sprintf("%s%s:\n", space, key))
				for _, item := range child {
					result.WriteString(fmt.Sprintf("%s    - %s\n", space, item))
				}
			case []interface{}:
				result.WriteString(fmt.Sprintf("%s%s:\n", space, key))
				for _, item := range child {
					result.WriteString(fmt.Sprintf("%s    - %s\n", space, item))
				}
			default:
				result.WriteString(fmt.Sprintf("%s%s: %v\n", space, key, val))
			}
		}
	}

	return result.String()
}

// orderedMap preserves insertion order of keys.
type orderedMap struct {
	keys   []string
	values map[string]interface{}
}

func newOrderedMap() *orderedMap {
	return &orderedMap{
		values: make(map[string]interface{}),
	}
}

func (m *orderedMap) Set(key string, value interface{}) {
	if _, exists := m.values[key]; !exists {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

func (m *orderedMap) ToMap() map[string]interface{} {
	result := make(map[string]interface{})
	// Store order as a special key
	result["_order"] = m.keys
	for _, k := range m.keys {
		v := m.values[k]
		if om, ok := v.(*orderedMap); ok {
			result[k] = om.ToMap()
		} else {
			result[k] = v
		}
	}
	return result
}

func getOrderedKeys(m map[string]interface{}) []string {
	if order, ok := m["_order"].([]string); ok {
		return order
	}
	// Fallback: collect keys
	var keys []string
	for k := range m {
		if k != "_order" {
			keys = append(keys, k)
		}
	}
	return keys
}
