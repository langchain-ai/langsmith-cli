// Package deploy is `langsmith deploy`: it builds a Managed Deep Agent project
// and ships it to LangSmith Deployment, as the mda CLI's `deploy` module
// (`deploy/{mod,archive,bake,context_hub,image,upsell}.rs`) does.
//
// The flow mirrors the langgraph CLI's remote (internal-source) deploy path,
// drawn with the [ui] session:
//
//  1. Validate the directory is a Managed Deep Agent project (`agent.ts` /
//     `agent.py`).
//  2. Resolve the LangSmith credential ([deployment.LangSmithContext]'s
//     order): `--api-key`, else a `LANGSMITH_API_KEY` from the shell or
//     `.env`, else the selected profile's API key or OAuth token, else, on a
//     terminal, the `langsmith auth login` OAuth device flow. The CLI's own
//     credentials only go to the LangSmith they belong to.
//  3. Verify the agent model's provider API key is available (in `.env` or as a
//     LangSmith workspace secret) before doing any work, failing fast otherwise.
//     When identity uses `trusted_backend` ingress, also require
//     `MDA_INGRESS_SECRET` the same way — otherwise the agent would deploy but
//     reject every request with 401.
//  4. Confirm before a first Managed Deep Agent deploy overwrites a same-name
//     Context Hub repo or non-MDA LangSmith deployment, then sync deploy-owned
//     context into Context Hub and compile `.mda/build`.
//  5. Look up (or create) a deployment named after the authored agent `name`
//     (or `langsmith deploy -n`).
//  6. Archive the build, request an upload URL, upload the source, and trigger
//     a remote build.
//  7. Poll the revision status with a live spinner until it settles.
//  8. Reconcile Managed Deep Agent schedules through Agent Server cron APIs.
//
// # Requests
//
// Every LangSmith request goes through [hostapi.Client] or
// [hostapi.ContextHubClient], which send it with the CLI's credential. The one
// exception is the source upload: the host backend hands out a signed Google
// Cloud Storage URL that carries its own authorization, so the archive is sent
// there with plain `net/http` and no LangSmith credential.
//
// # Cancellation
//
// The command cancels ctx on Ctrl-C ([proc.InstallInterrupt]). Each stage
// checks it before starting, prompts and children end with it, and the
// revision wait stops at once; the session then closes with a cancelled
// footer.
package deploy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/langchain-ai/langsmith-cli/internal/mda/channels"
	"github.com/langchain-ai/langsmith-cli/internal/mda/compile"
	"github.com/langchain-ai/langsmith-cli/internal/mda/connection"
	"github.com/langchain-ai/langsmith-cli/internal/mda/deployment"
	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
	"github.com/langchain-ai/langsmith-cli/internal/mda/introspect"
	"github.com/langchain-ai/langsmith-cli/internal/mda/proc"
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
	"github.com/langchain-ai/langsmith-cli/internal/mda/ui"
)

// pollInterval is the delay between revision status polls.
const pollInterval = 3 * time.Second

// pollMaxFailures is how many consecutive failed revision polls are allowed
// before the deploy stops waiting.
const pollMaxFailures = 5

// ContextStrategy is how an unattended deploy resolves Context Hub files
// edited since MDA last synced them.
type ContextStrategy int

// The strategies.
const (
	// ContextStrategyOverwrite replaces Hub-edited instructions or skills with
	// their local versions.
	ContextStrategyOverwrite ContextStrategy = iota
	// ContextStrategyKeepHub preserves Hub-edited instructions or skills.
	ContextStrategyKeepHub
)

// randomHandoffSecret is a high-entropy, per-revision credential for internal
// loopback calls.
func randomHandoffSecret() string {
	simple := func() string {
		return strings.ReplaceAll(uuid.New().String(), "-", "")
	}
	return simple() + simple()
}

// DeployOptions are the user-facing options of `langsmith deploy`.
type DeployOptions struct {
	// Path is the path to the Managed Deep Agent project.
	Path string
	// Name overrides the deployment name (defaults to the authored agent
	// `name`); "" for none.
	Name string
	// AgentEnvironment is the agent environment to target, nil outside agent
	// mode.
	AgentEnvironment *hostapi.AgentEnvironment
	// DeploymentType is the deployment type when creating.
	DeploymentType hostapi.DeploymentType
	// WorkspaceID overrides the workspace (takes precedence over
	// `LANGSMITH_WORKSPACE_ID`); "" for none.
	WorkspaceID string
	// HostURL overrides the host backend URL; "" for none.
	HostURL string
	// PushTo is the customer-managed registry repository to build and push
	// to; nil for none.
	PushTo *string
	// Wait waits for the deployment to finish (polls its status) when true.
	Wait bool
	// ContextStrategy resolves Hub-edited deploy-owned context without
	// prompting; nil to ask.
	ContextStrategy *ContextStrategy
	// Yes overwrites same-name resources on a first deploy without prompting.
	Yes bool
	// WaitTimeout is the maximum time to wait for a deployment revision to
	// reach a terminal status.
	WaitTimeout time.Duration
}

// Run runs the deploy command end-to-end. cli is what the LangSmith CLI
// resolved from its flags and profiles; ctx is cancelled when the user asks
// to stop.
//
// Pre-flight validation runs before the session starts, so those failures are
// returned for the caller to print plainly. Once the intro is on screen, any
// error is rendered as a cancelled-session footer instead, and only the
// completion reports it.
func Run(ctx context.Context, cli deployment.CLI, opts DeployOptions) (errs.CommandCompletion, error) {
	if err := project.EnsureProjectDir(opts.Path); err != nil {
		return errs.Failed(errs.CategoryOf(err)), err
	}
	p, installation, err := project.LoadProject(ctx, opts.Path)
	if err != nil {
		return errs.Failed(errs.CategoryOf(err)), err
	}
	if err := ui.ShowWordmarkIntro(); err != nil {
		return errs.Failed(errs.CategoryOf(err)), err
	}
	if err := ui.LogInfo("Project: " + p.Root); err != nil {
		return errs.Failed(errs.CategoryOf(err)), err
	}

	var client *hostapi.Client
	completion, err := runSession(ctx, cli, opts, p, installation, &client)
	switch {
	case err == nil:
		return completion, nil
	case errs.IsCancellation(err):
		if err := ui.OutroCancel("Cancelled"); err != nil {
			return errs.Cancelled, err
		}
		return errs.Cancelled, nil
	default:
		category := errs.CategoryOf(err)
		if err := reportSessionFailure(ctx, err, client); err != nil {
			return errs.Failed(category), err
		}
		return errs.Failed(category), nil
	}
}

// reportSessionFailure closes a failed session, upgrading a plan-entitlement
// 403 into an upsell.
//
// client is whatever runSession managed to build before failing: the upsell's
// billing link is workspace-scoped, so without a client there is nothing to
// link to and the message falls back to directions.
func reportSessionFailure(ctx context.Context, err error, client *hostapi.Client) error {
	if gate, ok := planGate(err); ok {
		url := ""
		if client != nil {
			url = upgradeURL(ctx, client)
		}
		return reportPlanGate(gate, url)
	}
	return ui.OutroCancel(err.Error())
}

