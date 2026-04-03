package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_NoFile(t *testing.T) {
	// Point default path to a non-existent file via a temp dir
	tmp := t.TempDir()
	path := filepath.Join(tmp, "does_not_exist.toml")

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil Config")
	}
	if cfg.CurrentProfile != "" {
		t.Errorf("expected empty CurrentProfile, got %q", cfg.CurrentProfile)
	}
	if len(cfg.Profiles) != 0 {
		t.Errorf("expected empty Profiles, got %v", cfg.Profiles)
	}
}

func TestLoad_ValidFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.toml")

	content := `current_profile = "staging"

[default]
api_key = "lsv2_pt_abc123"
api_url = "https://api.smith.langchain.com"

[staging]
api_key = "lsv2_pt_def456"
api_url = "https://staging.example.com"
workspace_id = "ws-123"
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.CurrentProfile != "staging" {
		t.Errorf("expected CurrentProfile=staging, got %q", cfg.CurrentProfile)
	}

	def, ok := cfg.Profiles["default"]
	if !ok {
		t.Fatal("expected 'default' profile to be present")
	}
	if def.APIKey != "lsv2_pt_abc123" {
		t.Errorf("expected default.APIKey=lsv2_pt_abc123, got %q", def.APIKey)
	}
	if def.APIURL != "https://api.smith.langchain.com" {
		t.Errorf("expected default.APIURL=https://api.smith.langchain.com, got %q", def.APIURL)
	}
	if def.WorkspaceID != "" {
		t.Errorf("expected default.WorkspaceID empty, got %q", def.WorkspaceID)
	}

	staging, ok := cfg.Profiles["staging"]
	if !ok {
		t.Fatal("expected 'staging' profile to be present")
	}
	if staging.APIKey != "lsv2_pt_def456" {
		t.Errorf("expected staging.APIKey=lsv2_pt_def456, got %q", staging.APIKey)
	}
	if staging.APIURL != "https://staging.example.com" {
		t.Errorf("expected staging.APIURL=https://staging.example.com, got %q", staging.APIURL)
	}
	if staging.WorkspaceID != "ws-123" {
		t.Errorf("expected staging.WorkspaceID=ws-123, got %q", staging.WorkspaceID)
	}
}

func TestSaveTo_CreatesFile(t *testing.T) {
	tmp := t.TempDir()
	// Use a nested path to verify parent dir creation
	path := filepath.Join(tmp, "subdir", "config.toml")

	cfg := &Config{
		CurrentProfile: "staging",
		Profiles: map[string]Profile{
			"default": {
				APIKey: "lsv2_pt_abc123",
				APIURL: "https://api.smith.langchain.com",
			},
			"staging": {
				APIKey:      "lsv2_pt_def456",
				APIURL:      "https://staging.example.com",
				WorkspaceID: "ws-123",
			},
		},
	}

	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	// Round-trip: load back and verify
	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom after save failed: %v", err)
	}

	if loaded.CurrentProfile != "staging" {
		t.Errorf("expected CurrentProfile=staging, got %q", loaded.CurrentProfile)
	}

	def, ok := loaded.Profiles["default"]
	if !ok {
		t.Fatal("expected 'default' profile after round-trip")
	}
	if def.APIKey != "lsv2_pt_abc123" {
		t.Errorf("expected default.APIKey=lsv2_pt_abc123, got %q", def.APIKey)
	}

	staging, ok := loaded.Profiles["staging"]
	if !ok {
		t.Fatal("expected 'staging' profile after round-trip")
	}
	if staging.WorkspaceID != "ws-123" {
		t.Errorf("expected staging.WorkspaceID=ws-123, got %q", staging.WorkspaceID)
	}
}

func TestSaveTo_FilePermissions(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.toml")

	cfg := &Config{
		Profiles: map[string]Profile{
			"default": {APIKey: "secret"},
		},
	}

	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected permissions 0600, got %04o", perm)
	}
}

func TestResolveProfile_FlagOverride(t *testing.T) {
	cfg := &Config{
		CurrentProfile: "current",
		Profiles: map[string]Profile{
			"flag-profile": {APIKey: "flag-key"},
			"env-profile":  {APIKey: "env-key"},
			"current":      {APIKey: "current-key"},
			"default":      {APIKey: "default-key"},
		},
	}

	p := cfg.ResolveProfile("flag-profile", "env-profile")
	if p == nil {
		t.Fatal("expected non-nil profile")
	}
	if p.APIKey != "flag-key" {
		t.Errorf("expected flag-key, got %q", p.APIKey)
	}
}

func TestResolveProfile_EnvOverride(t *testing.T) {
	cfg := &Config{
		CurrentProfile: "current",
		Profiles: map[string]Profile{
			"env-profile": {APIKey: "env-key"},
			"current":     {APIKey: "current-key"},
			"default":     {APIKey: "default-key"},
		},
	}

	p := cfg.ResolveProfile("", "env-profile")
	if p == nil {
		t.Fatal("expected non-nil profile")
	}
	if p.APIKey != "env-key" {
		t.Errorf("expected env-key, got %q", p.APIKey)
	}
}

func TestResolveProfile_CurrentProfile(t *testing.T) {
	cfg := &Config{
		CurrentProfile: "current",
		Profiles: map[string]Profile{
			"current": {APIKey: "current-key"},
			"default": {APIKey: "default-key"},
		},
	}

	p := cfg.ResolveProfile("", "")
	if p == nil {
		t.Fatal("expected non-nil profile")
	}
	if p.APIKey != "current-key" {
		t.Errorf("expected current-key, got %q", p.APIKey)
	}
}

func TestResolveProfile_FallbackDefault(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]Profile{
			"default": {APIKey: "default-key"},
		},
	}

	p := cfg.ResolveProfile("", "")
	if p == nil {
		t.Fatal("expected non-nil profile")
	}
	if p.APIKey != "default-key" {
		t.Errorf("expected default-key, got %q", p.APIKey)
	}
}

func TestResolveProfile_NoMatch(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]Profile{},
	}

	p := cfg.ResolveProfile("", "")
	if p != nil {
		t.Errorf("expected nil, got %+v", p)
	}
}

func TestResolveProfileName(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		flagProfile string
		envProfile  string
		want        string
	}{
		{
			name: "flag takes precedence",
			cfg: &Config{
				CurrentProfile: "current",
				Profiles: map[string]Profile{
					"flag-profile": {},
					"env-profile":  {},
					"current":      {},
					"default":      {},
				},
			},
			flagProfile: "flag-profile",
			envProfile:  "env-profile",
			want:        "flag-profile",
		},
		{
			name: "env overrides current_profile",
			cfg: &Config{
				CurrentProfile: "current",
				Profiles: map[string]Profile{
					"env-profile": {},
					"current":     {},
				},
			},
			flagProfile: "",
			envProfile:  "env-profile",
			want:        "env-profile",
		},
		{
			name: "current_profile when no flag/env",
			cfg: &Config{
				CurrentProfile: "current",
				Profiles: map[string]Profile{
					"current": {},
					"default": {},
				},
			},
			flagProfile: "",
			envProfile:  "",
			want:        "current",
		},
		{
			name: "falls back to default",
			cfg: &Config{
				Profiles: map[string]Profile{
					"default": {},
				},
			},
			flagProfile: "",
			envProfile:  "",
			want:        "default",
		},
		{
			name: "no match returns empty string",
			cfg: &Config{
				Profiles: map[string]Profile{},
			},
			flagProfile: "",
			envProfile:  "",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.ResolveProfileName(tt.flagProfile, tt.envProfile)
			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		// len > 12: show first 8 + "..." + last 4
		{"lsv2_pt_abc123def456", "lsv2_pt_...f456"},
		{"abcdefghijklmnop", "abcdefgh...mnop"},
		// len == 13: still > 12
		{"abcdefghijklm", "abcdefgh...jklm"},
		// len == 12: show first 4 + "..." + last 3
		{"abcdefghijkl", "abcd...jkl"},
		// len == 8: show first 4 + "..." + last 3
		{"abcdefgh", "abcd...fgh"},
		// len == 5: show first 4 + "..." + last 3 (overlap is ok)
		{"abcde", "abcd...cde"},
		// len == 4: return "****"
		{"abcd", "****"},
		// len == 1: return "****"
		{"a", "****"},
		// len == 0: return "****"
		{"", "****"},
	}

	for _, tt := range tests {
		got := MaskAPIKey(tt.key)
		if got != tt.want {
			t.Errorf("MaskAPIKey(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}
