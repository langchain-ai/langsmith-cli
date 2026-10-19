package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	generated "github.com/langchain-ai/langsmith-cli/internal/generated/pkg/cmd"
	"github.com/spf13/cobra"
	"github.com/urfave/cli/v3"
)

func TestGeneratedCommandsAreRegistered(t *testing.T) {
	root := NewRootCmd("dev", "dev")
	for _, resource := range generatedResources() {
		for _, op := range resource.Commands {
			args := append(strings.Split(resource.Name, ":"), op.Name)
			c, _, err := root.Find(args)
			if err != nil || c.Annotations[generatedAnnotation] != resource.Name {
				t.Errorf("langsmith %s does not reach generated %s", strings.Join(args, " "), resource.Name)
			}
		}
	}
}

func TestNoGeneratedOperationIsShadowed(t *testing.T) {
	if shadowed := shadowedGeneratedOperations(NewRootCmd("dev", "dev")); len(shadowed) > 0 {
		t.Errorf("hand-written commands hide these generated operations; add skip: [cli] to them in stainless.yml:\n%s",
			strings.Join(shadowed, "\n"))
	}
}

// handWrittenProjects returns a root with a hand-written `projects` command,
// aliased sessions, that has a `list` subcommand.
func handWrittenProjects() *cobra.Command {
	root := &cobra.Command{Use: "langsmith"}
	projects := &cobra.Command{Use: "projects", Aliases: []string{"project", "sessions"}}
	projects.AddCommand(&cobra.Command{Use: "list", Run: func(*cobra.Command, []string) {}})
	root.AddCommand(projects)
	return root
}

func resource(name string, ops ...string) *cli.Command {
	r := &cli.Command{Name: name, Category: "API RESOURCE"}
	for _, op := range ops {
		r.Commands = append(r.Commands, &cli.Command{Name: op, Usage: op + " things"})
	}
	return r
}

// commandPath resolves args from root and returns the matched command's path
// and the generated resource it runs, if any.
func commandPath(t *testing.T, root *cobra.Command, args ...string) (path, resource string) {
	t.Helper()
	c, rest, err := root.Find(args)
	if err != nil || len(rest) > 0 {
		t.Fatalf("Find(%q) = %v, rest %q", args, err, rest)
	}
	return c.CommandPath(), c.Annotations[generatedAnnotation]
}

func TestMountGeneratedMergesIntoHandWrittenCommand(t *testing.T) {
	root := handWrittenProjects()
	mountGeneratedResources(root, []*cli.Command{resource("sessions", "list", "create")})

	if path, r := commandPath(t, root, "projects", "create"); path != "langsmith projects create" || r != "sessions" {
		t.Errorf("projects create = %q (%q), want the generated sessions create", path, r)
	}
	if _, r := commandPath(t, root, "projects", "list"); r != "" {
		t.Errorf("projects list runs generated %q, want the hand-written command", r)
	}
	if got := len(root.Commands()); got != 1 {
		t.Errorf("root has %d commands, want 1", got)
	}
}

func TestMountGeneratedNestsSubresources(t *testing.T) {
	root := handWrittenProjects()
	mountGeneratedResources(root, []*cli.Command{resource("sessions:insights", "list"), resource("datasets:versions", "diff")})

	for _, args := range [][]string{{"projects", "insights"}, {"project", "insights"}, {"sessions", "insights"}} {
		if path, r := commandPath(t, root, args...); path != "langsmith projects insights" || r != "sessions:insights" {
			t.Errorf("%q = %q (%q), want langsmith projects insights", args, path, r)
		}
	}
	if path, r := commandPath(t, root, "datasets", "versions"); path != "langsmith datasets versions" || r != "datasets:versions" {
		t.Errorf("datasets versions = %q (%q)", path, r)
	}
}

func TestMountGeneratedIgnoresOrder(t *testing.T) {
	queues, items := resource("queues", "list"), resource("queues:items", "list")
	for _, order := range [][]*cli.Command{{queues, items}, {items, queues}} {
		root := &cobra.Command{Use: "langsmith"}
		mountGeneratedResources(root, order)
		if path, r := commandPath(t, root, "queues", "list"); path != "langsmith queues list" || r != "queues" {
			t.Errorf("order %s: queues list = %q (%q)", order[0].Name, path, r)
		}
		if path, r := commandPath(t, root, "queues", "items"); path != "langsmith queues items" || r != "queues:items" {
			t.Errorf("order %s: queues items = %q (%q)", order[0].Name, path, r)
		}
	}
}

