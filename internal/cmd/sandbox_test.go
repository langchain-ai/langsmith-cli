package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== parseByteSize ====================

func TestParseByteSize_Valid(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"1mb", 1024 * 1024},
		{"1MB", 1024 * 1024},
		{"1MiB", 1024 * 1024},
		{"1m", 1024 * 1024},
		{"512mb", 512 * 1024 * 1024},
		{"1gb", 1024 * 1024 * 1024},
		{"1GB", 1024 * 1024 * 1024},
		{"1GiB", 1024 * 1024 * 1024},
		{"1g", 1024 * 1024 * 1024},
		{"4gb", 4 * 1024 * 1024 * 1024},
		{"1.5gb", int64(1.5 * 1024 * 1024 * 1024)},
		{"1tb", 1024 * 1024 * 1024 * 1024},
		{"100mb", 100 * 1024 * 1024},
	}
	for _, tc := range tests {
		got, err := parseByteSize(tc.input)
		if err != nil {
			t.Errorf("parseByteSize(%q) error: %v", tc.input, err)
			continue
		}
		if got != tc.expected {
			t.Errorf("parseByteSize(%q) = %d, want %d", tc.input, got, tc.expected)
		}
	}
}

func TestParseByteSize_RequiresUnit(t *testing.T) {
	_, err := parseByteSize("512")
	if err == nil {
		t.Error("expected error for bare number without unit")
	}
}

func TestParseByteSize_RejectsEmpty(t *testing.T) {
	_, err := parseByteSize("")
	if err == nil {
		t.Error("expected error for empty string")
	}
}

func TestParseByteSize_RejectsTooSmall(t *testing.T) {
	_, err := parseByteSize("100b")
	if err == nil {
		t.Error("expected error for value < 1mb")
	}
}

func TestParseByteSize_RejectsTooLarge(t *testing.T) {
	_, err := parseByteSize("2pb")
	if err == nil {
		t.Error("expected error for value > 1pb")
	}
}

func TestParseByteSize_InvalidUnit(t *testing.T) {
	_, err := parseByteSize("4xx")
	if err == nil {
		t.Error("expected error for invalid unit")
	}
}

// ==================== Command structure ====================

func TestSandboxCmd_FlatSubcommands(t *testing.T) {
	cmd := newSandboxCmd()
	expected := map[string]bool{
		"create": false, "list": false, "get": false, "update": false,
		"delete": false, "start": false, "stop": false,
		"exec": false, "console": false, "service-url": false, "generate-download-url": false,
		"tunnel": false, "ssh-setup": false,
		"snapshot": false,
	}
	for _, sub := range cmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("sandbox missing subcommand %q", name)
		}
	}
}

func TestSnapshotCmd_Subcommands(t *testing.T) {
	cmd := sandboxSnapshotCommand.Cobra()
	expected := map[string]bool{
		"build": false, "capture": false, "list": false,
		"get": false, "delete": false,
	}
	for _, sub := range cmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("snapshot missing subcommand %q", name)
		}
	}
}

func TestSandboxCreateCmd_PositionalName(t *testing.T) {
	cmd := sandboxCreateCommand.Cobra()
	if cmd.Args == nil {
		t.Fatal("expected Args validator")
	}
	if err := cmd.Args(cmd, []string{}); err != nil {
		t.Errorf("expected no error with 0 args, got: %v", err)
	}
	if err := cmd.Args(cmd, []string{"my-vm"}); err != nil {
		t.Errorf("expected no error with 1 arg, got: %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("expected error with 2 args")
	}
}

func TestSandboxCreateCmd_AllowsNoArgs(t *testing.T) {
	var body map[string]any
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v2/sandboxes/boxes", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"id":"box-id","name":"my-vm","status":"running","vcpus":2,"mem_bytes":536870912}`))
		require.NoError(t, err)
	})

	out, err := executeCommand(t, "--api-key", "test-key", "--api-url", ts.URL, "sandbox", "create", "--format", "json")

	require.NoError(t, err)
	require.NotEmpty(t, out)
	require.NotContains(t, body, "name")
	require.NotContains(t, body, "snapshot_id")
}

