package config

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

const (
	ConfigFileEnv = "LANGSMITH_CONFIG_FILE"
	DefaultAPIURL = "https://api.smith.langchain.com"

	apiKeyCmdTimeout = 2 * time.Minute
)

// Profile represents one named LangSmith CLI profile.
//
// Instead of storing the API key, a profile can name where it lives:
// APIKeyEnv is an environment variable and APIKeyCmd a shell command that
// prints the key. APIKey wins over APIKeyEnv, which wins over APIKeyCmd.
//
// A SecretStore of "keychain" keeps APIKey in the OS keychain instead of the
// file, along with OAuth tokens the Go SDK moved there. Load reads them into
// the profile and Save writes them back where they came from, so the rest of
// the CLI sees them wherever they are stored. OAuth tokens in the file stay
// there: the Go SDK release the CLI uses refreshes tokens only from the file.
type Profile struct {
	APIKey      string `json:"api_key,omitempty"`
	APIKeyEnv   string `json:"api_key_env,omitempty"`
	APIKeyCmd   string `json:"api_key_cmd,omitempty"`
	APIURL      string `json:"api_url,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	SecretStore string `json:"secret_store,omitempty"`
	OAuth       OAuth  `json:"oauth,omitempty"`

	// keychainErr is why a keychain-backed profile's secrets could not be read.
	keychainErr error
	// accessInKeychain and refreshInKeychain report that each OAuth token
	// was read from the keychain rather than the file, so Save writes it back
	// to the same place.
	accessInKeychain  bool
	refreshInKeychain bool
}

// OAuth stores OAuth tokens written by `langsmith auth login`.
type OAuth struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	Issuer       string `json:"issuer,omitempty"`
}

// Config is the on-disk LangSmith CLI config.
type Config struct {
	CurrentProfile string             `json:"current_profile,omitempty"`
	Profiles       map[string]Profile `json:"profiles,omitempty"`
}

// DefaultConfigPath returns the LangSmith CLI config path.
func DefaultConfigPath() (string, error) {
	if p := os.Getenv(ConfigFileEnv); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".langsmith", "config.json"), nil
}

// Load reads the default config path. Missing files return an empty config.
func Load() (*Config, error) {
	path, err := DefaultConfigPath()
	if err != nil {
		return nil, err
	}
	return LoadFrom(path)
}

// LoadFrom reads a config file. Missing files return an empty config.
func LoadFrom(path string) (*Config, error) {
	cfg := &Config{Profiles: make(map[string]Profile)}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config JSON: %w", err)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]Profile)
	}
	for name, p := range cfg.Profiles {
		if p.SecretStore != SecretStoreKeychain {
			continue
		}
		secrets, err := loadKeychainSecrets(path, name)
		p.keychainErr = err
		if p.APIKey == "" {
			p.APIKey = secrets.APIKey
		}
		if p.OAuth.AccessToken == "" && secrets.AccessToken != "" {
			p.OAuth.AccessToken = secrets.AccessToken
			p.accessInKeychain = true
		}
		if p.OAuth.RefreshToken == "" && secrets.RefreshToken != "" {
			p.OAuth.RefreshToken = secrets.RefreshToken
			p.refreshInKeychain = true
		}
		cfg.Profiles[name] = p
	}
	warnIfGroupOrWorldReadable(path)
	return cfg, nil
}

// warnedConfigs keeps the warning to once per path; a command loads the config several times.
var warnedConfigs sync.Map

func warnIfGroupOrWorldReadable(path string) {
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	perm := info.Mode().Perm()
	if perm&0o077 == 0 {
		return
	}
	if _, seen := warnedConfigs.LoadOrStore(path, struct{}{}); seen {
		return
	}
	fmt.Fprintf(os.Stderr, "warning: %s is accessible to other users (mode %#o); run: chmod 600 %s\n", path, perm, path)
}

// Save writes the config to the default config path with owner-only permissions.
func (c *Config) Save() error {
	path, err := DefaultConfigPath()
	if err != nil {
		return err
	}
	return c.SaveTo(path)
}

// SaveTo writes the config to path with owner-only permissions. A
// keychain-backed profile's API key, and each OAuth token that was read from
// the keychain, are written to the OS keychain rather than the file; an OAuth
// token read from the file stays in the file. LANGSMITH_SECRET_STORE=keychain makes
// every API-key profile keychain-backed, leaving OAuth profiles alone. When the
// keychain cannot be written, the secrets stay in the file and a warning says
// so. A profile whose keychain could not be read is saved as it was loaded,
// so a save never replaces credentials it could not see.
func (c *Config) SaveTo(path string) error {
	if c.Profiles == nil {
		c.Profiles = make(map[string]Profile)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	// Write through a symlinked config to its target, as the Go SDK does, so
	// the link survives and keychain entries keep the name they were saved
	// under.
	path = canonicalConfigPath(path)

	onDisk := Config{CurrentProfile: c.CurrentProfile, Profiles: make(map[string]Profile, len(c.Profiles))}
	for name, p := range c.Profiles {
		if p.APIKey != "" && !p.HasOAuthTokens() && KeychainRequested() {
			p.SecretStore = SecretStoreKeychain
		}
		if p.SecretStore == SecretStoreKeychain && p.keychainErr == nil {
			secrets := keychainSecrets{APIKey: p.APIKey}
			if p.accessInKeychain {
				secrets.AccessToken = p.OAuth.AccessToken
			}
			if p.refreshInKeychain {
				secrets.RefreshToken = p.OAuth.RefreshToken
			}
			if err := saveKeychainSecrets(path, name, secrets); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not save profile %q credentials in the OS keychain (%v); saved them in %s instead\n", name, err, path)
				p.SecretStore = ""
			}
		}
		c.Profiles[name] = p
		if p.SecretStore == SecretStoreKeychain && p.keychainErr == nil {
			p.APIKey = ""
			if p.accessInKeychain {
				p.OAuth.AccessToken = ""
			}
			if p.refreshInKeychain {
				p.OAuth.RefreshToken = ""
			}
		}
		onDisk.Profiles[name] = p
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(onDisk); err != nil {
		return fmt.Errorf("encoding config JSON: %w", err)
	}

	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".langsmith-config-*")
	if err != nil {
		return fmt.Errorf("creating temporary config: %w", err)
	}
	tmpPath := f.Name()
	cleanup := func() {
		_ = os.Remove(tmpPath)
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		closeErr := f.Close()
		cleanup()
		if closeErr != nil {
			return fmt.Errorf("writing config: %v (closing temporary config: %v)", err, closeErr)
		}
		return fmt.Errorf("writing config: %w", err)
	}
	if err := f.Chmod(0600); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("setting config permissions: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("syncing temporary config: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return fmt.Errorf("closing temporary config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("replacing config: %w", err)
	}
	return nil
}

// ResolveProfileName returns the active profile name by precedence.
func (c *Config) ResolveProfileName(flagProfile, envProfile string) string {
	if flagProfile != "" {
		return flagProfile
	}
	if envProfile != "" {
		return envProfile
	}
	if c.CurrentProfile != "" {
		return c.CurrentProfile
	}
	if _, ok := c.Profiles["default"]; ok {
		return "default"
	}
	return ""
}

// ResolveProfile returns the active profile by precedence.
func (c *Config) ResolveProfile(flagProfile, envProfile string) (string, Profile, bool) {
	name := c.ResolveProfileName(flagProfile, envProfile)
	if name == "" {
		return "", Profile{}, false
	}
	p, ok := c.Profiles[name]
	return name, p, ok
}

// HasOAuthTokens reports whether the profile holds OAuth tokens from a login.
func (p Profile) HasOAuthTokens() bool {
	return p.OAuth.AccessToken != "" || p.OAuth.RefreshToken != ""
}

// AccessToken returns the OAuth access token to use for bearer auth.
func (p Profile) AccessToken() string {
	return p.OAuth.AccessToken
}

// HasAPIKey reports whether the profile stores an API key or names where one
// lives.
func (p Profile) HasAPIKey() bool {
	return p.APIKey != "" || p.APIKeyEnv != "" || p.APIKeyCmd != "" || p.keychainErr != nil
}

// APIKeySource describes where the profile's API key comes from, for display:
// "config", "keychain", "$NAME" for an environment variable, or "$(command)".
func (p Profile) APIKeySource() string {
	switch {
	case p.SecretStore == SecretStoreKeychain && (p.APIKey != "" || p.keychainErr != nil):
		return "keychain"
	case p.APIKey != "":
		return "config"
	case p.APIKeyEnv != "":
		return "$" + p.APIKeyEnv
	case p.APIKeyCmd != "":
		return "$(" + p.APIKeyCmd + ")"
	default:
		return ""
	}
}

// ResolveAPIKey returns the profile's API key, reading api_key_env or running
// api_key_cmd when the key is not stored in the config or the keychain.
func (p Profile) ResolveAPIKey() (string, error) {
	switch {
	case p.APIKey != "":
		return p.APIKey, nil
	case p.keychainErr != nil:
		return "", p.keychainErr
	case p.APIKeyEnv != "":
		if key := strings.TrimSpace(os.Getenv(p.APIKeyEnv)); key != "" {
			return key, nil
		}
		return "", fmt.Errorf("profile reads its API key from $%s, which is not set", p.APIKeyEnv)
	case p.APIKeyCmd != "":
		return runAPIKeyCmd(p.APIKeyCmd)
	default:
		return "", nil
	}
}

type apiKeyCmdResult struct {
	key string
	err error
}

// apiKeyCmdResults caches each api_key_cmd's outcome for the process, so a
// command such as a password manager lookup runs, and prompts, at most once
// per CLI invocation even though auth is resolved several times.
var apiKeyCmdResults sync.Map

func runAPIKeyCmd(command string) (string, error) {
	if r, ok := apiKeyCmdResults.Load(command); ok {
		res := r.(apiKeyCmdResult)
		return res.key, res.err
	}
	key, err := execAPIKeyCmd(command)
	apiKeyCmdResults.Store(command, apiKeyCmdResult{key: key, err: err})
	return key, err
}

func execAPIKeyCmd(command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), apiKeyCmdTimeout)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", command)
	}
	// The command may prompt, for example to unlock a password manager, so it
	// shares the terminal. Stdin is passed only when it is a terminal: under
	// `sandboxes tunnel --stdio` it carries the SSH stream. Only stdout is
	// captured.
	if term.IsTerminal(int(os.Stdin.Fd())) {
		cmd.Stdin = os.Stdin
	}
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("running api_key_cmd: %w", err)
	}
	key := strings.TrimSpace(string(out))
	if key == "" {
		return "", fmt.Errorf("api_key_cmd printed no API key")
	}
	return key, nil
}

// TokenExpiresAtTime parses oauth.expires_at.
func (p Profile) TokenExpiresAtTime() (time.Time, bool) {
	if p.OAuth.ExpiresAt == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, p.OAuth.ExpiresAt)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// MaskSecret returns a redacted value suitable for CLI output.
func MaskSecret(value string) string {
	if len(value) < 12 {
		return "****"
	}
	return value[:8] + "..." + value[len(value)-4:]
}