func TestMountGeneratedEmptyGroupRejectsUnknownSubcommand(t *testing.T) {
	root := &cobra.Command{Use: "langsmith"}
	mountGeneratedResources(root, []*cli.Command{resource("datasets:versions", "diff")})
	root.SetArgs([]string{"datasets", "versionz"})
	root.SetOut(&bytes.Buffer{})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), `unknown command "versionz"`) {
		t.Errorf("err = %v, want unknown command", err)
	}
}

func TestMountGeneratedKeepsOperationAliases(t *testing.T) {
	root := handWrittenProjects()
	r := resource("sessions", "remove")
	r.Commands[0].Aliases = []string{"rm"}
	mountGeneratedResources(root, []*cli.Command{r})
	if path, _ := commandPath(t, root, "projects", "rm"); path != "langsmith projects remove" {
		t.Errorf("projects rm = %q, want langsmith projects remove", path)
	}
}

func TestMountGeneratedPanicsOnGeneratedClash(t *testing.T) {
	queues, items := resource("queues", "items"), resource("queues:items", "list")
	for _, order := range [][]*cli.Command{{queues, items}, {items, queues}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("order %s: want a panic when the items operation clashes with the items subresource", order[0].Name)
				}
			}()
			mountGeneratedResources(&cobra.Command{Use: "langsmith"}, order)
		}()
	}
}

func TestShadowedGeneratedOperations(t *testing.T) {
	root := handWrittenProjects()
	prev := generated.Command.Commands
	t.Cleanup(func() { generated.Command.Commands = prev })
	sessions := resource("sessions", "list", "create", "remove")
	sessions.Commands[2].Aliases = []string{"list"}
	generated.Command.Commands = []*cli.Command{sessions}
	mountGeneratedResources(root, generated.Command.Commands)

	if got := strings.Join(shadowedGeneratedOperations(root), ","); got != "langsmith projects list,langsmith projects list (alias of remove)" {
		t.Errorf("shadowed = %q, want the list operation and the list alias of remove", got)
	}
}

func TestTranslateGlobalFlags(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantRest  []string
		wantFlags globalFlags
		wantErr   string
	}{
		{
			name:      "extracts global flags",
			args:      []string{"list", "--workspace=ws", "--api-key", "k", "--format", "json"},
			wantRest:  []string{"list", "--format", "json"},
			wantFlags: globalFlags{APIKey: "k", WorkspaceID: "ws"},
		},
		{
			name:      "accepts the generated spellings",
			args:      []string{"--base-url", "https://host", "--tenant-id=ws", "list"},
			wantRest:  []string{"list"},
			wantFlags: globalFlags{APIURL: "https://host", WorkspaceID: "ws"},
		},
		{
			name:      "extracts --profile and --workspace-id",
			args:      []string{"--profile", "dev", "list", "--workspace-id", "ws"},
			wantRest:  []string{"list"},
			wantFlags: globalFlags{Profile: "dev", WorkspaceID: "ws"},
		},
		{
			name:     "passes other flags through",
			args:     []string{"delete", "--webhook-id", "1", "--url=https://example.com"},
			wantRest: []string{"delete", "--webhook-id", "1", "--url=https://example.com"},
		},
		{
			name:    "missing value",
			args:    []string{"list", "--profile"},
			wantErr: "flag needs an argument: --profile",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rest, flags, err := translateGlobalFlags(tt.args)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(rest, " ") != strings.Join(tt.wantRest, " ") {
				t.Errorf("rest = %q, want %q", rest, tt.wantRest)
			}
			if flags != tt.wantFlags {
				t.Errorf("flags = %+v, want %+v", flags, tt.wantFlags)
			}
		})
	}
}