func TestSandboxExecCmd_PositionalNameAndCommandSeparator(t *testing.T) {
	cmd := newSandboxExecCmd()
	require.NoError(t, cmd.Args(cmd, []string{"my-vm", "echo", "hi"}))
	require.Error(t, cmd.Args(cmd, []string{}))
}

func TestSandboxExecCmd_WritesCommandOutputDirectly(t *testing.T) {
	var body map[string]any
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/sandboxes/boxes/my-vm":
			_, err := w.Write([]byte(`{"id":"box-id","name":"my-vm","status":"ready","dataplane_url":"` + tsURL(t, r) + `"}`))
			require.NoError(t, err)
		case r.Method == http.MethodPost && r.URL.Path == "/execute":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			_, err := w.Write([]byte(`{"stdout":"hello\n","stderr":"","exit_code":0}`))
			require.NoError(t, err)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})

	var out string
	var err error
	stdout := captureStdout(t, func() {
		out, err = executeCommand(t, "--api-key", "test-key", "--api-url", ts.URL, "sandbox", "exec", "my-vm", "--", "echo", "hello")
	})

	require.NoError(t, err)
	assert.Empty(t, out)
	assert.Equal(t, "hello\n", stdout)
	assert.Equal(t, "'echo' 'hello'", body["command"])
}

func tsURL(t *testing.T, r *http.Request) string {
	t.Helper()
	return "http://" + r.Host
}

func TestSandboxCustomOutputCommands_Flags(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
		want []string
	}{
		{"tunnel", newSandboxTunnelCmd(), []string{"url", "name", "remote-port", "local-port", "stdio", "log-level"}},
		{"ssh-setup", newSandboxSSHSetupCmd(), []string{"identity"}},
	}
	if runtime.GOOS != "windows" {
		tests = append(tests, struct {
			name string
			cmd  *cobra.Command
			want []string
		}{"console", newSandboxConsoleCmd(), []string{"shell", "forward-ssh-agent", "env"}})
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, name := range tc.want {
				require.NotNil(t, tc.cmd.Flags().Lookup(name), "flag --%s not found", name)
			}
		})
	}
}

func TestSandboxTunnelCmd_PositionalNameOrURL(t *testing.T) {
	cmd := newSandboxTunnelCmd()
	// Should accept 0 or 1 args
	if err := cmd.Args(cmd, []string{}); err != nil {
		t.Errorf("expected no error with 0 args, got: %v", err)
	}
	if err := cmd.Args(cmd, []string{"my-vm"}); err != nil {
		t.Errorf("expected no error with 1 arg, got: %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("expected error with 2 args")
	}
}

func TestSandboxTunnelCmd_HiddenNameFlag(t *testing.T) {
	cmd := newSandboxTunnelCmd()
	f := cmd.Flags().Lookup("name")
	if f == nil {
		t.Fatal("expected hidden --name flag for backward compat")
	}
	if f.Hidden != true {
		t.Error("--name flag should be hidden")
	}
}

func TestSnapshotBuildCmd_PositionalName(t *testing.T) {
	cmd := snapshotBuildCommand.Cobra()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error with 0 args")
	}
	if err := cmd.Args(cmd, []string{"my-snap"}); err != nil {
		t.Errorf("expected no error with 1 arg, got: %v", err)
	}
}

func TestSnapshotCaptureCmd_PositionalName(t *testing.T) {
	cmd := snapshotCaptureCommand.Cobra()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error with 0 args")
	}
	if err := cmd.Args(cmd, []string{"my-snap"}); err != nil {
		t.Errorf("expected no error with 1 arg, got: %v", err)
	}
}

