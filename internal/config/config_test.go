package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestLoadFromMissingFile(t *testing.T) {
	cfg, err := LoadFrom(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatalf("LoadFrom returned error: %v", err)
	}
	if cfg.CurrentProfile != "" {
		t.Fatalf("expected empty current profile, got %q", cfg.CurrentProfile)
	}
	if len(cfg.Profiles) != 0 {
		t.Fatalf("expected no profiles, got %d", len(cfg.Profiles))
	}
}

func TestLoadFromProfileWithOAuthTokens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data := `{
  "current_profile": "prod",
  "profiles": {
    "prod": {
      "api_url": "https://example.com",
      "oauth": {
        "access_token": "test-access-token",
        "refresh_token": "test-refresh-token",
        "expires_at": "2026-04-29T12:00:00Z"
      }
    }
  }
}
`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom returned error: %v", err)
	}
	profile := cfg.Profiles["prod"]
	if cfg.CurrentProfile != "prod" {
		t.Fatalf("expected prod current profile, got %q", cfg.CurrentProfile)
	}
	if profile.APIURL != "https://example.com" {
		t.Fatalf("unexpected api url: %q", profile.APIURL)
	}
	if profile.AccessToken() != "test-access-token" {
		t.Fatalf("unexpected access token: %q", profile.AccessToken())
	}
	if profile.OAuth.RefreshToken != "test-refresh-token" {
		t.Fatalf("unexpected refresh token: %q", profile.OAuth.RefreshToken)
	}
}

func TestSaveToRoundTripAndPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	cfg := &Config{
		CurrentProfile: "prod",
		Profiles: map[string]Profile{
			"prod": {
				APIURL: "https://example.com",
				OAuth: OAuth{
					AccessToken:  "test-access-token",
					RefreshToken: "test-refresh-token",
					ExpiresAt:    "2026-04-29T12:00:00Z",
				},
			},
		},
	}

	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo returned error: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("expected config permissions 0600, got %o", info.Mode().Perm())
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom returned error: %v", err)
	}
	if loaded.Profiles["prod"].OAuth.AccessToken != "test-access-token" {
		t.Fatalf("profile token did not round-trip")
	}
}

func TestSaveToFailureLeavesDestinationAndCleansTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}

	err := (&Config{Profiles: map[string]Profile{"prod": {APIKey: "new"}}}).SaveTo(path)
	if err == nil {
		t.Fatal("expected replacement of directory to fail")
	}
	if info, statErr := os.Stat(path); statErr != nil || !info.IsDir() {
		t.Fatalf("destination was changed after failed replacement: info=%v err=%v", info, statErr)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if len(entry.Name()) >= len(".langsmith-config-") && entry.Name()[:len(".langsmith-config-")] == ".langsmith-config-" {
			t.Fatalf("temporary config file was not removed: %s", entry.Name())
		}
	}
}

func TestResolveProfile(t *testing.T) {
	cfg := &Config{
		CurrentProfile: "current",
		Profiles: map[string]Profile{
			"default": {APIKey: "default-key"},
			"current": {APIKey: "current-key"},
			"env":     {APIKey: "env-key"},
			"flag":    {APIKey: "flag-key"},
		},
	}

	name, profile, ok := cfg.ResolveProfile("flag", "env")
	if !ok || name != "flag" || profile.APIKey != "flag-key" {
		t.Fatalf("expected flag profile, got name=%q profile=%+v ok=%v", name, profile, ok)
	}

	name, profile, ok = cfg.ResolveProfile("", "env")
	if !ok || name != "env" || profile.APIKey != "env-key" {
		t.Fatalf("expected env profile, got name=%q profile=%+v ok=%v", name, profile, ok)
	}

	name, profile, ok = cfg.ResolveProfile("", "")
	if !ok || name != "current" || profile.APIKey != "current-key" {
		t.Fatalf("expected current profile, got name=%q profile=%+v ok=%v", name, profile, ok)
	}
}

func TestTokenExpiresSoon(t *testing.T) {
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	profile := Profile{OAuth: OAuth{ExpiresAt: now.Add(30 * time.Second).Format(time.RFC3339)}}
	if !profile.TokenExpiresSoon(now, time.Minute) {
		t.Fatalf("expected token to expire soon")
	}

	profile.OAuth.ExpiresAt = now.Add(10 * time.Minute).Format(time.RFC3339)
	if profile.TokenExpiresSoon(now, time.Minute) {
		t.Fatalf("expected token not to expire soon")
	}
}

func TestMaskSecret(t *testing.T) {
	if got := MaskSecret("test-access-token"); got != "test-acc...oken" {
		t.Fatalf("unexpected masked secret: %q", got)
	}
	if got := MaskSecret("short"); got != "****" {
		t.Fatalf("unexpected short masked secret: %q", got)
	}
}