// runSession is the interactive portion of the deploy, framed by
// intro/outro.
//
// clientSlot hands the LangSmith client back to [Run] the moment it exists,
// before any host-backend call can fail. That is what lets a
// plan-entitlement 403 unwinding from anywhere in the session still be
// reported with a link to the right organization's billing page.
func runSession(
	ctx context.Context,
	cli deployment.CLI,
	opts DeployOptions,
	p *project.Project,
	installation *project.Installation,
	clientSlot **hostapi.Client,
) (errs.CommandCompletion, error) {
	failed := errs.Failed(errs.Unclassified)

	// -- 1. Target name, env + credentials ---------------------------------
	if err := ctx.Err(); err != nil {
		return failed, err
	}
	// Import the project's modules once; every question about them below is
	// answered from that import. They may read the environment as they import,
	// so they see the project's `.env`.
	envPath := filepath.Join(p.Root, ".env")
	envVars := project.ParseEnvFile(envPath)
	if err := importProjectModules(ctx, p, envVars); err != nil {
		return failed, err
	}
	// One agent-file read for deploy preflight (name / model / memory). The
	// name is resolved before credentials so a sign-in failure is still
	// reported against the deployment.
	agent, err := introspect.ReadAgentStaticConfig(ctx, p)
	if err != nil {
		return failed, err
	}
	assistantID := agent.AssistantID
	baseName := assistantID
	if opts.Name != "" {
		if baseName, err = project.NormalizeName(opts.Name); err != nil {
			return failed, err
		}
	}
	name, deploymentAgent, err := deployment.AgentTarget(baseName, opts.AgentEnvironment)
	if err != nil {
		return failed, err
	}

	endpoints := deployment.ResolveLangSmithEndpoints(cli, envVars, opts.HostURL)
	source, err := selectImageSource(requiresExternalImage(endpoints.HostURL), opts.PushTo)
	if err != nil {
		return failed, err
	}
	lsCtx, err := endpoints.Authenticate(ctx, cli, envPath, envVars, opts.WorkspaceID)
	if err != nil {
		return failed, err
	}
	platformAPIURL := lsCtx.PlatformAPIURL()
	workspaceID := lsCtx.WorkspaceID()

	secrets := deployment.SecretsFromEnv(envVars)
	deployment.SetMDAHandoffSecret(&secrets, randomHandoffSecret())
	// Scope the runtime to the same workspace the deploy clients use,
	// overriding any `LANGSMITH_WORKSPACE_ID` copied from `.env`.
	if workspaceID != "" {
		deployment.SetWorkspaceIDSecret(&secrets, workspaceID)
	}
	// Let the runtime name its sandboxes after this deployment so
	// `mda delete` can find and remove them later.
	deployment.EnsureDeploymentNameSecret(&secrets, name)
	// Channel ingress (Slack/GitHub) invokes runs by assistant / graph id —
	// always the langgraph.json graphs key, never a `--name` override.
	deployment.EnsureAssistantIDSecret(&secrets, assistantID)
	// Browser CORS is owned by LangGraph. Derive `CORS_ALLOW_ORIGINS` from the
	// configured public app URL / extra origins so the browser UI can call the
	// Agent Server without a second ACAO from the managed HTTP app.
	deployment.EnsureCORSAllowOrigins(&secrets, envVars)

	client, err := lsCtx.Client()
	if err != nil {
		return failed, err
	}
	*clientSlot = client
	dashboardURL, err := client.DashboardURL(ctx)
	if err != nil {
		slog.Debug("dashboard URL lookup failed", "error", err)
		dashboardURL = ""
	}
	if dashboardURL != "" {
		deployment.EnsureLangsmithURLSecret(&secrets, dashboardURL)
	}

	// -- 1b. Name where this deploy is going -------------------------------
	if err := ctx.Err(); err != nil {
		return failed, err
	}
	// Everything below writes remote state — Context Hub repos, secrets, the
	// deployment itself — into whichever organization and workspace the
	// credential resolves to. Say which before the first prompt that can
	// overwrite something.
	target := client.DescribeDeployTarget(ctx)
	if err := reportDeployTarget(target); err != nil {
		return failed, err
	}

	// -- 2. Verify the model provider API key is available -----------------
	if err := ctx.Err(); err != nil {
		return failed, err
	}
	// Direct models require a deployment key; Gateway models require a
	// workspace provider key. Check the source the model will actually use
	// before building. Workspace secret keys are fetched once for model /
	// ingress / connector checks.
	preflight := &secretPreflight{
		envPath:      envPath,
		envVars:      envVars,
		secrets:      &secrets,
		client:       client,
		dashboardURL: dashboardURL,
	}
	if err := preflight.ensureModelAPIKey(ctx, agent.Model); err != nil {
		return failed, err
	}

	// -- 2b. Verify trusted_backend ingress secret -------------------------
	if err := ctx.Err(); err != nil {
		return failed, err
	}
	// Identity with `trusted_backend` HTTP ingress (every shipped preset's
	// default) rejects every request unless `MDA_INGRESS_SECRET` is set on the
	// deployment. Fail before compiling when it is absent — otherwise the
	// agent deploys but is unusable.
	if err := preflight.ensureIngressSecret(ctx, p); err != nil {
		return failed, err
	}

	// A sandbox declaration with no recipe never hits the sandbox API during
	// bake, so this has to run as a preflight. After Context Hub sync it would
	// leave remote state changed on an org that cannot run the agent.
	usesSandbox, err := introspect.ProjectUsesSandbox(ctx, p)
	if err != nil {
		return failed, err
	}
	if usesSandbox {
		deployment.EnsureSandboxAPIEndpointSecret(&secrets, platformAPIURL)
		if err := client.EnsureSandboxAccess(ctx); err != nil {
			return failed, fmt.Errorf("project declares a managed sandbox, but sandbox access is unavailable for this organization: %w", err)
		}
	}

	// One decision for the whole deploy: the same value the generated entry
	// hands the runtime also decides whether the Hub gets a hot-memory seed.
	// Resolved here, above the gate, because it is the last preflight that can
	// still fail on purely local input — asking to overwrite and then aborting
	// on a bad `memory` declaration would be a prompt for a deploy that never
	// had a chance.
	memory, err := introspect.ResolveMemory(ctx, p)
	if err != nil {
		return failed, err
	}

	contextClient, err := lsCtx.ContextHubClient()
	if err != nil {
		return failed, err
	}

	// -- 2c. Confirm before overwriting same-name resources ----------------
	if err := ctx.Err(); err != nil {
		return failed, err
	}
	// Last gate before remote state changes: a first Managed Deep Agent deploy
	// overwrites a same-name Context Hub repo or LangSmith deployment, so make
	// that explicit. Deliberately after the preflights above — a deploy that
	// cannot succeed should never ask the question.
	proceed, err := confirmFirstDeployCollisions(ctx, client, contextClient, name, deploymentAgent, source, opts.Yes)
	if err != nil {
		return failed, err
	}
	if !proceed {
		if err := ui.OutroCancel("Cancelled — existing resources left unchanged."); err != nil {
			return errs.Cancelled, err
		}
		return errs.Cancelled, nil
	}

	// -- 3. Sync deploy-owned context to Context Hub -----------------------
	if err := ctx.Err(); err != nil {
		return failed, err
	}
	synced, err := syncContextHub(ctx, contextClient, p, name, memory.AgentEnabled(), opts.ContextStrategy)
	if err != nil {
		return failed, err
	}

	// -- 4. Compile --------------------------------------------------------
	if err := ctx.Err(); err != nil {
		return failed, err
	}
	outDir := filepath.Join(p.Root, ".mda", "build")
	summary, err := compileForDeployStep(ctx, p, outDir, synced, installation)
	if err != nil {
		return failed, err
	}

	// -- 4a. Bake sandbox recipe snapshot -----------------------------------
	if err := ctx.Err(); err != nil {
		return failed, err
	}
	// Fail closed before create/upload when setup exits non-zero or the
	// snapshot never becomes ready. A plain sandbox declaration uses Host's
	// default image and does not need a recipe snapshot.
	recipeSpec, err := recipeSpecFromProject(p, outDir)
	if err != nil {
		return failed, err
	}
	activeRecipeName := ""
	if recipeSpec != nil {
		outcome, registryPasswordEnv, err := bakeSandboxRecipeStep(ctx, client, name, envVars, recipeSpec)
		if err != nil {
			return failed, err
		}
		deployment.EnsureRecipeSnapshotSecret(&secrets, outcome.RecipeReference)
		// Registry credentials are bake-only. Never forward the pull token to
		// the deployed Agent Server after Host has stored it.
		if registryPasswordEnv != "" {
			secrets = slices.DeleteFunc(secrets, func(secret hostapi.Secret) bool {
				return secret.Name == registryPasswordEnv
			})
		}
		activeRecipeName = outcome.RecipeName
	} else {
		deployment.ClearRecipeSnapshotSecret(&secrets)
	}

	// -- 4b. Verify connector provider secrets ------------------------------
	if err := ctx.Err(); err != nil {
		return failed, err
	}
	// Requirements come from branded connector plugins via live-module emit
	// (`requirementsFromConnectors` / `requirements_from_connectors`), not
	// from a CLI helper-name allowlist. Reuse connector paths from compile so
	// `connectors/` is not walked again for names.
	emitted, err := emitRequirementsStep(ctx, p, outDir, summary)
	if err != nil {
		return failed, err
	}
	if err := ensureWorkspaceConnections(ctx, client, workspaceID, emitted.connectors, emitted.channels, emitted.mcpConnections); err != nil {
		return failed, err
	}
	if err := warnUnprovidedReservedConnections(emitted.connectors, emitted.channels); err != nil {
		return failed, err
	}
	channelSetup, err := channels.Prepare(emitted.channels, opts.Wait, p.Root, p.Files, p.Language, assistantID)
	if err != nil {
		return failed, err
	}
	findTarget := func(ctx context.Context) (map[string]any, error) {
		return deployment.FindTargetDeployment(ctx, client, name, deploymentAgent)
	}
	if err := channelSetup.Preflight(ctx, client, findTarget, envVars); err != nil {
		return failed, err
	}

	if err := preflight.ensureConnectorRequirementsSecrets(ctx, emitted.connectors); err != nil {
		return failed, err
	}
	if err := preflight.ensureChannelRequirementsSecrets(ctx, emitted.channels); err != nil {
		return failed, err
	}

	externalImage := ""
	if source.Kind == ImageSourceCustomerRegistry {
		sp := ui.NewSpinner()
		sp.Start("Building and pushing deployment image")
		image, err := step(sp, "Could not build and push the deployment image", func() (string, error) {
			return buildAndPush(ctx, p.Language, outDir, source.Target)
		})
		if err != nil {
			return failed, err
		}
		sp.Stop("Built and pushed deployment image")
		externalImage = image
	}

	// -- 5. Resolve / create deployment ------------------------------------
	if err := ctx.Err(); err != nil {
		return failed, err
	}
	var (
		deploymentID    string
		createdExternal map[string]any
	)
	switch {
	case externalImage != "":
		deploymentID, createdExternal, err = resolveOrCreateExternalDeployment(ctx, client, name, deploymentAgent, externalImage, secrets)
	case deploymentAgent != nil:
		deploymentID, err = resolveOrCreateAgentDeployment(ctx, client, *deploymentAgent, opts.DeploymentType, secrets)
	default:
		deploymentID, err = resolveOrCreateDeployment(ctx, client, name, opts.DeploymentType, secrets)
	}
	if err != nil {
		return failed, err
	}
	if err := linkContextRepoToAgent(ctx, client, contextClient, deploymentID, name); err != nil {
		return failed, err
	}
	// Slack OAuth redirect_uri needs the public Agent Server URL. Inject it
	// when the deployment already has a runtime URL (revises / existing
	// deploys).
	existing, err := client.GetDeployment(ctx, deploymentID)
	if err != nil {
		slog.Debug("deployment read after create/update failed", "error", err)
		existing = nil
	}
	reportNonInteractiveDeploymentID(deploymentID)
	if existing != nil {
		if runtimeURL := deployment.DeploymentRuntimeURL(existing); runtimeURL != "" {
			deployment.EnsurePublicAPIURL(&secrets, runtimeURL)
		}
	}

	// The deployment exists now, so the dashboard can already show the build:
	// hand over the link here rather than making the developer wait out the
	// build for it. An unscoped credential resolved its workspace server-side,
	// and this object is the first thing to name it.
	if target.Workspace == nil && existing != nil {
		if tenantID := deployment.DeploymentTenantID(existing); tenantID != "" {
			if err := ui.LogInfo("Workspace: " + target.LabelFor(tenantID)); err != nil {
				return failed, err
			}
		}
	}
	statusURL := ""
	if dashboardURL != "" && existing != nil {
		statusURL = deployment.DeploymentStatusURL(existing, deploymentID, dashboardURL)
	}
	if statusURL != "" {
		if err := ui.NoteLinks("Deployment dashboard", []ui.Link{{URL: statusURL}}); err != nil {
			return failed, err
		}
	}

	// -- 6. Supply source or image and trigger the revision ----------------
	if err := ctx.Err(); err != nil {
		return failed, err
	}
	var updated map[string]any
	switch {
	case externalImage != "" && createdExternal != nil:
		updated = createdExternal
	case externalImage != "":
		sp := ui.NewSpinner()
		sp.Start("Updating deployment image")
		updated, err = step(sp, "Could not update the deployment", func() (map[string]any, error) {
			return client.UpdateDeploymentExternalImage(ctx, deploymentID, externalImage, secrets)
		})
		if err != nil {
			return failed, err
		}
		sp.Stop("Deployment image updated")
	default:
		if updated, err = archiveUploadAndTrigger(ctx, client, outDir, deploymentID, secrets); err != nil {
			return failed, err
		}
	}
	// The build response names the tenant too. When the read above failed,
	// this is where the link first becomes available, so print it here: the
	// reports below assume a knowable link has already been handed over.
	if statusURL == "" && dashboardURL != "" {
		statusURL = deployment.DeploymentStatusURL(updated, deploymentID, dashboardURL)
		if statusURL != "" {
			if err := ui.NoteLinks("Deployment dashboard", []ui.Link{{URL: statusURL}}); err != nil {
				return failed, err
			}
		}
	}

	// -- 7. Wait + report --------------------------------------------------
	if err := ctx.Err(); err != nil {
		return failed, err
	}
	if !opts.Wait {
		if len(summary.ScheduleCrons) > 0 {
			if err := ui.LogWarning("Schedules were discovered, but --no-wait skips cron reconciliation until deploy completes."); err != nil {
				return failed, err
			}
		}
		if err := ui.Outro(fmt.Sprintf("Build triggered for '%s'. Not waiting (--no-wait).", name)); err != nil {
			return failed, err
		}
		return errs.Succeeded, nil
	}

	agentServerURL, err := pollAndReport(ctx, client, deploymentID, name, statusURL, summary.ScheduleCrons, opts.WaitTimeout)
	if err != nil {
		return failed, err
	}
	if err := requireTriggerRegistrationURL(agentServerURL, channelSetup.NeedsLiveURL()); err != nil {
		return failed, err
	}
	if agentServerURL == "" {
		return errs.Succeeded, nil
	}
	if err := ui.NoteLinksWithSection(
		"Deployment",
		deploymentLinks(agentServerURL, statusURL),
		"HTTP channels",
		httpChannelLinks(agentServerURL, emitted.channels),
	); err != nil {
		return failed, err
	}
	if err := channelSetup.SetUp(ctx, channels.SetupTarget{
		Client:         client,
		AgentServerURL: agentServerURL,
		DeploymentID:   deploymentID,
		AssistantID:    assistantID,
		ProjectRoot:    p.Root,
		EnvVars:        envVars,
	}); err != nil {
		return failed, err
	}
	cleanup, err := deleteStaleRecipeSnapshots(ctx, client, name, activeRecipeName)
	if err != nil {
		if err := ui.LogWarning(fmt.Sprintf("Deployment is live, but stale sandbox recipe snapshots could not be cleaned up: %v", err)); err != nil {
			return failed, err
		}
	} else if note, ok := cleanup.InUseNote(); ok {
		if err := ui.LogInfo(note); err != nil {
			return failed, err
		}
	}
	if err := ui.Outro(fmt.Sprintf("Deployed '%s' to LangSmith.", name)); err != nil {
		return failed, err
	}
	return errs.Succeeded, nil
}

