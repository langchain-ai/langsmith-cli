package channels

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
	"github.com/langchain-ai/langsmith-cli/internal/mda/introspect"
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

func prepareTrigger(requirements introspect.ChannelRequirements, wait bool) (*preparedTriggerChannel, error) {
	return prepareTriggerChannel(requirements, wait, ".", project.TypeScript, "assistant-id")
}

func pngIcon(width, height uint32) []byte {
	icon := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	icon = binary.BigEndian.AppendUint32(icon, width)
	icon = binary.BigEndian.AppendUint32(icon, height)
	return append(icon, make([]byte, 33-len(icon))...)
}

func slackRequirements(t *testing.T) introspect.ChannelRequirements {
	return introspect.ChannelRequirements{"slack": jsonObject(t, `{
		"name": "slack",
		"provider": "slack",
		"transport": "trigger_server",
		"autoReply": true,
		"requiredPermissions": [],
		"requiredEnv": [],
		"providerConfig": {
			"app": {
				"name": "Support Bot",
				"description": "Answers support questions",
				"backgroundColor": "#a1b2c3"
			},
			"triggerOnAllMessages": false,
			"allowBotTriggers": false
		}
	}`)}
}

// slackManifestOf is the requirements' Slack manifest, to change in a test.
func slackManifestOf(requirements introspect.ChannelRequirements) map[string]any {
	return requirements["slack"].(map[string]any)
}

func slackProviderConfigOf(requirements introspect.ChannelRequirements) map[string]any {
	return slackManifestOf(requirements)["providerConfig"].(map[string]any)
}

func directSlackManifest(t *testing.T, name string) map[string]any {
	manifest := jsonObject(t, `{
		"provider": "slack",
		"autoReply": true,
		"requiredPermissions": ["app_mentions:read", "chat:write"],
		"requiredEnv": ["SLACK_SIGNING_SECRET", "SLACK_BOT_TOKEN"],
		"providerConfig": {
			"mentionBehavior": "strip",
			"filters": {"allowSharedConversations": false},
			"conversation": {"appMention": "thread", "directMessage": "conversation"}
		}
	}`)
	manifest["name"] = name
	return manifest
}

func activeSetupResponse() hostapi.ChannelSetupResponse {
	return hostapi.ChannelSetupResponse{Status: hostapi.ChannelSetupStatusActive, ChannelID: "a6190f67-865d-4705-8171-35c90e64c730"}
}

func authorizationSetupResponse() hostapi.ChannelSetupResponse {
	return hostapi.ChannelSetupResponse{Status: hostapi.ChannelSetupStatusAuthorizationRequired, AuthorizationURL: "https://trigger.example/authorize"}
}

func pendingApprovalSetupResponse() hostapi.ChannelSetupResponse {
	return hostapi.ChannelSetupResponse{Status: hostapi.ChannelSetupStatusPendingApproval}
}

const holder = "b809e66e-0000-4000-8000-000000000002"

func inUse() error {
	return &hostapi.SlackAppInUse{DeploymentID: holder}
}

func TestASlackAppAnotherDeploymentUsesMovesOnceConfirmed(t *testing.T) {
	var moves []string
	asked := ""

	err := registerCustomerApp(true,
		func(moveFrom string) (hostapi.ChannelSetupResponse, error) {
			moves = append(moves, moveFrom)
			if moveFrom == "" {
				return hostapi.ChannelSetupResponse{}, inUse()
			}
			return activeSetupResponse(), nil
		},
		func(deploymentID string) (bool, error) {
			asked = deploymentID
			return true, nil
		})

	require.NoError(t, err)
	assert.Equal(t, holder, asked)
	assert.Equal(t, []string{"", holder}, moves)
}

func TestADeclinedSlackAppMoveLeavesItWhereItIs(t *testing.T) {
	calls := 0

	err := registerCustomerApp(true,
		func(string) (hostapi.ChannelSetupResponse, error) {
			calls++
			return hostapi.ChannelSetupResponse{}, inUse()
		},
		func(string) (bool, error) { return false, nil })

	assert.Equal(t, 1, calls)
	require.EqualError(t, err, "Slack app stays connected to deployment "+holder+". Use another Slack app for this deployment.")
}

