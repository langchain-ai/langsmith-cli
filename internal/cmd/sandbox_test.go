package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
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
		"delete": false, "start": false, "stop": false, "wait": false,
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
	cmd := newSandboxSnapshotCmd()
	expected := map[string]bool{
		"build": false, "capture": false, "list": false,
		"get": false, "delete": false, "wait": false,
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
	cmd := newSandboxCreateCmd()
	if cmd.Args == nil {
		t.Fatal("expected Args validator")
	}
	// Should require exactly 1 arg
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error with 0 args")
	}
	if err := cmd.Args(cmd, []string{"my-vm"}); err != nil {
		t.Errorf("expected no error with 1 arg, got: %v", err)
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
	cmd := newSnapshotBuildCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error with 0 args")
	}
	if err := cmd.Args(cmd, []string{"my-snap"}); err != nil {
		t.Errorf("expected no error with 1 arg, got: %v", err)
	}
}

func TestSnapshotCaptureCmd_PositionalName(t *testing.T) {
	cmd := newSnapshotCaptureCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error with 0 args")
	}
	if err := cmd.Args(cmd, []string{"my-snap"}); err != nil {
		t.Errorf("expected no error with 1 arg, got: %v", err)
	}
}

func TestSandboxCreateCmd_SizeFlags(t *testing.T) {
	cmd := newSandboxCreateCmd()
	for _, name := range []string{"memory", "rootfs-capacity"} {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Errorf("flag --%s not found", name)
		}
	}
}

func TestSandboxUpdateCmd_SizeFlags(t *testing.T) {
	cmd := newSandboxUpdateCmd()
	for _, name := range []string{"memory", "rootfs-capacity"} {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Errorf("flag --%s not found", name)
		}
	}
}

func TestSnapshotBuildCmd_CapacityFlag(t *testing.T) {
	cmd := newSnapshotBuildCmd()
	f := cmd.Flags().Lookup("capacity")
	if f == nil {
		t.Fatal("flag --capacity not found")
	}
	if f.DefValue != "4gb" {
		t.Errorf("expected default 4gb, got %q", f.DefValue)
	}
}

func TestSnapshotBuildCmd_RegistryIDFlag(t *testing.T) {
	cmd := newSnapshotBuildCmd()
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
	cmd := newSandboxCreateCmd()
	f := cmd.Flags().Lookup("proxy-config")
	if f == nil {
		t.Fatal("flag --proxy-config not found on create command")
	}
}

func TestSandboxUpdateCmd_ProxyConfigFlag(t *testing.T) {
	cmd := newSandboxUpdateCmd()
	f := cmd.Flags().Lookup("proxy-config")
	if f == nil {
		t.Fatal("flag --proxy-config not found on update command")
	}
}
