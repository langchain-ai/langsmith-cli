package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Separate from mergeJSONFile: retain arbitrary numbers exactly, reject invalid
// existing types, and atomically replace user settings as well as project ones.
func gatewayReadSettings(path string) ([]byte, map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("reading Claude settings: %w", err)
	}
	var doc map[string]json.RawMessage
	if json.Unmarshal(data, &doc) != nil || doc == nil {
		return nil, nil, errors.New("Claude settings must contain a valid JSON object; repair it before rerunning")
	}
	return data, doc, nil
}

func gatewaySettingsEnv(doc map[string]json.RawMessage) (map[string]string, error) {
	env := map[string]string{}
	if raw, ok := doc["env"]; ok {
		var values map[string]json.RawMessage
		if json.Unmarshal(raw, &values) != nil || values == nil {
			return nil, errors.New("Claude settings env must be an object of strings")
		}
		for k, v := range values {
			var text string
			if bytes.Equal(bytes.TrimSpace(v), []byte("null")) || json.Unmarshal(v, &text) != nil {
				return nil, fmt.Errorf("Claude settings env key %q must be a string", k)
			}
			env[k] = text
		}
	}
	return env, nil
}

var gatewayConflictingEnv = []string{
	"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN",
	"CLAUDE_CODE_OAUTH_TOKEN_FILE_DESCRIPTOR", "CLAUDE_CODE_API_KEY_FILE_DESCRIPTOR",
	"CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX", "CLAUDE_CODE_USE_FOUNDRY",
	"ANTHROPIC_FOUNDRY_API_KEY", "ANTHROPIC_FOUNDRY_BASE_URL",
	"CLAUDE_CODE_SKIP_BEDROCK_AUTH", "CLAUDE_CODE_SKIP_VERTEX_AUTH", "CLAUDE_CODE_SKIP_FOUNDRY_AUTH",
}

func gatewayCredentialConflicts(env map[string]string, getenv func(string) string) error {
	for _, key := range gatewayConflictingEnv {
		if env[key] != "" || getenv(key) != "" {
			return fmt.Errorf("remove/unset conflicting %s from Claude settings and the environment before rerunning; credentials and provider switches are never overridden", key)
		}
	}
	return nil
}

// Claude's header format is newline-separated Name: value. Never print values,
// including malformed input. Preserve unrelated saved header lines verbatim.
func gatewayMergeHeaders(raw, workspace string) (string, []string, error) {
	names := []string{}
	seen := map[string]bool{}
	tenant := ""
	if raw != "" {
		for _, line := range strings.Split(raw, "\n") {
			if line == "" {
				continue
			}
			name, value, ok := strings.Cut(line, ":")
			if !ok || name == "" || strings.TrimSpace(name) != name {
				return "", nil, errors.New("malformed ANTHROPIC_CUSTOM_HEADERS; use newline-separated Name: value lines")
			}
			for _, c := range name {
				if !strings.ContainsRune("!#$%&'*+-.^_`|~0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", c) {
					return "", nil, errors.New("invalid header name in ANTHROPIC_CUSTOM_HEADERS")
				}
			}
			for _, c := range value {
				if c < 32 || c == 127 {
					return "", nil, errors.New("invalid control character in ANTHROPIC_CUSTOM_HEADERS")
				}
			}
			lower := strings.ToLower(name)
			credentialName := strings.ReplaceAll(lower, "_", "-")
			if lower == "x-langsmith-auth-mode" {
				if strings.TrimSpace(value) != "oauth" {
					return "", nil, errors.New("X-LangSmith-Auth-Mode must be oauth")
				}
			} else if strings.Contains(credentialName, "auth") || strings.Contains(credentialName, "api-key") || strings.Contains(lower, "apikey") || strings.Contains(lower, "token") || strings.Contains(lower, "cookie") || lower == "host" || lower == "proxy-connection" || lower == "connection" || lower == "content-length" || lower == "transfer-encoding" || lower == "te" || lower == "trailer" || lower == "upgrade" {
				return "", nil, fmt.Errorf("remove conflicting header %q from saved/inherited ANTHROPIC_CUSTOM_HEADERS before rerunning", name)
			}
			if seen[lower] {
				return "", nil, fmt.Errorf("remove duplicate header %q from ANTHROPIC_CUSTOM_HEADERS", name)
			}
			seen[lower] = true
			names = append(names, name)
			if lower == "x-tenant-id" {
				tenant = strings.TrimSpace(value)
				if validateWorkspaceID(tenant) != nil {
					return "", nil, errors.New("X-Tenant-Id in ANTHROPIC_CUSTOM_HEADERS must be a UUID")
				}
				if workspace == "" || !strings.EqualFold(tenant, workspace) {
					return "", nil, errors.New("remove conflicting X-Tenant-Id from ANTHROPIC_CUSTOM_HEADERS or select the same workspace explicitly with --workspace")
				}
			}
		}
	}
	if workspace != "" && tenant == "" {
		if raw != "" && !strings.HasSuffix(raw, "\n") {
			raw += "\n"
		}
		raw += "X-Tenant-Id: " + workspace
		names = append(names, "X-Tenant-Id")
	}
	sort.Strings(names)
	return raw, names, nil
}

