package config

import (
	"os"
	"testing"

	"github.com/zalando/go-keyring"
)

// TestMain keeps tests off the developer's real keychain.
func TestMain(m *testing.M) {
	keyring.MockInit()
	_ = os.Unsetenv(SecretStoreEnv)
	os.Exit(m.Run())
}
