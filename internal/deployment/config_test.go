package deployment

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateConfigMinimal(t *testing.T) {
	raw := map[string]any{
		"dependencies": []any{"."},
		"graphs": map[string]any{
			"agent": "./agent.py:graph",
		},
	}

	config, err := ValidateConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config["python_version"] != "3.11" {
		t.Errorf("expected python_version=3.11, got %v", config["python_version"])
	}
	if config["image_distro"] != "debian" {
		t.Errorf("expected image_distro=debian, got %v", config["image_distro"])
	}
	if config["pip_installer"] != "auto" {
		t.Errorf("expected pip_installer=auto, got %v", config["pip_installer"])
	}
}

func TestValidateConfigFull(t *testing.T) {
	raw := map[string]any{
		"python_version":  "3.12",
		"pip_config_file": "pipconfig.txt",
		"dockerfile_lines": []any{"ARG meow"},
		"dependencies":    []any{".", "langchain"},
		"graphs": map[string]any{
			"agent": "./agent.py:graph",
		},
		"env": ".env",
	}

	config, err := ValidateConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config["python_version"] != "3.12" {
		t.Errorf("expected python_version=3.12, got %v", config["python_version"])
	}
}

func TestValidateConfigWrongPythonVersion(t *testing.T) {
	_, err := ValidateConfig(map[string]any{
		"python_version": "3.9",
		"dependencies":   []any{"."},
		"graphs":         map[string]any{"agent": "./agent.py:graph"},
	})
	if err == nil {
		t.Error("expected error for python_version 3.9")
	}
}

func TestValidateConfigMissingDependencies(t *testing.T) {
	_, err := ValidateConfig(map[string]any{
		"python_version": "3.11",
		"graphs":         map[string]any{"agent": "./agent.py:graph"},
	})
	if err == nil {
		t.Error("expected error for missing dependencies")
	}
}

func TestValidateConfigMissingGraphs(t *testing.T) {
	_, err := ValidateConfig(map[string]any{
		"python_version": "3.11",
		"dependencies":   []any{"."},
	})
	if err == nil {
		t.Error("expected error for missing graphs")
	}
}

func TestValidateConfigInvalidPythonFormat(t *testing.T) {
	tests := []string{"3.11.0", "3", "abc.def", "3.10"}
	for _, ver := range tests {
		_, err := ValidateConfig(map[string]any{
			"python_version": ver,
			"dependencies":   []any{"."},
			"graphs":         map[string]any{"agent": "./agent.py:graph"},
		})
		if err == nil {
			t.Errorf("expected error for python_version %q", ver)
		}
	}
}

func TestValidateConfigBullseye(t *testing.T) {
	_, err := ValidateConfig(map[string]any{
		"python_version": "3.11-bullseye",
		"dependencies":   []any{"."},
		"graphs":         map[string]any{"agent": "./agent.py:graph"},
	})
	if err == nil {
		t.Error("expected error for bullseye")
	}
}

func TestValidateConfigImageDistro(t *testing.T) {
	// Valid distros
	for _, distro := range []string{"debian", "wolfi", "bookworm"} {
		config, err := ValidateConfig(map[string]any{
			"python_version": "3.11",
			"dependencies":   []any{"."},
			"graphs":         map[string]any{"agent": "./agent.py:graph"},
			"image_distro":   distro,
		})
		if err != nil {
			t.Errorf("unexpected error for distro %q: %v", distro, err)
		}
		if config["image_distro"] != distro {
			t.Errorf("expected distro %q, got %v", distro, config["image_distro"])
		}
	}

	// Invalid distro
	_, err := ValidateConfig(map[string]any{
		"python_version": "3.11",
		"dependencies":   []any{"."},
		"graphs":         map[string]any{"agent": "./agent.py:graph"},
		"image_distro":   "alpine",
	})
	if err == nil {
		t.Error("expected error for invalid distro 'alpine'")
	}
}