func TestSandboxCreateCmd_SizeFlags(t *testing.T) {
	cmd := sandboxCreateCommand.Cobra()
	for _, name := range []string{"memory", "rootfs-capacity"} {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Errorf("flag --%s not found", name)
		}
	}
}

func TestSandboxBoxDetailRenderSupportsSDKResponseTypes(t *testing.T) {
	tests := []struct {
		name    string
		model   any
		wantID  string
		wantTTL string
	}{
		{
			name: "new response",
			model: langsmith.SandboxResponse{
				ID:              "box-new",
				Name:            "new-vm",
				Status:          "running",
				SizeClass:       "small",
				Vcpus:           2,
				MemBytes:        512 * 1024 * 1024,
				FsCapacityBytes: 4 * 1024 * 1024 * 1024,
				SnapshotID:      "1234567890abcdef",
				IdleTtlSeconds:  900,
				CreatedAt:       "2026-05-11T12:00:00Z",
			},
			wantID:  "ID:        box-new",
			wantTTL: "Idle TTL:  900s",
		},
		{
			name: "get response",
			model: langsmith.SandboxResponse{
				ID:              "box-get",
				Name:            "get-vm",
				Status:          "running",
				SizeClass:       "medium",
				Vcpus:           4,
				MemBytes:        1024 * 1024 * 1024,
				FsCapacityBytes: 8 * 1024 * 1024 * 1024,
				SnapshotID:      "abcdef1234567890",
				IdleTtlSeconds:  1800,
				CreatedAt:       "2026-05-11T12:00:00Z",
			},
			wantID:  "ID:        box-get",
			wantTTL: "Idle TTL:  1800s",
		},
		{
			name: "update response",
			model: langsmith.SandboxResponse{
				ID:              "box-update",
				Name:            "update-vm",
				Status:          "stopped",
				SizeClass:       "large",
				Vcpus:           8,
				MemBytes:        2 * 1024 * 1024 * 1024,
				FsCapacityBytes: 16 * 1024 * 1024 * 1024,
				SnapshotID:      "fedcba0987654321",
				IdleTtlSeconds:  3600,
				CreatedAt:       "2026-05-11T12:00:00Z",
			},
			wantID:  "ID:        box-update",
			wantTTL: "Idle TTL:  3600s",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer

			err := sandboxBoxDetailRender.RenderText(&out, tc.model)

			require.NoError(t, err)
			require.Contains(t, out.String(), tc.wantID)
			require.Contains(t, out.String(), "Size:")
			require.Contains(t, out.String(), tc.wantTTL)
		})
	}
}

func TestSandboxCreateParams_OmitsEmptySnapshotID(t *testing.T) {
	params, err := sandboxCreateParams("my-vm", &sandboxCreateInput{})
	require.NoError(t, err)

	raw, err := json.Marshal(params)
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))

	assert.NotContains(t, body, "snapshot_id")
	assert.NotContains(t, body, "vcpus")
	assert.NotContains(t, body, "mem_bytes")
	assert.Equal(t, "my-vm", body["name"])
}

func TestSandboxCreateParams_IncludesSnapshotIDWhenSet(t *testing.T) {
	params, err := sandboxCreateParams("my-vm", &sandboxCreateInput{
		SnapshotID: "snap-123",
	})
	require.NoError(t, err)

	raw, err := json.Marshal(params)
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))

	assert.Equal(t, "snap-123", body["snapshot_id"])
}

func TestSandboxUpdateCmd_SizeFlags(t *testing.T) {
	cmd := sandboxUpdateCommand.Cobra()
	for _, name := range []string{"memory", "rootfs-capacity"} {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Errorf("flag --%s not found", name)
		}
	}
}

func TestSandboxServiceURLCmd_Flags(t *testing.T) {
	cmd := sandboxServiceURLCommand.Cobra()

	require.NotNil(t, cmd.Flags().Lookup("port"))
	require.NotNil(t, cmd.Flags().Lookup("expires-in-seconds"))
}

