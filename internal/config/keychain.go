package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/zalando/go-keyring"
)

const (
	// SecretStoreEnv opts every profile saved with an API key into the OS
	// keychain when set to "keychain", including profiles saved before.
	SecretStoreEnv = "LANGSMITH_SECRET_STORE"
	// SecretStoreKeychain is the profile secret_store value saying its API key
	// lives in the OS keychain (macOS Keychain, Windows Credential Manager, or
	// the Linux Secret Service) rather than in the config file. The LangSmith
	// Go SDK reads the same entry.
	SecretStoreKeychain = "keychain"

	keychainService = "langsmith"
	// keychainTimeout bounds a keychain call, which can hang when the Secret
	// Service has no session bus to answer it.
	keychainTimeout = 3 * time.Second
)

var errKeychainTimeout = errors.New("timed out waiting for the OS keychain")

// keychainSecrets is a profile's keychain entry, shared with the Go SDK.
type keychainSecrets struct {
	APIKey       string `json:"api_key,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// keychainEntries caches each keychain entry as last read or written by this
// process, keyed by account, so loading the config several times reads the
// keychain once and a save that changes nothing writes nothing. Not rewriting
// unchanged tokens matters: the Go SDK may rotate them in the keychain while a
// command runs.
var keychainEntries sync.Map

// KeychainRequested reports whether LANGSMITH_SECRET_STORE asks for the
// keychain.
func KeychainRequested() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(SecretStoreEnv)), SecretStoreKeychain)
}

// keychainAccount names a profile's keychain entry: the profile name for the
// default config file, and the profile name qualified by the config path for
// any other, so profiles of the same name in two config files stay apart. It
// must match the Go SDK's keychainAccount.
func keychainAccount(path, profileName string) string {
	canonical := canonicalConfigPath(path)
	if home, err := os.UserHomeDir(); err == nil && canonical == canonicalConfigPath(filepath.Join(home, ".langsmith", "config.json")) {
		return profileName
	}
	return profileName + "@" + canonical
}

// canonicalConfigPath makes path absolute and resolves symlinks, so every
// alias of one config file names the same keychain entry. A file that does not
// exist yet resolves through its directory, so the name is the same before and
// after the file is first written.
func canonicalConfigPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	if dir, err := filepath.EvalSymlinks(filepath.Dir(abs)); err == nil {
		return filepath.Join(dir, filepath.Base(abs))
	}
	return abs
}

// withKeychainTimeout runs a keychain call, giving up after keychainTimeout.
// An abandoned call finishes in the background; its result is discarded.
func withKeychainTimeout[T any](fn func() (T, error)) (T, error) {
	type result struct {
		value T
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		value, err := fn()
		ch <- result{value, err}
	}()
	timer := time.NewTimer(keychainTimeout)
	defer timer.Stop()
	select {
	case r := <-ch:
		return r.value, r.err
	case <-timer.C:
		var zero T
		return zero, errKeychainTimeout
	}
}

func readKeychainSecrets(account string) (keychainSecrets, error) {
	data, err := withKeychainTimeout(func() (string, error) {
		return keyring.Get(keychainService, account)
	})
	if err != nil {
		return keychainSecrets{}, err
	}
	var secrets keychainSecrets
	if err := json.Unmarshal([]byte(data), &secrets); err != nil {
		return keychainSecrets{}, fmt.Errorf("parsing keychain entry: %w", err)
	}
	return secrets, nil
}

// loadKeychainSecrets returns a profile's keychain entry.
func loadKeychainSecrets(path, profileName string) (keychainSecrets, error) {
	account := keychainAccount(path, profileName)
	if cached, ok := keychainEntries.Load(account); ok {
		return cached.(keychainSecrets), nil
	}
	secrets, err := readKeychainSecrets(account)
	if errors.Is(err, keyring.ErrNotFound) {
		return keychainSecrets{}, fmt.Errorf("profile %q keeps its credentials in the OS keychain, but no entry was found; recreate it with 'langsmith profile create'", profileName)
	}
	if err != nil {
		return keychainSecrets{}, fmt.Errorf("reading profile %q credentials from the OS keychain: %w", profileName, err)
	}
	keychainEntries.Store(account, secrets)
	return secrets, nil
}

// saveKeychainSecrets replaces a profile's keychain entry with secrets. A
// profile that was not loaded from the keychain, such as a new one, overwrites
// whatever an earlier profile of the same name left there.
func saveKeychainSecrets(path, profileName string, secrets keychainSecrets) error {
	account := keychainAccount(path, profileName)
	if cached, ok := keychainEntries.Load(account); ok && cached.(keychainSecrets) == secrets {
		return nil
	}
	data, err := json.Marshal(secrets)
	if err != nil {
		return err
	}
	if _, err := withKeychainTimeout(func() (struct{}, error) {
		return struct{}{}, keyring.Set(keychainService, account, string(data))
	}); err != nil {
		return err
	}
	keychainEntries.Store(account, secrets)
	return nil
}

// DeleteKeychainEntry removes a profile's keychain entry, if it has one.
func DeleteKeychainEntry(path, profileName string) error {
	account := keychainAccount(path, profileName)
	keychainEntries.Delete(account)
	_, err := withKeychainTimeout(func() (struct{}, error) {
		return struct{}{}, keyring.Delete(keychainService, account)
	})
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