// Inspect ordinary Claude settings in increasing precedence: user, shared
// project, local project. Generated values may override lower scopes, but
// credentials/provider switches and config directory conflicts are never ignored.
// Managed settings and arbitrary --settings files cannot be discovered here;
// call that out in help.
func gatewayCheckOtherSettings(target, helper string, updates map[string]string) error {
	userPath, err := claudeSettingsPath("user")
	if err != nil {
		return err
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	effectiveHelper := helper
	effectiveEnv := map[string]string{}
	paths := []string{userPath, filepath.Join(".claude", "settings.json"), filepath.Join(".claude", "settings.local.json")}
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return err
		}
		_, doc, err := gatewayReadSettings(abs)
		if err != nil {
			return err
		}
		env, err := gatewaySettingsEnv(doc)
		if err != nil {
			return err
		}
		if abs == target {
			// Adding the OAuth marker would mask lower-scope headers. With no
			// selected tenant and no saved target header, reject inherited auth
			// or tenant pins rather than silently removing their meaning.
			if _, savedHeaders := env["ANTHROPIC_CUSTOM_HEADERS"]; !savedHeaders && updates["ANTHROPIC_CUSTOM_HEADERS"] == "X-LangSmith-Auth-Mode: oauth" {
				if _, _, err := gatewayMergeHeaders(effectiveEnv["ANTHROPIC_CUSTOM_HEADERS"], ""); err != nil {
					return err
				}
			}
			// Model families are replaced as a set. Omitted generated models
			// delete the target's keys, exposing any lower-scope values again.
			for _, family := range gatewayModelFamilies {
				key := gatewayModelEnv(family)
				if _, selected := updates[key]; !selected {
					delete(env, key)
				}
			}
			// The caller validates the target file. Preserve its saved values
			// too: an explicit empty header string still masks lower scopes,
			// whereas an absent header key inherits them.
			effectiveHelper = helper
			for key, value := range env {
				effectiveEnv[key] = value
			}
			for key, value := range updates {
				effectiveEnv[key] = value
			}
			continue
		}
		if err := gatewayCredentialConflicts(env, func(string) string { return "" }); err != nil {
			return err
		}
		if raw, ok := doc["apiKeyHelper"]; ok {
			var old string
			if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &old) != nil {
				return errors.New("apiKeyHelper in another Claude settings scope must be a string")
			}
			effectiveHelper = old
		}
		for key, value := range env {
			effectiveEnv[key] = value
		}
		if value, present := env["CLAUDE_CONFIG_DIR"]; present && value != os.Getenv("CLAUDE_CONFIG_DIR") {
			return errors.New("remove conflicting env.CLAUDE_CONFIG_DIR from other Claude settings scopes")
		}
	}
	for _, family := range gatewayModelFamilies {
		key := gatewayModelEnv(family)
		if _, selected := updates[key]; !selected && effectiveEnv[key] != "" {
			return fmt.Errorf("remove conflicting env.%s from other Claude settings scopes before restoring native Anthropic defaults; setup only clears model overrides in the target scope", key)
		}
	}
	if effectiveHelper != helper {
		return errors.New("remove conflicting apiKeyHelper from other Claude settings scopes before rerunning")
	}
	keys := make([]string, 0, len(updates))
	for key := range updates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if effectiveEnv[key] != updates[key] {
			return fmt.Errorf("remove conflicting env.%s from other Claude settings scopes before rerunning", key)
		}
	}
	if _, updated := updates["ANTHROPIC_CUSTOM_HEADERS"]; !updated {
		// No generated headers means no selected workspace. Validate the
		// effective inherited headers rather than treating omission as deletion:
		// tenant routing, auth headers, and malformed headers must still fail.
		if _, _, err := gatewayMergeHeaders(effectiveEnv["ANTHROPIC_CUSTOM_HEADERS"], ""); err != nil {
			return err
		}
	}
	return nil
}

func gatewayWriteSettings(path string, original, data []byte, project bool) error {
	if project {
		if err := assertSafeProjectPath(path); err != nil {
			return err
		}
	}
	// Reject leaf symlinks in user scope rather than replacing a dotfile-manager
	// link unexpectedly. User-owned parent directory links remain supported.
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return errors.New("refusing to replace symlinked Claude settings; configure the real settings file explicitly")
	}
	current, _, err := gatewayReadSettings(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, original) {
		return errors.New("Claude settings changed after preview; rerun setup")
	}
	if bytes.Equal(current, data) {
		if info, err := os.Stat(path); err == nil && info.Mode().Perm() == 0o600 {
			return nil
		}
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if project {
		if err := assertSafeProjectPath(path); err != nil {
			return err
		}
	}
	return atomicWriteFile(dir, path, data)
}
