package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
	"github.com/langchain-ai/langsmith-cli/internal/mda/deploy"
	"github.com/langchain-ai/langsmith-cli/internal/mda/deployment"
	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
	"github.com/langchain-ai/langsmith-cli/internal/mda/proc"
	"github.com/langchain-ai/langsmith-cli/internal/mda/ui"
)

// maxWaitTimeoutSeconds is the largest --wait-timeout-seconds.
const maxWaitTimeoutSeconds = 3600

var deploymentTypes = map[string]hostapi.DeploymentType{
	"dev":  hostapi.DeploymentTypeDev,
	"prod": hostapi.DeploymentTypeProd,
}

var agentEnvironments = map[string]hostapi.AgentEnvironment{
	"development": hostapi.AgentEnvironmentDevelopment,
	"staging":     hostapi.AgentEnvironmentStaging,
	"production":  hostapi.AgentEnvironmentProduction,
}

var contextStrategies = map[string]deploy.ContextStrategy{
	"overwrite": deploy.ContextStrategyOverwrite,
	"keep-hub":  deploy.ContextStrategyKeepHub,
}

// deployFlags are the flags of `langsmith deploy`.
type deployFlags struct {
	name               string
	agentEnvironment   string
	deploymentType     string
	hostURL            string
	pushTo             stringFlagValue
	noWait             bool
	contextStrategy    string
	yes                bool
	waitTimeoutSeconds uint64
}

func newDeployCmd() *cobra.Command {
	var flags deployFlags
	cmd := &cobra.Command{
		Use:   "deploy [path]",
		Short: "Build and deploy a Managed Deep Agent project to LangSmith",
		Long: `Build a Managed Deep Agent project and deploy it to LangSmith Deployment.

The project (default: the current directory) is a Deep Agent project with an
agent.ts or agent.py entry and its dependencies installed (npm install or
uv sync). Deploy checks that the model provider key and other required
secrets are available, syncs instructions and skills to Context Hub, compiles
the project into .mda/build, uploads it, and waits for the new revision to go
live, then reconciles schedules and sets up channels.

The deployment is named after the agent's name unless --name is given.

Authentication:
  --api-key comes first, then a LANGSMITH_API_KEY set in the shell or in the
  project's .env, then the selected profile's API key or OAuth login. The
  CLI's own credentials are only sent to the LangSmith they are for: when the
  project's LANGSMITH_ENDPOINT names another one (--api-url beats it), deploy
  needs a LANGSMITH_API_KEY or a sign-in there. With no usable credential,
  deploy runs 'langsmith auth login' in a terminal. --workspace (or
  LANGSMITH_WORKSPACE_ID in the shell or .env) selects the workspace.`,
		Example: `  langsmith deploy
  langsmith deploy ./my-agent --name support-agent
  langsmith deploy --no-wait --yes --context-strategy keep-hub`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) == 1 {
				path = args[0]
			}
			return runDeploy(cmd, path, flags)
		},
	}
	cmd.Flags().StringVarP(&flags.name, "name", "n", "", "Deployment name (defaults to the agent's name)")
	cmd.Flags().StringVar(&flags.agentEnvironment, "agent-environment", "", "Agent environment to target: development, staging, or production; the deployment name is used as the agent ID (requires agent mode enabled for the workspace)")
	cmd.Flags().StringVar(&flags.deploymentType, "deployment-type", "dev", "Deployment type when creating: dev or prod")
	cmd.Flags().StringVar(&flags.hostURL, "host-url", "", "Host backend URL override (advanced)")
	_ = cmd.Flags().MarkHidden("host-url")
	cmd.Flags().Var(&flags.pushTo, "push-to", "Build and push the image to `REPOSITORY[:TAG]` in a customer-managed registry, then deploy it (self-hosted LangSmith)")
	cmd.Flags().BoolVar(&flags.noWait, "no-wait", false, "Trigger the build but do not wait for it to finish")
	cmd.Flags().StringVar(&flags.contextStrategy, "context-strategy", "", "Resolve instructions/skills edited in Context Hub without prompting: overwrite or keep-hub")
	cmd.Flags().BoolVarP(&flags.yes, "yes", "y", false, "Overwrite existing same-name resources on a first deploy without asking (does not resolve Context Hub edits; use --context-strategy for those)")
	cmd.Flags().Uint64Var(&flags.waitTimeoutSeconds, "wait-timeout-seconds", 1800, "Maximum seconds to wait for the deployment to finish (1-3600)")
	return cmd
}

// options validates the flags and turns them into deploy options for the
// project at path.
func (f deployFlags) options(path string) (deploy.DeployOptions, error) {
	opts := deploy.DeployOptions{
		Path:        path,
		Name:        f.name,
		WorkspaceID: flagWorkspaceID,
		HostURL:     f.hostURL,
		PushTo:      f.pushTo.value,
		Wait:        !f.noWait,
		Yes:         f.yes,
	}
	deploymentType, err := enumFlag("deployment-type", f.deploymentType, deploymentTypes, []string{"dev", "prod"})
	if err != nil {
		return opts, err
	}
	opts.DeploymentType = deploymentType
	if f.agentEnvironment != "" {
		environment, err := enumFlag("agent-environment", f.agentEnvironment, agentEnvironments, []string{"development", "staging", "production"})
		if err != nil {
			return opts, err
		}
		opts.AgentEnvironment = &environment
	}
	if f.contextStrategy != "" {
		strategy, err := enumFlag("context-strategy", f.contextStrategy, contextStrategies, []string{"overwrite", "keep-hub"})
		if err != nil {
			return opts, err
		}
		opts.ContextStrategy = &strategy
	}
	if f.waitTimeoutSeconds < 1 || f.waitTimeoutSeconds > maxWaitTimeoutSeconds {
		return opts, fmt.Errorf("invalid value %d for --wait-timeout-seconds: must be between 1 and %d", f.waitTimeoutSeconds, maxWaitTimeoutSeconds)
	}
	opts.WaitTimeout = time.Duration(f.waitTimeoutSeconds) * time.Second
	return opts, nil
}