func TestUnattendedDeploysNeverMoveASlackApp(t *testing.T) {
	calls := 0

	err := registerCustomerApp(false,
		func(string) (hostapi.ChannelSetupResponse, error) {
			calls++
			return hostapi.ChannelSetupResponse{}, inUse()
		},
		func(string) (bool, error) {
			t.Fatal("an unattended deploy must not ask")
			return false, nil
		})

	assert.Equal(t, 1, calls)
	require.ErrorContains(t, err, holder)
}

func TestOtherCustomerAppSetupFailuresAreNotOfferedAMove(t *testing.T) {
	err := registerCustomerApp(true,
		func(string) (hostapi.ChannelSetupResponse, error) {
			return hostapi.ChannelSetupResponse{}, errors.New("Channel setup failed (HTTP 409): This Slack app is already in use.")
		},
		func(string) (bool, error) {
			t.Fatal("only an app another deployment uses is moved")
			return false, nil
		})

	require.ErrorContains(t, err, "already in use")
}

func TestUnattendedDeploysWithoutAnAppGoOnOnlyForARegisteredChannel(t *testing.T) {
	noApp := func() (string, bool) { return slackUnattendedMessage(true), true }
	canCreate := func() (string, bool) { return "", false }
	failed := func() *registrationLookup {
		return &registrationLookup{err: errors.New("Channel listing failed (HTTP 503).")}
	}

	require.NoError(t, unattendedOutcome(&registrationLookup{registered: true}, noApp))
	require.EqualError(t, unattendedOutcome(&registrationLookup{}, noApp), slackUnattendedMessage(true))
	require.EqualError(t, unattendedOutcome(nil, noApp), slackUnattendedMessage(true))
	// A failed listing names itself, not a missing Slack app.
	err := unattendedOutcome(failed(), noApp)
	require.ErrorContains(t, err, "Channel listing failed (HTTP 503).")
	require.ErrorContains(t, err, "Could not check")
	// LangSmith can create the app, so nothing stops the deploy.
	require.NoError(t, unattendedOutcome(failed(), canCreate))
	require.NoError(t, unattendedOutcome(&registrationLookup{}, canCreate))
}

func TestPrepareAcceptsTheTriggerSlackManifest(t *testing.T) {
	prepared, err := prepareTrigger(slackRequirements(t), true)

	require.NoError(t, err)
	assert.Equal(t, "slack", prepared.name)
	assert.False(t, prepared.triggerOnAllMessages)
	assert.Nil(t, prepared.triggerMode)
	assert.False(t, prepared.allowBotTriggers)
}

func TestPrepareSendsTriggerModeOnlyWhenItIsSet(t *testing.T) {
	for _, tc := range []struct{ providerConfig, options string }{
		{`{"triggerOnAllMessages": false}`, `{"triggerOnAllMessages": false, "allowBotTriggers": false}`},
		{`{"triggerOnAllMessages": true}`, `{"triggerOnAllMessages": true, "allowBotTriggers": false}`},
		{`{"triggerOnAllMessages": false, "triggerMode": "thread"}`, `{"triggerOnAllMessages": false, "triggerMode": "thread", "allowBotTriggers": false}`},
		{`{"triggerOnAllMessages": false, "triggerMode": "mention"}`, `{"triggerOnAllMessages": false, "triggerMode": "mention", "allowBotTriggers": false}`},
		{`{"triggerOnAllMessages": true, "triggerMode": "all"}`, `{"triggerOnAllMessages": true, "allowBotTriggers": false}`},
	} {
		requirements := slackRequirements(t)
		for key, value := range jsonObject(t, tc.providerConfig) {
			slackProviderConfigOf(requirements)[key] = value
		}
		prepared, err := prepareTrigger(requirements, true)
		require.NoError(t, err, tc.providerConfig)

		options, err := json.Marshal(slackSetup(prepared).Options)
		require.NoError(t, err)
		assert.JSONEq(t, tc.options, string(options), tc.providerConfig)
	}
}

