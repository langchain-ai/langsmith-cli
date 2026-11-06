package deploy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

func TestSelfHostedRequiresPushTo(t *testing.T) {
	_, err := selectImageSource(true, nil)
	assert.EqualError(t, err, "self-hosted LangSmith requires a customer-managed image; pass --push-to <repository[:tag]>")
}

func TestAPIHostPathIdentifiesSelfHostedDeployments(t *testing.T) {
	t.Setenv(hostBuildsImagesEnv, "")
	assert.True(t, requiresExternalImage("http://langsmith.localhost/api-host"))
}

func TestLocalStackHostRequiresAnExternalImageWithoutOptOut(t *testing.T) {
	assert.True(t, requiresExternalImageWith("http://localhost:21980/api-host", nil))
}

func TestHostBuildsImagesOptOutSkipsExternalImageForLocalStack(t *testing.T) {
	assert.False(t, requiresExternalImageWith("http://localhost:21980/api-host", ptr("1")))
}

func TestHostBuildsImagesIgnoresValuesOtherThanOneOrTrue(t *testing.T) {
	for _, value := range []string{"", "0", "false", "yes"} {
		assert.True(t, requiresExternalImageWith("http://localhost:21980/api-host", ptr(value)), "%q must not opt out", value)
	}
}

func TestCloudHostDoesNotRequireAnExternalImage(t *testing.T) {
	assert.False(t, requiresExternalImage("https://api.host.langchain.com"))
}

func TestCloudDefaultsToInternalSource(t *testing.T) {
	source, err := selectImageSource(false, nil)
	require.NoError(t, err)
	assert.Equal(t, ImageSource{Kind: ImageSourceInternalSource}, source)
}

func TestCloudRejectsPushTo(t *testing.T) {
	_, err := selectImageSource(false, ptr("example.com/team/agent"))
	assert.EqualError(t, err, "--push-to is only supported for self-hosted LangSmith deployments")
}

func TestSelfHostedRejectsAnEmptyPushTo(t *testing.T) {
	_, err := selectImageSource(true, ptr(""))
	assert.EqualError(t, err, "--push-to must be a valid Docker repository with an optional tag")
}

func TestPushTargetDefaultsToLatestAndAllowsRegistryPorts(t *testing.T) {
	target, err := parsePushTarget("localhost:5000/team/agent")
	require.NoError(t, err)
	assert.Equal(t, "localhost:5000/team/agent", target.repository)
	assert.Equal(t, "localhost:5000/team/agent:latest", target.taggedImage)
}

func TestPushTargetPreservesExplicitTag(t *testing.T) {
	target, err := parsePushTarget("registry.example/team/agent:preview")
	require.NoError(t, err)
	assert.Equal(t, "registry.example/team/agent", target.repository)
	assert.Equal(t, "registry.example/team/agent:preview", target.taggedImage)
}

func TestPushTargetRejectsADigest(t *testing.T) {
	_, err := parsePushTarget("example/agent@sha256:" + strings.Repeat("a", 64))
	assert.EqualError(t, err, "--push-to must be a Docker repository with an optional tag, such as registry.example.com/team/agent:latest")
}

func TestPushTargetRejectsInvalidRepositoryAndTagShapes(t *testing.T) {
	for _, value := range []string{
		"registry.example/Team/agent",
		"example/agent::tag",
		"example/agent:-tag",
	} {
		_, err := parsePushTarget(value)
		assert.EqualError(t, err, "--push-to must be a valid Docker repository with an optional tag", value)
	}
}

func TestBuildCommandsUseLanguageSpecificCLI(t *testing.T) {
	target, err := parsePushTarget("example/agent:test")
	require.NoError(t, err)

	ts := buildCommandForArch(project.TypeScript, target, "amd64")
	assert.Equal(t, "npx", ts.program)
	assert.Equal(t, []string{"build", "-t", "example/agent:test"}, ts.args[len(ts.args)-3:])

	py := buildCommandForArch(project.Python, target, "amd64")
	assert.Equal(t, "uv", py.program)
	dep := project.PythonDevServerCLI
	assert.Equal(t, []string{
		"run", "--no-project", "--with", dep.Name + dep.Version,
		"langgraph", "build", "-t", "example/agent:test",
	}, py.args)
}

func TestNonAmd64BuildTargetsLinuxAmd64(t *testing.T) {
	target, err := parsePushTarget("example/agent:test")
	require.NoError(t, err)

	command := buildCommandForArch(project.TypeScript, target, "arm64")

	assert.Equal(t, []string{"example/agent:test", "--platform", "linux/amd64"}, command.args[len(command.args)-3:])
}

func TestDigestMustMatchRequestedRepository(t *testing.T) {
	target, err := parsePushTarget("example/agent:preview")
	require.NoError(t, err)
	wanted := "example/agent@sha256:" + strings.Repeat("a", 64)
	other := "other/agent@sha256:" + strings.Repeat("b", 64)
	stdout, err := json.Marshal([]string{other, wanted})
	require.NoError(t, err)

	digest, err := resolvePushedDigest(target, stdout)

	require.NoError(t, err)
	assert.Equal(t, wanted, digest)
}
