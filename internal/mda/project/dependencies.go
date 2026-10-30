package project

// Central registry of the dependencies the CLI manages (`dependencies.rs`).
//
// An author's generated manifest only ever lists the single dependency they
// actually reason about: `managed-deepagents`, pinned to the installed runtime
// package version so the authoring SDK and the CLI that compiles it always
// move in lockstep. Everything else the managed runtime needs is *inferred*
// and injected into the build manifest at compile time, never written into the
// author's source:
//
//   - `deepagents` and the LangChain / LangGraph stack ([ManagedDependencies]),
//     and
//   - the chat-model provider integration package implied by the agent's
//     configured `model` ([ProviderDependencyForModel]), ported from
//     LangChain's `initChatModel` provider inference.
//
// TypeScript LangChain dependency ranges: the managed LangGraph runtime image
// validates the resolved `@langchain/core`, `@langchain/langgraph`, and
// `@langchain/langgraph-checkpoint` versions against the API server's
// dependencies. The graph also loads `@langchain/langgraph` from the **deploy
// `npm ci` tree**, not from the Host image, so Topic / `__pregel_tasks`
// checkpoint shape must match what Host's Python checkpointer accepts
// (`@langchain/langgraph@1.4.9+` emits a flat values list). Use caret ranges
// from known-compatible floors for the TypeScript LangChain stack, then record
// each compilation's concrete resolution in the generated lockfile. Bump the
// floors together when the managed image advances.

import (
	"fmt"
	"slices"
	"strings"
)

// Dependency is a single managed dependency and the version range written
// into a manifest.
type Dependency struct {
	// Name is the package name as published to npm / PyPI.
	Name string
	// Version is the version as it should appear in the manifest. May be an
	// exact pin (`1.4.9`) or carry a range operator (`^0.0.2`, `>=0.7.3`).
	Version string
}

// UserRuntimePackage is the user-facing dependency seeded into newly
// scaffolded manifests.
//
// Intentionally just the single package an author works with directly; the
// rest of the stack — including `deepagents` — is injected into the build by
// [ManagedDependencies].
const UserRuntimePackage = "managed-deepagents"

// typescriptManaged are the npm dependencies the managed runtime requires,
// injected into the build's `package.json` at compile time rather than written
// into the author's source.
var typescriptManaged = []Dependency{
	{Name: "@langchain/core", Version: "^1.2.17"},
	{Name: "@langchain/langgraph", Version: "^1.4.18"},
	{Name: "@langchain/langgraph-checkpoint", Version: "^1.1.3"},
	{Name: "@langchain/langgraph-sdk", Version: "^1.12.0"},
	{Name: "deepagents", Version: "^1.15.1"},
	// Serves MDA managed HTTP routes (channels / connectors) from the
	// generated `_mda_http.ts` module.
	{Name: "hono", Version: "^4.12.27"},
	// Verifies `validated_token` ingress: JWKS local verification in the
	// generated custom-auth handler.
	{Name: "jose", Version: "^6.2.3"},
	{Name: "langchain", Version: "^1.5.12"},
	// The built-in MCP connector ships with the managed runtime, so its adapter
	// rides along unconditionally rather than being inferred from `connectors/`.
	typescriptConnectors,
	// LangSmith sandbox client (`langsmith/sandbox`) used by the managed
	// sandbox manager at runtime.
	{Name: "langsmith", Version: "^0.8.9"},
	// Deterministic UUIDv5 thread IDs for Trigger-backed channels.
	{Name: "uuid", Version: "^13.0.2"},
	{Name: "zod", Version: "^4.4.3"},
}