func TestPrepareRejectsALegacyFlagThatConflictsWithTriggerMode(t *testing.T) {
	requirements := slackRequirements(t)
	slackProviderConfigOf(requirements)["triggerOnAllMessages"] = true
	slackProviderConfigOf(requirements)["triggerMode"] = "thread"
	_, err := prepareTrigger(requirements, true)
	require.EqualError(t, err, "Slack channel configuration has a triggerOnAllMessages that conflicts with triggerMode; set only triggerMode.")

	requirements = slackRequirements(t)
	slackProviderConfigOf(requirements)["triggerMode"] = "all"
	_, err = prepareTrigger(requirements, true)
	require.Error(t, err)

	requirements = slackRequirements(t)
	slackProviderConfigOf(requirements)["triggerMode"] = "channel"
	_, err = prepareTrigger(requirements, true)
	require.ErrorContains(t, err, "Slack channel configuration is invalid.: ")
}

func TestPrepareDefaultsMissingAppMetadataFromTheAssistantID(t *testing.T) {
	requirements := slackRequirements(t)
	slackProviderConfigOf(requirements)["app"] = map[string]any{}

	prepared, err := prepareTriggerChannel(requirements, true, ".", project.TypeScript, "abcdefghijklmnopqrstuvwxyzabcdefghij")
	require.NoError(t, err)
	expectedName := "abcdefghijklmnopqrstuvwxyzabcdefghi"
	generated, err := generateIcon(expectedName)
	require.NoError(t, err)

	assert.Equal(t, expectedName, prepared.app.name)
	assert.Empty(t, prepared.app.description)
	assert.Equal(t, generated.png, prepared.app.icon)
}

func TestPrepareAcceptsACustomIconPath(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "channels", "assets"), 0o755))
	icon := pngIcon(512, 512)
	require.NoError(t, os.WriteFile(filepath.Join(root, "channels", "assets", "support.png"), icon, 0o644))
	requirements := slackRequirements(t)
	slackProviderConfigOf(requirements)["app"].(map[string]any)["icon"] = "assets/support.png"

	prepared, err := prepareTriggerChannel(requirements, true, root, project.TypeScript, "assistant-id")

	require.NoError(t, err)
	assert.Equal(t, icon, prepared.app.icon)
	assert.Equal(t, "#a1b2c3", prepared.app.backgroundColor)
}

func TestResolveSlackIconDerivesImageAndBackgroundWhenIconIsOmitted(t *testing.T) {
	icon, err := resolveSlackIcon(t.TempDir(), "Support Bot", nil, nil)
	require.NoError(t, err)
	generated, err := generateIcon("Support Bot")
	require.NoError(t, err)

	assert.Equal(t, generated.png, icon.image)
	assert.Equal(t, generated.backgroundColor, icon.backgroundColor)
}

func TestResolveSlackIconKeepsAuthoredBackgroundWhenIconIsOmitted(t *testing.T) {
	for _, background := range []string{"#a1b2c3", defaultSlackBackgroundColor} {
		icon, err := resolveSlackIcon(t.TempDir(), "Support Bot", nil, &background)
		require.NoError(t, err)
		generated, err := generateIcon("Support Bot")
		require.NoError(t, err)

		assert.Equal(t, generated.png, icon.image)
		assert.Equal(t, background, icon.backgroundColor)
		assert.NotEqual(t, generated.backgroundColor, icon.backgroundColor)
	}
}

func TestReadSlackIconRejectsAnAbsolutePath(t *testing.T) {
	icon := filepath.Join(t.TempDir(), "icon.png")
	require.NoError(t, os.WriteFile(icon, pngIcon(512, 512), 0o644))

	_, err := readSlackIcon(t.TempDir(), icon)

	require.EqualError(t, err, "Slack channel icon must be a path relative to channels/")
}

func TestReadSlackIconTakesAnyRelativePathInsideChannels(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "channels", "assets"), 0o755))
	icon := pngIcon(512, 512)
	require.NoError(t, os.WriteFile(filepath.Join(root, "channels", "assets", "icon.png"), icon, 0o644))

	// Paths Slack's icon has always accepted, unlike Linear's plain paths.
	for _, path := range []string{"./assets/icon.png", "assets/./icon.png", "assets//icon.png"} {
		image, err := readSlackIcon(root, path)
		require.NoError(t, err, path)
		assert.Equal(t, icon, image, path)
	}
}

func TestReadSlackIconRejectsParentTraversal(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "channels"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "outside.png"), []byte("outside"), 0o644))

	_, err := readSlackIcon(root, "../outside.png")

	require.EqualError(t, err, "Slack channel icon must be a path relative to channels/")
}

