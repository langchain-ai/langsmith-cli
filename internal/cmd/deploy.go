package cmd

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/mda/deploy"
	"github.com/langchain-ai/langsmith-cli/internal/mda/deployment"
	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
	"github.com/langchain-ai/langsmith-cli/internal/mda/proc"
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
	name             string
	agentEnvironment string
	deploymentType   string
	hostURL          string
	pushTo           string
	// pushToSet tells `--push-to ""` from no --push-to.
	pushToSet          bool
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
			flags.pushToSet = cmd.Flags().Changed("push-to")
			return runDeploy(cmd, path, flags)
		},
	}
	cmd.Flags().StringVarP(&flags.name, "name", "n", "", "Deployment name (defaults to the agent's name)")
	cmd.Flags().StringVar(&flags.agentEnvironment, "agent-environment", "", "Agent environment to target: development, staging, or production; the deployment name is used as the agent ID (requires agent mode enabled for the workspace)")
	cmd.Flags().StringVar(&flags.deploymentType, "deployment-type", "dev", "Deployment type when creating: dev or prod")
	cmd.Flags().StringVar(&flags.hostURL, "host-url", "", "Host backend URL override (advanced)")
	_ = cmd.Flags().MarkHidden("host-url")
	cmd.Flags().StringVar(&flags.pushTo, "push-to", "", "Build and push the image to `REPOSITORY[:TAG]` in a customer-managed registry, then deploy it (self-hosted LangSmith)")
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
		Wait:        !f.noWait,
		Yes:         f.yes,
	}
	deploymentType, err := enumFlag("deployment-type", f.deploymentType, deploymentTypes, []string{"dev", "prod"})
	if err != nil {
		return opts, err
	}
	opts.DeploymentType = deploymentType
	if f.pushToSet {
		opts.PushTo = &f.pushTo
	}
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
		return opts, fmt.Errorf("invalid --wait-timeout-seconds %d: must be between 1 and %d", f.waitTimeoutSeconds, maxWaitTimeoutSeconds)
	}
	opts.WaitTimeout = time.Duration(f.waitTimeoutSeconds) * time.Second
	return opts, nil
}

// enumFlag looks value up among a flag's choices, listed in order.
func enumFlag[T any](flag, value string, choices map[string]T, order []string) (T, error) {
	choice, ok := choices[value]
	if !ok {
		var zero T
		return zero, fmt.Errorf("invalid --%s %q: must be one of %s", flag, value, strings.Join(order, ", "))
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
