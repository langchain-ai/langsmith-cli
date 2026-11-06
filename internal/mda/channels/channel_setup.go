// Package channels sets up the channels that live outside the deployment, as
// the mda CLI's `deploy/channel_setup.rs` and the modules under it do: Slack
// through Trigger Server, GitHub through an App in Agent Auth, Linear through
// an OAuth app in Agent Auth. `langsmith deploy` prepares them before
// uploading ([Prepare], [ChannelSetup.Preflight]) and sets them up once the
// revision is live ([ChannelSetup.SetUp]), without knowing which providers
// are involved.
//
// # Authentication
//
// Every LangSmith call (Agent Auth, Trigger Server) goes through
// [hostapi.Client], so it carries the CLI's credential, an API key or an
// OAuth profile's token, attached by the LangSmith SDK; nothing here builds a
// credential header. The hidden `MDA_AGENT_AUTH_URL` override, which points
// only Agent Auth calls at another LangSmith, builds its own client for that
// URL with `MDA_AGENT_AUTH_API_KEY`, which it then requires.
//
// Third parties get no LangSmith credential. GitHub and Slack get no request
// at all, only links the user opens in a browser. Linear's token endpoint is
// asked, with plain `net/http`, whether a client ID and secret the user set
// belong together.
//
// # Cancellation
//
// Ctrl-C cancels the command's context. A wait for the user's action in the
// browser ends with it and the channel is skipped, as the mda CLI does; the
// requests after it fail fast with the cancelled context.
package channels

import (
	"context"
	"fmt"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/mda/deployment"
	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
	"github.com/langchain-ai/langsmith-cli/internal/mda/introspect"
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

// ChannelSetup is the project's channels that need setup, validated.
type ChannelSetup struct {
	trigger *preparedTriggerChannel
	github  *githubChannel
	linear  *linearChannel
}

// SetupTarget is the live deployment the channels are set up for.
type SetupTarget struct {
	Client         *hostapi.Client
	AgentServerURL string
	DeploymentID   string
	AssistantID    string
	ProjectRoot    string
	EnvVars        map[string]string
}

// Prepare validates the channels' configuration. wait says whether deploy
// waits for the revision, which setup needs. projectFiles are the files the
// build carries, relative to projectRoot ([project.Project.Files]).
func Prepare(requirements introspect.ChannelRequirements, wait bool, projectRoot string, projectFiles []string, language project.Language, assistantID string) (*ChannelSetup, error) {
	trigger, err := prepareTriggerChannel(requirements, wait, projectRoot, language, assistantID)
	if err != nil {
		return nil, err
	}
	github, err := prepareGitHub(requirements, wait, assistantID)
	if err != nil {
		return nil, err
	}
	linear, err := prepareLinear(requirements, wait, assistantID, projectRoot, projectFiles)
	if err != nil {
		return nil, err
	}
	return &ChannelSetup{trigger: trigger, github: github, linear: linear}, nil
}

// Preflight fails before anything is created or uploaded when setup could
// not finish, e.g. without a terminal to answer a question it needs.
// existingDeployment looks up the deployment being updated (nil for none,
// as [deployment.FindTargetDeployment] returns it), only when a channel
// needs it.
func (s *ChannelSetup) Preflight(ctx context.Context, client *hostapi.Client, existingDeployment func(context.Context) (map[string]any, error), envVars map[string]string) error {
	if s.github == nil && s.linear == nil && s.trigger == nil {
		return nil
	}
	existing, err := existingDeployment(ctx)
	if err != nil {
		return err
	}
	deploymentID, _ := existing["id"].(string)
	if s.trigger != nil {
		if err := preflightTriggerChannel(ctx, client, s.trigger, deploymentID, envVars); err != nil {
			return err
		}
	}
	if s.github != nil {
		runtimeURL := ""
		if existing != nil {
			runtimeURL = deployment.DeploymentRuntimeURL(existing)
		}
		if err := preflightGitHub(ctx, client, s.github, deploymentID, runtimeURL, envVars); err != nil {
			return err
		}
	}
	if s.linear != nil {
		if err := preflightLinear(ctx, client, s.linear, deploymentID, envVars); err != nil {
			return err
		}
	}
	return nil
}

// NeedsLiveURL reports whether setup needs the live Agent Server URL.
func (s *ChannelSetup) NeedsLiveURL() bool {
	return s.trigger != nil || s.github != nil || s.linear != nil
}

// SetUp sets the channels up for the live deployment. Each channel is set up
// even when another's setup fails; the failures end the deploy together once
// every channel had its turn.
func (s *ChannelSetup) SetUp(ctx context.Context, target SetupTarget) error {
	var failures []channelFailure
	if s.trigger != nil {
		err := registerTriggerChannel(ctx, target.Client, s.trigger, target.AgentServerURL, target.DeploymentID, target.AssistantID, target.EnvVars)
		failures = collectFailure(failures, "Slack", err)
	}
	if s.github != nil {
		err := setUpGitHub(ctx, target.Client, s.github, target.AgentServerURL, target.DeploymentID, target.ProjectRoot, target.EnvVars)
		failures = collectFailure(failures, "GitHub", err)
	}
	if s.linear != nil {
		err := setUpLinear(ctx, target.Client, s.linear, target.AgentServerURL, target.DeploymentID, target.EnvVars)
		failures = collectFailure(failures, "Linear", err)
	}
	return combineFailures(failures)
}

// channelFailure is one channel's failed setup.
type channelFailure struct {
	channel string
	err     error
}

func collectFailure(failures []channelFailure, channel string, err error) []channelFailure {
	if err != nil {
		failures = append(failures, channelFailure{channel: channel, err: err})
	}
	return failures
}

// combineFailures is one channel's failure as it is; several, each under its
// channel's name.
func combineFailures(failures []channelFailure) error {
	switch len(failures) {
	case 0:
		return nil
	case 1:
		return failures[0].err
	default:
		lines := make([]string, 0, len(failures))
		for _, failure := range failures {
			lines = append(lines, fmt.Sprintf("%s: %v", failure.channel, failure.err))
		}
		return fmt.Errorf("The deployment is live, but these channels did not finish setting up:\n%s", strings.Join(lines, "\n"))
	}
}

// ListsEventsURL reports whether deploy lists a channel's events URL for the
// user to configure. Channels set up here point their provider at it
// themselves.
func ListsEventsURL(manifest any) bool {
	transport, _ := stringField(manifest, "transport")
	provider, _ := stringField(manifest, "provider")
	return transport != "trigger_server" && provider != githubProvider && provider != linearProvider
}