func TestReadSlackIconRejectsADirectory(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "channels", "assets"), 0o755))

	_, err := readSlackIcon(root, "assets")

	require.EqualError(t, err, "Slack channel icon must be a regular file")
}

func TestPrepareRejectsAJPEGIcon(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "channels"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "channels", "icon.jpg"), []byte("\xff\xd8\xff"), 0o644))
	requirements := slackRequirements(t)
	slackProviderConfigOf(requirements)["app"].(map[string]any)["icon"] = "icon.jpg"

	_, err := prepareTriggerChannel(requirements, true, root, project.TypeScript, "assistant-id")

	require.EqualError(t, err, "Slack channel icon must be a valid PNG")
}

func TestReadSlackIconRejectsIncorrectDimensions(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "channels"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "channels", "icon.png"), pngIcon(256, 512), 0o644))

	_, err := readSlackIcon(root, "icon.png")

	require.EqualError(t, err, "Slack channel icon must be 512x512px")
}

func TestReadSlackIconRejectsAnOversizedPNG(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "channels"), 0o755))
	icon := append(pngIcon(512, 512), make([]byte, 1_000_001-33)...)
	require.NoError(t, os.WriteFile(filepath.Join(root, "channels", "icon.png"), icon, 0o644))

	_, err := readSlackIcon(root, "icon.png")

	require.EqualError(t, err, "Slack channel icon must be at most 1 MB")
}

func TestPrepareIgnoresChannelsWithoutTheTriggerTransport(t *testing.T) {
	withoutTransport := slackRequirements(t)
	delete(slackManifestOf(withoutTransport), "transport")
	direct := slackRequirements(t)
	slackManifestOf(direct)["transport"] = "direct"

	for name, requirements := range map[string]introspect.ChannelRequirements{
		"direct manifest":   {"slack": directSlackManifest(t, "slack")},
		"without transport": withoutTransport,
		"direct transport":  direct,
	} {
		prepared, err := prepareTrigger(requirements, false)
		require.NoError(t, err, name)
		assert.Nil(t, prepared, name)
	}
}

func TestPrepareRejectsANonStringTransport(t *testing.T) {
	requirements := slackRequirements(t)
	slackManifestOf(requirements)["transport"] = 42

	_, err := prepareTrigger(requirements, true)

	require.ErrorContains(t, err, "channel requirements must include valid name, provider, and optional transport: ")
}

func TestPrepareSelectsTheTriggerManifestAmongOtherChannels(t *testing.T) {
	requirements := slackRequirements(t)
	requirements["internal"] = directSlackManifest(t, "internal")

	prepared, err := prepareTrigger(requirements, true)

	require.NoError(t, err)
	assert.Equal(t, "slack", prepared.name)
}

func TestPrepareRejectsNoWaitForATriggerChannel(t *testing.T) {
	_, err := prepareTrigger(slackRequirements(t), false)

	require.EqualError(t, err, "An agent with a Slack channel cannot be deployed with --no-wait.")
}

func TestPrepareRejectsATriggerChannelWithANonSlackProvider(t *testing.T) {
	requirements := introspect.ChannelRequirements{"teams": jsonObject(t, `{
		"name": "teams",
		"provider": "teams",
		"transport": "trigger_server",
		"providerConfig": {"tenantId": "tenant-id"}
	}`)}

	_, err := prepareTrigger(requirements, true)

	require.EqualError(t, err, "Deploy currently supports only Slack channels.")
}

func TestPrepareAcceptsATriggerChannelThatLeavesReplyingToTheAgent(t *testing.T) {
	requirements := slackRequirements(t)
	slackManifestOf(requirements)["autoReply"] = false

	prepared, err := prepareTrigger(requirements, true)

	require.NoError(t, err)
	assert.Equal(t, "slack", prepared.name)
}