// requireTriggerRegistrationURL fails when a channel needs the live Agent
// Server URL to finish setup but no revision was tracked to find it.
func requireTriggerRegistrationURL(agentServerURL string, needsLiveURL bool) error {
	if needsLiveURL && agentServerURL == "" {
		return errors.New("Deployment started, but channel setup could not complete because no revision was available to track; retry `langsmith deploy` once the deployment is live.")
	}
	return nil
}

// importProjectModules imports the project's modules under a spinner, which
// says so when the introspection helper installs itself first.
func importProjectModules(ctx context.Context, p *project.Project, envVars map[string]string) error {
	sp := ui.NewSpinner()
	started := false
	err := introspect.ImportProject(ctx, p, introspect.ImportOptions{
		Env: envVars,
		Progress: func(stage introspect.ImportStage) {
			if started {
				sp.SetMessage(stage.Message())
				return
			}
			started = true
			sp.Start(stage.Message())
		},
	})
	if !started {
		return err
	}
	if err := closeFailedStep(sp, "Could not import the project's modules", err); err != nil {
		return err
	}
	sp.Stop("Imported project modules")
	return nil
}

// step closes the spinner sp when run fails: in an error state with
// failMessage (so the session line is closed cleanly), or a cancelled one when
// the user stopped it.
func step[T any](sp *ui.Spinner, failMessage string, run func() (T, error)) (T, error) {
	value, err := run()
	return value, closeFailedStep(sp, failMessage, err)
}

// closeFailedStep is [step] for a step that returns only an error.
func closeFailedStep(sp *ui.Spinner, failMessage string, err error) error {
	switch {
	case err == nil:
	case errs.IsCancellation(err):
		sp.Cancel("Cancelled")
	default:
		sp.Error(failMessage)
	}
	return err
}

// bakeSpinnerTitle is the spinner title of the sandbox bake step; phases
// render beneath it.
const bakeSpinnerTitle = "Baking sandbox recipe snapshot"

// bakeSpinnerMessage is the bake spinner text: the stable title plus a dim
// sub-line for the running phase, aligned with the session's vertical bar.
func bakeSpinnerMessage(phase BakePhase) string {
	dim := ui.DimStyle(ui.ColorsEnabled())
	return bakeSpinnerTitle + "\n" + dim.Apply("│") + "  " + dim.Apply(phase.Label())
}

// bakeProgress is a progress callback for [bakeRecipe] that rewrites the
// spinner sub-line in place. Without a terminal the spinner prints each
// phase once instead.
func bakeProgress(sp *ui.Spinner) func(BakePhase) {
	return func(phase BakePhase) {
		sp.SetMessage(bakeSpinnerMessage(phase))
	}
}

// firstDeployCollisions are the same-name resources a first deploy would
// overwrite.
type firstDeployCollisions struct {
	contextHubRepo      bool
	langsmithDeployment bool
}

func newFirstDeployCollisions(existingDeployment map[string]any, contextHubRepoIsForeign bool) firstDeployCollisions {
	// A redeploy only ever overwrites resources this deployment already owns,
	// so there is nothing to confirm.
	if existingDeployment != nil && deployment.DeploymentIsManagedDeepAgent(existingDeployment) {
		return firstDeployCollisions{}
	}
	return firstDeployCollisions{
		contextHubRepo:      contextHubRepoIsForeign,
		langsmithDeployment: existingDeployment != nil,
	}
}

// collisionDecision is what to do about same-name resources a first deploy
// would overwrite.
type collisionDecision int

const (
	collisionDecisionProceed collisionDecision = iota
	collisionDecisionPrompt
)

// decideCollisions answers the overwrite question: `--yes` answers it up
// front; without it, only a terminal can, so an unattended run fails with the
// flag that would have answered it.
func decideCollisions(name string, collisions firstDeployCollisions, yes, isInteractive bool) (collisionDecision, error) {
	if collisions == (firstDeployCollisions{}) || yes {
		return collisionDecisionProceed, nil
	}
	if isInteractive {
		return collisionDecisionPrompt, nil
	}
	return 0, errs.Categorized(errs.UserInput, fmt.Sprintf(
		"Resources for deployment '%s' already exist and require confirmation "+
			"before they can be overwritten. Re-run with --yes to overwrite them, or pass "+
			"--name to deploy under a different name.", name))
}

func confirmFirstDeployCollisions(
	ctx context.Context,
	client *hostapi.Client,
	contextClient *hostapi.ContextHubClient,
	name string,
	agent *hostapi.DeploymentAgent,
	source ImageSource,
	yes bool,
) (bool, error) {
	sp := ui.NewSpinner()
	sp.Start(fmt.Sprintf("Checking for existing resources named '%s'", name))
	existing, err := step(sp, "Could not check existing resources", func() (map[string]any, error) {
		return deployment.FindTargetDeployment(ctx, client, name, agent)
	})
	if err != nil {
		return false, err
	}
	if existing != nil {
		existing = deployment.FetchDeploymentWithMarker(ctx, client, existing)
	}
	// Source types are immutable. Reject an incompatible image deploy before
	// inspecting or syncing Context Hub so a deploy that cannot succeed leaves
	// every remote resource unchanged.
	if existing != nil {
		if err := ensureRequestedDeploymentSource(name, existing, source); err != nil {
			sp.Error("Existing deployment uses a different source type")
			return false, err
		}
	}
	// Short-circuit the redeploy path before paying for the Context Hub
	// lookup; newFirstDeployCollisions re-checks the same marker as the
	// invariant.
	if existing != nil && deployment.DeploymentIsManagedDeepAgent(existing) {
		sp.Stop("Existing Managed Deep Agent deployment found")
		return true, nil
	}
	// A repo `langsmith deploy` created is not a collision even on a first
	// deploy — most often it is the leftover of an earlier deploy that failed
	// after the Context Hub sync but before the deployment was created, and
	// blocking that retry would wedge the very workflow this gate exists to
	// protect.
	contextHubRepo, err := step(sp, "Could not check existing resources", func() (hostapi.RepoOwnership, error) {
		return contextClient.RepoOwnership(ctx, name)
	})
	if err != nil {
		return false, err
	}
	sp.Stop("Checked for existing resources")

	collisions := newFirstDeployCollisions(existing, contextHubRepo == hostapi.RepoOwnershipForeign)
	if collisions == (firstDeployCollisions{}) {
		return true, nil
	}
	if collisions.contextHubRepo {
		if err := ui.LogWarning(fmt.Sprintf(
			"Context Hub repo '%s' already exists and was not created by `langsmith deploy`. Deploying overwrites its instructions and deletes everything under skills/ that this project does not define (memories are preserved).",
			name)); err != nil {
			return false, err
		}
	}
	if collisions.langsmithDeployment {
		deploymentName, ok := existing["name"].(string)
		if !ok {
			deploymentName = name
		}
		if err := ui.LogWarning(fmt.Sprintf(
			"First Managed Deep Agent deploy will overwrite the source and configuration of existing LangSmith deployment '%s'.",
			deploymentName)); err != nil {
			return false, err
		}
	}
	decision, err := decideCollisions(name, collisions, yes, ui.Interactive())
	if err != nil {
		return false, err
	}
	if decision == collisionDecisionProceed {
		return true, nil
	}
	return ui.Confirm(fmt.Sprintf("Overwrite existing resources for '%s' and continue?", name)).
		InitialValue(false).
		Interact(ctx)
}

