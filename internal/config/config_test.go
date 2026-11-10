package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
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

func TestSaveToKeepsAPIKeyInKeychain(t *testing.T) {
	for _, tc := range []struct {
		name         string
		profile      Profile
		env          string
		keychainErr  error
		wantKeychain bool
	}{
		{name: "opted in", profile: Profile{APIKey: "keychain-key", SecretStore: SecretStoreKeychain}, wantKeychain: true},
		{name: "env moves a plaintext key", profile: Profile{APIKey: "keychain-key"}, env: "keychain", wantKeychain: true},
		{name: "plaintext by default", profile: Profile{APIKey: "keychain-key"}},
		{name: "env leaves OAuth profiles in the file", profile: Profile{APIKey: "keychain-key", OAuth: OAuth{RefreshToken: "refresh"}}, env: "keychain"},
		{name: "no keychain falls back to the file", profile: Profile{APIKey: "keychain-key", SecretStore: SecretStoreKeychain}, keychainErr: errors.New("no session bus")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(SecretStoreEnv, tc.env)
			if tc.keychainErr != nil {
				keyring.MockInitWithError(tc.keychainErr)
				t.Cleanup(keyring.MockInit)
			}
			path := filepath.Join(t.TempDir(), "config.json")
			if err := (&Config{Profiles: map[string]Profile{"prod": tc.profile}}).SaveTo(path); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if inFile := strings.Contains(string(raw), "keychain-key"); inFile == tc.wantKeychain {
				t.Fatalf("expected key in file = %v, got config %s", !tc.wantKeychain, raw)
			}
			if inKeychain := strings.Contains(string(raw), `"secret_store": "keychain"`); inKeychain != tc.wantKeychain {
				t.Fatalf("expected secret_store keychain = %v, got config %s", tc.wantKeychain, raw)
			}

			// Read the key back from the keychain, not the process cache.
			keychainEntries.Clear()
			if tc.keychainErr != nil {
				keyring.MockInit()
			}
			loaded, err := LoadFrom(path)
			if err != nil {
				t.Fatal(err)
			}
			if key, err := loaded.Profiles["prod"].ResolveAPIKey(); err != nil || key != "keychain-key" {
				t.Fatalf("expected the key back after load, got %q, %v", key, err)
			}
		})
	}
}

func TestKeychainProfileKeepsSDKRefreshedTokens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"profiles": {"prod": {"secret_store": "keychain", "oauth": {"expires_at": "2030-01-01T00:00:00Z"}}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	// The Go SDK wrote rotated tokens to the shared keychain entry.
	if err := keyring.Set(keychainService, keychainAccount(path, "prod"), `{"api_key":"k","access_token":"sdk-access","refresh_token":"sdk-refresh"}`); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Profiles["prod"]
	if p.AccessToken() != "sdk-access" || p.OAuth.RefreshToken != "sdk-refresh" || p.APIKey != "k" {
		t.Fatalf("expected the keychain entry loaded, got %+v", p)
	}
	cfg.CurrentProfile = "prod"
	if err := cfg.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "sdk-") {
		t.Fatalf("expected no tokens in the file, got %s", raw)
	}
	keychainEntries.Clear()
	if got, err := loadKeychainSecrets(path, "prod"); err != nil || got.RefreshToken != "sdk-refresh" {
		t.Fatalf("expected the tokens kept in the keychain, got %+v, %v", got, err)
	}
}

func TestKeychainProfileKeepsEachSecretWhereItLives(t *testing.T) {
	for _, tc := range []struct {
		name      string
		file      string // the profile's api_key and oauth fields in the file
		entry     string // its keychain entry
		wantFile  []string
		wantEntry keychainSecrets
	}{
		{
			name:      "access in file, refresh in keychain",
			file:      `"oauth": {"access_token": "file-access"}`,
			entry:     `{"api_key":"k","refresh_token":"kc-refresh"}`,
			wantFile:  []string{"file-access"},
			wantEntry: keychainSecrets{APIKey: "k", RefreshToken: "kc-refresh"},
		},
		{
			name:      "access in keychain, refresh in file",
			file:      `"oauth": {"refresh_token": "file-refresh"}`,
			entry:     `{"api_key":"k","access_token":"kc-access"}`,
			wantFile:  []string{"file-refresh"},
			wantEntry: keychainSecrets{APIKey: "k", AccessToken: "kc-access"},
		},
		{
			name:      "both tokens in file",
			file:      `"oauth": {"access_token": "file-access", "refresh_token": "file-refresh"}`,
			entry:     `{"api_key":"k"}`,
			wantFile:  []string{"file-access", "file-refresh"},
			wantEntry: keychainSecrets{APIKey: "k"},
		},
		{
			name:      "both tokens in keychain, API key in file",
			file:      `"api_key": "file-key"`,
			entry:     `{"access_token":"kc-access","refresh_token":"kc-refresh"}`,
			wantEntry: keychainSecrets{APIKey: "file-key", AccessToken: "kc-access", RefreshToken: "kc-refresh"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			content := `{"profiles": {"prod": {"secret_store": "keychain", "workspace_id": "ws", ` + tc.file + `}}}`
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			if err := keyring.Set(keychainService, keychainAccount(path, "prod"), tc.entry); err != nil {
				t.Fatal(err)
			}

			cfg, err := LoadFrom(path)
			if err != nil {
				t.Fatal(err)
			}
			p := cfg.Profiles["prod"]
			p.WorkspaceID = "ws-2"
			cfg.Profiles["prod"] = p
			if err := cfg.SaveTo(path); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range append(tc.wantFile, "ws-2") {
				if !strings.Contains(string(raw), want) {
					t.Fatalf("expected %q kept in the file, got %s", want, raw)
				}
			}
			if strings.Contains(string(raw), "kc-") || strings.Contains(string(raw), "file-key") {
				t.Fatalf("expected keychain secrets and the API key kept out of the file, got %s", raw)
			}
			keychainEntries.Clear()
			if got, err := loadKeychainSecrets(path, "prod"); err != nil || got != tc.wantEntry {
				t.Fatalf("expected keychain entry %+v, got %+v, %v", tc.wantEntry, got, err)
			}
		})
	}
}

func TestUnreadableKeychainKeepsFileSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"profiles": {"prod": {"secret_store": "keychain", "api_key": "file-key", "oauth": {"access_token": "file-access", "refresh_token": "file-refresh"}}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	keyring.MockInitWithError(errors.New("keychain is locked"))
	t.Cleanup(keyring.MockInit)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.CurrentProfile = "prod"
	if err := cfg.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"file-key", "file-access", "file-refresh", `"secret_store": "keychain"`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("expected %q kept in the file, got %s", want, raw)
		}
	}
}

func TestSaveToKeepsSymlinkedConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks needs extra privileges on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles-config.json")
	if err := os.WriteFile(target, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "config.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{Profiles: map[string]Profile{"prod": {APIKey: "keychain-key", SecretStore: SecretStoreKeychain}}}
	if err := cfg.SaveTo(link); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected the config symlink kept, got %v, %v", info, err)
	}
	keychainEntries.Clear()
	loaded, err := LoadFrom(link)
	if err != nil {
		t.Fatal(err)
	}
	if key, err := loaded.Profiles["prod"].ResolveAPIKey(); err != nil || key != "keychain-key" {
		t.Fatalf("expected the key back through the symlink, got %q, %v", key, err)
	}
}

func TestNewKeychainProfileReplacesStaleEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := keyring.Set(keychainService, keychainAccount(path, "prod"), `{"access_token":"stale","refresh_token":"stale"}`); err != nil {
		t.Fatal(err)
	}
	if err := (&Config{Profiles: map[string]Profile{"prod": {APIKey: "new-key", SecretStore: SecretStoreKeychain}}}).SaveTo(path); err != nil {
		t.Fatal(err)
	}
	keychainEntries.Clear()
	if got, err := loadKeychainSecrets(path, "prod"); err != nil || got != (keychainSecrets{APIKey: "new-key"}) {
		t.Fatalf("expected the stale tokens dropped, got %+v, %v", got, err)
	}
}

func TestLoadFromMissingKeychainEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"profiles": {"prod": {"secret_store": "keychain"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("a keychain error must not fail loading the config: %v", err)
	}
	p := cfg.Profiles["prod"]
	if !p.HasAPIKey() || p.APIKeySource() != "keychain" {
		t.Fatalf("expected a keychain API key profile, got %+v", p)
	}
	if _, err := p.ResolveAPIKey(); err == nil || !strings.Contains(err.Error(), "no entry was found") {
		t.Fatalf("expected missing keychain entry error, got %v", err)
	}
	if err := cfg.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	if raw, _ := os.ReadFile(path); !strings.Contains(string(raw), `"secret_store": "keychain"`) {
		t.Fatalf("expected saving to keep the profile in the keychain, got %s", raw)
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

func TestMaskSecret(t *testing.T) {
	if got := MaskSecret("test-access-token"); got != "test-acc...oken" {
		t.Fatalf("unexpected masked secret: %q", got)
	}
	if got := MaskSecret("short"); got != "****" {
		t.Fatalf("unexpected short masked secret: %q", got)
	}
}

func TestResolveAPIKeyFromReferences(t *testing.T) {
	t.Setenv("TEST_LANGSMITH_KEY", " env-key \n")
	if key, err := (Profile{APIKeyEnv: "TEST_LANGSMITH_KEY"}).ResolveAPIKey(); err != nil || key != "env-key" {
		t.Fatalf("api_key_env: got %q, %v", key, err)
	}
	if key, _ := (Profile{APIKey: "saved-key", APIKeyEnv: "TEST_LANGSMITH_KEY"}).ResolveAPIKey(); key != "saved-key" {
		t.Fatalf("expected api_key to win over api_key_env, got %q", key)
	}

	t.Setenv("TEST_LANGSMITH_KEY", "")
	if _, err := (Profile{APIKeyEnv: "TEST_LANGSMITH_KEY"}).ResolveAPIKey(); err == nil || !strings.Contains(err.Error(), "$TEST_LANGSMITH_KEY, which is not set") {
		t.Fatalf("expected unset variable error, got %v", err)
	}
}

func TestResolveAPIKeyCmdRunsOncePerProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell")
	}
	runs := filepath.Join(t.TempDir(), "runs")
	profile := Profile{APIKeyCmd: fmt.Sprintf("echo run >> %q; echo cmd-key", runs)}
	for range 3 {
		key, err := profile.ResolveAPIKey()
		if err != nil || key != "cmd-key" {
			t.Fatalf("got %q, %v", key, err)
		}
	}
	data, err := os.ReadFile(runs)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), "run"); n != 1 {
		t.Fatalf("expected api_key_cmd to run once, ran %d times", n)
	}

	if _, err := (Profile{APIKeyCmd: "true"}).ResolveAPIKey(); err == nil || !strings.Contains(err.Error(), "printed no API key") {
		t.Fatalf("expected empty output error, got %v", err)
	}
}