func TestPrepareRejectsATriggerChannelWithRequiredPermissionsOrEnv(t *testing.T) {
	requirements := slackRequirements(t)
	slackManifestOf(requirements)["requiredPermissions"] = []any{"chat:write"}
	_, err := prepareTrigger(requirements, true)
	require.EqualError(t, err, "Slack channel configuration requires requiredPermissions=[].")

	requirements = slackRequirements(t)
	slackManifestOf(requirements)["requiredEnv"] = []any{"SLACK_BOT_TOKEN"}
	_, err = prepareTriggerChannel(requirements, true, ".", project.Python, "assistant-id")
	require.EqualError(t, err, "Slack channel configuration requires required_env=[].")
}

func TestPrepareAcceptsANoncanonicalChannelName(t *testing.T) {
	manifest := slackManifestOf(slackRequirements(t))
	manifest["name"] = "support"

	prepared, err := prepareTrigger(introspect.ChannelRequirements{"support": manifest}, true)

	require.NoError(t, err)
	assert.Equal(t, "support", prepared.name)
}

func TestPrepareRejectsAChannelNameThatDiffersFromItsMapKey(t *testing.T) {
	manifest := slackManifestOf(slackRequirements(t))
	manifest["name"] = "support"

	_, err := prepareTrigger(introspect.ChannelRequirements{"help": manifest}, true)

	require.EqualError(t, err, "Channel name must match its manifest key.")
}

func TestPrepareRejectsMultipleTriggerChannels(t *testing.T) {
	support := slackManifestOf(slackRequirements(t))
	support["name"] = "support"
	requirements := slackRequirements(t)
	requirements["support"] = support

	_, err := prepareTrigger(requirements, true)

	require.EqualError(t, err, "Deploy currently supports one channel per agent.")
}

func TestSetupRequestMatchesTheTriggerServerContract(t *testing.T) {
	prepared, err := prepareTrigger(slackRequirements(t), true)
	require.NoError(t, err)

	request, err := buildSetupRequest(prepared, "https://agent.example/base", "deployment-id", "assistant-id")
	require.NoError(t, err)
	encoded, err := json.Marshal(request)
	require.NoError(t, err)
	value := jsonObject(t, string(encoded))
	app := value["slack"].(map[string]any)["app"].(map[string]any)
	image := app["image"].(string)
	app["image"] = "<embedded png>"

	assert.Equal(t, jsonObject(t, `{
		"provider": "slack",
		"target": {
			"deployment_id": "deployment-id",
			"endpoint_url": "https://agent.example/base/v1/channels/slack/events",
			"channel_name": "slack",
			"agent_id": "assistant-id"
		},
		"slack": {
			"app": {
				"name": "Support Bot",
				"description": "Answers support questions",
				"image": "<embedded png>",
				"background_color": "#a1b2c3"
			},
			"options": {"triggerOnAllMessages": false, "allowBotTriggers": false}
		}
	}`), value)
	assert.True(t, strings.HasPrefix(image, "data:image/png;base64,"), image)
}

func TestSetupRequestEmbedsThePreparedCustomIcon(t *testing.T) {
	prepared, err := prepareTrigger(slackRequirements(t), true)
	require.NoError(t, err)
	prepared.app.icon = []byte("custom-icon")

	request, err := buildSetupRequest(prepared, "https://agent.example", "deployment-id", "assistant-id")

	require.NoError(t, err)
	assert.Equal(t, "data:image/png;base64,Y3VzdG9tLWljb24=", request.Slack.App.Image)
}

func TestSetupRequestDerivesTheIconAndBackgroundFromTheAppName(t *testing.T) {
	supportRequirements := slackRequirements(t)
	delete(slackProviderConfigOf(supportRequirements)["app"].(map[string]any), "backgroundColor")
	support, err := prepareTrigger(supportRequirements, true)
	require.NoError(t, err)
	billingRequirements := slackRequirements(t)
	billingApp := slackProviderConfigOf(billingRequirements)["app"].(map[string]any)
	delete(billingApp, "backgroundColor")
	billingApp["name"] = "Billing Bot"
	billing, err := prepareTrigger(billingRequirements, true)
	require.NoError(t, err)

	supportRequest, err := buildSetupRequest(support, "https://agent.example", "deployment-id", "assistant-id")
	require.NoError(t, err)
	billingRequest, err := buildSetupRequest(billing, "https://agent.example", "deployment-id", "assistant-id")
	require.NoError(t, err)

	assert.NotEqual(t,
		[]string{supportRequest.Slack.App.Image, supportRequest.Slack.App.BackgroundColor},
		[]string{billingRequest.Slack.App.Image, billingRequest.Slack.App.BackgroundColor})
}