// pythonManaged are the PyPI dependencies the managed runtime requires,
// injected into the build's `pyproject.toml` at compile time.
var pythonManaged = []Dependency{
	{Name: "deepagents", Version: ">=0.7.30,<1"},
	{Name: "langchain[mcp]", Version: ">=1.4.6,<2"},
	{Name: "langgraph", Version: ">=1.2.15,<2"},
	// 1.1.1 lets an invalid answer to an interrupt inside a tool (such as a
	// subagent) fail the resume instead of reaching the model as a tool error.
	// It needs langgraph 1.2.15, which marks those errors.
	{Name: "langgraph-prebuilt", Version: ">=1.1.1,<2"},
	{Name: "langgraph-checkpoint", Version: ">=4.2.0"},
	// Provides `langgraph_sdk.Auth`, which the generated `_mda_auth.py` uses
	// to build the managed custom-auth handler. Pulled transitively by
	// `langgraph`, but injected explicitly so identity-enabled deploys have it
	// deterministically (mirrors the TypeScript `@langchain/langgraph-sdk`
	// injection).
	{Name: "langgraph-sdk", Version: ">=0.4.6,<0.5"},
	// Verifies `validated_token` ingress: JWKS local verification and
	// MDA-signed OAuth `state` JWTs in the generated custom-auth handler.
	// `[crypto]` pulls the `cryptography` backend needed for RS256/ES256
	// signatures.
	{Name: "pyjwt[crypto]", Version: ">=2.8.0"},
	// The built-in MCP connector ships with the managed runtime, so its adapter
	// rides along unconditionally rather than being inferred from `connectors/`.
	pythonConnectors,
	{Name: "mcp", Version: ">=2.2.0,<3"},
	// LangSmith SDK used by deepagents' LangSmith sandbox backend at runtime.
	{Name: "langsmith", Version: ">=0.10.17"},
}

var (
	// PythonBrowser is the PyPI package that drives a LangSmith browser from
	// the deployment, for projects that declare one in `browser.py`. Pinned:
	// the browser loads the extension from the same Stagehand release
	// (`browser.py`'s `STAGEHAND_VERSION`).
	PythonBrowser = Dependency{Name: "stagehand", Version: "==4.3.0"}

	// PythonBrowserTools is the PyPI package behind that browser's `run`,
	// `snapshot` and `screenshot` tools: Stagehand's facade. Pinned like the
	// package's typing group: it is a first release, and `browser.py` is
	// written against how it closes and saves.
	PythonBrowserTools = Dependency{Name: "stagehand-middleware", Version: "==0.1.0"}

	// TypeScriptBrowser is the npm package that drives a LangSmith browser
	// from the deployment, for projects that declare one in `browser.ts`.
	// Pinned to the release that `managed-deepagents` lists as a peer
	// dependency, so a deployment runs the Stagehand the package is tested
	// against.
	TypeScriptBrowser = Dependency{Name: "@browserbasehq/stagehand", Version: "4.3.0"}

	// TypeScriptBrowserTools is the npm package behind that browser's `run`,
	// `snapshot` and `screenshot` tools: Stagehand's facade, the npm twin of
	// [PythonBrowserTools]. Pinned to the release that `managed-deepagents`
	// lists as a peer dependency.
	TypeScriptBrowserTools = Dependency{Name: "stagehand-middleware", Version: "0.1.1"}

	// TypeScriptDevServerCLI is the npm package that provides the local
	// LangGraph in-memory dev server.
	//
	// The caret range keeps local development on compatible LangGraph CLI
	// updates; the compile-generated lockfile records the concrete resolved
	// version.
	TypeScriptDevServerCLI = Dependency{Name: "@langchain/langgraph-cli", Version: "^1.5.1"}

	// PythonDevServerCLI is the Python package that provides the local
	// LangGraph in-memory dev server and the `langgraph build` used by
	// `langsmith deploy --push-to`.
	PythonDevServerCLI = Dependency{Name: "langgraph-cli[inmem]", Version: ">=0.4.36"}

	// PythonDevServerAPI is the Python LangGraph API server behind `mda dev`.
	// 0.16 keeps each interrupt's `response_schema` in thread state, so a
	// conversation reopened with `mda chat --thread` still knows which
	// decisions a tool approval allows. Deploys pin their own image
	// (`LANGGRAPH_API_VERSION`).
	PythonDevServerAPI = Dependency{Name: "langgraph-api", Version: ">=0.16.0"}
)

