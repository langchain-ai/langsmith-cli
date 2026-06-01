package cmd

import (
	"fmt"
	"os"
	"strings"
)

func sandboxConsoleEnv(extra []string) (map[string]string, error) {
	return sandboxConsoleEnvFrom(os.Environ(), extra)
}

func sandboxConsoleEnvFrom(environ []string, extra []string) (map[string]string, error) {
	env := make(map[string]string)
	local := make(map[string]string)
	for _, kv := range environ {
		key, value, ok := strings.Cut(kv, "=")
		if !ok || key == "" {
			continue
		}
		local[key] = value
		if value != "" && sandboxConsoleEnvAllowed(key) {
			env[key] = value
		}
	}
	for _, spec := range extra {
		key, value, hasValue := strings.Cut(spec, "=")
		if key == "" {
			return nil, fmt.Errorf("--env must be KEY or KEY=VALUE")
		}
		if hasValue {
			env[key] = value
			continue
		}
		value, ok := local[key]
		if !ok {
			return nil, fmt.Errorf("--env %s is not set in the local environment", key)
		}
		env[key] = value
	}
	return env, nil
}

func sandboxConsoleEnvAllowed(key string) bool {
	switch key {
	case "TERM", "COLORTERM", "LANG", "CLICOLOR", "CLICOLOR_FORCE", "FORCE_COLOR", "NO_COLOR":
		return true
	default:
		return strings.HasPrefix(key, "LC_")
	}
}