func syncContextHub(
	ctx context.Context,
	contextClient *hostapi.ContextHubClient,
	p *project.Project,
	name string,
	memoryEnabled bool,
	strategy *ContextStrategy,
) (syncSummary, error) {
	sp := ui.NewSpinner()
	sp.Start(fmt.Sprintf("Checking Context Hub context '%s'", name))
	plan, err := step(sp, "Could not inspect Context Hub context", func() (*syncPlan, error) {
		return prepareContextSync(ctx, contextClient, p, name, memoryEnabled)
	})
	if err != nil {
		return syncSummary{}, err
	}
	sp.Stop("Checked Context Hub context")

	conflicts := plan.Conflicts()
	resolution, err := resolveContextConflicts(ctx, conflicts, strategy)
	if err != nil {
		return syncSummary{}, err
	}
	preserved := resolution == conflictResolutionKeepHub && len(conflicts) > 0

	sp = ui.NewSpinner()
	sp.Start(fmt.Sprintf("Syncing Context Hub context '%s'", name))
	synced, err := step(sp, "Could not sync Context Hub context", func() (syncSummary, error) {
		return applyContextSync(ctx, contextClient, plan, resolution)
	})
	if err != nil {
		return syncSummary{}, err
	}
	if synced.Changed {
		sp.Stop(fmt.Sprintf("Synced Context Hub context (%s)", name))
	} else {
		sp.Stop("Context Hub context already current")
	}
	if preserved {
		if err := ui.LogWarning("Preserved Context Hub edits; local context was not applied to those sections."); err != nil {
			return syncSummary{}, err
		}
	}
	if synced.HadLocalMemories {
		if err := ui.LogWarning("Local memories/ is not deploy-owned. Agent memory lives in Context Hub and will not be overwritten by deploy."); err != nil {
			return syncSummary{}, err
		}
	}
	// Memory is opt-in, so a project that stops declaring it keeps whatever
	// the Hub already holds — untouched, but unmounted. Say so rather than
	// letting durable knowledge go quiet. Paths only; contents are never read
	// here.
	if synced.OrphanedHubMemoryCount > 0 {
		if err := ui.LogWarning(fmt.Sprintf(
			"This project declares no memory, so %d existing Context Hub "+
				"memory file(s) will not be mounted (they are preserved, not deleted). "+
				"Add memory.%s with `%s` to mount them again.",
			synced.OrphanedHubMemoryCount, p.Language.Extension(), introspect.DeclareMemorySnippet(p.Language))); err != nil {
			return syncSummary{}, err
		}
	}
	return synced, nil
}

func resolveContextConflicts(ctx context.Context, conflicts []contextSection, strategy *ContextStrategy) (conflictResolution, error) {
	if len(conflicts) == 0 {
		return conflictResolutionOverwrite, nil
	}
	if strategy != nil {
		if *strategy == ContextStrategyKeepHub {
			return conflictResolutionKeepHub, nil
		}
		return conflictResolutionOverwrite, nil
	}

	labels := make([]string, len(conflicts))
	for index, section := range conflicts {
		labels[index] = section.Label()
	}
	changed := strings.Join(labels, " and ")
	if !ui.Interactive() {
		return 0, fmt.Errorf("Context Hub %s changed since the last MDA sync. Re-run with "+
			"--context-strategy overwrite or --context-strategy keep-hub.", changed)
	}

	if err := ui.LogWarning(fmt.Sprintf("Context Hub %s changed since the last MDA sync.", changed)); err != nil {
		return 0, err
	}
	overwrite, err := ui.Confirm(fmt.Sprintf("Overwrite Hub-edited %s with the local version?", changed)).
		InitialValue(false).
		Interact(ctx)
	if err != nil {
		return 0, err
	}
	if overwrite {
		return conflictResolutionOverwrite, nil
	}
	return conflictResolutionKeepHub, nil
}

func compileForDeployStep(ctx context.Context, p *project.Project, outDir string, synced syncSummary, installation *project.Installation) (*compile.CompileSummary, error) {
	deployContext := compile.DeployContext{
		Repo:      synced.Repo,
		HasSkills: synced.HasSkills,
	}
	sp := ui.NewSpinner()
	sp.Start("Compiling project")
	summary, err := step(sp, "Could not compile the project", func() (*compile.CompileSummary, error) {
		summary, err := compile.CompileForDeploy(ctx, p, outDir, deployContext, installation)
		if err != nil {
			return nil, compile.Compilation(err)
		}
		return summary, nil
	})
	if err != nil {
		return nil, err
	}
	sp.Stop(fmt.Sprintf("Compiled %d file(s) into .mda/build", len(summary.Copied)))
	// After Stop: writing during the spinner would race its redraw.
	for _, warning := range summary.Warnings {
		if err := ui.LogWarning(warning); err != nil {
			return nil, err
		}
	}
	return summary, nil
}

// emittedRequirements are the requirements emitted from the compiled
// connector and channel modules.
type emittedRequirements struct {
	connectors     compile.ConnectorRequirements
	mcpConnections []compile.McpConnectionEntry
	channels       compile.ChannelRequirements
}

// emitRequirementsStep writes the connector, MCP, and channel requirements the
// project import captured into outDir and reports what it resolved. Without
// connectors or channels it only clears stale manifests and reports nothing.
func emitRequirementsStep(ctx context.Context, p *project.Project, outDir string, summary *compile.CompileSummary) (emittedRequirements, error) {
	emit := func() (emittedRequirements, error) {
		connectors, err := introspect.EmitConnectorRequirementsForEntries(ctx, p, outDir, summary.Connectors)
		if err != nil {
			return emittedRequirements{}, err
		}
		mcpConnections, err := introspect.EmitMcpConnectionEntriesForEntries(ctx, p, outDir, summary.Connectors)
		if err != nil {
			return emittedRequirements{}, err
		}
		channelRequirements, err := introspect.EmitChannelRequirementsForEntries(ctx, p, outDir, summary.Channels)
		if err != nil {
			return emittedRequirements{}, err
		}
		return emittedRequirements{
			connectors:     connectors,
			mcpConnections: mcpConnections,
			channels:       channelRequirements,
		}, nil
	}
	var modules []string
	if len(summary.Connectors) > 0 {
		modules = append(modules, fmt.Sprintf("%d connector(s)", len(summary.Connectors)))
	}
	if len(summary.Channels) > 0 {
		modules = append(modules, fmt.Sprintf("%d channel(s)", len(summary.Channels)))
	}
	if len(modules) == 0 {
		return emit()
	}
	sp := ui.NewSpinner()
	sp.Start("Resolving connector and channel requirements")
	emitted, err := step(sp, "Could not resolve connector and channel requirements", emit)
	if err != nil {
		return emittedRequirements{}, err
	}
	sp.Stop("Resolved requirements for " + strings.Join(modules, " and "))
	return emitted, nil
}

// bakeSandboxRecipeStep bakes (or reuses) the sandbox recipe snapshot and
// reports progress. It also returns the registry password env var the bake
// consumed, "" for none.
func bakeSandboxRecipeStep(ctx context.Context, client *hostapi.Client, deploymentName string, envVars map[string]string, spec *RecipeSpec) (BakeOutcome, string, error) {
	registryPasswordEnv := ""
	if spec.Registry != nil {
		registryPasswordEnv = spec.Registry.PasswordEnv
	}
	bakeEnv := bakeEnvVars(envVars)
	sp := ui.NewSpinner()
	sp.Start(bakeSpinnerTitle)
	outcome, err := step(sp, "Could not bake the sandbox recipe", func() (BakeOutcome, error) {
		return bakeRecipe(ctx, client, deploymentName, spec, bakeEnv, bakeProgress(sp))
	})
	if err != nil {
		return BakeOutcome{}, "", err
	}
	if outcome.Skipped {
		sp.Stop(fmt.Sprintf("Reusing ready recipe snapshot '%s'", outcome.RecipeName))
	} else {
		sp.Stop(fmt.Sprintf("Baked recipe snapshot '%s'", outcome.RecipeName))
	}
	return outcome, registryPasswordEnv, nil
}

// recipeSpecFromProject builds a [RecipeSpec] from the project setup script
// and compile manifest; nil when the build has no recipe.
func recipeSpecFromProject(p *project.Project, outDir string) (*RecipeSpec, error) {
	manifest, err := compile.LoadSandboxRecipeManifest(outDir)
	if err != nil || manifest == nil {
		return nil, err
	}
	setup, _, err := introspect.ReadSetupScript(p)
	if err != nil {
		return nil, err
	}
	return &RecipeSpec{
		SetupScript:     setup,
		SnapshotName:    manifest.SnapshotName,
		SnapshotID:      manifest.SnapshotID,
		DockerImage:     manifest.DockerImage,
		Registry:        manifest.Registry,
		FsCapacityBytes: manifest.FsCapacityBytes,
	}, nil
}

// bakeEnvVars are the non-reserved env vars forwarded onto the bake builder
// box.
func bakeEnvVars(envVars map[string]string) map[string]string {
	forwarded := map[string]string{}
	for name, value := range envVars {
		if value != "" && !slices.Contains(deployment.ReservedEnvVars, name) {
			forwarded[name] = value
		}
	}
	return forwarded
}

// resolveOrCreateDeployment looks up a LangSmith deployment by name, or
// creates one when missing.
func resolveOrCreateDeployment(ctx context.Context, client *hostapi.Client, name string, deploymentType hostapi.DeploymentType, secrets []hostapi.Secret) (string, error) {
	sp := ui.NewSpinner()
	sp.Start(fmt.Sprintf("Looking up deployment '%s'", name))
	listed, err := step(sp, "Could not reach LangSmith", func() (map[string]any, error) {
		return client.ListDeployments(ctx, name)
	})
	if err != nil {
		return "", err
	}
	if id := deployment.FindDeploymentIDByName(listed, name); id != "" {
		sp.Stop("Found existing deployment " + id)
		return id, nil
	}
	sp.Stop("No existing deployment — creating a new one")
	sp = ui.NewSpinner()
	sp.Start(fmt.Sprintf("Creating deployment '%s'", name))
	created, err := client.CreateDeployment(ctx, name, deploymentType, secrets)
	switch {
	case err == nil:
		id, err := step(sp, "Could not create the deployment", func() (string, error) {
			return hostapi.RequireStr(created, "id", "create deployment")
		})
		if err != nil {
			return "", err
		}
		sp.Stop("Created deployment " + id)
		return id, nil
	case deployment.IsRequestTimeout(err):
		// The host backend may still finish provisioning even when the client
		// times out waiting for the response.
		sp.Stop("Create request timed out — checking whether the deployment exists")
		sp = ui.NewSpinner()
		sp.Start(fmt.Sprintf("Looking up deployment '%s'", name))
		listed, listErr := step(sp, "Could not reach LangSmith", func() (map[string]any, error) {
			return client.ListDeployments(ctx, name)
		})
		if listErr != nil {
			return "", listErr
		}
		if id := deployment.FindDeploymentIDByName(listed, name); id != "" {
			sp.Stop(fmt.Sprintf("Found deployment %s (created despite timeout)", id))
			return id, nil
		}
		sp.Error("Could not create the deployment")
		return "", err
	default:
		return "", closeFailedStep(sp, "Could not create the deployment", err)
	}
}

