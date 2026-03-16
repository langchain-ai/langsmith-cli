package deployment

import (
	"strings"
	"testing"
)

var defaultDockerCapabilities = DockerCapabilities{
	VersionDocker:            Version{26, 1, 1},
	VersionCompose:           Version{2, 27, 0},
	HealthcheckStartInterval: false,
	ComposeType:              ComposePlugin,
}

func TestComposeWithNoDebuggerAndCustomDB(t *testing.T) {
	port := 8123
	customPostgresURI := "custom_postgres_uri"
	actual := Compose(&defaultDockerCapabilities, ComposeOptions{
		Port:        port,
		PostgresURI: customPostgresURI,
	})
	actual = CleanEmptyLines(actual)

	mustContain(t, actual, "langgraph-redis:")
	mustContain(t, actual, "image: redis:6")
	mustContain(t, actual, "langgraph-api:")
	mustContain(t, actual, "\"8123:8000\"")
	mustContain(t, actual, "POSTGRES_URI: custom_postgres_uri")
	mustNotContain(t, actual, "langgraph-postgres:")
	mustNotContain(t, actual, "volumes:")
}

func TestComposeWithNoDebuggerAndCustomDBWithHealthcheck(t *testing.T) {
	caps := defaultDockerCapabilities
	caps.HealthcheckStartInterval = true
	actual := Compose(&caps, ComposeOptions{
		Port:        8123,
		PostgresURI: "custom_postgres_uri",
	})
	actual = CleanEmptyLines(actual)

	mustContain(t, actual, "langgraph-api:")
	mustContain(t, actual, "healthcheck:")
	mustContain(t, actual, "test: python /api/healthcheck.py")
	mustContain(t, actual, "start_interval: 1s")
}

func TestComposeWithDebuggerAndDefaultDB(t *testing.T) {
	actual := Compose(&defaultDockerCapabilities, ComposeOptions{
		Port: 8123,
	})
	actual = CleanEmptyLines(actual)

	mustContain(t, actual, "volumes:")
	mustContain(t, actual, "langgraph-data:")
	mustContain(t, actual, "langgraph-redis:")
	mustContain(t, actual, "langgraph-postgres:")
	mustContain(t, actual, "image: pgvector/pgvector:pg16")
	mustContain(t, actual, "langgraph-api:")
	mustContain(t, actual, DefaultPostgresURI)
}

func TestComposeWithDebuggerPort(t *testing.T) {
	actual := Compose(&defaultDockerCapabilities, ComposeOptions{
		Port:         8123,
		DebuggerPort: 8001,
	})
	actual = CleanEmptyLines(actual)

	mustContain(t, actual, "langgraph-debugger:")
	mustContain(t, actual, "image: langchain/langgraph-debugger")
	mustContain(t, actual, "\"8001:3968\"")
}

func TestComposeDistributedMode(t *testing.T) {
	actual := Compose(&defaultDockerCapabilities, ComposeOptions{
		Port:              8123,
		PostgresURI:       "custom_postgres_uri",
		EngineRuntimeMode: "distributed",
	})
	actual = CleanEmptyLines(actual)

	mustContain(t, actual, "N_JOBS_PER_WORKER: \"0\"")
}

func TestComposeCombinedModeNoNJobs(t *testing.T) {
	actual := Compose(&defaultDockerCapabilities, ComposeOptions{
		Port:              8123,
		EngineRuntimeMode: "combined_queue_worker",
	})

	mustNotContain(t, actual, "N_JOBS_PER_WORKER")
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected Version
	}{
		{"1.2.3", Version{1, 2, 3}},
		{"v1.2.3", Version{1, 2, 3}},
		{"1.2.3-alpha", Version{1, 2, 3}},
		{"1.2.3+1", Version{1, 2, 3}},
		{"1.2.3-alpha+build", Version{1, 2, 3}},
		{"1.2", Version{1, 2, 0}},
		{"1", Version{1, 0, 0}},
		{"v28.1.1+1", Version{28, 1, 1}},
		{"2.0.0-beta.1+exp.sha.5114f85", Version{2, 0, 0}},
		{"v3.4.5-rc1+build.123", Version{3, 4, 5}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseVersion(tt.input)
			if result != tt.expected {
				t.Errorf("ParseVersion(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestVersionComparison(t *testing.T) {
	v25 := Version{25, 0, 0}
	v24 := Version{24, 9, 9}
	v26 := Version{26, 1, 1}

	if !v25.GreaterOrEqual(v25) {
		t.Error("25.0.0 should be >= 25.0.0")
	}
	if !v26.GreaterOrEqual(v25) {
		t.Error("26.1.1 should be >= 25.0.0")
	}
	if v24.GreaterOrEqual(v25) {
		t.Error("24.9.9 should not be >= 25.0.0")
	}
}

func mustContain(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected output to contain %q, got:\n%s", substr, s)
	}
}

func mustNotContain(t *testing.T, s, substr string) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Errorf("expected output to NOT contain %q, got:\n%s", substr, s)
	}
}
