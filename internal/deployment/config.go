package deployment

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	DefaultConfig        = "langgraph.json"
	DefaultPort          = 8123
	MinPythonVersion     = "3.11"
	DefaultPythonVersion = "3.11"
	DefaultNodeVersion   = "20"
	MinNodeVersion       = "20"
	DefaultImageDistro   = "debian"
)

// DisallowedBuildCommandChars are characters not allowed in build commands.
var DisallowedBuildCommandChars = []string{
	`"`, "`", `\`, "\n", "\r", "\x00", "\t", "|", ";", "$", ">", "<",
}

// singleAmpersandRe matches a single & that is NOT part of &&.
var singleAmpersandRe = regexp.MustCompile(`(?:^|[^&])&(?:[^&]|$)`)

// HasDisallowedBuildCommandContent checks if a command string contains disallowed characters.
func HasDisallowedBuildCommandContent(command string) bool {
	for _, ch := range DisallowedBuildCommandChars {
		if strings.Contains(command, ch) {
			return true
		}
	}
	return singleAmpersandRe.MatchString(command)
}

// Config represents the validated langgraph.json configuration.
type Config struct {
	PythonVersion  string                 `json:"python_version,omitempty"`
	NodeVersion    string                 `json:"node_version,omitempty"`
	APIVersion     string                 `json:"api_version,omitempty"`
	BaseImage      string                 `json:"base_image,omitempty"`
	ImageDistro    string                 `json:"image_distro,omitempty"`
	PipConfigFile  string                 `json:"pip_config_file,omitempty"`
	PipInstaller   string                 `json:"pip_installer,omitempty"`
	DockerfileLines []string              `json:"dockerfile_lines,omitempty"`
	Dependencies   []string              `json:"dependencies,omitempty"`
	Graphs         map[string]interface{} `json:"graphs,omitempty"`
	Env            interface{}            `json:"env,omitempty"` // string (file path) or map[string]string
	Store          interface{}            `json:"store,omitempty"`
	Auth           interface{}            `json:"auth,omitempty"`
	Encryption     interface{}            `json:"encryption,omitempty"`
	Webhooks       interface{}            `json:"webhooks,omitempty"`
	Checkpointer   interface{}            `json:"checkpointer,omitempty"`
	HTTP           interface{}            `json:"http,omitempty"`
	UI             interface{}            `json:"ui,omitempty"`
	KeepPkgTools   interface{}            `json:"keep_pkg_tools,omitempty"`
}

// LoadConfig loads and parses a langgraph.json configuration file.
func LoadConfig(configPath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", configPath, err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", configPath, err)
	}
	return raw, nil
}

// ValidateConfig validates and normalizes a raw config map.
func ValidateConfig(raw map[string]interface{}) (map[string]interface{}, error) {
	config := make(map[string]interface{})
	for k, v := range raw {
		config[k] = v
	}

	// Check for node_version - if present, this is a JS project
	isNodeProject := false
	if nv, ok := config["node_version"]; ok && nv != nil {
		isNodeProject = true
		nodeStr := fmt.Sprintf("%v", nv)
		nodeVer, err := strconv.Atoi(nodeStr)
		if err != nil {
			return nil, fmt.Errorf("invalid node_version: must be a major version number (e.g., '20')")
		}
		minNode, _ := strconv.Atoi(MinNodeVersion)
		if nodeVer < minNode {
			return nil, fmt.Errorf("minimum required Node.js version is %s, got %s", MinNodeVersion, nodeStr)
		}
	}

	if !isNodeProject {
		// Validate python_version
		pyVer := DefaultPythonVersion
		if pv, ok := config["python_version"]; ok && pv != nil {
			pyVer = fmt.Sprintf("%v", pv)
		}

		// Check for bullseye
		if strings.Contains(pyVer, "bullseye") {
			return nil, fmt.Errorf("bullseye images were deprecated: please use 'bookworm' or 'wolfi' instead")
		}

		// Extract base version for validation (handle suffixes like -slim)
		basePyVer := pyVer
		if idx := strings.Index(pyVer, "-"); idx > 0 {
			basePyVer = pyVer[:idx]
		}

		// Validate format: must be major.minor
		parts := strings.Split(basePyVer, ".")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid Python version format: '%s', must be in 'major.minor' format (e.g., '3.11')", pyVer)
		}
		major, errMajor := strconv.Atoi(parts[0])
		minor, errMinor := strconv.Atoi(parts[1])
		if errMajor != nil || errMinor != nil {
			return nil, fmt.Errorf("invalid Python version format: '%s', must be in 'major.minor' format (e.g., '3.11')", pyVer)
		}

		minParts := strings.Split(MinPythonVersion, ".")
		minMajor, _ := strconv.Atoi(minParts[0])
		minMinor, _ := strconv.Atoi(minParts[1])
		if major < minMajor || (major == minMajor && minor < minMinor) {
			return nil, fmt.Errorf("minimum required version is %s, got %s", MinPythonVersion, basePyVer)
		}

		config["python_version"] = pyVer

		// Validate dependencies
		if _, ok := config["dependencies"]; !ok {
			return nil, fmt.Errorf("'dependencies' is required for Python projects")
		}

		// Validate graphs
		if _, ok := config["graphs"]; !ok {
			return nil, fmt.Errorf("'graphs' is required")
		}
	} else {
		// JS project validation
		if _, ok := config["graphs"]; !ok {
			return nil, fmt.Errorf("'graphs' is required")
		}
	}

	// Validate image_distro
	if distro, ok := config["image_distro"]; ok && distro != nil {
		distroStr := fmt.Sprintf("%v", distro)
		if distroStr == "bullseye" {
			return nil, fmt.Errorf("bullseye images were deprecated: please use 'bookworm' or 'wolfi' instead")
		}
		validDistros := map[string]bool{"debian": true, "wolfi": true, "bookworm": true}
		if !validDistros[distroStr] {
			return nil, fmt.Errorf("invalid image_distro '%s'. Must be one of: debian, wolfi, bookworm", distroStr)
		}
	} else {
		config["image_distro"] = DefaultImageDistro
	}

	// Validate pip_installer
	if pi, ok := config["pip_installer"]; ok && pi != nil {
		piStr := fmt.Sprintf("%v", pi)
		validInstallers := map[string]bool{"auto": true, "pip": true, "uv": true}
		if !validInstallers[piStr] {
			return nil, fmt.Errorf("invalid pip_installer '%s'. Must be one of: auto, pip, uv", piStr)
		}
	} else {
		config["pip_installer"] = "auto"
	}

	// Validate http.app format if present
	if httpConfig, ok := config["http"].(map[string]interface{}); ok {
		if app, ok := httpConfig["app"].(string); ok {
			if !strings.Contains(app, ":") || strings.HasPrefix(app, "..") {
				return nil, fmt.Errorf("invalid http.app format: '%s', must be 'path/to/file.py:attribute'", app)
			}
		}
	}

	// Set defaults for missing optional fields
	defaults := map[string]interface{}{
		"base_image":      nil,
		"node_version":    nil,
		"pip_config_file": nil,
		"dockerfile_lines": []interface{}{},
		"env":             map[string]interface{}{},
		"store":           nil,
		"auth":            nil,
		"encryption":      nil,
		"webhooks":        nil,
		"checkpointer":    nil,
		"http":            nil,
		"ui":              nil,
		"keep_pkg_tools":  nil,
	}
	for k, v := range defaults {
		if _, ok := config[k]; !ok {
			config[k] = v
		}
	}

	return config, nil
}

// ValidateConfigFile loads and validates a config file.
func ValidateConfigFile(configPath string) (map[string]interface{}, error) {
	raw, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}
	return ValidateConfig(raw)
}

// DefaultBaseImage returns the default base image for a config.
func DefaultBaseImage(config map[string]interface{}) string {
	if nv, ok := config["node_version"]; ok && nv != nil {
		return "langchain/langgraphjs-api"
	}
	return "langchain/langgraph-api"
}

// DockerTag computes the Docker tag for a config.
func DockerTag(baseImage string, config map[string]interface{}, apiVersion string, engineRuntimeMode string) string {
	if baseImage == "" {
		if engineRuntimeMode == "distributed" {
			baseImage = "langchain/langgraph-executor"
		} else {
			baseImage = DefaultBaseImage(config)
		}
	}

	// Build tag suffix
	var tag string
	if apiVersion != "" {
		tag = apiVersion + "-"
	}

	if nv, ok := config["node_version"]; ok && nv != nil {
		tag += fmt.Sprintf("node%v", nv)
	} else {
		pv := DefaultPythonVersion
		if pyVer, ok := config["python_version"]; ok && pyVer != nil {
			pv = fmt.Sprintf("%v", pyVer)
		}
		tag += fmt.Sprintf("py%s", pv)
	}

	// Add distro suffix
	if distro, ok := config["image_distro"]; ok && distro != nil {
		distroStr := fmt.Sprintf("%v", distro)
		if distroStr != "" && distroStr != "debian" {
			tag += "-" + distroStr
		}
	}

	if tag != "" {
		return baseImage + ":" + tag
	}
	return baseImage
}

// ConfigToDocker generates a Dockerfile string from a config.
func ConfigToDocker(config map[string]interface{}, configDir string, baseImage string, apiVersion string, engineRuntimeMode string) (string, error) {
	if engineRuntimeMode == "" {
		engineRuntimeMode = "combined_queue_worker"
	}

	fromImage := DockerTag(baseImage, config, apiVersion, engineRuntimeMode)

	var lines []string
	lines = append(lines, "# syntax=docker/dockerfile:1.4")
	lines = append(lines, fmt.Sprintf("FROM %s", fromImage))

	// Handle dependencies
	deps, _ := config["dependencies"].([]interface{})
	if len(deps) > 0 {
		var localDeps []string
		var workDir string

		for i, dep := range deps {
			depStr, ok := dep.(string)
			if !ok {
				continue
			}

			// Check if it's a local dependency
			if depStr == "." || strings.HasPrefix(depStr, "./") || strings.HasPrefix(depStr, "../") {
				localDeps = append(localDeps, depStr)

				// Resolve the dependency path
				var containerPath string
				if depStr == "." {
					containerPath = "/deps/" + filepath.Base(configDir)
					if workDir == "" {
						workDir = containerPath
					}
				} else {
					absPath, _ := filepath.Abs(filepath.Join(configDir, depStr))
					containerPath = fmt.Sprintf("/deps/%s_%d", filepath.Base(absPath), i)
				}

				// Check if it's a parent directory reference
				if strings.HasPrefix(depStr, "../") {
					lines = append(lines, fmt.Sprintf("# -- Adding local package %s --", depStr))
					lines = append(lines, fmt.Sprintf("COPY --from=cli_%d . %s", i, containerPath))
					lines = append(lines, fmt.Sprintf("# -- End of local package %s --", depStr))
				} else {
					lines = append(lines, fmt.Sprintf("# -- Adding local package %s --", depStr))
					lines = append(lines, fmt.Sprintf("ADD %s %s", depStr, containerPath))
					lines = append(lines, fmt.Sprintf("# -- End of local package %s --", depStr))
				}
			}
		}

		if len(localDeps) > 0 {
			// Install all local dependencies
			lines = append(lines, "# -- Installing all local dependencies --")
			installCmd := `RUN for dep in /deps/*; do \
            echo "Installing $$dep"; \
            if [ -d "$$dep" ]; then \
                echo "Installing $$dep"; \
                (cd "$$dep" && PYTHONDONTWRITEBYTECODE=1 uv pip install --system --no-cache-dir -c /api/constraints.txt -e .); \
            fi; \
        done`
			lines = append(lines, installCmd)
			lines = append(lines, "# -- End of local dependencies install --")
		}
	}

	// Set LANGSERVE_GRAPHS environment variable
	if graphs, ok := config["graphs"].(map[string]interface{}); ok && len(graphs) > 0 {
		graphsJSON, _ := json.Marshal(graphs)
		lines = append(lines, fmt.Sprintf("ENV LANGSERVE_GRAPHS='%s'", string(graphsJSON)))
	}

	// Handle additional dockerfile_lines
	if dfLines, ok := config["dockerfile_lines"].([]interface{}); ok {
		for _, line := range dfLines {
			if lineStr, ok := line.(string); ok {
				lines = append(lines, lineStr)
			}
		}
	}

	// Set working directory
	if deps, ok := config["dependencies"].([]interface{}); ok {
		for _, dep := range deps {
			if depStr, ok := dep.(string); ok && depStr == "." {
				lines = append(lines, fmt.Sprintf("WORKDIR /deps/%s", filepath.Base(configDir)))
				break
			}
		}
	}

	return strings.Join(lines, "\n"), nil
}

// ConfigToCompose generates a docker-compose YAML string for a config.
// Returns the compose args (for docker compose command) and the stdin content.
func ConfigToCompose(
	caps *DockerCapabilities,
	configPath string,
	config map[string]interface{},
	opts ComposeOptions,
	watch bool,
	dockerCompose string,
	apiVersion string,
	engineRuntimeMode string,
) (args []string, stdin string, err error) {
	configDir := filepath.Dir(configPath)
	absConfigDir, _ := filepath.Abs(configDir)

	args = []string{
		"--project-directory", absConfigDir,
	}

	if dockerCompose != "" {
		args = append(args, "-f", dockerCompose)
	}
	args = append(args, "-f", "-")

	// Generate compose content
	composeStr := Compose(caps, opts)

	// If no image specified, add build section
	if opts.Image == "" {
		dockerfile, err := ConfigToDocker(config, absConfigDir, opts.BaseImage, apiVersion, engineRuntimeMode)
		if err != nil {
			return nil, "", err
		}

		// Build the build context section
		var buildContexts []string
		if deps, ok := config["dependencies"].([]interface{}); ok {
			for i, dep := range deps {
				if depStr, ok := dep.(string); ok && strings.HasPrefix(depStr, "../") {
					absPath, _ := filepath.Abs(filepath.Join(absConfigDir, depStr))
					buildContexts = append(buildContexts, fmt.Sprintf("            - cli_%d: %s", i, absPath))
				}
			}
		}

		buildSection := "\n        pull_policy: build\n        build:\n            context: .\n"
		if len(buildContexts) > 0 {
			buildSection += "            additional_contexts:\n" + strings.Join(buildContexts, "\n") + "\n"
		}
		buildSection += "            dockerfile_inline: |\n"
		for _, line := range strings.Split(dockerfile, "\n") {
			buildSection += "                " + line + "\n"
		}

		// Add watch section
		if watch {
			watchSection := "\n        develop:\n            watch:\n"
			watchSection += "                - path: " + DefaultConfig + "\n"
			watchSection += "                  action: rebuild\n"
			if deps, ok := config["dependencies"].([]interface{}); ok {
				for _, dep := range deps {
					if depStr, ok := dep.(string); ok {
						if depStr == "." || strings.HasPrefix(depStr, "./") || strings.HasPrefix(depStr, "../") {
							watchSection += "                - path: " + depStr + "\n"
							watchSection += "                  action: rebuild\n"
						}
					}
				}
			}
			buildSection += watchSection
		}

		composeStr += buildSection
	}

	return args, composeStr, nil
}

// ReservedEnvVars are environment variables that should not be sent to deployments.
var ReservedEnvVars = map[string]bool{
	"LANGCHAIN_TRACING_V2":              true,
	"LANGSMITH_TRACING_V2":              true,
	"LANGCHAIN_ENDPOINT":                true,
	"LANGCHAIN_PROJECT":                 true,
	"LANGSMITH_PROJECT":                 true,
	"LANGSMITH_LANGGRAPH_GIT_REPO":      true,
	"LANGGRAPH_GIT_REPO_PATH":           true,
	"LANGCHAIN_API_KEY":                 true,
	"LANGSMITH_CONTROL_PLANE_API_KEY":   true,
	"POSTGRES_URI":                      true,
	"POSTGRES_PASSWORD":                 true,
	"DATABASE_URI":                      true,
	"REDIS_URI":                         true,
	"LANGSMITH_API_KEY":                 true,
	"LANGSMITH_ENDPOINT":                true,
	"LANGSMITH_RUNS_ENDPOINTS":          true,
	"LANGSMITH_API_URL":                 true,
	"DD_API_KEY":                        true,
	"DD_APP_KEY":                        true,
	"DD_DOGSTATSD_URL":                  true,
	"DD_TRACE_ENABLED":                  true,
	"DD_ENV":                            true,
	"DD_SERVICE":                        true,
	"DD_VERSION":                        true,
	"DATADOG_API_KEY":                   true,
	"DATADOG_APP_KEY":                   true,
	"SENTRY_DSN":                        true,
	"N_JOBS_PER_WORKER":                 true,
	"LANGGRAPH_CLOUD_LICENSE_KEY":       true,
	"LANGCHAIN_LANGGRAPH_API_VARIANT":   true,
	"LANGGRAPH_CLOUD_VARIANT":           true,
	"LANGSMITH_TENANT_ID":               true,
	"LANGSMITH_LANGCHAIN_API_VARIANT":   true,
	"LANGSMITH_WORKSPACE_ID":            true,
	"PORT":                              true,
	"BLOB_STORAGE_URI":                  true,
	"BLOB_STORAGE_BUCKET":               true,
	"BLOB_STORAGE_PROJECT":              true,
	"LANGSMITH_GIT_COMMIT_SHA":          true,
	"LANGSMITH_SOURCE_IMAGE_URI":        true,
	"LANGSMITH_DEPLOYMENT_ID":           true,
	"LANGSMITH_REVISION_ID":             true,
	"CONTAINER_CGROUP_MEMORY_LIMIT":     true,
	"LANGGRAPH_ORCHESTRATOR_URI":        true,
	"LANGGRAPH_RUNNER_ID":               true,
	"LANGGRAPH_RUNTIME_MODE":            true,
	"LANGGRAPH_AUTO_SCALING_GROUP_ID":   true,
	"LANGGRAPH_ENGINE_RUNTIME_MODE":     true,
}

// SecretsFromEnv filters environment variables for deployment.
func SecretsFromEnv(config map[string]interface{}) []map[string]string {
	env := config["env"]
	var envMap map[string]string

	switch e := env.(type) {
	case string:
		// Load from .env file
		envMap = loadDotEnv(e)
	case map[string]interface{}:
		envMap = make(map[string]string)
		for k, v := range e {
			envMap[k] = fmt.Sprintf("%v", v)
		}
	case map[string]string:
		envMap = e
	default:
		return nil
	}

	var secrets []map[string]string
	// Sort keys for deterministic output
	var keys []string
	for k := range envMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if !ReservedEnvVars[k] {
			secrets = append(secrets, map[string]string{
				"key":   k,
				"value": envMap[k],
			})
		}
	}
	return secrets
}

func loadDotEnv(path string) map[string]string {
	result := make(map[string]string)
	data, err := os.ReadFile(path)
	if err != nil {
		return result
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Remove surrounding quotes
			value = strings.Trim(value, `"'`)
			result[key] = value
		}
	}
	return result
}

// WarnNonWolfiDistro prints a warning if image_distro is not wolfi.
func WarnNonWolfiDistro(config map[string]interface{}) {
	distro := "debian"
	if d, ok := config["image_distro"].(string); ok {
		distro = d
	}
	if distro != "wolfi" {
		fmt.Fprintln(os.Stderr, "Warning: Security Recommendation: Consider switching to Wolfi Linux for enhanced security.")
		fmt.Fprintln(os.Stderr, "  Wolfi is a security-oriented, minimal Linux distribution designed for containers.")
		fmt.Fprintln(os.Stderr, `  To switch, add '"image_distro": "wolfi"' to your langgraph.json config file.`)
		fmt.Fprintln(os.Stderr)
	}
}