func resolveOrCreateAgentDeployment(ctx context.Context, client *hostapi.Client, agent hostapi.DeploymentAgent, deploymentType hostapi.DeploymentType, secrets []hostapi.Secret) (string, error) {
	listed, err := client.ListAgentDeployments(ctx, agent)
	if err != nil {
		return "", err
	}
	found := deployment.FindAgentDeployment(listed)
	if found == nil {
		if found, err = client.CreateAgentDeployment(ctx, agent, deploymentType, secrets); err != nil {
			return "", err
		}
	}
	name, err := hostapi.RequireStr(found, "name", "agent deployment")
	if err != nil {
		return "", err
	}
	if err := ui.LogInfo("Deployment: " + name); err != nil {
		return "", err
	}
	return hostapi.RequireStr(found, "id", "agent deployment")
}

// resolveOrCreateExternalDeployment finds or creates the deployment of an
// external image. It returns the created deployment, nil when it already
// existed.
func resolveOrCreateExternalDeployment(ctx context.Context, client *hostapi.Client, name string, agent *hostapi.DeploymentAgent, imageURI string, secrets []hostapi.Secret) (string, map[string]any, error) {
	sp := ui.NewSpinner()
	sp.Start("Checking for an existing deployment")
	existing, err := step(sp, "Could not check existing deployments", func() (map[string]any, error) {
		if agent != nil {
			listed, err := client.ListAgentDeployments(ctx, *agent)
			if err != nil {
				return nil, err
			}
			return deployment.FindAgentDeployment(listed), nil
		}
		listed, err := client.ListDeployments(ctx, name)
		if err != nil {
			return nil, err
		}
		return deployment.FindDeploymentByName(listed, name), nil
	})
	if err != nil {
		return "", nil, err
	}

	if existing != nil {
		id, err := step(sp, "Existing deployment has no id", func() (string, error) {
			return hostapi.RequireStr(existing, "id", "existing deployment")
		})
		if err != nil {
			return "", nil, err
		}
		if err := ensureExternalDeploymentSource(name, existing); err != nil {
			sp.Error("Existing deployment uses a different source type")
			return "", nil, err
		}
		sp.Stop("Found deployment " + id)
		return id, nil, nil
	}

	sp.Stop("No existing deployment — creating a new one")
	sp = ui.NewSpinner()
	sp.Start(fmt.Sprintf("Creating deployment '%s'", name))
	created, err := step(sp, "Could not create the deployment", func() (map[string]any, error) {
		if agent != nil {
			return client.CreateExternalAgentDeployment(ctx, *agent, imageURI, secrets)
		}
		return client.CreateExternalDeployment(ctx, name, imageURI, secrets)
	})
	if err != nil {
		return "", nil, err
	}
	id, err := step(sp, "Could not create the deployment", func() (string, error) {
		return hostapi.RequireStr(created, "id", "create deployment")
	})
	if err != nil {
		return "", nil, err
	}
	sp.Stop("Created deployment " + id)
	return id, created, nil
}

func ensureExternalDeploymentSource(name string, existing map[string]any) error {
	source, ok := existing["source"].(string)
	if ok && source == "external_docker" {
		return nil
	}
	if !ok {
		source = "unknown"
	}
	return fmt.Errorf("deployment '%s' uses source '%s'; deployment source types cannot be changed, so delete it or use a different --name", name, source)
}

func ensureRequestedDeploymentSource(name string, existing map[string]any, source ImageSource) error {
	if source.Kind == ImageSourceCustomerRegistry {
		return ensureExternalDeploymentSource(name, existing)
	}
	return nil
}

// uploadTarget is where a source archive goes: the signed URL to upload it
// to, and the object path the build is then triggered with.
type uploadTarget struct {
	signedURL  string
	objectPath string
}

func requestUploadTarget(ctx context.Context, client *hostapi.Client, deploymentID string) (uploadTarget, error) {
	upload, err := client.RequestUploadURL(ctx, deploymentID)
	if err != nil {
		return uploadTarget{}, err
	}
	signedURL, err := hostapi.RequireStr(upload, "upload_url", "upload URL")
	if err != nil {
		return uploadTarget{}, err
	}
	objectPath, err := hostapi.RequireStr(upload, "object_path", "upload URL")
	if err != nil {
		return uploadTarget{}, err
	}
	return uploadTarget{signedURL: signedURL, objectPath: objectPath}, nil
}

func archiveUploadAndTrigger(ctx context.Context, client *hostapi.Client, outDir, deploymentID string, secrets []hostapi.Secret) (map[string]any, error) {
	sp := ui.NewSpinner()
	sp.Start("Creating source archive")
	archive, err := step(sp, "Could not archive the build", func() (*Archive, error) {
		return CreateArchive(outDir)
	})
	if err != nil {
		return nil, err
	}
	sp.Stop(fmt.Sprintf("Archived build (%.1f KB)", float64(len(archive.Bytes))/1024.0))

	sp = ui.NewSpinner()
	sp.Start("Requesting upload URL")
	upload, err := step(sp, "Could not request an upload URL", func() (uploadTarget, error) {
		return requestUploadTarget(ctx, client, deploymentID)
	})
	if err != nil {
		return nil, err
	}
	sp.Stop("Received upload URL")

	sp = ui.NewSpinner()
	sp.Start("Uploading source")
	if err := closeFailedStep(sp, "Could not upload the source", uploadToGCS(ctx, upload.signedURL, archive.Bytes)); err != nil {
		return nil, err
	}
	sp.Stop("Uploaded source")

	sp = ui.NewSpinner()
	sp.Start("Triggering remote build")
	updated, err := step(sp, "Could not trigger the remote build", func() (map[string]any, error) {
		return client.UpdateDeploymentInternalSource(ctx, deploymentID, upload.objectPath, archive.ConfigRel, secrets)
	})
	if err != nil {
		return nil, err
	}
	sp.Stop("Remote build triggered")
	return updated, nil
}

// ensureIngressSecret ensures `MDA_INGRESS_SECRET` will be available when the
// project's identity uses `trusted_backend` ingress, failing the deploy before
// any build work otherwise.
//
// A deployment without this secret accepts no authenticated traffic (auth
// fails closed with 401), so shipping one is never useful. Unlike the
// model-key check, a failed workspace lookup does not soft-pass — the secret
// must be positively confirmed.
func (s *secretPreflight) ensureIngressSecret(ctx context.Context, p *project.Project) error {
	// One identity read (auth + scope share it); compile reuses the same helper.
	required, err := introspect.IdentityRequiresIngressSecret(ctx, p)
	if err != nil || !required {
		return err
	}
	return s.ensureRequiredSecret(ctx, introspect.IngressSecretEnv,
		"This project declares identity with trusted_backend ingress, which requires an ingress "+
			"secret (generate with `openssl rand -hex 32`; clients send it as `x-mda-ingress-secret`)")
}

// ensureConnectorRequirementsSecrets ensures connector provider secrets from
// compiled requirements' `requiredEnv`.
func (s *secretPreflight) ensureConnectorRequirementsSecrets(ctx context.Context, connectorRequirements compile.ConnectorRequirements) error {
	if len(connectorRequirements) == 0 {
		return nil
	}
	for _, keyEnv := range compile.RequiredEnvFromConnectorRequirements(connectorRequirements) {
		if err := s.ensureRequiredSecret(ctx, keyEnv,
			"This project declares connectors/ with requirements, which requires provider secrets from each connector's requirements"); err != nil {
			return err
		}
	}
	return nil
}

// ensureWorkspaceConnections creates missing user-owned MCP connections
// through discovery + DCR, then verifies every remaining connector connection
// exists in the workspace.
func ensureWorkspaceConnections(
	ctx context.Context,
	client *hostapi.Client,
	workspaceID string,
	connectorRequirements compile.ConnectorRequirements,
	channelRequirements compile.ChannelRequirements,
	mcpConnections []compile.McpConnectionEntry,
) error {
	if len(compile.ReferencedConnectionSlugs(connectorRequirements)) == 0 {
		return nil
	}
	// Workspace scope is carried on the client (tenant header). Require the ID
	// up front so deploy fails with a clear local error instead of a 401/403.
	if workspaceID == "" {
		return fmt.Errorf("connector connections are declared, but no workspace ID is configured. "+
			"Set %s or pass --workspace so deploy can verify connections.", deployment.WorkspaceIDEnv)
	}
	listed, err := client.ListWorkspaceConnections(ctx)
	if err != nil {
		return err
	}
	existing := map[string]hostapi.ConnectionSummary{}
	for _, listedConnection := range listed {
		existing[listedConnection.Slug] = listedConnection
	}
	provided := compile.ChannelProvidedConnections(channelRequirements)
	missing, err := missingUserOwnedMcpConnections(connectorRequirements, provided, existing, mcpConnections)
	if err != nil {
		return err
	}
	for _, mcp := range missing {
		created, err := connection.CreateDiscoveredMcpConnection(ctx, client, mcp.Slug, mcp.URL)
		if err != nil {
			return err
		}
		if err := ui.LogSuccess(fmt.Sprintf("Created MCP connection '%s' through OAuth discovery", mcp.Slug)); err != nil {
			return err
		}
		existing[mcp.Slug] = created
	}
	return validateWorkspaceConnectionsForManifest(connectorRequirements, provided, existing)
}

// warnUnprovidedReservedConnections warns about `mda/` agent connections the
// project reads that none of its channels provides: deploy goes on, but
// reading one fails at runtime.
func warnUnprovidedReservedConnections(connectorRequirements compile.ConnectorRequirements, channelRequirements compile.ChannelRequirements) error {
	for _, slug := range compile.UnprovidedReservedConnections(connectorRequirements, channelRequirements) {
		if err := ui.LogWarning(fmt.Sprintf(
			"'%s' is reserved for a channel's app, but no channel in this project provides "+
				"it, so reading it will fail. Add the channel that provides it, or use another "+
				"connection.", slug)); err != nil {
			return err
		}
	}
	return nil
}

