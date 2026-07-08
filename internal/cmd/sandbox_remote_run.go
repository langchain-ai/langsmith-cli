package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

const (
	rrDefaultVCPUs      = 8
	rrDefaultMemory     = "32256mb"
	rrDefaultRootFS     = "64gb"
	rrRemoteUser        = "root"
	rrRemoteSourceDir   = "/root/src"
	rrSandboxNameEnv    = "LANGSMITH_SANDBOX_REMOTE_RUN_NAME"
	rrLegacyNameEnv     = "LANGDEV_RR_SANDBOX"
	rrPrivateRefPrefix  = "refs/langsmith-remote-run/"
	rrSSHControlTimeout = "120s"
)

var rrAlwaysSyncDirs = []string{"third_party"}

type sandboxRemoteRunInput struct {
	Sandbox         string
	NoSync          bool
	NoMise          bool
	Tunnel          bool
	WorktreeSandbox bool
	Identity        string
	SyncSecrets     bool
	ForwardAgent    bool
	SnapshotID      string
	VCPUs           int
	Memory          string
	RootFS          string
}

type rrRepo struct {
	worktree string
	mainName string
	wtName   string
	linked   bool
	origin   string
	email    string
}

type rrSSHConn struct {
	user       string
	host       string
	port       int
	identity   string
	knownHosts string
}

type rrSSHOpts struct {
	agent    bool
	tty      bool
	quiet    bool
	forwards []string
}

func newSandboxRemoteRunCmd() *cobra.Command {
	in := &sandboxRemoteRunInput{VCPUs: rrDefaultVCPUs, Memory: rrDefaultMemory, RootFS: rrDefaultRootFS}
	cmd := &cobra.Command{
		Use:     "remote-run <command> [args...]",
		Aliases: []string{"rr"},
		Short:   "Run a local git worktree inside a mirrored sandbox",
		Long: `Run a command inside a LangSmith sandbox against a mirror of the current git repo.

The command creates or starts a deterministic per-user sandbox by default, syncs
HEAD plus local uncommitted changes into the sandbox, then executes the command
from the mirrored checkout.

Examples:
  langsmith sandbox rr -- make lint
  langsmith sandbox rr --sandbox dev-alice -- pnpm install
  langsmith sandbox rr --tunnel -- make start`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 && !in.Tunnel {
				return fmt.Errorf("requires a command or --tunnel")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSandboxRemoteRun(cmd, in, args)
		},
	}
	cmd.Flags().SetInterspersed(false)
	cmd.Flags().StringVar(&in.Sandbox, "sandbox", in.Sandbox, "Sandbox name (default: dev-<git user.email>; env: LANGSMITH_SANDBOX_REMOTE_RUN_NAME)")
	cmd.Flags().BoolVar(&in.WorktreeSandbox, "worktree-sandbox", in.WorktreeSandbox, "Use a sandbox name derived from the current worktree instead of git user.email")
	cmd.Flags().BoolVar(&in.NoSync, "no-sync", in.NoSync, "Skip pushing and rsyncing the repo; run against the sandbox's existing checkout")
	cmd.Flags().BoolVar(&in.NoMise, "no-mise", in.NoMise, "Run the command directly instead of through mise")
	cmd.Flags().BoolVar(&in.Tunnel, "tunnel", in.Tunnel, "Forward every port in .env.ports.override to localhost and keep running after the command")
	cmd.Flags().BoolVar(&in.ForwardAgent, "forward-agent", in.ForwardAgent, "Forward the local SSH agent into the sandbox (default off; enables SSH-authenticated git from inside the box)")
	cmd.Flags().StringVar(&in.Identity, "identity", in.Identity, "Path to SSH private key (default: auto-detect)")
	cmd.Flags().BoolVar(&in.SyncSecrets, "sync-secrets", in.SyncSecrets, "Also mirror a local secrets/ directory into the sandbox")
	cmd.Flags().StringVar(&in.SnapshotID, "snapshot-id", in.SnapshotID, "Snapshot ID when creating the sandbox")
	cmd.Flags().IntVar(&in.VCPUs, "vcpus", in.VCPUs, "vCPU count when creating the sandbox")
	cmd.Flags().StringVar(&in.Memory, "memory", in.Memory, "Memory when creating the sandbox, with unit (e.g. 8gb)")
	cmd.Flags().StringVar(&in.RootFS, "rootfs-capacity", in.RootFS, "Root filesystem capacity when creating the sandbox, with unit (e.g. 64gb)")
	return cmd
}