// typescriptLocalDev are the npm dependencies only needed by `mda dev`.
var typescriptLocalDev = []Dependency{TypeScriptDevServerCLI}

// pythonLocalDev are the PyPI dependencies only needed by `mda dev`.
var pythonLocalDev = []Dependency{PythonDevServerCLI, PythonDevServerAPI}

// ManagedDependencies are the managed LangChain / LangGraph dependencies for
// language, injected into the build manifest at compile time.
func ManagedDependencies(language Language) []Dependency {
	return slices.Clone(byLanguage(language, typescriptManaged, pythonManaged))
}

// BrowserDependencies are the extra dependencies for projects that declare a
// `browser`.
func BrowserDependencies(language Language) []Dependency {
	return byLanguage(language,
		[]Dependency{TypeScriptBrowser, TypeScriptBrowserTools},
		[]Dependency{PythonBrowser, PythonBrowserTools})
}

// LocalDevDependencies are the extra dependencies needed only for local
// `mda dev`.
func LocalDevDependencies(language Language) []Dependency {
	return slices.Clone(byLanguage(language, typescriptLocalDev, pythonLocalDev))
}

func byLanguage[T any](language Language, typescript, python T) T {
	if language == Python {
		return python
	}
	return typescript
}

// typescriptConnectors is the npm package backing the built-in MCP connector.
// Its peer ranges accept the managed Core and LangGraph ranges. Part of the
// always-injected managed stack — the CLI no longer infers connector
// dependencies from `connectors/`.
var typescriptConnectors = Dependency{Name: "@langchain/mcp-adapters", Version: "^2.0.0"}

// pythonConnectors is the PyPI package backing the built-in MCP connector,
// injected with a `>=` floor to match the Python manifest convention. Part of
// the always-injected managed stack — the CLI no longer infers connector
// dependencies from `connectors/`.
var pythonConnectors = Dependency{Name: "fastmcp", Version: ">=4.0.10,<5"}

// modelProvider is a chat-model provider and the integration packages that
// back it in each ecosystem.
//
// Ported from LangChain's `MODEL_PROVIDER_CONFIG` (`initChatModel`), mapping a
// provider key to the package that exports its chat-model class.
type modelProvider struct {
	// keys are the accepted provider keys (model-string prefixes). Covers both
	// the JS-style (`google-vertexai`) and Python-style (`google_vertexai`)
	// spellings, plus aliases, so a model string authored in either ecosystem
	// resolves.
	keys []string
	// npm is the npm integration package + pinned version for a TypeScript
	// build.
	npm Dependency
	// pypi is the PyPI integration package + floor version for a Python build.
	pypi Dependency
	// apiKeyEnv is the environment variable that must carry this provider's
	// API key at runtime, or empty when the provider authenticates by some
	// other means (e.g. Bedrock's AWS credentials, Vertex AI's
	// application-default credentials) or needs no key at all (a local Ollama
	// server). Used by [RequiredAPIKeyEnvForModel] to pre-flight credentials
	// before a deploy.
	apiKeyEnv string
}

