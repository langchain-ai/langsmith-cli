package cmd

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	"github.com/langchain-ai/langsmith-cli/internal/mda/deploy"
	"github.com/langchain-ai/langsmith-cli/internal/mda/deployment"
	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
	"github.com/langchain-ai/langsmith-cli/internal/mda/proc"
	"github.com/langchain-ai/langsmith-cli/internal/structured"
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

// deployInput is the flags of `langsmith deploy`.
type deployInput struct {
	Name               string
	AgentEnvironment   string
	DeploymentType     string
	HostURL            string
	PushTo             string
	NoWait             bool
	ContextStrategy    string
	Yes                bool
	WaitTimeoutSeconds uint64
}

var deployCommand = structured.Command[*deployInput]{
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
  LANGSMITH_WORKSPACE_ID in the shell or .env) selects the workspace.

Examples:
  langsmith deploy
  langsmith deploy ./my-agent --name support-agent
  langsmith deploy --no-wait --yes --context-strategy keep-hub`,
	Args: cobra.MaximumNArgs(1),
	Input: func(cmd *cobra.Command) *deployInput {
		in := &deployInput{DeploymentType: "dev", WaitTimeoutSeconds: 1800}
		cmd.Flags().StringVarP(&in.Name, "name", "n", in.Name, "Deployment name (defaults to the agent's name)")
		cmd.Flags().StringVar(&in.AgentEnvironment, "agent-environment", in.AgentEnvironment, "Agent environment to target: development, staging, or production; the deployment name is used as the agent ID (requires agent mode enabled for the workspace)")
		cmd.Flags().StringVar(&in.DeploymentType, "deployment-type", in.DeploymentType, "Deployment type when creating: dev or prod")
		cmd.Flags().StringVar(&in.HostURL, "host-url", in.HostURL, "Host backend URL override (advanced)")
		_ = cmd.Flags().MarkHidden("host-url")
		cmd.Flags().StringVar(&in.PushTo, "push-to", in.PushTo, "Build and push the image to `REPOSITORY[:TAG]` in a customer-managed registry, then deploy it (self-hosted LangSmith)")
		cmd.Flags().BoolVar(&in.NoWait, "no-wait", in.NoWait, "Trigger the build but do not wait for it to finish")
		cmd.Flags().StringVar(&in.ContextStrategy, "context-strategy", in.ContextStrategy, "Resolve instructions/skills edited in Context Hub without prompting: overwrite or keep-hub")
		cmd.Flags().BoolVarP(&in.Yes, "yes", "y", in.Yes, "Overwrite existing same-name resources on a first deploy without asking (does not resolve Context Hub edits; use --context-strategy for those)")
		cmd.Flags().Uint64Var(&in.WaitTimeoutSeconds, "wait-timeout-seconds", in.WaitTimeoutSeconds, "Maximum seconds to wait for the deployment to finish (1-3600)")
		return in
	},
	// The deploy session draws its own output.
	CustomOutput: true,
	Action: func(ctx context.Context, cmd *cobra.Command, in *deployInput, args []string) (any, error) {
		path := "."
		if len(args) == 1 {
			path = args[0]
		}
		opts, err := in.options(cmd, path)
		if err != nil {
			return nil, err
		}
		return nil, runDeploy(ctx, cmd, opts)
	},
}

// options validates the flags and turns them into deploy options for the
// project at path.
func (in *deployInput) options(cmd *cobra.Command, path string) (deploy.DeployOptions, error) {
	opts := deploy.DeployOptions{
		Path:        path,
		Name:        in.Name,
		WorkspaceID: cmd.Flag("workspace").Value.String(),
		HostURL:     in.HostURL,
		Wait:        !in.NoWait,
		Yes:         in.Yes,
	}
	deploymentType, err := enumFlag("deployment-type", in.DeploymentType, deploymentTypes, []string{"dev", "prod"})
	if err != nil {
		return opts, err
	}
	opts.DeploymentType = deploymentType
	// `--push-to ""` is given, and refused later on LangSmith cloud.
	if cmd.Flags().Changed("push-to") {
		opts.PushTo = &in.PushTo
	}
	if in.AgentEnvironment != "" {
		environment, err := enumFlag("agent-environment", in.AgentEnvironment, agentEnvironments, []string{"development", "staging", "production"})
		if err != nil {
			return opts, err
		}
		opts.AgentEnvironment = &environment
	}
	if in.ContextStrategy != "" {
		strategy, err := enumFlag("context-strategy", in.ContextStrategy, contextStrategies, []string{"overwrite", "keep-hub"})
		if err != nil {
			return opts, err
		}
		opts.ContextStrategy = &strategy
	}
	if in.WaitTimeoutSeconds < 1 || in.WaitTimeoutSeconds > maxWaitTimeoutSeconds {
		return opts, fmt.Errorf("invalid --wait-timeout-seconds %d: must be between 1 and %d", in.WaitTimeoutSeconds, maxWaitTimeoutSeconds)
	}
	opts.WaitTimeout = time.Duration(in.WaitTimeoutSeconds) * time.Second
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

// runDeploy runs the deploy session with the CLI's global flags, profiles and
// sign-in.
func runDeploy(ctx context.Context, cmd *cobra.Command, opts deploy.DeployOptions) error {
	errOut := cmd.ErrOrStderr()
	cli := deployment.CLI{
		APIURL: cmd.Flag("api-url").Value.String(),
		APIKey: cmd.Flag("api-key").Value.String(),
		Options: sync.OnceValues(func() (client.Options, error) {
			return cmdutil.ResolveClientOptions(cmd)
		}),
		Login: func(ctx context.Context, platformAPIURL string) (client.Options, error) {
			return loginAt(ctx, platformAPIURL, errOut)
		},
	}

	ctx, stop := proc.InstallInterrupt(ctx)
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