func runSandboxRemoteRun(cmd *cobra.Command, in *sandboxRemoteRunInput, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	r, err := inspectRRRepo()
	if err != nil {
		return fmt.Errorf("inspect repo: %w", err)
	}
	name := sandboxRemoteRunName(in, r)

	c, err := cmdutil.GetClient(cmd)
	if err != nil {
		return err
	}
	box, err := ensureRemoteRunSandbox(ctx, c, name, in)
	if err != nil {
		return err
	}
	if box.DataplaneURL == "" {
		return fmt.Errorf("sandbox %q has no dataplane URL", name)
	}

	privKey, pubKey, err := resolveRRSSHKey(in.Identity)
	if err != nil {
		return err
	}
	if err := ensureRemoteRunSSHD(ctx, c, name, pubKey); err != nil {
		return err
	}

	tunnel, err := c.SDK.Sandboxes.Boxes.TunnelWithDataplaneURL(ctx, box.DataplaneURL, 22, langsmith.SandboxTunnelParams{})
	if err != nil {
		return fmt.Errorf("open ssh tunnel to sandbox %q: %w", name, err)
	}
	defer func() { _ = tunnel.Close() }()

	conn := &rrSSHConn{user: rrRemoteUser, host: "127.0.0.1", port: tunnel.LocalPort, identity: privKey}
	if err := ensureRRSSHConn(conn, false, fmt.Sprintf("; check that sshd is running in sandbox %q", name)); err != nil {
		return err
	}

	var forwards []string
	if in.Tunnel {
		forwards, err = rrTunnelForwards(r.worktree)
		if err != nil {
			return err
		}
	}
	return rrSyncAndRun(ctx, r, conn, in, args, forwards)
}

func sandboxRemoteRunName(in *sandboxRemoteRunInput, r *rrRepo) string {
	if in.Sandbox != "" {
		return in.Sandbox
	}
	if env := os.Getenv(rrSandboxNameEnv); env != "" {
		return env
	}
	if env := os.Getenv(rrLegacyNameEnv); env != "" {
		return env
	}
	base := r.email
	if in.WorktreeSandbox {
		base = r.wtName
	}
	name := "dev-" + rrSanitize(base)
	if len(name) > 63 {
		name = name[:63]
	}
	return strings.TrimRight(name, "-")
}

func ensureRemoteRunSandbox(ctx context.Context, c *client.Client, name string, in *sandboxRemoteRunInput) (*langsmith.SandboxBoxGetResponse, error) {
	box, err := c.SDK.Sandboxes.Boxes.Get(ctx, name)
	if isRRNotFound(err) {
		var memBytes, rootBytes int64
		memBytes, err = parseByteSize(in.Memory)
		if err != nil {
			return nil, fmt.Errorf("invalid --memory: %w", err)
		}
		rootBytes, err = parseByteSize(in.RootFS)
		if err != nil {
			return nil, fmt.Errorf("invalid --rootfs-capacity: %w", err)
		}
		fmt.Fprintf(os.Stderr, "sandbox %q not found, creating (%d vCPU / %d MiB / %d GiB rootfs)...\n", name, in.VCPUs, memBytes>>20, rootBytes>>30)
		params := langsmith.SandboxBoxNewParams{
			Name:            langsmith.F(name),
			Vcpus:           langsmith.F(int64(in.VCPUs)),
			MemBytes:        langsmith.F(memBytes),
			FsCapacityBytes: langsmith.F(rootBytes),
		}
		if in.SnapshotID != "" {
			params.SnapshotID = langsmith.F(in.SnapshotID)
		}
		if _, err := c.SDK.Sandboxes.Boxes.New(ctx, params); err != nil {
			return nil, fmt.Errorf("create sandbox %q: %w", name, err)
		}
		if _, err := waitForBoxReady(ctx, c, name); err != nil {
			return nil, fmt.Errorf("wait for new sandbox %q: %w", name, err)
		}
		box, err = c.SDK.Sandboxes.Boxes.Get(ctx, name)
	}
	if err != nil {
		return nil, fmt.Errorf("get sandbox %q: %w", name, err)
	}
	if box.Status == "ready" && box.DataplaneURL != "" {
		return box, nil
	}
	fmt.Fprintf(os.Stderr, "sandbox %q is %s, starting...\n", name, box.Status)
	if _, err := c.SDK.Sandboxes.Boxes.Start(ctx, name); err != nil {
		return nil, fmt.Errorf("start sandbox %q: %w", name, err)
	}
	if _, err := waitForBoxReady(ctx, c, name); err != nil {
		return nil, fmt.Errorf("wait for sandbox %q: %w", name, err)
	}
	box, err = c.SDK.Sandboxes.Boxes.Get(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get sandbox %q after start: %w", name, err)
	}
	return box, nil
}