func TestSetupRequestTargetsTheDeclaredChannelName(t *testing.T) {
	manifest := slackManifestOf(slackRequirements(t))
	manifest["name"] = "support"
	prepared, err := prepareTrigger(introspect.ChannelRequirements{"support": manifest}, true)
	require.NoError(t, err)

	request, err := buildSetupRequest(prepared, "https://agent.example", "deployment-id", "assistant-id")

	require.NoError(t, err)
	assert.Equal(t, "https://agent.example/v1/channels/support/events", request.Target.EndpointURL)
}

func TestActiveResponses(t *testing.T) {
	state, err := classifySetupResponse(activeSetupResponse())
	require.NoError(t, err)
	assert.Equal(t, setupActive, state.kind)

	for _, channelID := range []string{"channel-id", "", "   "} {
		_, err := classifySetupResponse(hostapi.ChannelSetupResponse{Status: hostapi.ChannelSetupStatusActive, ChannelID: channelID})
		require.EqualError(t, err, "Channel setup returned an invalid response.", "%q", channelID)
	}
}

func TestAuthorizationResponseClassifiesAValidHTTPSURL(t *testing.T) {
	state, err := classifySetupResponse(hostapi.ChannelSetupResponse{
		Status:           hostapi.ChannelSetupStatusAuthorizationRequired,
		AuthorizationURL: "https://trigger.example/authorize?state=resume-setup",
	})

	require.NoError(t, err)
	assert.Equal(t, setupActionRequired, state.kind)
	assert.Equal(t, actionAuthorize, state.action.kind)
	assert.Equal(t, "Slack channel setup requires authorization.", state.action.message)
	assert.Equal(t, "https://trigger.example/authorize?state=resume-setup", state.action.url.String())
}

func TestPendingApprovalOmitsAnUnsafeURL(t *testing.T) {
	state, err := classifySetupResponse(hostapi.ChannelSetupResponse{
		Status:           hostapi.ChannelSetupStatusPendingApproval,
		AuthorizationURL: "http://unsafe.example/approve",
	})

	require.NoError(t, err)
	assert.Equal(t, requiredAction{kind: actionApprove, message: "Slack channel setup is pending workspace approval."}, state.action)
}

func TestAuthorizationResponseRejectsAnUnsafeURL(t *testing.T) {
	for _, unsafe := range []string{
		"http://unsafe.example/authorize",
		"https://user@trigger.example/authorize",
		"https://trigger.example/authorize#fragment",
		"https://trigger.example/authorize\n",
	} {
		_, err := classifySetupResponse(hostapi.ChannelSetupResponse{
			Status:           hostapi.ChannelSetupStatusAuthorizationRequired,
			AuthorizationURL: unsafe,
		})

		require.EqualError(t, err, "Slack channel setup requires authorization. Authorize the Slack app, then rerun `langsmith deploy`.", unsafe)
	}
}

func TestPendingApprovalIncludesASafeTriggerMessage(t *testing.T) {
	state, err := classifySetupResponse(hostapi.ChannelSetupResponse{
		Status:  hostapi.ChannelSetupStatusPendingApproval,
		Message: "Reconnect the existing Slack installation.",
	})

	require.NoError(t, err)
	assert.Equal(t, requiredAction{
		kind:    actionApprove,
		message: "Slack channel setup is pending workspace approval. Reconnect the existing Slack installation.",
	}, state.action)

	state, err = classifySetupResponse(hostapi.ChannelSetupResponse{
		Status:  hostapi.ChannelSetupStatusPendingApproval,
		Message: "\x1b[2J",
	})
	require.NoError(t, err)
	assert.Equal(t, "Slack channel setup is pending workspace approval.", state.action.message)
}