func TestValidateConfigPythonVersionSlim(t *testing.T) {
	config, err := ValidateConfig(map[string]any{
		"python_version": "3.12-slim",
		"dependencies":   []any{"."},
		"graphs":         map[string]any{"agent": "./agent.py:graph"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config["python_version"] != "3.12-slim" {
		t.Errorf("expected 3.12-slim, got %v", config["python_version"])
	}
}

func TestValidateConfigInvalidHTTPApp(t *testing.T) {
	_, err := ValidateConfig(map[string]any{
		"python_version": "3.12",
		"dependencies":   []any{"."},
		"graphs":         map[string]any{"agent": "./agent.py:graph"},
		"http":           map[string]any{"app": "../../examples/my_app.py"},
	})
	if err == nil {
		t.Error("expected error for invalid http.app format")
	}
}

func TestValidateConfigNodeProject(t *testing.T) {
	config, err := ValidateConfig(map[string]any{
		"node_version": "20",
		"graphs":       map[string]any{"agent": "agent.js:graph"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config["node_version"] != "20" {
		t.Errorf("expected node_version=20, got %v", config["node_version"])
	}
}

func TestDefaultBaseImage(t *testing.T) {
	pyConfig := map[string]any{"python_version": "3.11"}
	if got := DefaultBaseImage(pyConfig); got != "langchain/langgraph-api" {
		t.Errorf("expected langchain/langgraph-api, got %s", got)
	}

	jsConfig := map[string]any{"node_version": "20"}
	if got := DefaultBaseImage(jsConfig); got != "langchain/langgraphjs-api" {
		t.Errorf("expected langchain/langgraphjs-api, got %s", got)
	}
}

func TestDockerTag(t *testing.T) {
	tests := []struct {
		name      string
		baseImage string
		config    map[string]any
		apiVer    string
		mode      string
		expected  string
	}{
		{
			name:     "default python",
			config:   map[string]any{"python_version": "3.11"},
			expected: "langchain/langgraph-api:py3.11",
		},
		{
			name:     "python with wolfi",
			config:   map[string]any{"python_version": "3.12", "image_distro": "wolfi"},
			expected: "langchain/langgraph-api:py3.12-wolfi",
		},
		{
			name:     "with api version",
			config:   map[string]any{"python_version": "3.11"},
			apiVer:   "0.2.74",
			expected: "langchain/langgraph-api:0.2.74-py3.11",
		},
		{
			name:     "with api version and wolfi",
			config:   map[string]any{"python_version": "3.12", "image_distro": "wolfi"},
			apiVer:   "1.0.0",
			expected: "langchain/langgraph-api:1.0.0-py3.12-wolfi",
		},
		{
			name:      "custom base image with api version",
			baseImage: "my-registry/custom-api",
			config:    map[string]any{"python_version": "3.12", "image_distro": "wolfi"},
			apiVer:    "1.0.0",
			expected:  "my-registry/custom-api:1.0.0-py3.12-wolfi",
		},
		{
			name:     "node project",
			config:   map[string]any{"node_version": "20"},
			expected: "langchain/langgraphjs-api:node20",
		},
		{
			name:     "node with api version",
			config:   map[string]any{"node_version": "20"},
			apiVer:   "0.2.74",
			expected: "langchain/langgraphjs-api:0.2.74-node20",
		},
		{
			name:     "distributed mode",
			config:   map[string]any{"python_version": "3.11"},
			mode:     "distributed",
			expected: "langchain/langgraph-executor:py3.11",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DockerTag(tt.baseImage, tt.config, tt.apiVer, tt.mode)
			if got != tt.expected {
				t.Errorf("DockerTag() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestConfigToDocker(t *testing.T) {
	config := map[string]any{
		"python_version": "3.11",
		"dependencies":   []any{"."},
		"graphs":         map[string]any{"agent": "agent.py:graph"},
	}

	dockerfile, err := ConfigToDocker(config, "/tmp/test-project", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mustContain(t, dockerfile, "FROM langchain/langgraph-api:")
	mustContain(t, dockerfile, "LANGSERVE_GRAPHS")
}

func TestConfigToDockerDistributedMode(t *testing.T) {
	config := map[string]any{
		"python_version": "3.11",
		"dependencies":   []any{"."},
		"graphs":         map[string]any{"agent": "agent.py:graph"},
	}

	dockerfile, err := ConfigToDocker(config, "/tmp/test-project", "", "", "distributed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mustContain(t, dockerfile, "FROM langchain/langgraph-executor:")
}

func TestHasDisallowedBuildCommandContent(t *testing.T) {
	// Allowed
	if HasDisallowedBuildCommandContent("npm install") {
		t.Error("'npm install' should be allowed")
	}
	if HasDisallowedBuildCommandContent("npm run build && npm test") {
		t.Error("'npm run build && npm test' should be allowed")
	}

	// Disallowed
	if !HasDisallowedBuildCommandContent("npm install | cat") {
		t.Error("pipe should be disallowed")
	}
	if !HasDisallowedBuildCommandContent("npm install > /dev/null") {
		t.Error("redirect should be disallowed")
	}
	if !HasDisallowedBuildCommandContent("npm install; rm -rf /") {
		t.Error("semicolon should be disallowed")
	}
}

func TestLoadConfig(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "langgraph.json")
	config := map[string]any{
		"dependencies": []any{"."},
		"graphs":       map[string]any{"agent": "./agent.py:graph"},
	}
	data, _ := json.Marshal(config)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deps, ok := loaded["dependencies"].([]any)
	if !ok || len(deps) != 1 {
		t.Errorf("expected 1 dependency, got %v", loaded["dependencies"])
	}
}

func TestSecretsFromEnv(t *testing.T) {
	config := map[string]any{
		"env": map[string]any{
			"MY_KEY":       "my_value",
			"POSTGRES_URI": "should_be_filtered",
			"ANOTHER_KEY":  "another_value",
		},
	}

	secrets := SecretsFromEnv(config)

	// Check that reserved vars are filtered
	for _, s := range secrets {
		if s["key"] == "POSTGRES_URI" {
			t.Error("POSTGRES_URI should be filtered")
		}
	}

	// Check that non-reserved vars are present
	found := false
	for _, s := range secrets {
		if s["key"] == "MY_KEY" && s["value"] == "my_value" {
			found = true
		}
	}
	if !found {
		t.Error("MY_KEY should be in secrets")
	}
}
