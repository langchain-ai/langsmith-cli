package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSandboxRemoteRunName(t *testing.T) {
	t.Setenv(rrSandboxNameEnv, "")
	t.Setenv(rrLegacyNameEnv, "")
	repo := &rrRepo{email: "Alice.Dev+Test@example.com", wtName: "Feature Worktree"}

	require.Equal(t, "dev-alice-dev-test-example-com", sandboxRemoteRunName(&sandboxRemoteRunInput{}, repo))
	require.Equal(t, "custom-box", sandboxRemoteRunName(&sandboxRemoteRunInput{Sandbox: "custom-box"}, repo))
	require.Equal(t, "dev-feature-worktree", sandboxRemoteRunName(&sandboxRemoteRunInput{WorktreeSandbox: true}, repo))

	t.Setenv(rrSandboxNameEnv, "env-box")
	require.Equal(t, "env-box", sandboxRemoteRunName(&sandboxRemoteRunInput{}, repo))
}

func TestRRHTTPSGitURL(t *testing.T) {
	tests := []struct {
		name       string
		origin     string
		wantURL    string
		wantToken  bool
		wantErrMsg string
	}{
		{name: "github ssh", origin: "git@github.com:langchain-ai/langchainplus.git", wantURL: "https://github.com/langchain-ai/langchainplus.git", wantToken: true},
		{name: "github ssh url", origin: "ssh://git@github.com/langchain-ai/langchainplus.git", wantURL: "https://github.com/langchain-ai/langchainplus.git", wantToken: true},
		{name: "github https", origin: "https://github.com/langchain-ai/langchainplus.git", wantURL: "https://github.com/langchain-ai/langchainplus.git", wantToken: true},
		{name: "other https", origin: "https://example.com/repo.git", wantURL: "https://example.com/repo.git", wantToken: false},
		{name: "unsupported", origin: "git@example.com:repo.git", wantErrMsg: "cannot convert origin"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotURL, gotToken, err := rrHTTPSGitURL(tc.origin)
			if tc.wantErrMsg != "" {
				require.ErrorContains(t, err, tc.wantErrMsg)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantURL, gotURL)
			require.Equal(t, tc.wantToken, gotToken)
		})
	}
}

func TestRRRemoteRunArgvQuotesCommandArgs(t *testing.T) {
	argv := rrRemoteRunArgv("/root/src/repo", true, []string{"bash", "-lc", "echo 'hi'"})

	require.Equal(t, []string{"bash", "-c", rrRemoteRunBody, "bash", "/root/src/repo", "1", "bash", "-lc", "echo 'hi'"}, argv)
	joined := rrShellJoin(argv)
	require.Contains(t, joined, "'bash' '-c'")
	require.Contains(t, joined, "'echo '\\''hi'\\'''")
}

func TestRRPortsFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env.ports.override")
	require.NoError(t, os.WriteFile(path, []byte("export API_PORT=8000\nWEB_PORT=3000\nAPI_PORT=8000\nOTHER=value\n"), 0600))

	ports, err := rrPortsFromFile(path)

	require.NoError(t, err)
	require.Equal(t, []string{"8000", "3000"}, ports)
}

func TestSandboxRemoteRunCmdFlags(t *testing.T) {
	cmd := newSandboxRemoteRunCmd()

	require.Equal(t, "remote-run", cmd.Name())
	require.Contains(t, cmd.Aliases, "rr")
	for _, name := range []string{"sandbox", "worktree-sandbox", "no-sync", "no-mise", "tunnel", "identity", "sync-secrets", "snapshot-id", "vcpus", "memory", "rootfs-capacity"} {
		require.NotNil(t, cmd.Flags().Lookup(name), "flag --%s not found", name)
	}
	require.NoError(t, cmd.Args(cmd, []string{"make", "lint"}))
	require.Error(t, cmd.Args(cmd, nil))
}