// missingUserOwnedMcpConnections selects only missing, uniquely declared
// user-owned MCP connections.
//
// Other missing connection kinds remain for normal validation, while an
// ambiguous MCP slug fails before deploy creates any remote resources.
func missingUserOwnedMcpConnections(
	requirements compile.ConnectorRequirements,
	provided map[string]string,
	existing map[string]hostapi.ConnectionSummary,
	mcpConnections []compile.McpConnectionEntry,
) ([]compile.McpConnectionEntry, error) {
	if err := validateConnectionOwnership(requirements, provided); err != nil {
		return nil, err
	}
	var missing []compile.McpConnectionEntry
	for _, slug := range compile.UserConnectionsFromConnectorRequirements(requirements) {
		if _, ok := existing[slug]; ok {
			continue
		}
		var matches []compile.McpConnectionEntry
		for _, entry := range mcpConnections {
			if entry.Slug == slug && entry.OwnerPolicy == hostapi.ConnectionOwnerPolicyUser.String() {
				matches = append(matches, entry)
			}
		}
		switch len(matches) {
		case 0:
		case 1:
			missing = append(missing, matches[0])
		default:
			servers := make([]string, len(matches))
			for index, entry := range matches {
				servers[index] = entry.ConnectorName + "::" + entry.ServerName
			}
			return nil, fmt.Errorf("connection '%s' is used by multiple user-owned MCP servers in this "+
				"project: %s. Keep the slug unique per MCP server.", slug, strings.Join(servers, ", "))
		}
	}
	return missing, nil
}

// validateWorkspaceConnectionsForManifest verifies every connector connection
// slug declared in the manifest exists in the workspace.
//
// Presence only. Ownership is not checked here: the connections API does not
// report it on a listed row, and runtime ownership lives on credentials.
func validateWorkspaceConnectionsForManifest(
	requirements compile.ConnectorRequirements,
	provided map[string]string,
	existing map[string]hostapi.ConnectionSummary,
) error {
	if err := validateConnectionOwnership(requirements, provided); err != nil {
		return err
	}
	agentSlugs := compile.AgentConnectionsFromConnectorRequirements(requirements)
	userSlugs := compile.UserConnectionsFromConnectorRequirements(requirements)

	var missing []missingConnection
	for _, slug := range agentSlugs {
		if _, ok := existing[slug]; !ok {
			missing = append(missing, missingConnection{slug: slug, owner: "agent-owned"})
		}
	}
	for _, slug := range userSlugs {
		if _, ok := existing[slug]; !ok {
			missing = append(missing, missingConnection{slug: slug, owner: "user-owned"})
		}
	}
	if len(missing) == 0 {
		return nil
	}

	return fmt.Errorf("connections declared by this project are not in the workspace\n\n%s\n\nCreate "+
		"each with `mda connections create`.", formatMissingConnections(missing))
}

// validateConnectionOwnership rejects user-owned reserved slugs and slugs
// declared with both owners. provided is what
// [compile.ChannelProvidedConnections] returns.
func validateConnectionOwnership(requirements compile.ConnectorRequirements, provided map[string]string) error {
	agentSlugs := compile.AgentConnectionsFromConnectorRequirements(requirements)
	userSlugs := compile.UserConnectionsFromConnectorRequirements(requirements)
	for _, slug := range userSlugs {
		if message, ok := compile.ReservedUserConnectionError(slug, provided); ok {
			return errors.New(message)
		}
	}

	for _, slug := range agentSlugs {
		if slices.Contains(userSlugs, slug) {
			return fmt.Errorf("connection '%s' is declared as both agent-owned and user-owned in this project. "+
				"Use one owner per slug in connectors.", slug)
		}
	}
	return nil
}

// missingConnection is a declared connection the workspace lacks.
type missingConnection struct {
	slug  string
	owner string
}

// formatMissingConnections is one indented `'slug'  owner` line per
// connection, slug column padded so the owners line up.
func formatMissingConnections(missing []missingConnection) string {
	quoted := make([]string, len(missing))
	width := 0
	for index, entry := range missing {
		quoted[index] = "'" + entry.slug + "'"
		width = max(width, len(quoted[index]))
	}
	lines := make([]string, len(missing))
	for index, entry := range missing {
		lines[index] = fmt.Sprintf("  %-*s  %s", width, quoted[index], entry.owner)
	}
	return strings.Join(lines, "\n")
}

// ensureChannelRequirementsSecrets ensures channel provider secrets from
// compiled requirements' `requiredEnv`.
func (s *secretPreflight) ensureChannelRequirementsSecrets(ctx context.Context, channelRequirements compile.ChannelRequirements) error {
	required, optionalSlack := channelSecretPolicy(channelRequirements)
	slackInactive := false
	for _, keyEnv := range optionalSlack {
		available, err := s.secretIsAvailable(ctx, keyEnv)
		if err != nil {
			return err
		}
		if !available {
			slackInactive = true
		}
	}

	for _, keyEnv := range required {
		if err := s.ensureRequiredSecret(ctx, keyEnv,
			"This project declares channels/ with requirements, which requires provider secrets from each channel's requirements"); err != nil {
			return err
		}
	}
	if slackInactive {
		return ui.LogWarning("Slack is not configured yet. The agent will deploy with normal Slack events disabled.")
	}
	return nil
}

// channelSecretPolicy splits the channels' `requiredEnv` into the secrets a
// deploy requires and the Slack credentials it can do without, each sorted.
func channelSecretPolicy(channelRequirements compile.ChannelRequirements) (required, optionalSlack []string) {
	requiredSet := map[string]bool{}
	optionalSet := map[string]bool{}
	for _, manifest := range channelRequirements {
		object, _ := manifest.(map[string]any)
		provider, _ := object["provider"].(string)
		entries, ok := object["requiredEnv"].([]any)
		if !ok {
			continue
		}
		for _, entry := range entries {
			keyEnv, ok := entry.(string)
			if !ok {
				continue
			}
			if provider == "slack" && (keyEnv == "SLACK_SIGNING_SECRET" || keyEnv == "SLACK_BOT_TOKEN") {
				optionalSet[keyEnv] = true
			} else {
				requiredSet[keyEnv] = true
			}
		}
	}
	return sortedSet(requiredSet), sortedSet(optionalSet)
}

func sortedSet(set map[string]bool) []string {
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	slices.Sort(values)
	return values
}

// secretPreflight checks, before any build work, that the secrets a deploy
// needs will reach it: from the project `.env`, the secrets staged for this
// deploy, or LangSmith workspace secrets, whose names it lists at most once.
type secretPreflight struct {
	// envPath is the project `.env`, named in the guidance.
	envPath string
	envVars map[string]string
	// secrets are the deployment secrets staged so far.
	secrets      *[]hostapi.Secret
	client       *hostapi.Client
	dashboardURL string
	// workspaceKeys caches the workspace secret listing.
	workspaceKeys workspaceSecretKeys
}

// secretIsAvailable reports whether keyEnv is set locally or in the
// workspace; a workspace that cannot be checked does not count. Fails only
// when the user stopped the check.
func (s *secretPreflight) secretIsAvailable(ctx context.Context, keyEnv string) (bool, error) {
	if claimSecretFromLocalSources(keyEnv, s.envVars, *s.secrets) {
		return true, nil
	}
	status, err := s.verifyWorkspaceSecret(ctx, keyEnv)
	if errs.IsCancellation(err) {
		return false, err
	}
	return status == workspaceSecretPresent, nil
}

// ensureRequiredSecret requires a named secret from `.env` or LangSmith
// workspace secrets; reason starts the error.
func (s *secretPreflight) ensureRequiredSecret(ctx context.Context, keyEnv, reason string) error {
	if claimSecretFromLocalSources(keyEnv, s.envVars, *s.secrets) {
		return nil
	}

	return s.requireWorkspaceSecret(ctx, keyEnv, workspaceSecretCheck{
		found: keyEnv + " found in LangSmith workspace secrets",
		absent: func(dashboard string) error {
			return fmt.Errorf("%s. %s is not set in %s and not configured as a LangSmith "+
				"workspace secret.\n"+
				"Add `%s=...` to %s, or add it under Settings > Workspaces > Secrets "+
				"in LangSmith%s, then deploy again. Values exported in your shell "+
				"are not read.", reason, keyEnv, s.envPath, keyEnv, s.envPath, dashboard)
		},
		unverified: func(lookupErr error) (string, error) {
			return fmt.Sprintf("Could not verify %s (%v)", keyEnv, lookupErr),
				fmt.Errorf("%s. %s is not set in %s, and LangSmith workspace secrets "+
					"could not be checked (%v).\n"+
					"Add `%s=...` to %s, then deploy again. Values exported in your "+
					"shell are not read.", reason, keyEnv, s.envPath, lookupErr, keyEnv, s.envPath)
		},
	})
}

// workspaceSecretCheck is what [secretPreflight.requireWorkspaceSecret] says
// for each outcome.
type workspaceSecretCheck struct {
	// found is the step line when the workspace has the secret.
	found string
	// absent is the error when it does not; dashboard is " (<dashboard URL>)",
	// or "" when the URL is unknown.
	absent func(dashboard string) error
	// unverified is the step line and the error when the workspace could not
	// be checked.
	unverified func(lookupErr error) (string, error)
}

// requireWorkspaceSecret checks, under a spinner, that the workspace has
// keyEnv.
func (s *secretPreflight) requireWorkspaceSecret(ctx context.Context, keyEnv string, check workspaceSecretCheck) error {
	sp := ui.NewSpinner()
	sp.Start(fmt.Sprintf("Checking that %s is configured", keyEnv))

	status, lookupErr := s.verifyWorkspaceSecret(ctx, keyEnv)
	switch {
	case errs.IsCancellation(lookupErr):
		sp.Cancel("Cancelled")
		return lookupErr
	case status == workspaceSecretPresent:
		sp.Stop(check.found)
		return nil
	case status == workspaceSecretAbsent:
		sp.Error(keyEnv + " is not configured")
		dashboard := ""
		if s.dashboardURL != "" {
			dashboard = " (" + s.dashboardURL + ")"
		}
		return check.absent(dashboard)
	default:
		step, err := check.unverified(lookupErr)
		sp.Error(step)
		return err
	}
}

// modelAPIKeyRequirementForDeploy resolves the provider secret required by
// deploy preflight.
//
// Gateway models must either use an explicitly supported keyless identifier
// or resolve to a known workspace Provider Secret.
func modelAPIKeyRequirementForDeploy(model string) (project.ModelAPIKeyRequirement, error) {
	requirement := project.DeployModelAPIKeyRequirement(model)
	if requirement.Kind == project.UnresolvedLangsmithProvider {
		return project.ModelAPIKeyRequirement{}, fmt.Errorf("The agent model '%s' uses LangSmith Gateway, but its provider is not "+
			"supported. Use a supported `langsmith:<provider>/<model>` identifier, then "+
			"configure that provider under Settings > Integrations > Provider Secrets in "+
			"LangSmith before deploying.", model)
	}
	return requirement, nil
}

