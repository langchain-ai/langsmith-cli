package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/joho/godotenv"
	"github.com/langchain-ai/langsmith-cli/internal/langgraphapi"
)

const (
	deployDefaultConfig   = "langgraph.json"
	deploymentNameEnv     = "LANGSMITH_DEPLOYMENT_NAME"
	deployDefaultImageTag = "latest"
)

// Reserved by the control plane; never sent as secrets.
var deployReservedEnvVars = map[string]bool{
	"LANGCHAIN_TRACING_V2":            true,
	"LANGSMITH_TRACING_V2":            true,
	"LANGCHAIN_ENDPOINT":              true,
	"LANGCHAIN_PROJECT":               true,
	"LANGSMITH_PROJECT":               true,
	"LANGSMITH_LANGGRAPH_GIT_REPO":    true,
	"LANGGRAPH_GIT_REPO_PATH":         true,
	"LANGCHAIN_API_KEY":               true,
	"LANGSMITH_CONTROL_PLANE_API_KEY": true,
	"POSTGRES_URI":                    true,
	"POSTGRES_PASSWORD":               true,
	"DATABASE_URI":                    true,
	"LANGSMITH_LANGGRAPH_GIT_REF":     true,
	"LANGSMITH_LANGGRAPH_GIT_REF_SHA": true,
	"LANGGRAPH_AUTH_TYPE":             true,
	"LANGSMITH_AUTH_ENDPOINT":         true,
	"LANGSMITH_TENANT_ID":             true,
	"LANGSMITH_AUTH_VERIFY_TENANT_ID": true,
	"LANGSMITH_HOST_PROJECT_ID":       true,
	"LANGSMITH_HOST_PROJECT_NAME":     true,
	"LANGSMITH_HOST_REVISION_ID":      true,
	"LOG_JSON":                        true,
	"LOG_DICT_TRACEBACKS":             true,
	"REDIS_URI":                       true,
	"LANGCHAIN_CALLBACKS_BACKGROUND":  true,
	"DD_TRACE_PSYCOPG_ENABLED":        true,
	"DD_TRACE_REDIS_ENABLED":          true,
	"LANGSMITH_DEPLOYMENT_NAME":       true,
	"LANGGRAPH_CLOUD_LICENSE_KEY":     true,
	"LANGSMITH_API_KEY":               true,
	"LANGSMITH_ENDPOINT":              true,
	"POSTGRES_URI_CUSTOM":             true,
	"REDIS_URI_CUSTOM":                true,
	"PATH":                            true,
	"PORT":                            true,
	"MOUNT_PREFIX":                    true,
	"LSD_ENV":                         true,
	"LSD_DD_API_KEY":                  true,
	"LSD_DD_ENDPOINT":                 true,
	"LSD_DEPLOYMENT_TYPE":             true,
}

var (
	deployNameInvalidChars = regexp.MustCompile(`[^a-z0-9-]+`)
	deployImageTagPattern  = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
)

// langgraphConfig holds only the fields deploy reads.
type langgraphConfig struct {
	path         string
	env          json.RawMessage
	dependencies []string
}

func loadLanggraphConfig(path string) (*langgraphConfig, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("no %s found at %s; run 'langsmith deploy' from the root of a LangSmith Deployment project (see https://docs.langchain.com/langsmith/deployment-quickstart)", filepath.Base(path), abs)
	}
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Env          json.RawMessage `json:"env"`
		Dependencies []string        `json:"dependencies"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", abs, err)
	}
	return &langgraphConfig{path: abs, env: parsed.Env, dependencies: parsed.Dependencies}, nil
}

func (c *langgraphConfig) dir() string { return filepath.Dir(c.path) }

// envVars reads the inline env, the env file, or ./.env.
func (c *langgraphConfig) envVars(p *deployProgress) (map[string]string, error) {
	var inline map[string]json.RawMessage
	if json.Unmarshal(c.env, &inline) == nil && len(inline) > 0 {
		out := make(map[string]string, len(inline))
		for k, v := range inline {
			var s string
			if json.Unmarshal(v, &s) != nil {
				s = string(v)
			}
			out[k] = s
		}
		return out, nil
	}
	var envFile string
	if json.Unmarshal(c.env, &envFile) == nil && envFile != "" {
		path := filepath.Join(c.dir(), envFile)
		if _, err := os.Stat(path); err != nil {
			p.Note("Warning: env file %q specified in %s not found.", envFile, filepath.Base(c.path))
			return map[string]string{}, nil
		}
		return readDotenv(path)
	}
	return readDotenv(".env")
}

// Bare KEY lines are unset in python-dotenv but fatal to godotenv.
var dotenvBareKey = regexp.MustCompile(`(?m)^[ \t]*(export[ \t]+)?[A-Za-z_][A-Za-z0-9_.]*[ \t]*\r?$`)

func readDotenv(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	vars, err := godotenv.Unmarshal(dotenvBareKey.ReplaceAllString(string(data), ""))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return vars, nil
}

func deploySecrets(envVars map[string]string, p *deployProgress) []langgraphapi.Secret {
	secrets := []langgraphapi.Secret{}
	for _, name := range slices.Sorted(maps.Keys(envVars)) {
		if name == deploymentNameEnv {
			continue
		}
		if deployReservedEnvVars[name] {
			p.Note("Skipping reserved env var: %s", name)
			continue
		}
		if envVars[name] == "" {
			continue
		}
		secrets = append(secrets, langgraphapi.Secret{Name: name, Value: envVars[name]})
	}
	return secrets
}

// Deployment names allow only [a-z0-9-].
func normalizeDeploymentName(value string) string {
	slug := strings.Trim(deployNameInvalidChars.ReplaceAllString(strings.ToLower(value), "-"), "-")
	if slug == "" {
		return "app"
	}
	return slug
}

func normalizeImageTag(value string) (string, error) {
	if value == "" {
		value = deployDefaultImageTag
	}
	if !deployImageTagPattern.MatchString(value) {
		return "", errors.New("image tag may only contain characters A-Z, a-z, 0-9, '_', '-', '.'")
	}
	return value, nil
}

func cwdDeploymentName() string {
	wd, err := os.Getwd()
	if err != nil {
		return normalizeDeploymentName("")
	}
	return normalizeDeploymentName(filepath.Base(wd))
}