func TestOperationName(t *testing.T) {
	resource := &cli.Command{Commands: []*cli.Command{{Name: "delete"}, {Name: "list"}}}
	valueFlags := generatedValueFlags()
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"operation first", []string{"delete", "--webhook-id", "1"}, "delete"},
		{"after --transform and its value", []string{"--transform", "id", "delete"}, "delete"},
		{"after --transform=value", []string{"--transform=id", "delete"}, "delete"},
		{"after --transform-error", []string{"--transform-error", "x", "delete"}, "delete"},
		{"after --format-error", []string{"--format-error", "json", "delete"}, "delete"},
		{"after --format", []string{"--format", "json", "list"}, "list"},
		{"after a boolean flag", []string{"--debug", "delete"}, "delete"},
		{"after a boolean short flag", []string{"-r", "list"}, "list"},
		{"flag value equal to an operation name", []string{"list", "--url", "delete"}, "list"},
		{"unknown first positional", []string{"other", "delete"}, ""},
		{"no positional", []string{"--help"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := operationName(resource, tt.args, valueFlags); got != tt.want {
				t.Errorf("operationName(%q) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

// generatedRequest is a request a generated command sent to a fake API.
type generatedRequest struct {
	host, method, path, apiKey, tenant, authorization string
}

// fakeGeneratedAPI returns a server that records each request into requests.
func fakeGeneratedAPI(t *testing.T, requests *[]generatedRequest) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*requests = append(*requests, generatedRequest{
			host: r.Host, method: r.Method, path: r.URL.Path,
			apiKey: r.Header.Get("X-Api-Key"), tenant: r.Header.Get("X-Tenant-Id"),
			authorization: r.Header.Get("Authorization"),
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	t.Cleanup(ts.Close)
	return ts
}

// writeGeneratedConfig points the CLI and the SDK at a config file with the
// given profiles.
func writeGeneratedConfig(t *testing.T, config string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
}

// runGeneratedCommand runs `langsmith <args>` in-process and returns the
// command error and the generated tree's exit code.
func runGeneratedCommand(t *testing.T, stdin string, args ...string) (stderr string, exitCode int, err error) {
	t.Helper()
	prevExit := exitGenerated
	exitGenerated = func(code int) { exitCode = code }
	t.Cleanup(func() { exitGenerated = prevExit })

	root := NewRootCmd("dev", "dev")
	var errBuf bytes.Buffer
	root.SetArgs(args)
	root.SetIn(strings.NewReader(stdin))
	root.SetErr(&errBuf)
	captureStdout(t, func() { err = root.Execute() })
	return errBuf.String(), exitCode, err
}

func TestGeneratedDeleteConfirmsAfterGeneratedValueFlags(t *testing.T) {
	isolateGeneratedAuth(t)
	var requests []generatedRequest
	ts := fakeGeneratedAPI(t, &requests)
	for _, transform := range [][]string{{"--transform", "id"}, {"--transform=id"}, {"--format-error", "json"}} {
		t.Run(strings.Join(transform, " "), func(t *testing.T) {
			requests = nil
			args := append([]string{"--api-url", ts.URL, "--api-key", "k", "prompt-webhooks"}, transform...)
			args = append(args, "delete", "--webhook-id", "11111111-1111-1111-1111-111111111111")
			_, _, err := runGeneratedCommand(t, "n\n", args...)
			if err == nil || !strings.Contains(err.Error(), "not confirmed") {
				t.Errorf("err = %v, want the delete to need confirmation", err)
			}
			if len(requests) != 0 {
				t.Errorf("sent %d requests without confirmation", len(requests))
			}
		})
	}
}

func TestGeneratedCommandAuth(t *testing.T) {
	tests := []struct {
		name string
		// config holds the profiles; {{profile}} is the profile server's URL.
		config  string
		env     map[string]string
		args    []string
		wantErr string
		// wantProfileHost says the request goes to the profile server rather
		// than the LANGSMITH_ENDPOINT server.
		wantProfileHost bool
		wantAPIKey      string
		wantTenant      string
		wantAuth        string
	}{
		{
			name:            "--profile ignores LANGSMITH_ENDPOINT",
			config:          `{"current_profile":"other","profiles":{"other":{"api_key":"other-key"},"dev":{"api_url":"{{profile}}","api_key":"dev-key","workspace_id":"dev-ws"}}}`,
			env:             map[string]string{"LANGSMITH_ENDPOINT": "{{endpoint}}"},
			args:            []string{"--profile", "dev"},
			wantProfileHost: true,
			wantAPIKey:      "dev-key",
			wantTenant:      "dev-ws",
		},
		{
			name:            "--profile with OAuth sends its token only to the profile host",
			config:          `{"profiles":{"dev":{"api_url":"{{profile}}","oauth":{"access_token":"dev-token","expires_at":"2999-01-01T00:00:00Z"}}}}`,
			env:             map[string]string{"LANGSMITH_ENDPOINT": "{{endpoint}}"},
			args:            []string{"--profile", "dev"},
			wantProfileHost: true,
			wantAuth:        "Bearer dev-token",
		},
		{
			name:    "missing --profile fails",
			config:  `{"profiles":{"dev":{"api_url":"{{profile}}","api_key":"dev-key"}}}`,
			env:     map[string]string{"LANGSMITH_API_KEY": "env-key", "LANGSMITH_ENDPOINT": "{{endpoint}}"},
			args:    []string{"--profile", "typo"},
			wantErr: "profile not found: typo",
		},
		{
			name:       "LANGSMITH_WORKSPACE_ID wins over LANGSMITH_TENANT_ID",
			env:        map[string]string{"LANGSMITH_API_KEY": "env-key", "LANGSMITH_ENDPOINT": "{{endpoint}}", "LANGSMITH_TENANT_ID": "tenant-env", "LANGSMITH_WORKSPACE_ID": "workspace-env"},
			wantAPIKey: "env-key",
			wantTenant: "workspace-env",
		},
		{
			name:       "--workspace wins over the environment",
			env:        map[string]string{"LANGSMITH_API_KEY": "env-key", "LANGSMITH_ENDPOINT": "{{endpoint}}", "LANGSMITH_TENANT_ID": "tenant-env", "LANGSMITH_WORKSPACE_ID": "workspace-env"},
			args:       []string{"--workspace", "flag-ws"},
			wantAPIKey: "env-key",
			wantTenant: "flag-ws",
		},
		{
			name:    "no credentials",
			env:     map[string]string{"LANGSMITH_ENDPOINT": "{{endpoint}}"},
			wantErr: "not authenticated",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateGeneratedAuth(t)
			var profileRequests, endpointRequests []generatedRequest
			profileServer := fakeGeneratedAPI(t, &profileRequests)
			endpointServer := fakeGeneratedAPI(t, &endpointRequests)
			expand := strings.NewReplacer("{{profile}}", profileServer.URL, "{{endpoint}}", endpointServer.URL).Replace
			config := tt.config
			if config == "" {
				config = `{}`
			}
			writeGeneratedConfig(t, expand(config))
			for key, value := range tt.env {
				t.Setenv(key, expand(value))
			}

			args := append(append([]string{}, tt.args...), "prompt-webhooks", "list")
			stderr, exitCode, err := runGeneratedCommand(t, "", args...)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tt.wantErr)
				}
				if n := len(profileRequests) + len(endpointRequests); n != 0 {
					t.Errorf("sent %d requests, want none", n)
				}
				return
			}
			if err != nil || exitCode != 0 {
				t.Fatalf("err = %v, exit code %d, stderr: %s", err, exitCode, stderr)
			}
			got, other := endpointRequests, profileRequests
			if tt.wantProfileHost {
				got, other = profileRequests, endpointRequests
			}
			if len(got) != 1 || len(other) != 0 {
				t.Fatalf("expected server got %d requests, the other %d; want 1 and 0", len(got), len(other))
			}
			if got[0].apiKey != tt.wantAPIKey {
				t.Errorf("X-Api-Key = %q, want %q", got[0].apiKey, tt.wantAPIKey)
			}
			if got[0].authorization != tt.wantAuth {
				t.Errorf("Authorization = %q, want %q", got[0].authorization, tt.wantAuth)
			}
			if got[0].tenant != tt.wantTenant {
				t.Errorf("X-Tenant-Id = %q, want %q", got[0].tenant, tt.wantTenant)
			}
		})
	}
}

func TestGeneratedDeletePromptRedactsAPIKey(t *testing.T) {
	isolateGeneratedAuth(t)
	var requests []generatedRequest
	ts := fakeGeneratedAPI(t, &requests)
	const secret = "secret-api-key-value"
	for _, keyArgs := range [][]string{{"--api-key", secret}, {"--api-key=" + secret}} {
		t.Run(keyArgs[0], func(t *testing.T) {
			args := append([]string{"--api-url", ts.URL}, keyArgs...)
			args = append(args, "prompt-webhooks", "delete", "--webhook-id", "11111111-1111-1111-1111-111111111111")
			stderr, _, err := runGeneratedCommand(t, "n\n", args...)
			if err == nil || !strings.Contains(err.Error(), "not confirmed") {
				t.Fatalf("err = %v, want the delete prompt", err)
			}
			if !strings.Contains(stderr, "Command: langsmith prompt-webhooks") {
				t.Errorf("stderr = %q, want the command echoed", stderr)
			}
			if strings.Contains(stderr, secret) || strings.Contains(err.Error(), secret) {
				t.Errorf("prompt output leaks the API key: %q", stderr)
			}
		})
	}
}
