package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// APIKeyFilePrefix marks an API key value as a path to read the key from.
const APIKeyFilePrefix = "@"

// ResolveAPIKeyValue expands an "@path" API key reference to the trimmed
// contents of that file. Values without the prefix are returned unchanged.
func ResolveAPIKeyValue(value string) (string, error) {
	if !strings.HasPrefix(value, APIKeyFilePrefix) {
		return value, nil
	}

	path := APIKeyFilePath(value)
	if path == "" {
		return "", fmt.Errorf("api key %q is missing a file path after %q", value, APIKeyFilePrefix)
	}
	return readAPIKeyFile(path)
}

// APIKeyFilePath returns the file path an "@path" API key value refers to, or
// "" if the value is not a file reference.
func APIKeyFilePath(value string) string {
	if !strings.HasPrefix(value, APIKeyFilePrefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(value, APIKeyFilePrefix))
}

// readAPIKeyFile reads an API key from path, expanding a leading "~".
func readAPIKeyFile(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory for api key file %s: %w", path, err)
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading api key file %s: %w", path, err)
	}
	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", fmt.Errorf("api key file %s is empty", path)
	}
	if strings.ContainsFunc(key, unicode.IsSpace) {
		return "", fmt.Errorf("api key file %s must contain only the key", path)
	}
	return key, nil
}