// modelProviders is the provider → integration package registry.
//
// Versions track the current published releases. npm packages use caret
// ranges and compilation records the concrete resolution in its lockfile; PyPI
// packages use `>=` floors (matching the Python manifest convention).
var modelProviders = []modelProvider{
	{
		keys: []string{"openai", "azure_openai", "azure-openai"},
		npm:  Dependency{Name: "@langchain/openai", Version: "^1.5.5"},
		pypi: Dependency{Name: "langchain-openai", Version: ">=1.4.2"},
		// Azure deployments authenticate with AZURE_OPENAI_API_KEY, but they
		// are always addressed via the explicit `azure_openai:` prefix; the
		// bare/`openai:` path that this covers is the OpenAI platform key.
		apiKeyEnv: "OPENAI_API_KEY",
	},
	{
		keys: []string{"langsmith"},
		npm:  Dependency{Name: "@langchain/openai", Version: "^1.5.5"},
		pypi: Dependency{Name: "langchain-openai", Version: ">=1.4.2"},
	},
	{
		keys:      []string{"anthropic"},
		npm:       Dependency{Name: "@langchain/anthropic", Version: "^1.5.1"},
		pypi:      Dependency{Name: "langchain-anthropic", Version: ">=1.5.4"},
		apiKeyEnv: "ANTHROPIC_API_KEY",
	},
	{
		keys:      []string{"cohere"},
		npm:       Dependency{Name: "@langchain/cohere", Version: "^1.1.0"},
		pypi:      Dependency{Name: "langchain-cohere", Version: ">=0.6.0"},
		apiKeyEnv: "COHERE_API_KEY",
	},
	{
		// Google's Vertex AI surface. `google` and the Python-only
		// `google_anthropic_vertex` resolve here too, matching `initChatModel`.
		keys: []string{
			"google-vertexai",
			"google_vertexai",
			"google",
			"google-vertexai-web",
			"google_anthropic_vertex",
		},
		npm:  Dependency{Name: "@langchain/google-vertexai", Version: "^2.2.0"},
		pypi: Dependency{Name: "langchain-google-vertexai", Version: ">=3.2.4"},
		// Vertex AI authenticates with Google application-default credentials,
		// not a single API-key env var, so there is nothing to pre-flight.
	},
	{
		keys:      []string{"google-genai", "google_genai"},
		npm:       Dependency{Name: "@langchain/google-genai", Version: "^2.2.0"},
		pypi:      Dependency{Name: "langchain-google-genai", Version: ">=4.3.2"},
		apiKeyEnv: "GOOGLE_API_KEY",
	},
	{
		keys: []string{"ollama"},
		npm:  Dependency{Name: "@langchain/ollama", Version: "^1.3.0"},
		pypi: Dependency{Name: "langchain-ollama", Version: ">=1.1.0"},
		// A local Ollama server needs no API key.
	},
	{
		keys:      []string{"mistralai", "mistral"},
		npm:       Dependency{Name: "@langchain/mistralai", Version: "^1.2.0"},
		pypi:      Dependency{Name: "langchain-mistralai", Version: ">=1.1.6"},
		apiKeyEnv: "MISTRAL_API_KEY",
	},
	{
		keys:      []string{"groq"},
		npm:       Dependency{Name: "@langchain/groq", Version: "^1.3.1"},
		pypi:      Dependency{Name: "langchain-groq", Version: ">=1.1.3"},
		apiKeyEnv: "GROQ_API_KEY",
	},
	{
		keys: []string{"bedrock", "aws", "bedrock_converse"},
		npm:  Dependency{Name: "@langchain/aws", Version: "^1.4.2"},
		pypi: Dependency{Name: "langchain-aws", Version: ">=1.7.0"},
		// Bedrock authenticates with AWS credentials (access key/secret/region),
		// not a single API-key env var, so there is nothing to pre-flight.
	},
	{
		keys:      []string{"deepseek"},
		npm:       Dependency{Name: "@langchain/deepseek", Version: "^1.1.3"},
		pypi:      Dependency{Name: "langchain-deepseek", Version: ">=1.1.0"},
		apiKeyEnv: "DEEPSEEK_API_KEY",
	},
	{
		keys:      []string{"xai"},
		npm:       Dependency{Name: "@langchain/xai", Version: "^1.4.3"},
		pypi:      Dependency{Name: "langchain-xai", Version: ">=1.3.0"},
		apiKeyEnv: "XAI_API_KEY",
	},
	{
		keys:      []string{"perplexity"},
		npm:       Dependency{Name: "@langchain/perplexity", Version: "^0.3.0"},
		pypi:      Dependency{Name: "langchain-perplexity", Version: ">=1.4.0"},
		apiKeyEnv: "PPLX_API_KEY",
	},
	{
		keys:      []string{"fireworks"},
		npm:       Dependency{Name: "@langchain/fireworks", Version: "^0.2.3"},
		pypi:      Dependency{Name: "langchain-fireworks", Version: ">=1.5.2"},
		apiKeyEnv: "FIREWORKS_API_KEY",
	},
	{
		keys:      []string{"together"},
		npm:       Dependency{Name: "@langchain/together-ai", Version: "^0.2.3"},
		pypi:      Dependency{Name: "langchain-together", Version: ">=0.4.0"},
		apiKeyEnv: "TOGETHER_AI_API_KEY",
	},
}

