package config

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	ConfigFileEnv = "LANGSMITH_CONFIG_FILE"
	DefaultAPIURL = "https://api.smith.langchain.com"
)

// Profile represents one named LangSmith CLI profile.
type Profile struct {
	APIKey      string
	APIURL      string
	WorkspaceID string
	OAuth       OAuth
}

// OAuth stores OAuth tokens written by `langsmith login`.
type OAuth struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    string
}

// Config is the on-disk LangSmith CLI config.
type Config struct {
	CurrentProfile string
	Profiles       map[string]Profile
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
	return filepath.Join(home, ".langsmith", "config.toml"), nil
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

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("opening config: %w", err)
	}
	defer f.Close()

	var section string
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			if section == "" {
				return nil, fmt.Errorf("parsing config line %d: empty profile section", lineNo)
			}
			profileName, _, _ := strings.Cut(section, ".")
			if _, ok := cfg.Profiles[profileName]; !ok {
				cfg.Profiles[profileName] = Profile{}
			}
			continue
		}

		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("parsing config line %d: expected key = value", lineNo)
		}
		key = strings.TrimSpace(key)
		value, err := parseStringValue(strings.TrimSpace(rawValue))
		if err != nil {
			return nil, fmt.Errorf("parsing config line %d: %w", lineNo, err)
		}

		if section == "" {
			if key == "current_profile" {
				cfg.CurrentProfile = value
			}
			continue
		}

		profileName, subsection, hasSubsection := strings.Cut(section, ".")
		profile := cfg.Profiles[profileName]
		switch key {
		case "api_key":
			if !hasSubsection {
				profile.APIKey = value
			}
		case "api_url":
			if !hasSubsection {
				profile.APIURL = value
			}
		case "workspace_id":
			if !hasSubsection {
				profile.WorkspaceID = value
			}
		case "access_token":
			if subsection == "oauth" {
				profile.OAuth.AccessToken = value
			}
		case "refresh_token":
			if subsection == "oauth" {
				profile.OAuth.RefreshToken = value
			}
		case "expires_at":
			if subsection == "oauth" {
				profile.OAuth.ExpiresAt = value
			}
		}
		cfg.Profiles[profileName] = profile
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	return cfg, nil
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
	if c.CurrentProfile != "" {
		fmt.Fprintf(&buf, "current_profile = %s\n", strconv.Quote(c.CurrentProfile))
	}

	names := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		p := c.Profiles[name]
		if buf.Len() > 0 {
			buf.WriteByte('\n')
		}
		fmt.Fprintf(&buf, "[%s]\n", name)
		writeString(&buf, "api_key", p.APIKey)
		writeString(&buf, "api_url", p.APIURL)
		writeString(&buf, "workspace_id", p.WorkspaceID)
		if p.OAuth.AccessToken != "" || p.OAuth.RefreshToken != "" || p.OAuth.ExpiresAt != "" {
			fmt.Fprintf(&buf, "\n[%s.oauth]\n", name)
			writeString(&buf, "access_token", p.OAuth.AccessToken)
			writeString(&buf, "refresh_token", p.OAuth.RefreshToken)
			writeString(&buf, "expires_at", p.OAuth.ExpiresAt)
		}
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

func parseStringValue(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if strings.HasPrefix(raw, "\"") || strings.HasPrefix(raw, "`") {
		v, err := strconv.Unquote(raw)
		if err != nil {
			return "", fmt.Errorf("invalid quoted string: %w", err)
		}
		return v, nil
	}
	return raw, nil
}

func writeString(buf *bytes.Buffer, key, value string) {
	if value == "" {
		return
	}
	fmt.Fprintf(buf, "%s = %s\n", key, strconv.Quote(value))
}
