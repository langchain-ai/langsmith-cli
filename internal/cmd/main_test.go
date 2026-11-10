package cmd

import (
	"os"
	"testing"

	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
	"github.com/zalando/go-keyring"
)

// TestMain keeps tests off the developer's real keychain.
func TestMain(m *testing.M) {
	keyring.MockInit()
	_ = os.Unsetenv(lsconfig.SecretStoreEnv)
	os.Exit(m.Run())
}