// ensureModelAPIKey ensures the agent's model provider API key will be
// available to the deployment at runtime, failing the deploy before any build
// work otherwise. model is nil for a dynamic entry.
//
// Direct model keys may come from `.env`. Gateway provider credentials must
// be positively confirmed in workspace Provider Secrets because local
// deployment secrets never reach the Gateway.
func (s *secretPreflight) ensureModelAPIKey(ctx context.Context, model *string) error {
	if model == nil {
		return nil
	}
	requirement, err := modelAPIKeyRequirementForDeploy(*model)
	if err != nil {
		return err
	}
	keyEnv := requirement.Secret
	switch requirement.Kind {
	case project.DeploymentSecret:
		if claimSecretFromLocalSources(keyEnv, s.envVars, *s.secrets) {
			return nil
		}
		return errors.New(missingModelAPIKeyMessage(*model, keyEnv, s.envPath, "then deploy again."))
	case project.WorkspaceProviderSecret:
	default:
		return nil
	}

	return s.requireWorkspaceSecret(ctx, keyEnv, workspaceSecretCheck{
		found: keyEnv + " found in LangSmith workspace Provider Secrets",
		absent: func(dashboard string) error {
			return fmt.Errorf("The agent model '%s' uses LangSmith Gateway and requires %s in "+
				"LangSmith workspace Provider Secrets. Add `%s` under Settings > "+
				"Integrations > Provider Secrets in LangSmith%s, then deploy again.",
				*model, keyEnv, keyEnv, dashboard)
		},
		unverified: func(lookupErr error) (string, error) {
			return "Could not check LangSmith workspace Provider Secrets",
				fmt.Errorf("The agent model '%s' uses LangSmith Gateway and requires %s in "+
					"LangSmith workspace Provider Secrets, but Provider Secrets could not be checked "+
					"(%v).", *model, keyEnv, lookupErr)
		},
	})
}

// missingModelAPIKeyMessage tells the author model needs keyEnv in env; next
// ends the sentence with what happens once they add it.
func missingModelAPIKeyMessage(model, keyEnv, env, next string) string {
	return fmt.Sprintf("The agent model '%s' requires %s in %s. Add `%s=...` to %s, %s",
		model, keyEnv, env, keyEnv, env, next)
}

// workspaceSecretKeys is the cached result of listing LangSmith workspace
// secret keys for one deploy.
type workspaceSecretKeys struct {
	state workspaceSecretKeysState
	// keys are the names, when loaded.
	keys []string
	// lookupErr is why the listing failed, when it did.
	lookupErr error
}

type workspaceSecretKeysState int

const (
	workspaceSecretKeysUnloaded workspaceSecretKeysState = iota
	workspaceSecretKeysLoaded
	workspaceSecretKeysLookupFailed
)

// workspaceSecretStatus is the outcome of checking whether a named secret
// exists in the LangSmith workspace.
type workspaceSecretStatus int

const (
	workspaceSecretPresent workspaceSecretStatus = iota
	workspaceSecretAbsent
	workspaceSecretLookupFailed
)

// claimSecretFromLocalSources reports whether keyEnv is already staged for
// this deploy or set in the project `.env`.
func claimSecretFromLocalSources(keyEnv string, envVars map[string]string, secrets []hostapi.Secret) bool {
	for _, secret := range secrets {
		if secret.Name == keyEnv {
			return true
		}
	}
	return envVars[keyEnv] != ""
}

// verifyWorkspaceSecret looks up keyEnv in LangSmith workspace secrets (no
// spinner / messaging); the error is set for [workspaceSecretLookupFailed].
//
// Loads and caches the workspace key list on first use so model / ingress /
// connector requirements preflight share a single platform API call.
func (s *secretPreflight) verifyWorkspaceSecret(ctx context.Context, keyEnv string) (workspaceSecretStatus, error) {
	if s.workspaceKeys.state == workspaceSecretKeysUnloaded {
		keys, err := s.client.WorkspaceSecretKeys(ctx)
		switch {
		case errs.IsCancellation(err):
			// The user stopping the deploy says nothing about the secrets.
			return workspaceSecretLookupFailed, err
		case err != nil:
			s.workspaceKeys = workspaceSecretKeys{state: workspaceSecretKeysLookupFailed, lookupErr: err}
		default:
			s.workspaceKeys = workspaceSecretKeys{state: workspaceSecretKeysLoaded, keys: keys}
		}
	}

	switch {
	case s.workspaceKeys.state == workspaceSecretKeysLookupFailed:
		return workspaceSecretLookupFailed, s.workspaceKeys.lookupErr
	case slices.Contains(s.workspaceKeys.keys, keyEnv):
		return workspaceSecretPresent, nil
	default:
		return workspaceSecretAbsent, nil
	}
}

// gcsUploadTimeout bounds the source upload.
const gcsUploadTimeout = 300 * time.Second

// uploadToGCS PUTs the gzipped source archive to the signed GCS upload URL.
//
// The URL carries its own authorization, so this is a plain request with no
// LangSmith credential.
func uploadToGCS(ctx context.Context, signedURL string, body []byte) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, signedURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build upload request: %w", err)
	}
	request.Header.Set("Content-Type", "application/gzip")
	request.Header.Set("X-Goog-Content-Length-Range", "0,209715200")
	response, err := (&http.Client{Timeout: gcsUploadTimeout}).Do(request)
	if err != nil {
		return fmt.Errorf("source upload failed: %w", withoutQuery(err))
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			slog.Debug("failed to close the source upload response", "error", err)
		}
	}()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		detail, err := io.ReadAll(response.Body)
		if err != nil {
			slog.Debug("failed to read the source upload response", "error", err)
		}
		return fmt.Errorf("source upload failed with status %s: %s", response.Status, detail)
	}
	return nil
}

// withoutQuery drops the query of the URL a transport error names: the
// signed upload URL's query is its credential.
func withoutQuery(err error) error {
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		return err
	}
	redacted := *urlErr
	if parsed, parseErr := url.Parse(urlErr.URL); parseErr == nil {
		parsed.RawQuery, parsed.Fragment = "", ""
		redacted.URL = parsed.String()
	} else {
		redacted.URL = "the signed upload URL"
	}
	return &redacted
}

func pollFailureLimitReached(failures *int) bool {
	*failures++
	return *failures >= pollMaxFailures
}

// deploymentIDStatusRecord is the `Deployment ID:` record for a well-formed
// deployment id, canonicalized; "" for anything else.
func deploymentIDStatusRecord(deploymentID string) string {
	parsed, err := uuid.Parse(deploymentID)
	if err != nil {
		return ""
	}
	return "Deployment ID: " + parsed.String()
}

// reportNonInteractiveDeploymentID prints the deployment id as a record when
// stderr is not a terminal.
func reportNonInteractiveDeploymentID(deploymentID string) {
	if ui.IsOutputTerminal() {
		return
	}
	if record := deploymentIDStatusRecord(deploymentID); record != "" {
		fmt.Fprintln(os.Stderr, record)
	}
}

// pollAndReport polls the latest revision's status with a live spinner, then
// prints a result. It returns the Agent Server URL of the live deployment, ""
// when there was no revision to track.
func pollAndReport(
	ctx context.Context,
	client *hostapi.Client,
	deploymentID, name, statusURL string,
	scheduleCrons compile.ScheduleCrons,
	waitTimeout time.Duration,
) (string, error) {
	revisions, err := client.ListRevisions(ctx, deploymentID, 1)
	if err != nil {
		return "", fmt.Errorf("failed to list revisions: %w", err)
	}
	revisionID := ""
	if resources, ok := revisions["resources"].([]any); ok && len(resources) > 0 {
		if revision, ok := resources[0].(map[string]any); ok {
			revisionID, _ = revision["id"].(string)
		}
	}

	if revisionID == "" {
		// The dashboard link was handed over as soon as it was knowable.
		return "", ui.Outro(fmt.Sprintf("Build triggered for '%s'. No revision to track yet.", name))
	}

	sp := ui.NewSpinner()
	sp.Start("Waiting for deployment")
	deadline := time.Now().Add(waitTimeout)
	lastStatus := ""
	failures := 0
	interactive := ui.IsOutputTerminal()
	finalStatus := ""
	for {
		if err := ctx.Err(); err != nil {
			sp.Cancel("Stopped waiting; the deployment continues")
			return "", err
		}
		if !time.Now().Before(deadline) {
			finalStatus = lastStatus
			break
		}
		revision, err := client.GetRevision(ctx, deploymentID, revisionID)
		if err != nil {
			// A cancelled request is the user stopping the wait, not a failed
			// poll.
			if ctx.Err() != nil {
				continue
			}
			if pollFailureLimitReached(&failures) {
				sp.Error("Lost connection while polling status")
				return "", fmt.Errorf("lost connection while polling deployment status: %w", err)
			}
			if interactive {
				sp.SetMessage("Could not check deployment status. Retrying…")
			} else {
				fmt.Fprintf(os.Stderr, "warning: could not check deployment status (%v); retrying\n", err)
			}
			proc.SleepUntilInterrupted(ctx, pollInterval)
			continue
		}
		recovered := failures > 0
		failures = 0
		status, ok := revision["status"].(string)
		if !ok {
			status = "UNKNOWN"
		}

		if status != lastStatus || recovered {
			lastStatus = status
			displayStatus := displayRevisionStatus(status)
			// Without a terminal the spinner prints each status once.
			sp.SetMessage(displayStatus + "…")
		}
		if deployment.IsTerminalStatus(status) {
			finalStatus = status
			break
		}
		proc.SleepUntilInterrupted(ctx, pollInterval)
	}

	inspect := func(title string) error {
		if statusURL == "" {
			return nil
		}
		return ui.NoteLinks(title, []ui.Link{{URL: statusURL}})
	}
	switch {
	case finalStatus == "DEPLOYED":
		sp.Stop("Deployment live")
		agentServerURL, err := fetchAgentServerURL(ctx, client, deploymentID)
		if err != nil {
			return "", err
		}
		if err := reconcileSchedules(ctx, client, agentServerURL, scheduleCrons); err != nil {
			return "", err
		}
		return agentServerURL, nil
	case deployment.IsInterruptedStatus(finalStatus):
		sp.Error("Deployment interrupted from another interface")
		if err := inspect("Inspect the deployment"); err != nil {
			return "", err
		}
		// The cancelled-session footer is rendered by Run.
		return "", errs.Categorized(errs.Deployment,
			fmt.Sprintf("deployment of '%s' was interrupted (status %s)", name, finalStatus))
	case deployment.IsFailedStatus(finalStatus):
		sp.Error(fmt.Sprintf("Deployment failed (%s)", finalStatus))
		if err := inspect("Inspect the failure"); err != nil {
			return "", err
		}
		// The cancelled-session footer is rendered by Run.
		return "", errs.Categorized(errs.Deployment,
			fmt.Sprintf("deployment of '%s' failed with status %s", name, finalStatus))
	case finalStatus == "":
		// The deadline elapsed before any status was observed.
		sp.Error("Timed out waiting for the deployment")
		if err := inspect("Inspect the deployment"); err != nil {
			return "", err
		}
		return "", errs.Categorized(errs.Deployment,
			fmt.Sprintf("timed out waiting for '%s' to deploy (no status reported)", name))
	default:
		// Either the deadline elapsed while still mid-deploy, or a terminal
		// status that isn't DEPLOYED (e.g. SKIPPED). Either way, fail.
		sp.Error(fmt.Sprintf("Deployment did not reach DEPLOYED (%s)", finalStatus))
		if err := inspect("Inspect the deployment"); err != nil {
			return "", err
		}
		return "", errs.Categorized(errs.Deployment,
			fmt.Sprintf("deployment of '%s' did not reach DEPLOYED (last status: %s)", name, finalStatus))
	}
}

