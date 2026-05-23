package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	langsmith "github.com/langchain-ai/langsmith-go"
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
		"exec": false, "console": false, "tunnel": false, "ssh-setup": false,
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
		require.Equal(t, "/v2/sandboxes/boxes", r.URL.Path)
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
			model: langsmith.SandboxBoxNewResponse{
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
			model: langsmith.SandboxBoxGetResponse{
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
			model: langsmith.SandboxBoxUpdateResponse{
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

func TestSandboxCreateCmd_RendersAPIValidationError(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/sandboxes/boxes" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"detail": []map[string]any{{
				"loc":  []string{"body"},
				"msg":  "one of snapshot_id or snapshot_name is required",
				"type": "value_error",
			}},
		})
	})

	root := NewRootCmd("test", "test")
	root.SetArgs([]string{"--api-key", "test-key", "--api-url", ts.URL, "sandbox", "create", "ramonn-test", "--snapshot-id", "snap-123"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	got := err.Error()
	if !strings.Contains(got, `POST "`) || !strings.Contains(got, `"detail"`) {
		t.Fatalf("expected original SDK error, got %q", got)
	}

	display := FormatErrorMessage(err)
	if !strings.Contains(display, "creating sandbox: 422 Unprocessable Entity: one of snapshot_id or snapshot_name is required") {
		t.Fatalf("unexpected error: %q", got)
	}
	if strings.Contains(display, "POST ") || strings.Contains(display, `"detail"`) {
		t.Fatalf("expected simplified error, got %q", display)
	}
}

func TestSandboxUpdateCmd_ProxyConfigFlag(t *testing.T) {
	cmd := sandboxUpdateCommand.Cobra()
	f := cmd.Flags().Lookup("proxy-config")
	if f == nil {
		t.Fatal("flag --proxy-config not found on update command")
	}
}