// ProviderDependencyForModel resolves the chat-model provider integration
// package implied by model for language; false when no provider can be
// determined.
//
// The provider is taken from an explicit `provider:model` prefix when it names
// a known provider, otherwise inferred from the bare model name (e.g. `gpt-*`
// → OpenAI). When neither yields a known provider — including dynamically
// constructed models the CLI cannot read — nothing is injected and the author
// remains responsible for the integration package.
func ProviderDependencyForModel(model string, language Language) (Dependency, bool) {
	provider := providerForModel(model)
	if provider == nil {
		return Dependency{}, false
	}
	return byLanguage(language, provider.npm, provider.pypi), true
}

// RequiredAPIKeyEnvForModel is the environment variable that must carry an
// API key for model's provider at runtime; false when no key can be
// pre-flighted.
//
// False when the provider cannot be determined from the model string (e.g. a
// dynamically constructed model the CLI cannot read) or when the provider does
// not authenticate via a single API-key env var (Bedrock's AWS credentials,
// Vertex AI's application-default credentials, a local Ollama server). Callers
// treat false as "nothing to verify".
func RequiredAPIKeyEnvForModel(model string) (string, bool) {
	provider := providerForModel(model)
	if provider == nil || provider.apiKeyEnv == "" {
		return "", false
	}
	return provider.apiKeyEnv, true
}

// ModelAPIKeyRequirementKind says which API-key preflight a deploy model needs.
type ModelAPIKeyRequirementKind int

// The preflights.
const (
	// NoPreflight: there is nothing to verify.
	NoPreflight ModelAPIKeyRequirementKind = iota
	// DeploymentSecret: the deployment needs the secret named by
	// [ModelAPIKeyRequirement.Secret].
	DeploymentSecret
	// WorkspaceProviderSecret: the workspace needs the Provider Secret named by
	// [ModelAPIKeyRequirement.Secret].
	WorkspaceProviderSecret
	// UnresolvedLangsmithProvider: a LangSmith Gateway model whose provider is
	// unknown.
	UnresolvedLangsmithProvider
)

// ModelAPIKeyRequirement is the API-key preflight for a deploy model.
type ModelAPIKeyRequirement struct {
	Kind ModelAPIKeyRequirementKind
	// Secret is the secret name for [DeploymentSecret] and
	// [WorkspaceProviderSecret], empty otherwise.
	Secret string
}

var keylessLangsmithModels = []string{"langsmith:moonshotai/kimi-k3"}

type langsmithGatewayProviderSecret struct {
	provider   string
	secretName string
}

var langsmithGatewayProviderSecrets = []langsmithGatewayProviderSecret{
	{provider: "anthropic", secretName: "ANTHROPIC_API_KEY"},
	{provider: "baseten", secretName: "BASETEN_API_KEY"},
	{provider: "bedrock", secretName: "AWS_BEARER_TOKEN_BEDROCK"},
	{provider: "fireworks", secretName: "FIREWORKS_API_KEY"},
	{provider: "gemini", secretName: "GOOGLE_API_KEY"},
	{provider: "openai", secretName: "OPENAI_API_KEY"},
	{provider: "vertex", secretName: "VERTEX_SERVICE_ACCOUNT_JSON"},
}