// displayRevisionStatus constrains a Host-returned revision status before
// rendering it in terminal output.
func displayRevisionStatus(status string) string {
	if status == "" || len(status) > 64 {
		return "UNKNOWN"
	}
	for index := range len(status) {
		b := status[index]
		if (b < 'A' || b > 'Z') && (b < '0' || b > '9') && b != '_' {
			return "UNKNOWN"
		}
	}
	return status
}

// fetchAgentServerURL looks up the public Agent Server base URL of a live
// deployment.
//
// This is the URL callers point an SDK or HTTP client at, and the base the
// CLI uses for Agent Server control-plane calls (cron reconciliation).
func fetchAgentServerURL(ctx context.Context, client *hostapi.Client, deploymentID string) (string, error) {
	live, err := client.GetDeployment(ctx, deploymentID)
	if err != nil {
		return "", fmt.Errorf("failed to look up the deployment's Agent Server URL: %w", err)
	}
	url := deployment.DeploymentRuntimeURL(live)
	if url == "" {
		return "", errors.New("deployment is live, but the host backend response did not include an Agent Server URL")
	}
	return url, nil
}

// reportDeployTarget prints the organization and workspace a deploy is about
// to write into.
//
// Both lines are best effort: a platform API that cannot be reached, or a
// credential without organization read, prints one line or none rather than
// failing a deploy that is otherwise fine.
func reportDeployTarget(target hostapi.DeployTarget) error {
	if target.Organization != "" {
		if err := ui.LogInfo("Organization: " + target.Organization); err != nil {
			return err
		}
	}
	if target.Workspace != nil {
		return ui.LogInfo("Workspace: " + target.Workspace.Label())
	}
	return nil
}

// deploymentLinks are the post-deploy links: where to call the agent, and
// where to inspect it.
func deploymentLinks(agentServerURL, dashboardURL string) []ui.Link {
	links := []ui.Link{{Label: "Agent Server", URL: agentServerURL}}
	if dashboardURL != "" {
		links = append(links, ui.Link{Label: "Dashboard", URL: dashboardURL})
	}
	return links
}

// httpChannelLinks lists the events URL of each direct HTTP channel, by
// channel name. Only direct HTTP channels advertise an events path; sending
// capability is irrelevant.
func httpChannelLinks(agentServerURL string, requirements compile.ChannelRequirements) []ui.Link {
	names := make([]string, 0, len(requirements))
	for name := range requirements {
		names = append(names, name)
	}
	slices.Sort(names)
	var links []ui.Link
	for _, name := range names {
		manifest := requirements[name]
		if !channels.ListsEventsURL(manifest) {
			continue
		}
		object, _ := manifest.(map[string]any)
		path, ok := object["eventsPath"].(string)
		if !ok {
			continue
		}
		links = append(links, ui.Link{Label: name, URL: strings.TrimRight(agentServerURL, "/") + path})
	}
	return links
}

// reconcileSchedules reconciles MDA-owned schedules with LangSmith Agent
// Server crons.
//
// The deployment is already live by this point, and the Agent Server it talks
// to may still be cold, so a failure here degrades to a warning rather than
// failing the deploy: the code is deployed either way, and rerunning
// `langsmith deploy` reconciles again.
func reconcileSchedules(ctx context.Context, client *hostapi.Client, deploymentURL string, scheduleCrons compile.ScheduleCrons) error {
	sp := ui.NewSpinner()
	sp.Start("Reconciling schedules")
	removed, err := reconcileSchedulesAtURL(ctx, client, deploymentURL, scheduleCrons)
	if err != nil {
		if errs.IsCancellation(err) {
			sp.Cancel("Cancelled")
			return err
		}
		sp.Error("Could not reconcile schedules")
		return ui.LogWarning(scheduleReconcileWarning(err, len(scheduleCrons)))
	}

	if message, ok := schedulesSummary(len(scheduleCrons), removed); ok {
		sp.Stop(message)
	} else {
		sp.Clear()
	}
	return nil
}

// scheduleReconcileWarning explains a failed reconcile: what still happened,
// what did not, and how to finish the job.
func scheduleReconcileWarning(err error, authored int) string {
	consequence := "The deployment is live, but its schedules may be missing or stale."
	if authored == 0 {
		consequence = "Any schedules from a previous deploy may still be running."
	}
	return fmt.Sprintf("%v\n%s Re-run `langsmith deploy` to reconcile them.", err, consequence)
}

// schedulesSummary is the line to report after reconciling, or false when
// nothing changed and the deploy log is better off without it.
func schedulesSummary(created, removed int) (string, bool) {
	switch {
	case created == 0 && removed == 0:
		return "", false
	case created == 0:
		return fmt.Sprintf("Removed %d schedule(s)", removed), true
	default:
		return fmt.Sprintf("Reconciled %d schedule(s)", created), true
	}
}

// cronAPI is the Agent Server cron calls that reconciling schedules makes.
//
// [hostapi.Client] makes them as the workspace.
type cronAPI interface {
	SearchManagedCrons(ctx context.Context, deploymentURL string) ([]any, error)
	CreateCron(ctx context.Context, deploymentURL string, payload map[string]any) (map[string]any, error)
	CreateThreadCron(ctx context.Context, deploymentURL, threadID string, payload map[string]any) (map[string]any, error)
	DeleteCron(ctx context.Context, deploymentURL, cronID string) (any, error)
}

// reconcileSchedulesAtURL replaces the deploy-managed crons on the Agent
// Server at deploymentURL with scheduleCrons.
//
// Returns how many previously managed crons were deleted.
func reconcileSchedulesAtURL(ctx context.Context, client cronAPI, deploymentURL string, scheduleCrons compile.ScheduleCrons) (int, error) {
	stale, err := deleteScheduleCrons(ctx, client, deploymentURL, true)
	if err != nil {
		return 0, err
	}

	names := make([]string, 0, len(scheduleCrons))
	for name := range scheduleCrons {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if err := createScheduleCron(ctx, client, deploymentURL, name, scheduleCrons[name]); err != nil {
			return 0, fmt.Errorf("failed to create cron for schedule '%s': %w", name, err)
		}
	}

	return stale, nil
}

// deleteScheduleCrons deletes the MDA-owned crons on a deployment's Agent
// Server, only those deploy owns when deployOnly. Returns how many were
// deleted.
func deleteScheduleCrons(ctx context.Context, client cronAPI, deploymentURL string, deployOnly bool) (int, error) {
	existing, err := client.SearchManagedCrons(ctx, deploymentURL)
	if err != nil {
		return 0, fmt.Errorf("failed to search existing managed crons: %w", err)
	}
	stale, err := managedCronIDs(existing, deployOnly)
	if err != nil {
		return 0, fmt.Errorf("failed to parse managed cron search response: %w", err)
	}
	for _, cronID := range stale {
		if _, err := client.DeleteCron(ctx, deploymentURL, cronID); err != nil {
			return 0, fmt.Errorf("failed to delete existing cron %s: %w", cronID, err)
		}
	}

	return len(stale), nil
}

func managedCronIDs(response any, deployOnly bool) ([]string, error) {
	// [cronAPI.SearchManagedCrons] flattens every page into one array.
	items, ok := response.([]any)
	if !ok {
		return nil, errors.New("unexpected cron search response shape: expected a JSON array")
	}

	ids := []string{}
	for _, item := range items {
		cron, _ := item.(map[string]any)
		metadata, _ := cron["metadata"].(map[string]any)
		if metadata["mda_managed"] != true {
			continue
		}
		managedBy, hasManagedBy := metadata["mda_managed_by"]
		schedule, _ := metadata["mda_schedule"].(string)
		if deployOnly && managedBy != "deploy" && (hasManagedBy || schedule == "") {
			continue
		}
		if id, ok := cron["cron_id"].(string); ok {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func createScheduleCron(ctx context.Context, client cronAPI, deploymentURL, name string, cron map[string]any) error {
	threadID, payload, err := scheduleCronRequest(name, cron)
	if err != nil {
		return err
	}
	if threadID != "" {
		_, err = client.CreateThreadCron(ctx, deploymentURL, threadID, payload)
	} else {
		_, err = client.CreateCron(ctx, deploymentURL, payload)
	}
	return err
}

// scheduleCronRequest turns a compiled schedule into an Agent Server cron
// payload, and the thread it runs on ("" for a new thread each time).
func scheduleCronRequest(name string, cron map[string]any) (string, map[string]any, error) {
	if cron == nil {
		return "", nil, fmt.Errorf("schedule '%s' did not compile to an object", name)
	}
	payload := make(map[string]any, len(cron))
	for key, value := range cron {
		payload[key] = value
	}
	graph, ok := payload["graph"].(string)
	if !ok {
		graph = "agent"
	}
	delete(payload, "graph")
	threadID, _ := payload["thread_id"].(string)
	delete(payload, "thread_id")
	payload["assistant_id"] = graph

	return threadID, payload, nil
}