func isRRNotFound(err error) bool {
	var apiErr *langsmith.Error
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

func ensureRemoteRunSSHD(ctx context.Context, c *client.Client, name string, pubKey []byte) error {
	if err := c.SDK.Sandboxes.Boxes.WriteFile(ctx, name, "/root/.ssh/authorized_keys", pubKey); err != nil {
		return fmt.Errorf("upload ssh public key to sandbox: %w", err)
	}
	res, err := c.SDK.Sandboxes.Boxes.Run(ctx, name, langsmith.SandboxBoxRunParams{
		Command: langsmith.F(rrEnsureSSHDScript),
		Timeout: langsmith.Int(300),
	})
	if err != nil {
		return fmt.Errorf("provision sshd in sandbox: %w", err)
	}
	if res.ExitCode != 0 {
		detail := strings.TrimSpace(res.Stderr)
		if detail == "" {
			detail = strings.TrimSpace(res.Stdout)
		}
		return fmt.Errorf("provision sshd in sandbox failed (exit %d): %s", res.ExitCode, detail)
	}
	return nil
}

func resolveRRSSHKey(explicit string) (string, []byte, error) {
	if explicit != "" {
		priv := explicit
		pub := explicit + ".pub"
		if strings.HasSuffix(explicit, ".pub") {
			pub = explicit
			priv = strings.TrimSuffix(explicit, ".pub")
		}
		if _, err := os.Stat(priv); err != nil {
			return "", nil, fmt.Errorf("private key not found: %s", priv)
		}
		b, err := os.ReadFile(pub)
		if err != nil {
			return "", nil, fmt.Errorf("read public key %s: %w", pub, err)
		}
		return priv, bytes.TrimSpace(b), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", nil, err
	}
	for _, base := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
		priv := filepath.Join(home, ".ssh", base)
		pub := priv + ".pub"
		b, err := os.ReadFile(pub)
		if err != nil {
			continue
		}
		if _, err := os.Stat(priv); err != nil {
			continue
		}
		return priv, bytes.TrimSpace(b), nil
	}
	return "", nil, fmt.Errorf("no SSH keypair in ~/.ssh (tried id_ed25519, id_rsa, id_ecdsa); generate one with: ssh-keygen -t ed25519")
}

func inspectRRRepo() (*rrRepo, error) {
	worktree, err := rrGitOut("", "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	commonDir, err := rrGitOut(worktree, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return nil, err
	}
	mainPath := filepath.Dir(commonDir)
	origin, err := rrGitOut(worktree, "remote", "get-url", "origin")
	if err != nil {
		return nil, err
	}
	email, err := rrGitOut(worktree, "config", "user.email")
	if err != nil || email == "" {
		return nil, fmt.Errorf("git config user.email is not set")
	}
	return &rrRepo{
		worktree: worktree,
		mainName: filepath.Base(mainPath),
		wtName:   filepath.Base(worktree),
		linked:   filepath.Clean(worktree) != filepath.Clean(mainPath),
		origin:   origin,
		email:    email,
	}, nil
}

func (r *rrRepo) refName() string {
	name := r.mainName
	if r.linked {
		name = r.wtName
	}
	return rrPrivateRefPrefix + rrSanitize(name)
}

