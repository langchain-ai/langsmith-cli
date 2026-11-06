package project

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEveryDependencyHasANameAndVersion(t *testing.T) {
	assert.NotEmpty(t, UserRuntimePackage)
	for _, dep := range slices.Concat(typescriptManaged, pythonManaged) {
		assert.NotEmpty(t, dep.Name, "dependency name must not be empty")
		assert.NotEmpty(t, dep.Version, "%s must declare a version", dep.Name)
	}
	for _, provider := range modelProviders {
		assert.NotEmpty(t, provider.keys, "provider must declare keys")
		assert.NotEmpty(t, provider.npm.Name)
		assert.NotEmpty(t, provider.pypi.Name)
	}
}

func TestDeepagentsIsInjectedByTheManagedStack(t *testing.T) {
	assert.Contains(t, names(typescriptManaged), "deepagents")
	assert.Contains(t, names(pythonManaged), "deepagents")
}

func TestLanggraphSDKIsInjectedForTheManagedAuthHandler(t *testing.T) {
	// The generated `_mda_auth.py` builds `langgraph_sdk.Auth`, and the
	// TypeScript stack injects `@langchain/langgraph-sdk`, so the Python
	// manifest must inject `langgraph-sdk` explicitly for parity.
	assert.Contains(t, names(pythonManaged), "langgraph-sdk")
	assert.Contains(t, names(pythonManaged), "langgraph-prebuilt")
	assert.Contains(t, names(typescriptManaged), "@langchain/langgraph-sdk")
}

func TestTypeScriptDevServerCLIUsesACompatibleRange(t *testing.T) {
	assert.Equal(t, "^1.5.1", TypeScriptDevServerCLI.Version)
}

func TestLanggraphStackUsesServerCompatibleFloors(t *testing.T) {
	// Host JS graphs load these from the deploy npm tree. The generated
	// lockfile records a concrete compatible resolution for these floors.
	assert.Contains(t, typescriptManaged, Dependency{Name: "@langchain/langgraph", Version: "^1.4.18"})
	assert.Contains(t, typescriptManaged, Dependency{Name: "@langchain/langgraph-checkpoint", Version: "^1.1.3"})
	assert.Contains(t, typescriptManaged, Dependency{Name: "@langchain/langgraph-sdk", Version: "^1.12.0"})
}

func TestProviderInferredFromExplicitPrefix(t *testing.T) {
	assert.Equal(t, "@langchain/openai", providerName(t, "openai:gpt-5.5", TypeScript))
	assert.Equal(t, "langchain-anthropic", providerName(t, "anthropic:claude-x", Python))
}

func TestLangsmithProviderUsesOpenAIIntegrationPackages(t *testing.T) {
	assert.Equal(t, "@langchain/openai", providerName(t, "langsmith:moonshotai/kimi-k3", TypeScript))
	assert.Equal(t, "langchain-openai", providerName(t, "langsmith:moonshotai/kimi-k3", Python))
}

func TestProviderInferredFromBareModelName(t *testing.T) {
	assert.Equal(t, "@langchain/openai", providerName(t, "gpt-5.5", TypeScript))
	assert.Equal(t, "langchain-anthropic", providerName(t, "claude-sonnet-4-5", Python))
	assert.Equal(t, "@langchain/google-vertexai", providerName(t, "gemini-2.0-flash", TypeScript))
}

func TestProviderAliasesResolveToTheSamePackage(t *testing.T) {
	// Python-style underscore spelling resolves identically to the hyphenated
	// JS spelling.
	assert.Equal(t, "langchain-google-genai", providerName(t, "google_genai:gemini-2.0", Python))
	assert.Equal(t, "@langchain/mistralai", providerName(t, "mistral:mistral-large", TypeScript))
}

func TestManagedStackIncludesBuiltinConnectorAdapter(t *testing.T) {
	assert.Contains(t, ManagedDependencies(TypeScript), Dependency{Name: "@langchain/mcp-adapters", Version: "^2.0.0"})
	python := ManagedDependencies(Python)
	assert.Contains(t, python, Dependency{Name: "langchain[mcp]", Version: ">=1.4.6,<2"})
	assert.Contains(t, python, Dependency{Name: "fastmcp", Version: ">=4.0.10,<5"})
	assert.Contains(t, python, Dependency{Name: "mcp", Version: ">=2.2.0,<3"})
	assert.NotContains(t, names(python), "langchain-mcp-adapters")
}

func TestManagedTypeScriptStackIncludesUUIDForTriggerThreadIDs(t *testing.T) {
	assert.Contains(t, ManagedDependencies(TypeScript), Dependency{Name: "uuid", Version: "^13.0.2"})
}