// registerWith runs [runRegistration] over responses, recording each setup
// call, shown action and wait. interruptAt is the wait that gets Ctrl-C, 0
// for none.
func registerWith(t *testing.T, responses []hostapi.ChannelSetupResponse, interruptAt int) ([]string, error) {
	var log []string
	waits := 0
	err := runRegistration(
		func() (hostapi.ChannelSetupResponse, error) {
			log = append(log, "setup")
			require.NotEmpty(t, responses, "unexpected setup call")
			next := responses[0]
			responses = responses[1:]
			return next, nil
		},
		func(action requiredAction) error {
			log = append(log, "show "+action.heading())
			return nil
		},
		func(duration time.Duration) waitTick {
			assert.Equal(t, actionPollInterval, duration)
			waits++
			log = append(log, "wait")
			if waits == interruptAt {
				return waitInterrupted
			}
			return waitElapsed
		})
	return log, err
}

type setupResult struct {
	response hostapi.ChannelSetupResponse
	err      error
}

func runRegistrationOver(results []setupResult, shows *int) error {
	return runRegistration(
		func() (hostapi.ChannelSetupResponse, error) {
			next := results[0]
			results = results[1:]
			return next.response, next.err
		},
		func(requiredAction) error {
			*shows++
			return nil
		},
		func(time.Duration) waitTick { return waitElapsed })
}

func TestRegistrationKeepsWaitingThroughAFailedRequestOnceTheLinkIsShown(t *testing.T) {
	transient := setupResult{err: &hostapi.TransientSetupError{Message: "Channel setup failed (HTTP 503)."}}
	shows := 0
	err := runRegistrationOver([]setupResult{
		{response: authorizationSetupResponse()},
		transient,
		{response: activeSetupResponse()},
	}, &shows)
	require.NoError(t, err)
	assert.Equal(t, 1, shows)

	// Before anything is shown, and for any other error, it ends.
	for _, first := range []setupResult{transient, {err: errors.New("Channel setup failed (HTTP 400).")}} {
		require.Error(t, runRegistrationOver([]setupResult{first}, &shows))
	}
	err = runRegistrationOver([]setupResult{
		{response: authorizationSetupResponse()},
		{err: errors.New("Channel setup failed (HTTP 400).")},
	}, &shows)
	require.ErrorContains(t, err, "HTTP 400")
}

func TestRegistrationShowsTheLinkAndPollsUntilActive(t *testing.T) {
	log, err := registerWith(t, []hostapi.ChannelSetupResponse{
		authorizationSetupResponse(),
		authorizationSetupResponse(),
		activeSetupResponse(),
	}, 0)

	require.NoError(t, err)
	assert.Equal(t, []string{"setup", "show Connect Slack", "wait", "setup", "wait", "setup"}, log)
}

func TestRegistrationShowsEachNewStepOnce(t *testing.T) {
	log, err := registerWith(t, []hostapi.ChannelSetupResponse{
		authorizationSetupResponse(),
		pendingApprovalSetupResponse(),
		pendingApprovalSetupResponse(),
		activeSetupResponse(),
	}, 0)

	require.NoError(t, err)
	assert.Equal(t, []string{
		"setup", "show Connect Slack", "wait",
		"setup", "show Approve Slack", "wait",
		"setup", "wait",
		"setup",
	}, log)
}

func TestRegistrationStopsWaitingOnCtrlC(t *testing.T) {
	log, err := registerWith(t, []hostapi.ChannelSetupResponse{
		authorizationSetupResponse(),
		authorizationSetupResponse(),
	}, 2)

	assert.True(t, errs.IsCancellation(err), "%v", err)
	assert.Equal(t, []string{"setup", "show Connect Slack", "wait", "setup", "wait"}, log)
}

func TestRegistrationGivesUpAfterAnHourWithGuidance(t *testing.T) {
	polls := int(actionWaitLimit / actionPollInterval)
	responses := make([]hostapi.ChannelSetupResponse, polls+1)
	for i := range responses {
		responses[i] = authorizationSetupResponse()
	}

	log, err := registerWith(t, responses, 0)

	require.EqualError(t, err, "Stopped waiting for Slack after an hour. Slack channel setup requires authorization. Authorize the Slack app at https://trigger.example/authorize, then rerun `langsmith deploy`.")
	count := func(prefix string) int {
		n := 0
		for _, entry := range log {
			if strings.HasPrefix(entry, prefix) {
				n++
			}
		}
		return n
	}
	assert.Equal(t, polls+1, count("setup"))
	assert.Equal(t, polls, count("wait"))
	assert.Equal(t, 1, count("show"))
}