func TestSandboxServiceURLCmd_GeneratesServiceURL(t *testing.T) {
	var body map[string]any
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v2/sandboxes/boxes/my-vm/service-url", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"service_url":"https://service.example","browser_url":"https://browser.example","expires_at":"2026-05-27T12:00:00Z","token":"token-123"}`))
		require.NoError(t, err)
	})

	out, err := executeCommand(t, "--api-key", "test-key", "--api-url", ts.URL, "sandbox", "service-url", "my-vm", "--port", "8000", "--expires-in-seconds", "3600")

	require.NoError(t, err)
	assert.Equal(t, float64(8000), body["port"])
	assert.Equal(t, float64(3600), body["expires_in_seconds"])
	assert.Contains(t, out, "Service URL:")
	assert.Contains(t, out, "https://service.example")
	assert.Contains(t, out, "Browser URL:")
	assert.Contains(t, out, "https://browser.example")
	assert.Contains(t, out, "Service Token:")
	assert.Contains(t, out, `curl -H "X-Langsmith-Sandbox-Service-Token: token-123" "https://service.example"`)
}

func TestSandboxServiceURLCmd_OmitsExpiresInSecondsWhenUnset(t *testing.T) {
	var body map[string]any
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"service_url":"https://service.example","browser_url":"https://browser.example","expires_at":"2026-05-27T12:00:00Z","token":"token-123"}`))
		require.NoError(t, err)
	})

	_, err := executeCommand(t, "--api-key", "test-key", "--api-url", ts.URL, "sandbox", "service-url", "my-vm", "--port", "8000")

	require.NoError(t, err)
	assert.Equal(t, float64(8000), body["port"])
	assert.NotContains(t, body, "expires_in_seconds")
}

func TestSandboxServiceURLCmd_RejectsInvalidPort(t *testing.T) {
	_, err := executeCommand(t, "--api-key", "test-key", "sandbox", "service-url", "my-vm", "--port", "0")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--port must be between 1 and 65535")
}

func TestSandboxDownloadURLCmd_Flags(t *testing.T) {
	cmd := sandboxDownloadURLCommand.Cobra()

	require.NotNil(t, cmd.Flags().Lookup("path"))
	require.NotNil(t, cmd.Flags().Lookup("expires-in-seconds"))
	require.NotNil(t, cmd.Flags().Lookup("content-type"))
	require.NotNil(t, cmd.Flags().Lookup("content-disposition"))
}

func TestSandboxDownloadURLCmd_GeneratesDownloadURL(t *testing.T) {
	var body map[string]any
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v2/sandboxes/boxes/my-vm/download-url", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"download_url":"https://box--dl.example/tok","token":"tok","expires_at":"2026-05-27T12:00:00Z"}`))
		require.NoError(t, err)
	})

	out, err := executeCommand(t, "--api-key", "test-key", "--api-url", ts.URL, "sandbox", "generate-download-url", "my-vm",
		"--path", "/tmp/report.pdf", "--expires-in-seconds", "3600", "--content-type", "application/pdf", "--content-disposition", "inline")

	require.NoError(t, err)
	assert.Equal(t, "/tmp/report.pdf", body["path"])
	assert.Equal(t, float64(3600), body["expires_in_seconds"])
	assert.Equal(t, "application/pdf", body["content_type"])
	assert.Equal(t, "inline", body["content_disposition"])
	assert.Contains(t, out, "Download URL:")
	assert.Contains(t, out, "https://box--dl.example/tok")
	assert.Contains(t, out, `curl -LO "https://box--dl.example/tok"`)
}

