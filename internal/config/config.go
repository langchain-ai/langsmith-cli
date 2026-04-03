// Package config handles reading, writing, and querying a TOML config file
// for named profiles. The config file lives at ~/.langsmith/config.toml.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/BurntSushi/toml"
)

// Profile holds per-profile configuration.
type Profile struct {
	APIKey      string
	APIURL      string
	WorkspaceID string
}

// Config holds the full config file contents.
type Config struct {
	CurrentProfile string
	Profiles       map[string]Profile
}

// DefaultConfigPath returns the default config file path: ~/.langsmith/config.toml.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".langsmith", "config.toml")
	}
	return filepath.Join(home, ".langsmith", "config.toml")
}

// Load loads the config from the default path.
func Load() (*Config, error) {
	return LoadFrom(DefaultConfigPath())
}

// LoadFrom loads the config from the given path. Returns an empty Config if the
// file does not exist.
func LoadFrom(path string) (*Config, error) {
	cfg := &Config{
		Profiles: make(map[string]Profile),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Decode into a raw map so we can handle the mixed structure (scalar
	// current_profile + profile sections).
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	// Extract current_profile scalar.
	if v, ok := raw["current_profile"]; ok {
		if s, ok := v.(string); ok {
			cfg.CurrentProfile = s
		}
	}

	// Every other top-level key is a profile section.
	for key, val := range raw {
		if key == "current_profile" {
			continue
		}
		section, ok := val.(map[string]any)
		if !ok {
			continue
		}
		p := Profile{}
		if v, ok := section["api_key"].(string); ok {
			p.APIKey = v
		}
		if v, ok := section["api_url"].(string); ok {
			p.APIURL = v
		}
		if v, ok := section["workspace_id"].(string); ok {
			p.WorkspaceID = v
		}
		cfg.Profiles[key] = p
	}

	return cfg, nil
}

// Save saves the config to the default path.
func (c *Config) Save() error {
	return c.SaveTo(DefaultConfigPath())
}

// validProfileName matches alphanumeric, hyphens, and underscores only.
var validProfileName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ValidateProfileName checks that a profile name is safe for use as a TOML section header.
func ValidateProfileName(name string) error {
	if !validProfileName.MatchString(name) {
		return fmt.Errorf("invalid profile name %q: must contain only alphanumeric characters, hyphens, and underscores", name)
	}
	return nil
}

// SaveTo saves the config to the given path, creating parent directories as
// needed. The file is written with 0600 permissions since it contains secrets.
func (c *Config) SaveTo(path string) error {
	for name := range c.Profiles {
		if err := ValidateProfileName(name); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("opening config file for write: %w", err)
	}
	defer f.Close()

	var writeErr error
	w := func(format string, args ...any) {
		if writeErr != nil {
			return
		}
		_, writeErr = fmt.Fprintf(f, format, args...)
	}

	if c.CurrentProfile != "" {
		w("current_profile = %q\n", c.CurrentProfile)
	}

	// Sort profile names for deterministic output.
	names := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		p := c.Profiles[name]
		w("\n[%s]\n", name)
		if p.APIKey != "" {
			w("api_key = %q\n", p.APIKey)
		}
		if p.APIURL != "" {
			w("api_url = %q\n", p.APIURL)
		}
		if p.WorkspaceID != "" {
			w("workspace_id = %q\n", p.WorkspaceID)
		}
	}

	if writeErr != nil {
		return fmt.Errorf("writing config file: %w", writeErr)
	}
	return nil
}

// ResolveProfile resolves the active profile using the following precedence:
// flagProfile → envProfile → CurrentProfile → "default" → nil.
func (c *Config) ResolveProfile(flagProfile, envProfile string) *Profile {
	name := c.ResolveProfileName(flagProfile, envProfile)
	if name == "" {
		return nil
	}
	p, ok := c.Profiles[name]
	if !ok {
		return nil
	}
	return &p
}

// ResolveProfileName returns the name of the active profile using the same
// precedence as ResolveProfile. Returns "" when no profile can be found.
func (c *Config) ResolveProfileName(flagProfile, envProfile string) string {
	for _, candidate := range []string{flagProfile, envProfile, c.CurrentProfile, "default"} {
		if candidate == "" {
			continue
		}
		if _, ok := c.Profiles[candidate]; ok {
			return candidate
		}
	}
	return ""
}

// MaskAPIKey masks an API key for display.
//   - len > 12:  first 8 chars + "..." + last 4 chars
//   - len 5–12: first 4 chars + "..." + last 3 chars
//   - len <= 4:  "****"
func MaskAPIKey(key string) string {
	n := len(key)
	switch {
	case n > 12:
		return key[:8] + "..." + key[n-4:]
	case n >= 5:
		return key[:4] + "..." + key[n-3:]
	default:
		return "****"
	}
}