func TestRegistrationPropagatesAFailedSetupCallWithoutWaiting(t *testing.T) {
	waits := 0
	err := runRegistration(
		func() (hostapi.ChannelSetupResponse, error) {
			return hostapi.ChannelSetupResponse{}, errors.New("Channel setup request failed.")
		},
		func(requiredAction) error { return nil },
		func(time.Duration) waitTick {
			waits++
			return waitElapsed
		})

	require.EqualError(t, err, "Channel setup request failed.")
	assert.Equal(t, 0, waits)
}

func TestActionsHavePresentationValues(t *testing.T) {
	authorizeURL, err := url.Parse("https://trigger.example/authorize?state=resume-setup")
	require.NoError(t, err)
	authorize := requiredAction{kind: actionAuthorize, message: "Slack channel setup requires authorization.", url: authorizeURL}
	assert.Equal(t, "Connect Slack", authorize.heading())
	assert.Equal(t, "Authorize LangSmith to set up your Slack app.", authorize.instruction())

	approve := requiredAction{kind: actionApprove, message: "Slack channel setup is pending workspace approval."}
	assert.Equal(t, "Approve Slack", approve.heading())
	assert.Equal(t, "Install the Slack app. A workspace admin may need to approve it first.", approve.instruction())
	assert.Equal(t, "Slack channel setup is pending workspace approval. Approve the Slack app, then rerun `langsmith deploy`.", approve.retryGuidance())
}

func TestUnattendedPreflightKeepsAChannelTriggerServerHas(t *testing.T) {
	unattended(t)
	trigger := serveSequence(t, reply{http.StatusOK, `{"channels":[{"provider":"slack","channel_name":"slack"}]}`})
	prepared, err := prepareTrigger(slackRequirements(t), true)
	require.NoError(t, err)

	err = preflightTriggerChannel(t.Context(), oauthClient(t, trigger.URL), prepared, "dep-1", map[string]string{})

	require.NoError(t, err)
	assert.Equal(t, []string{"GET /v1/channels?deployment_id=dep-1"}, trigger.lines())
	assert.Equal(t, "Bearer oauth-token", trigger.received()[0].header.Get("Authorization"))
}

func TestUnattendedPreflightOfANewChannelNeedsLangSmithToCreateTheApp(t *testing.T) {
	unattended(t)
	trigger := serveSequence(t,
		reply{http.StatusOK, `{"channels":[]}`},
		reply{http.StatusOK, `{"authorized":false}`})
	prepared, err := prepareTrigger(slackRequirements(t), true)
	require.NoError(t, err)

	err = preflightTriggerChannel(t.Context(), oauthClient(t, trigger.URL), prepared, "dep-1", map[string]string{})

	require.EqualError(t, err, slackUnattendedMessage(false))
	assert.Equal(t, []string{
		"GET /v1/channels?deployment_id=dep-1",
		"GET /v1/triggers/slack-custom-app/manager",
	}, trigger.lines())
}

func TestUnattendedRegistrationWithTheEnvAppSetsTheChannelUp(t *testing.T) {
	output := unattended(t)
	trigger := serveSequence(t,
		reply{http.StatusOK, `{"channels":[]}`},
		reply{http.StatusOK, `{"status":"active","channel_id":"a6190f67-865d-4705-8171-35c90e64c730"}`})
	prepared, err := prepareTrigger(slackRequirements(t), true)
	require.NoError(t, err)
	envVars := map[string]string{SlackBotTokenEnv: "xoxb-token", SlackSigningSecretEnv: "signing"}

	err = registerTriggerChannel(t.Context(), oauthClient(t, trigger.URL), prepared, "https://agent.example", "dep-1", "assistant-id", envVars)

	require.NoError(t, err)
	assert.Equal(t, []string{"GET /v1/channels?deployment_id=dep-1", "POST /v1/channels"}, trigger.lines())
	setup := jsonObject(t, trigger.received()[1].body)
	assert.Equal(t, map[string]any{"bot_token": "xoxb-token", "signing_secret": "signing"}, setup["slack"].(map[string]any)["customer_app"])
	assert.Contains(t, output.String(), "Using the Slack app from MDA_SLACK_BOT_TOKEN and MDA_SLACK_SIGNING_SECRET.")
	assert.Contains(t, output.String(), slackReady)
}
