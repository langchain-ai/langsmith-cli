package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const (
	ConfigFileEnv = "LANGSMITH_CONFIG_FILE"
	DefaultAPIURL = "https://api.smith.langchain.com"
)

// Profile represents one named LangSmith CLI profile.
type Profile struct {
	APIKey      string `json:"api_key,omitempty"`
	APIURL      string `json:"api_url,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	OAuth       OAuth  `json:"oauth,omitempty"`
}

// OAuth stores OAuth tokens written by `langsmith login`.
type OAuth struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
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

// SaveTo writes the config to path with owner-only permissions.
func (c *Config) SaveTo(path string) error {
	if c.Profiles == nil {
		c.Profiles = make(map[string]Profile)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(c); err != nil {
		return fmt.Errorf("encoding config JSON: %w", err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("opening config for write: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	if err := f.Chmod(0600); err != nil {
		return fmt.Errorf("setting config permissions: %w", err)
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

// AccessToken returns the OAuth access token to use for bearer auth.
func (p Profile) AccessToken() string {
	return p.OAuth.AccessToken
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

// TokenExpiresSoon reports whether the access token should be refreshed.
func (p Profile) TokenExpiresSoon(now time.Time, leeway time.Duration) bool {
	expiresAt, ok := p.TokenExpiresAtTime()
	if !ok {
		return false
	}
	return !expiresAt.After(now.Add(leeway))
}

// MaskSecret returns a redacted value suitable for CLI output.
func MaskSecret(value string) string {
	if len(value) < 12 {
		return "****"
	}
	return value[:8] + "..." + value[len(value)-4:]
}