func (r *rrRepo) changedFiles() ([]string, error) {
	tracked, err := rrGitLines(r.worktree, "diff", "--name-only", "HEAD")
	if err != nil {
		return nil, err
	}
	untracked, err := rrGitLines(r.worktree, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	extra, err := r.alwaysSyncedFiles()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range append(append(tracked, untracked...), extra...) {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		if _, err := os.Lstat(filepath.Join(r.worktree, p)); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *rrRepo) alwaysSyncedFiles() ([]string, error) {
	var out []string
	for _, dir := range rrAlwaysSyncDirs {
		root := filepath.Join(r.worktree, dir)
		if _, err := os.Stat(root); err != nil {
			continue
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			rel, err := filepath.Rel(r.worktree, path)
			if err != nil {
				return err
			}
			out = append(out, rel)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *rrRepo) deletedFiles() ([]string, error) {
	return rrGitLines(r.worktree, "diff", "--name-only", "--diff-filter=D", "HEAD")
}

func rrSyncAndRun(ctx context.Context, r *rrRepo, conn *rrSSHConn, in *sandboxRemoteRunInput, args, forwards []string) error {
	mainPath := rrRemoteSourceDir + "/" + r.mainName
	target := mainPath
	wtName := ""
	if r.linked {
		wtName = r.wtName
		target = rrRemoteSourceDir + "/" + r.wtName
	}
	if !in.NoSync {
		cloneOrigin, gitToken, err := rrCloneOriginAndToken(ctx, r.origin)
		if err != nil {
			return err
		}
		// Pass the git token through the piped script body (stdin), not as a
		// positional argument, so it never lands in the remote process argv
		// (ps, /proc/*/cmdline, or command tracing).
		precloneScript := rrPrecloneScript
		if gitToken != "" {
			precloneScript = "GIT_TOKEN=" + shellQuote(gitToken) + "\n" + rrPrecloneScript
		}
		if err := rrSSHScript(conn, precloneScript, []string{rrRemoteSourceDir, r.mainName, cloneOrigin}, rrSSHOpts{agent: in.ForwardAgent}); err != nil {
			return fmt.Errorf("remote preclone: %w", err)
		}
		if err := rrPushLocalGit(r, conn, mainPath, r.refName()); err != nil {
			return err
		}
		if err := rrSSHScript(conn, rrPostcheckoutScript, []string{mainPath, target, wtName, r.refName()}, rrSSHOpts{quiet: true}); err != nil {
			return fmt.Errorf("remote checkout: %w", err)
		}
		localPorts := filepath.Join(r.worktree, ".env.ports.override")
		if _, err := os.Stat(localPorts); err == nil {
			if err := rrRsyncFile(conn, localPorts, target+"/.env.ports.override"); err != nil {
				return err
			}
		}
		if err := rrSyncChangedFiles(r, conn, target); err != nil {
			return err
		}
		if err := rrSyncDeletions(r, conn, target); err != nil {
			return err
		}
		if in.SyncSecrets {
			if err := rrRsyncDir(conn, filepath.Join(r.worktree, "secrets")+"/", target+"/secrets/", nil); err != nil {
				return err
			}
		}
		depModules := filepath.Join(filepath.Dir(r.worktree), "deployments", "modules") + "/"
		if err := rrRsyncDir(conn, depModules, rrRemoteSourceDir+"/deployments/modules/", nil); err != nil {
			return err
		}
	}
	if len(args) == 0 {
		if len(forwards) > 0 {
			return rrSSHTunnelConn(conn, forwards)
		}
		return nil
	}
	argv := rrRemoteRunArgv(target, !in.NoMise, args)
	return rrSSHExec(conn, argv, rrSSHOpts{agent: in.ForwardAgent, tty: rrStdinIsTTY(), forwards: forwards})
}

func rrCloneOriginAndToken(ctx context.Context, origin string) (string, string, error) {
	cloneOrigin, needsToken, err := rrHTTPSGitURL(origin)
	if err != nil {
		return "", "", err
	}
	if !needsToken {
		return cloneOrigin, "", nil
	}
	token, err := rrGitHubToken(ctx)
	if err != nil {
		return "", "", err
	}
	return cloneOrigin, token, nil
}

func rrGitHubToken(ctx context.Context) (string, error) {
	if token := strings.TrimSpace(os.Getenv("GH_TOKEN")); token != "" {
		return token, nil
	}
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		return token, nil
	}
	ghBin, err := rrBinary("gh")
	if err != nil {
		return "", fmt.Errorf("gh not found; install GitHub CLI or set GH_TOKEN/GITHUB_TOKEN for private repo cloning: %w", err)
	}
	out, err := exec.CommandContext(ctx, ghBin, "auth", "token").Output()
	if err != nil {
		return "", fmt.Errorf("gh auth token (is gh installed and logged in?): %w", err)
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("gh auth token returned empty")
	}
	return token, nil
}

func rrHTTPSGitURL(origin string) (string, bool, error) {
	o := strings.TrimSpace(origin)
	switch {
	case strings.HasPrefix(o, "https://github.com/"):
		return o, true, nil
	case strings.HasPrefix(o, "git@github.com:"):
		return "https://github.com/" + strings.TrimPrefix(o, "git@github.com:"), true, nil
	case strings.HasPrefix(o, "ssh://git@github.com/"):
		return "https://github.com/" + strings.TrimPrefix(o, "ssh://git@github.com/"), true, nil
	case strings.HasPrefix(o, "https://"):
		return o, false, nil
	}
	return "", false, fmt.Errorf("cannot convert origin %q to an HTTPS git URL", origin)
}

func rrPushLocalGit(r *rrRepo, conn *rrSSHConn, mainPath, ref string) error {
	gitBin, err := rrBinary("git")
	if err != nil {
		return err
	}
	url := conn.target() + ":" + mainPath
	cmd := exec.Command(gitBin, "push", "--force", "--no-verify", url, "HEAD:"+ref)
	cmd.Dir = r.worktree
	cmd.Env = append(os.Environ(), "GIT_SSH_COMMAND="+conn.commandString())
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	if err := cmd.Run(); err != nil {
		_, _ = os.Stderr.Write(buf.Bytes())
		return fmt.Errorf("git push to sandbox: %w", err)
	}
	return nil
}

func rrSyncChangedFiles(r *rrRepo, conn *rrSSHConn, target string) error {
	files, err := r.changedFiles()
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	list, err := os.CreateTemp("", "langsmith-remote-run-files-*.txt")
	if err != nil {
		return err
	}
	defer os.Remove(list.Name())
	if _, err := list.WriteString(strings.Join(files, "\n") + "\n"); err != nil {
		_ = list.Close()
		return err
	}
	if err := list.Close(); err != nil {
		return err
	}
	rsyncBin, err := rrBinary("rsync")
	if err != nil {
		return err
	}
	cmd := exec.Command(rsyncBin, "-rlptz", "--files-from="+list.Name(), "-e", conn.commandString(), r.worktree+"/", conn.target()+":"+target+"/")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsync changed files: %w", err)
	}
	return nil
}

func rrSyncDeletions(r *rrRepo, conn *rrSSHConn, target string) error {
	deleted, err := r.deletedFiles()
	if err != nil {
		return err
	}
	if len(deleted) == 0 {
		return nil
	}
	argv := []string{"rm", "-f", "--"}
	for _, f := range deleted {
		argv = append(argv, target+"/"+f)
	}
	return rrSSHExec(conn, argv, rrSSHOpts{})
}

func rrRsyncDir(conn *rrSSHConn, localDir, remotePath string, excludes []string) error {
	if fi, err := os.Stat(localDir); err != nil || !fi.IsDir() {
		return nil
	}
	rsyncBin, err := rrBinary("rsync")
	if err != nil {
		return err
	}
	args := []string{"-rlptz", "--delete"}
	for _, e := range excludes {
		args = append(args, "--exclude="+e)
	}
	args = append(args, "-e", conn.commandString(), localDir, conn.target()+":"+remotePath)
	cmd := exec.Command(rsyncBin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsync %s: %w", localDir, err)
	}
	return nil
}

func rrRsyncFile(conn *rrSSHConn, localPath, remotePath string) error {
	rsyncBin, err := rrBinary("rsync")
	if err != nil {
		return err
	}
	cmd := exec.Command(rsyncBin, "-ptz", "-e", conn.commandString(), localPath, conn.target()+":"+remotePath)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsync %s: %w", localPath, err)
	}
	return nil
}

func rrGitOut(dir string, args ...string) (string, error) {
	gitBin, err := rrBinary("git")
	if err != nil {
		return "", err
	}
	cmd := exec.Command(gitBin, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, detail)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func rrGitLines(dir string, args ...string) ([]string, error) {
	out, err := rrGitOut(dir, args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

func (conn *rrSSHConn) controlArgs() []string {
	home, _ := os.UserHomeDir()
	args := []string{
		"-o", "LogLevel=ERROR",
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=" + filepath.Join(home, ".ssh", "langsmith-rr-cm-%C"),
		"-o", "ControlPersist=" + rrSSHControlTimeout,
	}
	if conn.identity != "" {
		args = append(args, "-i", conn.identity)
	}
	if conn.port != 0 {
		args = append(args, "-p", strconv.Itoa(conn.port))
	}
	if conn.knownHosts != "" {
		args = append(args, "-o", "StrictHostKeyChecking=accept-new", "-o", "UserKnownHostsFile="+conn.knownHosts)
	} else {
		args = append(args, "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null")
	}
	return args
}

func (conn *rrSSHConn) commandString() string {
	sshBin, err := rrBinary("ssh")
	if err != nil {
		sshBin = "ssh"
	}
	return rrShellJoin(append([]string{sshBin}, conn.controlArgs()...))
}

func (conn *rrSSHConn) target() string {
	return conn.user + "@" + conn.host
}

func (conn *rrSSHConn) args(o rrSSHOpts) []string {
	args := conn.controlArgs()
	if o.agent {
		args = append(args, "-A")
	}
	if o.tty {
		args = append(args, "-tt")
	}
	if len(o.forwards) > 0 {
		args = append(args, "-o", "ExitOnForwardFailure=yes")
	}
	for _, f := range o.forwards {
		args = append(args, "-L", f)
	}
	return append(args, conn.target())
}

func rrSSHExec(conn *rrSSHConn, argv []string, o rrSSHOpts) error {
	sshBin, err := rrBinary("ssh")
	if err != nil {
		return err
	}
	cmd := exec.Command(sshBin, append(conn.args(o), rrShellJoin(argv))...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func rrSSHScript(conn *rrSSHConn, script string, scriptArgs []string, o rrSSHOpts) error {
	sshBin, err := rrBinary("ssh")
	if err != nil {
		return err
	}
	remote := "bash -s -- " + rrShellJoin(scriptArgs)
	cmd := exec.Command(sshBin, append(conn.args(o), remote)...)
	cmd.Stdin = strings.NewReader(script)
	if o.quiet {
		var buf bytes.Buffer
		cmd.Stdout, cmd.Stderr = &buf, &buf
		if err := cmd.Run(); err != nil {
			_, _ = os.Stderr.Write(buf.Bytes())
			return err
		}
		return nil
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ensureRRSSHConn(conn *rrSSHConn, wasRunning bool, wedgedHint string) error {
	if wasRunning && rrSSHProbe(conn) == nil {
		return nil
	}
	deadline := time.Now().Add(5 * time.Minute)
	announced := false
	for {
		if rrSSHProbe(conn) == nil {
			return nil
		}
		if time.Now().After(deadline) {
			port := conn.port
			if port == 0 {
				port = 22
			}
			return fmt.Errorf("timed out waiting for ssh as %s (port %d)%s", conn.target(), port, wedgedHint)
		}
		if !announced {
			fmt.Fprintln(os.Stderr, "waiting for ssh...")
			announced = true
		}
		time.Sleep(5 * time.Second)
	}
}

func rrSSHProbe(conn *rrSSHConn) error {
	sshBin, err := rrBinary("ssh")
	if err != nil {
		return err
	}
	args := append(conn.controlArgs(), "-o", "ConnectTimeout=4", conn.target(), "true")
	return exec.Command(sshBin, args...).Run()
}

func rrSSHTunnelConn(conn *rrSSHConn, forwards []string) error {
	sshBin, err := rrBinary("ssh")
	if err != nil {
		return err
	}
	args := []string{"-o", "LogLevel=ERROR", "-o", "ServerAliveInterval=30", "-o", "ControlMaster=no", "-o", "ControlPath=none", "-o", "ExitOnForwardFailure=yes", "-N"}
	if conn.identity != "" {
		args = append(args, "-i", conn.identity)
	}
	if conn.port != 0 {
		args = append(args, "-p", strconv.Itoa(conn.port))
	}
	if conn.knownHosts != "" {
		args = append(args, "-o", "StrictHostKeyChecking=accept-new", "-o", "UserKnownHostsFile="+conn.knownHosts)
	} else {
		args = append(args, "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null")
	}
	for _, f := range forwards {
		args = append(args, "-L", f)
	}
	args = append(args, conn.target())
	fmt.Fprintln(os.Stderr, "tunnel open; Ctrl-C to stop")
	cmd := exec.Command(sshBin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func rrRemoteRunArgv(target string, useMise bool, args []string) []string {
	mise := "0"
	if useMise {
		mise = "1"
	}
	argv := []string{"bash", "-c", rrRemoteRunBody, "bash", target, mise}
	return append(argv, args...)
}

func rrStdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func rrShellJoin(args []string) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = shellQuote(a)
	}
	return strings.Join(parts, " ")
}

func rrSanitize(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			out = append(out, r)
		case r >= 'A' && r <= 'Z':
			out = append(out, r+('a'-'A'))
		default:
			out = append(out, '-')
		}
	}
	return string(out)
}

func rrBinary(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%s resolved to non-absolute path %q", name, path)
	}
	return path, nil
}

var rrPortVarRe = regexp.MustCompile(`(?m)^(?:export\s+)?[A-Z0-9_]*PORT=(\d+)\s*$`)

func rrPortsFromFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ports []string
	seen := map[string]bool{}
	for _, m := range rrPortVarRe.FindAllStringSubmatch(string(data), -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			ports = append(ports, m[1])
		}
	}
	return ports, nil
}

func rrTunnelForwards(worktree string) ([]string, error) {
	path := filepath.Join(worktree, ".env.ports.override")
	if _, err := os.Stat(path); err != nil {
		bashBin, berr := rrBinary("bash")
		if berr != nil {
			return nil, berr
		}
		gen := exec.Command(bashBin, "scripts/generate-ports-env.sh")
		gen.Dir = worktree
		gen.Stdout = os.Stderr
		gen.Stderr = os.Stderr
		if err := gen.Run(); err != nil {
			return nil, fmt.Errorf("generate ports env: %w", err)
		}
	}
	ports, err := rrPortsFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("read ports: %w", err)
	}
	if len(ports) == 0 {
		return nil, fmt.Errorf("no ports found in .env.ports.override")
	}
	var forwards []string
	for _, p := range ports {
		forwards = append(forwards, fmt.Sprintf("%s:127.0.0.1:%s", p, p))
	}
	fmt.Fprintf(os.Stderr, "forwarding %d ports from .env.ports.override to localhost\n", len(ports))
	return forwards, nil
}

const rrRemoteRunBody = `t=$1 use_mise=$2; shift 2
cd "$t" || exit 1
[ -f .env.ports.override ] && . ./.env.ports.override
if [ "$use_mise" = 1 ]; then
  export PATH="$HOME/.local/bin:$PATH" MISE_PIPX_UVX=1
  mise trust . >/dev/null 2>&1 || true
  exec mise exec -- "$@"
fi
exec "$@"`

const rrEnsureSSHDScript = `set -e
mkdir -p /root/.ssh && chmod 700 /root/.ssh
[ -f /root/.ssh/authorized_keys ] && chmod 600 /root/.ssh/authorized_keys
need=""
command -v sshd >/dev/null 2>&1 || need="openssh-server"
command -v rsync >/dev/null 2>&1 || need="$need rsync"
command -v bash >/dev/null 2>&1 || need="$need bash"
if [ -n "$need" ]; then
  if command -v apt-get >/dev/null 2>&1; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq && apt-get install -y -qq $need
  elif command -v apk >/dev/null 2>&1; then
    apk add --no-cache openssh rsync bash
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y openssh-server rsync bash
  elif command -v yum >/dev/null 2>&1; then
    yum install -y openssh-server rsync bash
  fi
fi
command -v sshd >/dev/null 2>&1 || { echo "sshd missing and could not be installed" >&2; exit 1; }
command -v rsync >/dev/null 2>&1 || { echo "rsync missing and could not be installed" >&2; exit 1; }
command -v bash >/dev/null 2>&1 || { echo "bash missing and could not be installed" >&2; exit 1; }
[ -f /etc/ssh/ssh_host_ed25519_key ] || ssh-keygen -A >/dev/null 2>&1 || true
mkdir -p /run/sshd 2>/dev/null || true
if ! ss -tln 2>/dev/null | grep -q ':22 ' && ! netstat -tln 2>/dev/null | grep -q ':22 '; then
  "$(command -v sshd)"
fi`

const rrPrecloneScript = `set -e
SRC_DIR="$1"; MAIN_NAME="$2"; ORIGIN="$3"; GIT_TOKEN="${GIT_TOKEN:-}"
SUDO=""; [ "$(id -u)" -ne 0 ] && SUDO="sudo"
export GIT_SSH_COMMAND='ssh -o StrictHostKeyChecking=accept-new'
pkgs=""
command -v git >/dev/null 2>&1 || pkgs="$pkgs git"
command -v rsync >/dev/null 2>&1 || pkgs="$pkgs rsync"
command -v python3 >/dev/null 2>&1 || pkgs="$pkgs python3"
command -v curl >/dev/null 2>&1 || pkgs="$pkgs curl"
if [ -n "$pkgs" ]; then
  if command -v apt-get >/dev/null 2>&1; then
    $SUDO DEBIAN_FRONTEND=noninteractive apt-get update -qq
    $SUDO DEBIAN_FRONTEND=noninteractive apt-get install -y -qq $pkgs
  elif command -v apk >/dev/null 2>&1; then
    apk add --no-cache $pkgs
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y $pkgs
  elif command -v yum >/dev/null 2>&1; then
    yum install -y $pkgs
  fi
fi
command -v docker >/dev/null 2>&1 || { curl -fsSL https://get.docker.com | $SUDO sh && { [ -n "${USER:-}" ] && $SUDO usermod -aG docker "$USER" || true; }; }
command -v mise >/dev/null 2>&1 || [ -x "$HOME/.local/bin/mise" ] || curl -fsSL https://mise.run | sh
command -v uv >/dev/null 2>&1 || [ -x "$HOME/.local/bin/uv" ] || curl -LsSf https://astral.sh/uv/install.sh | sh
mkdir -p "$SRC_DIR"
MAIN="$SRC_DIR/$MAIN_NAME"
if [ ! -d "$MAIN/.git" ]; then
  if [ -n "$GIT_TOKEN" ]; then
    AUTH=$(printf 'x-access-token:%s' "$GIT_TOKEN" | base64 | tr -d '\n')
    git -c http.extraHeader="Authorization: Basic $AUTH" clone "$ORIGIN" "$MAIN"
  else
    git clone "$ORIGIN" "$MAIN"
  fi
fi`

const rrPostcheckoutScript = `set -e
MAIN="$1"; TARGET="$2"; WT_NAME="$3"; REF="$4"
if [ -n "$WT_NAME" ] && [ ! -e "$TARGET/.git" ]; then
  git -C "$MAIN" worktree add --force --detach "$TARGET" "$REF"
fi
reclaim_git_owner() {
  local me; me="$(id -un):$(id -gn)"
  chown -R "$me" "$TARGET/.git" 2>/dev/null || true
  ( cd "$TARGET" \
      && git ls-files --cached --others --exclude-standard \
      | awk -F/ '{ p=""; for (i=1;i<NF;i++){ p=(i==1?$i:p"/"$i); print p } print }' \
      | sort -u \
      | xargs -d '\n' --no-run-if-empty chown "$me" ) 2>/dev/null || true
}
if ! git -C "$TARGET" reset --hard "$REF"; then
  echo "langsmith remote-run: reset blocked; reclaiming git ownership and retrying" >&2
  reclaim_git_owner
  git -C "$TARGET" reset --hard "$REF"
fi
export PATH="$HOME/.local/bin:$PATH" MISE_PIPX_UVX=1
mise trust "$TARGET" >/dev/null 2>&1 || true
( cd "$TARGET" && mise install ) || echo "langsmith remote-run: mise install reported errors (continuing)"
if [ ! -e "$TARGET/.env.ports.override" ] && [ -x "$TARGET/scripts/generate-ports-env.sh" ]; then
  ( cd "$TARGET" && ./scripts/generate-ports-env.sh ) || echo "langsmith remote-run: generate-ports-env.sh failed (continuing)"
fi`