// stringFlagValue is a string flag that tells unset from set to "".
type stringFlagValue struct {
	value *string
}

func (o *stringFlagValue) String() string {
	if o.value == nil {
		return ""
	}
	return *o.value
}

func (o *stringFlagValue) Set(value string) error {
	o.value = &value
	return nil
}

func (o *stringFlagValue) Type() string { return "string" }

// enumFlag looks value up among a flag's choices, listed in order.
func enumFlag[T any](flag, value string, choices map[string]T, order []string) (T, error) {
	choice, ok := choices[value]
	if !ok {
		var zero T
		return zero, fmt.Errorf("invalid value %q for --%s: expected one of %s", value, flag, strings.Join(order, ", "))
	}
	return choice, nil
}

func runDeploy(cmd *cobra.Command, path string, flags deployFlags) error {
	opts, err := flags.options(path)
	if err != nil {
		return err
	}
	errOut := cmd.ErrOrStderr()
	global := currentGlobalFlags()
	cli := deployment.CLI{
		APIURL: global.APIURL,
		APIKey: global.APIKey,
		Options: sync.OnceValues(func() (client.Options, error) {
			return resolveClientOptionsFor(global)
		}),
		Login: func(ctx context.Context, platformAPIURL string) (client.Options, error) {
			return loginAt(ctx, platformAPIURL, errOut)
		},
	}

	ctx, stop := proc.InstallInterrupt(cmd.Context())
	defer stop()
	completion, err := deploy.Run(ctx, cli, opts)
	proc.HandOverIfTerminating()
	switch {
	case err != nil && errs.IsCancellation(err):
		// As in mda, a deploy the user stopped (a declined prompt, Esc,
		// Ctrl-C) did not fail.
		return nil
	case err != nil:
		return err
	case completion == errs.Succeeded, completion == errs.Cancelled:
		return nil
	default:
		// The session already reported how it ended.
		return ReportedError{Err: fmt.Errorf("deploy %s", completion)}
	}
}

// loginAt signs in with the device flow at apiURL, the way `langsmith auth
// login` does, writing the instructions to out, and saves the tokens to the
// selected profile; the returned options authenticate with it. The profile
// becomes the current one only when there is none.
//
// apiURL can come from the project being deployed rather than from the CLI's
// own settings. Signing in anywhere but where `langsmith auth login` would
// opens a page that server names and repoints the profile there, so it is
// confirmed first, naming both; without a terminal to ask on, it fails
// with the `langsmith auth login` command that does it.
func loginAt(ctx context.Context, apiURL string, out io.Writer) (client.Options, error) {
	cfg, err := lsconfig.Load()
	if err != nil {
		return client.Options{}, err
	}
	profileName := loginProfileName(cfg)
	if err := validateProfileName(profileName); err != nil {
		return client.Options{}, err
	}
	apiURL = client.NormalizeURL(apiURL)
	if configured := loginAPIURL(cfg, profileName); !client.SameOrigin(configured, apiURL) {
		if err := confirmLoginElsewhere(ctx, profileName, configured, apiURL); err != nil {
			return client.Options{}, err
		}
	}
	fmt.Fprintf(out, "Signing in to LangSmith at %s\n\n", apiURL)
	oauthMeta, token, err := deviceAuthorization{apiURL: apiURL, out: out}.run(ctx)
	if err != nil {
		return client.Options{}, err
	}
	if cfg.CurrentProfile == "" {
		cfg.CurrentProfile = profileName
	}
	profile, err := saveOAuthLogin(cfg, profileName, apiURL, "", oauthMeta, token)
	if err != nil {
		return client.Options{}, err
	}
	return client.Options{APIURL: apiURL, ProfileName: profileName, WorkspaceID: profile.WorkspaceID}, nil
}

// confirmLoginElsewhere asks before signing profileName in at apiURL instead
// of configured, the LangSmith it is set up for. Declining cancels.
func confirmLoginElsewhere(ctx context.Context, profileName, configured, apiURL string) error {
	confirmed, err := ui.Confirm(fmt.Sprintf(
		"Sign in to LangSmith at %s? Profile '%s' is set up for %s and will point at %s instead.",
		apiURL, profileName, configured, apiURL)).
		InitialValue(false).
		Interact(ctx)
	switch {
	case errors.Is(err, ui.ErrNotInteractive):
		return fmt.Errorf("profile '%s' is set up for %s, not %s; run `langsmith auth login --api-url %s` to sign in there",
			profileName, configured, apiURL, apiURL)
	case err != nil:
		return err
	case !confirmed:
		return errs.ErrCancelled
	default:
		return nil
	}
}