// DeployModelAPIKeyRequirement resolves the API-key preflight for a deploy
// model.
//
// A Gateway model must either match a keyless identifier exactly or resolve to
// a workspace Provider Secret. An unmapped provider is rejected instead of
// silently skipping the preflight.
func DeployModelAPIKeyRequirement(model string) ModelAPIKeyRequirement {
	if slices.Contains(keylessLangsmithModels, model) {
		return ModelAPIKeyRequirement{Kind: NoPreflight}
	}

	gatewayModel, ok := strings.CutPrefix(model, "langsmith:")
	if !ok {
		if secret, ok := RequiredAPIKeyEnvForModel(model); ok {
			return ModelAPIKeyRequirement{Kind: DeploymentSecret, Secret: secret}
		}
		return ModelAPIKeyRequirement{Kind: NoPreflight}
	}

	providerKey, modelName, ok := strings.Cut(gatewayModel, "/")
	if !ok || providerKey == "" || modelName == "" {
		return ModelAPIKeyRequirement{Kind: UnresolvedLangsmithProvider}
	}

	for _, provider := range langsmithGatewayProviderSecrets {
		if provider.provider == providerKey {
			return ModelAPIKeyRequirement{Kind: WorkspaceProviderSecret, Secret: provider.secretName}
		}
	}
	return ModelAPIKeyRequirement{Kind: UnresolvedLangsmithProvider}
}

// ProviderAPIKeyEnvs is every distinct API-key environment variable declared
// by the managed provider registry, sorted. Used by `mda evals` Harbor jobs so
// supported models keep working when their documented key is set (including
// providers not listed in the scaffolded `.env` comments).
func ProviderAPIKeyEnvs() []string {
	var names []string
	for _, provider := range modelProviders {
		if provider.apiKeyEnv != "" {
			names = append(names, provider.apiKeyEnv)
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// providerForModel matches model to a [modelProvider], preferring an explicit,
// known `provider:` prefix and falling back to name-based inference.
func providerForModel(model string) *modelProvider {
	if prefix, _, ok := strings.Cut(model, ":"); ok {
		if provider := providerForKey(prefix); provider != nil {
			return provider
		}
	}
	key, ok := inferProviderFromModelName(model)
	if !ok {
		return nil
	}
	return providerForKey(key)
}

func providerForKey(key string) *modelProvider {
	for i := range modelProviders {
		if slices.Contains(modelProviders[i].keys, key) {
			return &modelProviders[i]
		}
	}
	return nil
}

// inferProviderFromModelName infers a provider key from a bare model name's
// prefix.
//
// Ported from LangChain's `_inferModelProvider`, extended with the additional
// prefixes the Python `init_chat_model` recognizes (`deepseek`, `grok`).
func inferProviderFromModelName(model string) (string, bool) {
	hasPrefix := func(prefixes ...string) bool {
		return slices.ContainsFunc(prefixes, func(prefix string) bool {
			return strings.HasPrefix(model, prefix)
		})
	}
	switch {
	case hasPrefix("gpt-3", "gpt-4", "gpt-5", "o1", "o3", "o4"):
		return "openai", true
	case hasPrefix("claude"):
		return "anthropic", true
	case hasPrefix("command"):
		return "cohere", true
	case hasPrefix("accounts/fireworks"):
		return "fireworks", true
	case hasPrefix("gemini"):
		return "google-vertexai", true
	case hasPrefix("amazon."):
		return "bedrock", true
	case hasPrefix("mistral"):
		return "mistralai", true
	case hasPrefix("deepseek"):
		return "deepseek", true
	case hasPrefix("grok"):
		return "xai", true
	case hasPrefix("sonar", "pplx"):
		return "perplexity", true
	default:
		return "", false
	}
}

// PackageJSONEntries renders the user-facing TypeScript dependency (the
// runtime package every project gets, pinned to packageVersion, normally
// [Installation.PackageVersion]) as a `"name": "version"` JSON member indented
// by indent spaces.
func PackageJSONEntries(indent int, packageVersion string) string {
	return fmt.Sprintf("%s\"%s\": \"%s\"", strings.Repeat(" ", indent), UserRuntimePackage, packageVersion)
}

// PyprojectEntries renders the user-facing Python dependency as a quoted TOML
// array element (e.g. `"managed-deepagents==0.7.3"`) indented by indent
// spaces.
func PyprojectEntries(indent int, packageVersion string) string {
	return fmt.Sprintf("%s\"%s==%s\"", strings.Repeat(" ", indent), UserRuntimePackage, packageVersion)
}