func TestSandboxDownloadURLCmd_RendersNeverExpires(t *testing.T) {
	var body map[string]any
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"download_url":"https://box--dl.example/tok","token":"tok","expires_at":null}`))
		require.NoError(t, err)
	})

	out, err := executeCommand(t, "--api-key", "test-key", "--api-url", ts.URL, "sandbox", "generate-download-url", "my-vm", "--path", "/tmp/report.pdf")

	require.NoError(t, err)
	assert.NotContains(t, body, "expires_in_seconds")
	assert.NotContains(t, body, "content_type")
	assert.NotContains(t, body, "content_disposition")
	assert.Contains(t, out, "never")
}

func TestSandboxDownloadURLCmd_RejectsNonPositiveExpiry(t *testing.T) {
	_, err := executeCommand(t, "--api-key", "test-key", "sandbox", "generate-download-url", "my-vm", "--path", "/tmp/f", "--expires-in-seconds", "0")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--expires-in-seconds must be greater than 0")
}

func TestSnapshotBuildCmd_CapacityFlag(t *testing.T) {
	cmd := snapshotBuildCommand.Cobra()
	f := cmd.Flags().Lookup("capacity")
	if f == nil {
		t.Fatal("flag --capacity not found")
	}
	if f.DefValue != "4gb" {
		t.Errorf("expected default 4gb, got %q", f.DefValue)
	}
}

func TestSnapshotBuildCmd_RegistryIDFlag(t *testing.T) {
	cmd := snapshotBuildCommand.Cobra()
	if f := cmd.Flags().Lookup("registry-id"); f == nil {
		t.Fatal("flag --registry-id not found")
	}
	for _, name := range []string{"registry-url", "registry-username", "registry-password"} {
		if f := cmd.Flags().Lookup(name); f != nil {
			t.Fatalf("obsolete flag --%s should not exist", name)
		}
	}
}

// ==================== loadJSONArg ====================

func TestLoadJSONArg_InlineValid(t *testing.T) {
	input := `{"rules":[{"name":"test","match_hosts":["example.com"]}]}`
	raw, err := loadJSONArg(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	rules, ok := parsed["rules"].([]interface{})
	if !ok || len(rules) != 1 {
		t.Errorf("expected 1 rule, got %v", parsed["rules"])
	}
}

func TestLoadJSONArg_InlineInvalid(t *testing.T) {
	_, err := loadJSONArg(`{not json}`)
	if err == nil {
		t.Error("expected error for invalid inline JSON")
	}
}

func TestLoadJSONArg_FileValid(t *testing.T) {
	path := filepath.Join("testdata", "proxy_config.json")
	raw, err := loadJSONArg("@" + path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	rules, ok := parsed["rules"].([]interface{})
	if !ok || len(rules) != 1 {
		t.Errorf("expected 1 rule, got %v", parsed["rules"])
	}
	noProxy, ok := parsed["no_proxy"].([]interface{})
	if !ok || len(noProxy) != 1 {
		t.Errorf("expected 1 no_proxy entry, got %v", parsed["no_proxy"])
	}
}

func TestLoadJSONArg_FileInvalidJSON(t *testing.T) {
	path := filepath.Join("testdata", "proxy_config_invalid.json")
	_, err := loadJSONArg("@" + path)
	if err == nil {
		t.Error("expected error for invalid JSON file")
	}
}

func TestLoadJSONArg_FileMissing(t *testing.T) {
	_, err := loadJSONArg("@testdata/nonexistent.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadJSONArg_FileAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pc.json")
	if err := os.WriteFile(path, []byte(`{"rules":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	raw, err := loadJSONArg("@" + path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
}

// ==================== proxy-config flag ====================

func TestSandboxCreateCmd_ProxyConfigFlag(t *testing.T) {
	cmd := sandboxCreateCommand.Cobra()
	f := cmd.Flags().Lookup("proxy-config")
	if f == nil {
		t.Fatal("flag --proxy-config not found on create command")
	}
}

func TestSandboxUpdateCmd_ProxyConfigFlag(t *testing.T) {
	cmd := sandboxUpdateCommand.Cobra()
	f := cmd.Flags().Lookup("proxy-config")
	if f == nil {
		t.Fatal("flag --proxy-config not found on update command")
	}
}