func TestUnknownProviderYieldsNoDependency(t *testing.T) {
	_, ok := ProviderDependencyForModel("acme-llm-2", TypeScript)
	assert.False(t, ok)
	// An unknown prefix is not stripped, and the full string doesn't infer.
	_, ok = ProviderDependencyForModel("acme:super-model", Python)
	assert.False(t, ok)
}

func TestRequiredAPIKeyEnvResolvesForKeyedProviders(t *testing.T) {
	for model, key := range map[string]string{
		"openai:gpt-5.5":        "OPENAI_API_KEY",
		"gpt-5.5":               "OPENAI_API_KEY",
		"claude-sonnet-4-5":     "ANTHROPIC_API_KEY",
		"groq:llama-3.1":        "GROQ_API_KEY",
		"mistral:mistral-large": "MISTRAL_API_KEY",
	} {
		env, ok := RequiredAPIKeyEnvForModel(model)
		assert.True(t, ok, model)
		assert.Equal(t, key, env, model)
	}
}

func TestDeployModelAPIKeyRequirement(t *testing.T) {
	for model, want := range map[string]ModelAPIKeyRequirement{
		// The exact keyless model skips the preflight.
		"langsmith:moonshotai/kimi-k3": {Kind: NoPreflight},
		// Unqualified, empty, case-variant, colon-separated and unknown
		// Gateway models are rejected.
		"langsmith:kimi-k3":                     {Kind: UnresolvedLangsmithProvider},
		"langsmith:openai/":                     {Kind: UnresolvedLangsmithProvider},
		"langsmith:moonshotai/Kimi-K3":          {Kind: UnresolvedLangsmithProvider},
		"langsmith:moonshotai/other-model":      {Kind: UnresolvedLangsmithProvider},
		"langsmith:anthropic:claude-sonnet-4-5": {Kind: UnresolvedLangsmithProvider},
		"langsmith:acme/model":                  {Kind: UnresolvedLangsmithProvider},
		// Gateway providers resolve to workspace Provider Secrets.
		"langsmith:openai/gpt-5.5":          {Kind: WorkspaceProviderSecret, Secret: "OPENAI_API_KEY"},
		"langsmith:baseten/zai-org/GLM-5.2": {Kind: WorkspaceProviderSecret, Secret: "BASETEN_API_KEY"},
		"langsmith:gemini/gemini-3.6-flash": {Kind: WorkspaceProviderSecret, Secret: "GOOGLE_API_KEY"},
		"langsmith:vertex/gemini-3.1-pro":   {Kind: WorkspaceProviderSecret, Secret: "VERTEX_SERVICE_ACCOUNT_JSON"},
		"langsmith:bedrock/amazon.nova":     {Kind: WorkspaceProviderSecret, Secret: "AWS_BEARER_TOKEN_BEDROCK"},
		"openai:gpt-5.5":                    {Kind: DeploymentSecret, Secret: "OPENAI_API_KEY"},
		"anthropic:claude-sonnet-4-5":       {Kind: DeploymentSecret, Secret: "ANTHROPIC_API_KEY"},
		"cohere:command-r":                  {Kind: DeploymentSecret, Secret: "COHERE_API_KEY"},
		"ollama:llama3":                     {Kind: NoPreflight},
	} {
		assert.Equal(t, want, DeployModelAPIKeyRequirement(model), model)
	}
}

func TestRequiredAPIKeyEnvIsNoneForCredentialOrKeylessProviders(t *testing.T) {
	// Bedrock (AWS credentials), Vertex AI (ADC), and Ollama (local) have no
	// single API-key env var to pre-flight.
	for _, model := range []string{"amazon.titan-text", "gemini-2.0-flash", "ollama:llama3"} {
		_, ok := RequiredAPIKeyEnvForModel(model)
		assert.False(t, ok, model)
	}
}

func TestRequiredAPIKeyEnvIsNoneForUnknownProvider(t *testing.T) {
	for _, model := range []string{"acme-llm-2", "acme:super-model"} {
		_, ok := RequiredAPIKeyEnvForModel(model)
		assert.False(t, ok, model)
	}
}

func providerName(t *testing.T, model string, language Language) string {
	t.Helper()
	dep, ok := ProviderDependencyForModel(model, language)
	require.True(t, ok, model)
	return dep.Name
}

func names(deps []Dependency) []string {
	out := make([]string, len(deps))
	for i, dep := range deps {
		out[i] = dep.Name
	}
	return out
}
