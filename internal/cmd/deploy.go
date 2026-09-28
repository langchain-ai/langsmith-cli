package cmd

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	"github.com/langchain-ai/langsmith-cli/internal/langgraphapi"
	"github.com/langchain-ai/langsmith-cli/internal/structured"
	"github.com/spf13/cobra"
)

const (
	sourceInternalSource = "internal_source"
	sourceInternalDocker = "internal_docker"
	sourceExternalDocker = "external_docker"

	listenerRequiredMarker = "listener_id' is required"
	listenersDocsURL       = "https://docs.langchain.com/langsmith/control-plane#listeners"
	listenersShown         = 10
	deployUploadTimeout    = 300 * time.Second
	deployUploadSizeRange  = "0,209715200"
)

var (
	deployTerminalStatuses = []string{"DEPLOYED", "CREATE_FAILED", "BUILD_FAILED", "DEPLOY_FAILED", "SKIPPED"}
	deployFailedStatuses   = []string{"CREATE_FAILED", "BUILD_FAILED", "DEPLOY_FAILED"}
	deployAgentEnvs        = []string{"development", "staging", "production"}
)

type deployWait struct {
	timeout  time.Duration
	interval time.Duration
	remote   bool
}

var (
	deployRemoteBuildWait = deployWait{timeout: 900 * time.Second, interval: 3 * time.Second, remote: true}
	deployImageWait       = deployWait{timeout: 300 * time.Second, interval: time.Second}
)

// deployProgress writes to stderr; stdout carries only the result.
type deployProgress struct {
	w    io.Writer
	step int
}

func (p *deployProgress) Step(format string, args ...any) {
	p.step++
	fmt.Fprintf(p.w, "%d. %s\n", p.step, fmt.Sprintf(format, args...))
}

func (p *deployProgress) Info(format string, args ...any) {
	fmt.Fprintf(p.w, "   %s\n", fmt.Sprintf(format, args...))
}

func (p *deployProgress) Note(format string, args ...any) {
	fmt.Fprintf(p.w, format+"\n", args...)
}

func (p *deployProgress) Log(message string) {
	fmt.Fprintf(p.w, "   | %s\n", message)
}

type deployInput struct {
	Config           string
	Name             string
	DeploymentID     string
	DeploymentType   string
	Tag              string
	Image            string
	PushTo           string
	ListenerID       string
	K8sNamespace     string
	AgentID          string
	AgentEnvironment string
	ImageName        string
	InstallCommand   string
	BuildCommand     string
	NoWait           bool
	Verbose          bool
	Remote           bool
	NoRemote         bool
	JSON             bool
}

type deploymentSelector struct {
	id    string
	name  string
	agent *langgraphapi.Agent
}

type deploySourceKind int

const (
	deployRemoteBuild deploySourceKind = iota
	deployManagedImage
	deployCustomerRegistry
)

type deployPlan struct {
	kind      deploySourceKind
	image     string
	tag       string
	pushTo    imageReference
	placement requestedPlacement
}

type deployOutcome struct {
	deploymentID string
	updated      *langgraphapi.Deployment
	wait         deployWait
	message      string
}

type deployResult struct {
	DeploymentID   string `json:"deployment_id"`
	RevisionID     string `json:"revision_id,omitempty"`
	Status         string `json:"status"`
	RevisionStatus string `json:"revision_status,omitempty"`
	Message        string `json:"message"`
	URL            string `json:"url,omitempty"`
	StatusURL      string `json:"status_url,omitempty"`
}

var deployResultRender = structured.PropertyList{
	Properties: []structured.Property{
		{Label: "Result", Template: "{{.Message}}"},
		{Label: "Deployment ID", Template: "{{.DeploymentID}}"},
		{Label: "Revision ID", Template: "{{.RevisionID}}", OmitEmpty: true},
		{Label: "Revision status", Template: "{{.RevisionStatus}}", OmitEmpty: true},
		{Label: "URL", Template: "{{.URL}}", OmitEmpty: true},
		{Label: "View status", Template: "{{.StatusURL}}", OmitEmpty: true},
	},
}

var deployCommand = structured.Command[*deployInput]{
	Use:   "deploy",
	Short: "Deploy a LangGraph project to LangSmith Deployment (beta)",
	Long: `Deploy a LangGraph project to LangSmith Deployment (beta).

Run from the root of a LangGraph project (where langgraph.json is). By default the
project source is uploaded and built remotely, so Docker is not required. Pass
--image to deploy an image you already built for linux/amd64 (for example with
'langgraph build -t my-agent'), or add --push-to to push it to a registry you manage
(self-hosted and hybrid LangSmith).

Environment variables from langgraph.json's "env" (or ./.env) are sent as deployment
secrets, except variables the platform reserves.

The deployment is found or created by --name, which defaults to
LANGSMITH_DEPLOYMENT_NAME and then to the current directory's name.

Examples:
  langsmith deploy
  langsmith deploy --name my-agent --deployment-type prod
  langsmith deploy --image my-agent:latest
  langsmith deploy --image my-agent:latest --push-to registry.example.com/team/my-agent
  langsmith deploy list
  langsmith deploy logs --name my-agent --follow`,
	Args: cobra.NoArgs,
	Input: func(cmd *cobra.Command) *deployInput {
		in := &deployInput{}
		f := cmd.Flags()
		f.StringVarP(&in.Config, "config", "c", deployDefaultConfig, "Path to langgraph.json")
		f.StringVar(&in.Name, "name", "", "Deployment name [env: LANGSMITH_DEPLOYMENT_NAME] (default: current directory name)")
		f.StringVar(&in.DeploymentID, "deployment-id", "", "ID of an existing deployment to update (instead of --name)")
		f.StringVar(&in.DeploymentType, "deployment-type", "dev", "Deployment type when creating a deployment: dev or prod (ignored with --push-to)")
		f.StringVarP(&in.Tag, "tag", "t", "", "Tag for the pushed image with --image or --push-to (default: latest)")
		f.StringVar(&in.Image, "image", "", "Deploy this existing local image (repo:tag) instead of building remotely; it must target linux/amd64")
		f.StringVar(&in.PushTo, "push-to", "", "Push --image to this repository in a registry you manage and deploy it from there (self-hosted and hybrid)")
		f.StringVar(&in.ListenerID, "listener-id", "", "Listener that runs the deployment; only when creating one with --push-to")
		f.StringVar(&in.K8sNamespace, "k8s-namespace", "", "Kubernetes namespace the listener deploys into; only when creating one with --push-to")
		f.StringVar(&in.AgentID, "agent-id", "", "Logical agent ID, private beta [env: LANGSMITH_AGENT_ID]")
		f.StringVar(&in.AgentEnvironment, "agent-environment", "", "Agent environment: development, staging, or production; private beta [env: LANGSMITH_AGENT_ENVIRONMENT]")
		f.BoolVar(&in.NoWait, "no-wait", false, "Return once the revision is submitted instead of waiting for it to deploy")
		f.BoolVar(&in.Verbose, "verbose", false, "Stream remote build logs and docker output")
		f.String("jq", "", "Filter JSON output using a jq expression")
		f.BoolVar(&in.Remote, "remote", false, "Build remotely (the default)")
		f.BoolVar(&in.NoRemote, "no-remote", false, "Refuse to build remotely; requires --image")
		f.BoolVar(&in.JSON, "json", false, "Same as --format json")
		f.StringVar(&in.ImageName, "image-name", "", "Repository name for the image pushed with --image")
		f.StringVar(&in.InstallCommand, "install-command", "", "Install command for the remote build")
		f.StringVar(&in.BuildCommand, "build-command", "", "Build command for the remote build")
		f.Bool("no-input", false, "Never prompt (the CLI never prompts)")
		f.Bool("pull", true, "Only affects local builds, which this CLI does not run")
		f.String("base-image", "", "Only affects local builds, which this CLI does not run")
		f.String("api-version", "", "Only affects local builds, which this CLI does not run")
		for _, name := range []string{"image-name", "install-command", "build-command", "no-input", "pull", "base-image", "api-version"} {
			_ = f.MarkHidden(name)
		}
		return in
	},
	CustomOutput: true,
	Action: func(ctx context.Context, cmd *cobra.Command, in *deployInput, args []string) (any, error) {
		return nil, runDeploy(ctx, cmd, in)
	},
}

func newDeployCmd() *cobra.Command {
	cmd := deployCommand.Cobra()
	cmd.PersistentFlags().String("host-url", "", "Deployment control plane URL [env: LANGGRAPH_HOST_URL]")
	_ = cmd.PersistentFlags().MarkHidden("host-url")

	cmd.AddCommand(deployListCommand.Cobra())
	cmd.AddCommand(deployRevisionsCommand.Cobra())
	cmd.AddCommand(deployDeleteCommand.Cobra())
	cmd.AddCommand(deployLogsCommand.Cobra())
	return cmd
}

func newLangGraphAPIClient(cmd *cobra.Command) (*langgraphapi.Client, error) {
	opts, err := cmdutil.ResolveClientOptions(cmd, true)
	if err != nil {
		return nil, err
	}
	if opts.APIKey == "" && opts.OAuthAccessToken == "" {
		return nil, errors.New("not authenticated; run 'langsmith auth login', set LANGSMITH_API_KEY, or pass --api-key")
	}
	hostURL := os.Getenv("LANGGRAPH_HOST_URL")
	if f := cmd.Flags().Lookup("host-url"); f != nil && f.Value.String() != "" {
		hostURL = f.Value.String()
	}
	return langgraphapi.New(langgraphapi.ResolveEndpoints(hostURL, opts.APIURL), langgraphapi.Auth{
		APIKey:      opts.APIKey,
		BearerToken: opts.OAuthAccessToken,
		TenantID:    opts.WorkspaceID,
	}), nil
}

func envOrFlag(cmd *cobra.Command, flag, value, env string) string {
	if cmd.Flags().Changed(flag) {
		return value
	}
	return os.Getenv(env)
}

func (in *deployInput) plan(cmd *cobra.Command) (deployPlan, *langgraphapi.Agent, error) {
	var plan deployPlan
	if in.DeploymentType != "dev" && in.DeploymentType != "prod" {
		return plan, nil, fmt.Errorf("--deployment-type must be dev or prod (got %q)", in.DeploymentType)
	}

	var agent *langgraphapi.Agent
	agentID := envOrFlag(cmd, "agent-id", in.AgentID, "LANGSMITH_AGENT_ID")
	agentEnv := envOrFlag(cmd, "agent-environment", in.AgentEnvironment, "LANGSMITH_AGENT_ENVIRONMENT")
	if agentID != "" || agentEnv != "" {
		if strings.TrimSpace(agentID) == "" || agentEnv == "" {
			return plan, nil, errors.New("--agent-id and --agent-environment are required together")
		}
		if !slices.Contains(deployAgentEnvs, agentEnv) {
			return plan, nil, fmt.Errorf("--agent-environment must be one of %s (got %q)", strings.Join(deployAgentEnvs, ", "), agentEnv)
		}
		if cmd.Flags().Changed("name") || in.DeploymentID != "" {
			return plan, nil, errors.New("--agent-id and --agent-environment cannot be combined with --name or --deployment-id")
		}
		agent = &langgraphapi.Agent{AgentID: agentID, Environment: agentEnv}
	}

	plan.placement = requestedPlacement{listenerID: in.ListenerID, namespace: in.K8sNamespace}
	if plan.placement.requested() {
		if in.PushTo == "" {
			return plan, nil, errors.New("--listener-id and --k8s-namespace only apply when creating a deployment with --push-to")
		}
		if in.DeploymentID != "" {
			return plan, nil, errors.New("listener and namespace are fixed when a deployment is created, so they cannot be set for an existing --deployment-id; drop them, or create a new deployment with --name")
		}
	}

	if in.Remote && in.NoRemote {
		return plan, nil, errors.New("use either --remote or --no-remote, not both")
	}
	if in.Remote && in.PushTo != "" {
		return plan, nil, errors.New("--push-to cannot be combined with --remote")
	}
	if in.Remote && in.Image != "" {
		return plan, nil, errors.New("--image cannot be combined with --remote builds")
	}

	switch {
	case in.PushTo != "":
		ref, err := parseImageReference(in.PushTo)
		if err != nil {
			return plan, nil, errors.New("--push-to takes a repository with an optional tag, not a digest")
		}
		if ref.tag != "" && in.Tag != "" {
			return plan, nil, errors.New("--push-to already includes a tag; do not combine it with --tag")
		}
		if ref.tag == "" {
			if ref.tag, err = normalizeImageTag(in.Tag); err != nil {
				return plan, nil, err
			}
		}
		if in.Image == "" {
			return plan, nil, errors.New("--push-to needs --image: build the image for linux/amd64 first (for example 'langgraph build -t my-agent') and pass it with --image")
		}
		plan.kind, plan.pushTo, plan.image = deployCustomerRegistry, ref, in.Image
	case in.Image != "":
		tag, err := normalizeImageTag(in.Tag)
		if err != nil {
			return plan, nil, err
		}
		plan.kind, plan.image, plan.tag = deployManagedImage, in.Image, tag
	case in.NoRemote:
		return plan, nil, errors.New("this CLI does not build images locally; build one for linux/amd64 (for example 'langgraph build -t my-agent') and pass it with --image")
	default:
		plan.kind = deployRemoteBuild
	}
	return plan, agent, nil
}

func runDeploy(ctx context.Context, cmd *cobra.Command, in *deployInput) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	p := &deployProgress{w: cmd.ErrOrStderr()}
	if in.JSON {
		if err := cmd.Flags().Set("format", "json"); err != nil {
			return err
		}
	}

	plan, agent, err := in.plan(cmd)
	if err != nil {
		return err
	}
	if plan.kind != deployRemoteBuild {
		if err := requireDocker(); err != nil {
			return err
		}
	}
	cfg, err := loadLanggraphConfig(in.Config)
	if err != nil {
		return err
	}
	envVars, err := cfg.envVars(p)
	if err != nil {
		return err
	}

	sel := deploymentSelector{id: in.DeploymentID, agent: agent}
	if agent == nil && in.DeploymentID == "" {
		sel.name = normalizeDeploymentName(cmp.Or(
			envOrFlag(cmd, "name", in.Name, deploymentNameEnv),
			envVars[deploymentNameEnv],
			cwdDeploymentName(),
		))
	}

	hc, err := newLangGraphAPIClient(cmd)
	if err != nil {
		return err
	}
	r := &deployRun{
		hc:             hc,
		p:              p,
		cfg:            cfg,
		sel:            sel,
		deploymentType: in.DeploymentType,
		secrets:        deploySecrets(envVars, p),
		imageName:      in.ImageName,
		sourceConfig:   remoteBuildSourceConfig(cmd, in),
		docker:         docker{verbose: in.Verbose, stderr: cmd.ErrOrStderr()},
		verbose:        in.Verbose,
	}

	var out deployOutcome
	switch plan.kind {
	case deployRemoteBuild:
		out, err = r.remoteBuild(ctx)
	case deployManagedImage:
		out, err = r.managedImage(ctx, plan.image, plan.tag)
	case deployCustomerRegistry:
		out, err = r.customerRegistry(ctx, plan)
	}
	if err != nil {
		return err
	}

	result := deployResult{DeploymentID: out.deploymentID, Status: "submitted", Message: out.message}
	if out.updated != nil && out.updated.TenantID != "" {
		result.StatusURL = fmt.Sprintf("%s/o/%s/host/deployments/%s", hc.Endpoints().DashboardURL, out.updated.TenantID, out.deploymentID)
		p.Info("View status: %s", result.StatusURL)
	}
	if in.NoWait {
		return structured.Render(cmd, result, deployResultRender)
	}

	status, revisionID, err := r.wait(ctx, out.deploymentID, out.wait)
	if err != nil {
		if out.wait.remote && errors.Is(err, context.Canceled) {
			p.Info("Interrupted. Deployment ID: %s, Revision ID: %s", out.deploymentID, revisionID)
			p.Info("The build will continue remotely.")
		}
		return err
	}
	if status == "" {
		return structured.Render(cmd, result, deployResultRender)
	}
	result.RevisionID, result.RevisionStatus = revisionID, status

	if status == "BUILD_FAILED" && out.wait.remote && !in.Verbose {
		r.printBuildLogTail(ctx, out.deploymentID, revisionID)
	}
	deployment, err := hc.GetDeployment(ctx, out.deploymentID)
	if err != nil {
		return err
	}
	result.URL = deployment.SourceConfig.CustomURL

	switch {
	case status == "DEPLOYED":
		result.Status, result.Message = "succeeded", "Deployment successful!"
	case slices.Contains(deployFailedStatuses, status):
		result.Status, result.Message = "failed", "Deployment failed"
	default:
		result.Status, result.Message = "timed_out", "Timed out waiting for deployment"
	}
	if err := structured.Render(cmd, result, deployResultRender); err != nil {
		return err
	}
	if result.Status == "failed" {
		return fmt.Errorf("deployment %s failed: revision %s is %s", out.deploymentID, revisionID, status)
	}
	return nil
}

type deployRun struct {
	hc             *langgraphapi.Client
	p              *deployProgress
	cfg            *langgraphConfig
	sel            deploymentSelector
	deploymentType string
	secrets        []langgraphapi.Secret
	imageName      string
	sourceConfig   map[string]any
	docker         docker
	verbose        bool
}

func remoteBuildSourceConfig(cmd *cobra.Command, in *deployInput) map[string]any {
	config := map[string]any{}
	if cmd.Flags().Changed("install-command") {
		config["install_command"] = in.InstallCommand
	}
	if cmd.Flags().Changed("build-command") {
		config["build_command"] = in.BuildCommand
	}
	return config
}

func (r *deployRun) lookup(ctx context.Context, notFound string) (*langgraphapi.Deployment, error) {
	if r.sel.id != "" {
		r.p.Step("Using deployment %s", r.sel.id)
		return r.hc.GetDeployment(ctx, r.sel.id)
	}
	var (
		found *langgraphapi.Deployment
		err   error
	)
	if r.sel.agent != nil {
		r.p.Step("Looking up agent '%s' in %s", r.sel.agent.AgentID, r.sel.agent.Environment)
		found, err = r.findAgentDeployment(ctx)
	} else {
		r.p.Step("Looking up deployment '%s'", r.sel.name)
		found, err = findDeploymentByName(ctx, r.hc, r.sel.name)
	}
	if err != nil {
		return nil, err
	}
	if found == nil {
		r.p.Info("%s", notFound)
	} else {
		r.p.Info("Found existing deployment (ID: %s)", found.ID)
	}
	return found, nil
}

func (r *deployRun) findAgentDeployment(ctx context.Context) (*langgraphapi.Deployment, error) {
	deployments, err := r.hc.ListDeployments(ctx, langgraphapi.DeploymentFilter{
		AgentID:          r.sel.agent.AgentID,
		AgentEnvironment: r.sel.agent.Environment,
		Limit:            langgraphapi.MaxPageSize,
	})
	if err != nil {
		return nil, err
	}
	if len(deployments) > 1 {
		return nil, fmt.Errorf("this control plane does not filter deployments by agent, so the CLI cannot tell which one belongs to '%s' in %s; deploy by --name instead", r.sel.agent.AgentID, r.sel.agent.Environment)
	}
	for i := range deployments {
		if deployments[i].ID != "" && !deployments[i].IsPreview {
			return &deployments[i], nil
		}
	}
	return nil, nil
}

func findDeploymentByName(ctx context.Context, hc *langgraphapi.Client, name string) (*langgraphapi.Deployment, error) {
	deployments, err := hc.ListDeployments(ctx, langgraphapi.DeploymentFilter{Name: name, NameContains: name, Limit: langgraphapi.MaxPageSize})
	if err != nil {
		return nil, err
	}
	for i := range deployments {
		if deployments[i].Name == name && deployments[i].ID != "" {
			return &deployments[i], nil
		}
	}
	if len(deployments) >= langgraphapi.MaxPageSize {
		return nil, fmt.Errorf("this workspace has more deployments than the CLI can search, so it cannot tell whether '%s' already exists; pass --deployment-id to update an existing deployment", name)
	}
	return nil, nil
}

func (r *deployRun) create(ctx context.Context, body langgraphapi.DeploymentCreate) (*langgraphapi.Deployment, error) {
	body.Secrets = r.secrets
	if r.sel.agent != nil {
		body.Agent = r.sel.agent
		r.p.Step("Creating deployment for agent '%s' in %s", r.sel.agent.AgentID, r.sel.agent.Environment)
	} else {
		body.Name = r.sel.name
		r.p.Step("Creating deployment '%s'", r.sel.name)
	}
	created, err := r.hc.CreateDeployment(ctx, body)
	if err != nil {
		if r.sel.agent != nil && langgraphapi.StatusCode(err) == http.StatusConflict {
			return nil, fmt.Errorf("this agent already has a deployment in this environment: %w", err)
		}
		return nil, err
	}
	if r.sel.agent != nil {
		r.p.Info("Deployment name: %s", created.Name)
	}
	r.p.Info("Deployment ID: %s", created.ID)
	return created, nil
}

func (r *deployRun) resolveOrCreate(ctx context.Context, source, notFound string) (string, error) {
	found, err := r.lookup(ctx, notFound)
	if err != nil {
		return "", err
	}
	if found != nil {
		return found.ID, nil
	}
	created, err := r.create(ctx, langgraphapi.DeploymentCreate{
		Source:               source,
		SourceConfig:         map[string]any{"deployment_type": r.deploymentType},
		SourceRevisionConfig: map[string]any{},
	})
	if err != nil {
		if needsListener(err) {
			return "", listenerRequiredError("The image has to come from a registry you manage, so re-run with --image <image> --push-to <registry>/<repository>.")
		}
		return "", err
	}
	return created.ID, nil
}

func needsListener(err error) bool {
	e, ok := errors.AsType[*langgraphapi.Error](err)
	return ok && e.StatusCode == http.StatusBadRequest && strings.Contains(cmp.Or(e.Detail, e.Body), listenerRequiredMarker)
}

func listenerRequiredError(remedy string) error {
	return fmt.Errorf("this workspace deploys through a listener in your own cluster. %s\nLearn about listeners: %s", remedy, listenersDocsURL)
}

func (r *deployRun) remoteBuild(ctx context.Context) (deployOutcome, error) {
	r.p.Step("Creating source archive")
	archive, err := createSourceArchive(r.cfg, r.p)
	if err != nil {
		return deployOutcome{}, err
	}
	defer archive.Close()
	r.p.Info("Archive created (%s)", formatArchiveBytes(archive.size))

	deploymentID, err := r.resolveOrCreate(ctx, sourceInternalSource, "No deployment found. Will create.")
	if err != nil {
		return deployOutcome{}, err
	}

	r.p.Step("Requesting upload URL")
	upload, err := r.hc.RequestUploadURL(ctx, deploymentID)
	if err != nil {
		return deployOutcome{}, err
	}
	if upload.UploadURL == "" || upload.ObjectPath == "" {
		return deployOutcome{}, errors.New("upload URL response is missing upload_url or object_path")
	}

	r.p.Step("Uploading source (%s)", formatArchiveBytes(archive.size))
	if err := uploadSourceArchive(ctx, upload.UploadURL, archive); err != nil {
		return deployOutcome{}, err
	}

	r.p.Step("Triggering remote build")
	updated, err := r.hc.UpdateDeployment(ctx, deploymentID, langgraphapi.DeploymentUpdate{
		RevisionSource: sourceInternalSource,
		SourceConfig:   r.sourceConfig,
		SourceRevisionConfig: map[string]any{
			"source_tarball_path":   upload.ObjectPath,
			"langgraph_config_path": archive.configRel,
		},
		Secrets: r.secrets,
	})
	if err != nil {
		return deployOutcome{}, err
	}
	return deployOutcome{deploymentID: deploymentID, updated: updated, wait: deployRemoteBuildWait, message: "Build triggered"}, nil
}

func uploadSourceArchive(ctx context.Context, signedURL string, archive *sourceArchive) error {
	f, err := os.Open(archive.path)
	if err != nil {
		return err
	}
	defer f.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, signedURL, f)
	if err != nil {
		return fmt.Errorf("creating upload request: %w", err)
	}
	req.ContentLength = archive.size
	req.Header.Set("Content-Type", "application/gzip")
	req.Header.Set("X-Goog-Content-Length-Range", deployUploadSizeRange)
	resp, err := (&http.Client{Timeout: deployUploadTimeout}).Do(req)
	if err != nil {
		return fmt.Errorf("uploading source: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if err != nil {
			return fmt.Errorf("upload failed with status %d", resp.StatusCode)
		}
		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (r *deployRun) managedImage(ctx context.Context, image, tag string) (deployOutcome, error) {
	r.p.Step("Validating image %s", image)
	if err := r.docker.validatePrebuiltImage(ctx, image); err != nil {
		return deployOutcome{}, err
	}
	r.p.Info("Image is available for %s", deployPlatform)

	deploymentID, err := r.resolveOrCreate(ctx, sourceInternalDocker, "No deployment found. Will create before pushing.")
	if err != nil {
		return deployOutcome{}, err
	}

	r.p.Step("Requesting push token")
	token, err := r.hc.RequestPushToken(ctx, deploymentID)
	if err != nil {
		if langgraphapi.StatusCode(err) == http.StatusBadRequest && strings.Contains(err.Error(), "only available for 'internal_docker' source deployments") {
			return deployOutcome{}, fmt.Errorf("deployment %s was not created from a pushed image and cannot be updated with --image; deploy without --image to keep its build mode, or use a different --name", deploymentID)
		}
		return deployOutcome{}, err
	}
	if token.Token == "" || token.RegistryURL == "" {
		return deployOutcome{}, errors.New("push token response is missing token or registry_url")
	}

	registry := strings.TrimRight(token.RegistryURL, "/")
	if _, after, ok := strings.Cut(registry, "://"); ok {
		registry = after
	}
	registryHost, _, _ := strings.Cut(registry, "/")
	remote := imageReference{
		repository: registry + "/" + normalizeDeploymentName(cmp.Or(r.imageName, r.sel.name, filepath.Base(r.cfg.dir()))),
		tag:        tag,
	}.String()

	r.p.Step("Pushing image %s", remote)
	configDir, err := tokenDockerConfig(registryHost, token.Token)
	if err != nil {
		return deployOutcome{}, err
	}
	defer os.RemoveAll(configDir)
	if _, err := r.docker.run(ctx, "", "tag", image, remote); err != nil {
		return deployOutcome{}, err
	}
	pusher := r.docker
	pusher.configDir = configDir
	if err := pusher.push(ctx, remote, r.p); err != nil {
		return deployOutcome{}, err
	}
	imageURI := r.docker.pushedDigest(ctx, remote, r.p)

	r.p.Step("Updating deployment %s", deploymentID)
	updated, err := r.hc.UpdateDeployment(ctx, deploymentID, langgraphapi.DeploymentUpdate{
		RevisionSource:       sourceInternalDocker,
		SourceRevisionConfig: map[string]any{"image_uri": imageURI},
		Secrets:              r.secrets,
	})
	if err != nil {
		return deployOutcome{}, err
	}
	return deployOutcome{deploymentID: deploymentID, updated: updated, wait: deployImageWait, message: "Deployment updated"}, nil
}

func (r *deployRun) customerRegistry(ctx context.Context, plan deployPlan) (deployOutcome, error) {
	existing, err := r.lookup(ctx, "No deployment found. Will create after push.")
	if err != nil {
		return deployOutcome{}, err
	}
	if existing != nil {
		if existing.Source != sourceExternalDocker {
			return deployOutcome{}, fmt.Errorf("deployment %s was not created from an external image and cannot be updated with --push-to; run without --push-to to keep its build mode, or use a different --name to create a new deployment", existing.ID)
		}
		if plan.placement.requested() {
			return deployOutcome{}, fmt.Errorf("listener and namespace are fixed when a deployment is created; deployment %s already exists, so drop --listener-id and --k8s-namespace, or create a new deployment with a different --name", existing.ID)
		}
		imageURI, err := r.publish(ctx, plan)
		if err != nil {
			return deployOutcome{}, err
		}
		r.p.Step("Updating deployment %s", existing.ID)
		updated, err := r.hc.UpdateDeployment(ctx, existing.ID, langgraphapi.DeploymentUpdate{
			SourceRevisionConfig: map[string]any{"image_uri": imageURI},
			Secrets:              r.secrets,
		})
		if err != nil {
			return deployOutcome{}, err
		}
		return deployOutcome{deploymentID: existing.ID, updated: updated, wait: deployImageWait, message: "Deployment updated"}, nil
	}

	placed, err := r.resolvePlacement(ctx, plan.placement)
	if err != nil {
		return deployOutcome{}, err
	}
	sourceConfig := map[string]any{"resource_spec": map[string]any{}}
	if placed != nil {
		r.p.Info("Deploying through listener %s in namespace %s", placed.listenerID, placed.namespace)
		sourceConfig["listener_id"] = placed.listenerID
		sourceConfig["listener_config"] = map[string]any{"k8s_namespace": placed.namespace}
	}
	imageURI, err := r.publish(ctx, plan)
	if err != nil {
		return deployOutcome{}, err
	}
	created, err := r.create(ctx, langgraphapi.DeploymentCreate{
		Source:               sourceExternalDocker,
		SourceConfig:         sourceConfig,
		SourceRevisionConfig: map[string]any{"image_uri": imageURI},
	})
	if err != nil {
		if needsListener(err) {
			return deployOutcome{}, listenerRequiredError("Re-run with --listener-id and --k8s-namespace.\n" + err.Error())
		}
		return deployOutcome{}, err
	}
	return deployOutcome{deploymentID: created.ID, updated: created, wait: deployImageWait, message: "Deployment created"}, nil
}

func (r *deployRun) publish(ctx context.Context, plan deployPlan) (string, error) {
	image := plan.pushTo.String()
	r.p.Step("Validating image %s", plan.image)
	if err := r.docker.validatePrebuiltImage(ctx, plan.image); err != nil {
		return "", err
	}
	if _, err := r.docker.run(ctx, "", "tag", plan.image, image); err != nil {
		return "", err
	}
	r.p.Step("Pushing image %s", image)
	if err := r.docker.push(ctx, image, r.p); err != nil {
		return "", err
	}
	return r.docker.pushedDigest(ctx, image, r.p), nil
}

type requestedPlacement struct {
	listenerID string
	namespace  string
}

type placement struct {
	listenerID string
	namespace  string
}

func (rp requestedPlacement) requested() bool { return rp.listenerID != "" || rp.namespace != "" }

// resolvePlacement returns nil when the deployment is not placed on a listener.
func (r *deployRun) resolvePlacement(ctx context.Context, rp requestedPlacement) (*placement, error) {
	if rp.listenerID != "" {
		listener, err := r.hc.GetListener(ctx, rp.listenerID)
		if err != nil {
			if code := langgraphapi.StatusCode(err); code != http.StatusNotFound && code != http.StatusUnprocessableEntity {
				return nil, err
			}
			available, listErr := r.hc.ListListeners(ctx)
			if listErr != nil {
				return nil, listErr
			}
			if len(available) == 0 {
				return nil, errNoListeners
			}
			return nil, fmt.Errorf("listener %s was not found in this workspace. Available listeners:\n%s", rp.listenerID, describeListeners(available))
		}
		return rp.on(*listener)
	}
	if !r.hc.Endpoints().IsCloud() && !rp.requested() {
		return nil, nil
	}
	listeners, err := r.hc.ListListeners(ctx)
	if err != nil {
		return nil, err
	}
	switch len(listeners) {
	case 0:
		if rp.requested() {
			return nil, errNoListeners
		}
		return nil, nil
	case 1:
		return rp.on(listeners[0])
	default:
		return nil, fmt.Errorf("this workspace has several listeners; choose one with --listener-id:\n%s", describeListeners(listeners))
	}
}

var errNoListeners = errors.New("this workspace has no listeners, so --listener-id and --k8s-namespace do not apply")

func (rp requestedPlacement) on(l langgraphapi.Listener) (*placement, error) {
	namespaces := l.ComputeConfig.K8sNamespaces
	switch {
	case len(namespaces) == 0:
		return nil, fmt.Errorf("listener %s serves no namespaces; check its configuration", l.ID)
	case rp.namespace == "" && len(namespaces) == 1:
		return &placement{listenerID: l.ID, namespace: namespaces[0]}, nil
	case rp.namespace == "":
		return nil, fmt.Errorf("listener %s serves several namespaces; choose one with --k8s-namespace: %s", l.ID, strings.Join(namespaces, ", "))
	case !slices.Contains(namespaces, rp.namespace):
		return nil, fmt.Errorf("listener %s does not serve namespace '%s'; choose one of: %s", l.ID, rp.namespace, strings.Join(namespaces, ", "))
	default:
		return &placement{listenerID: l.ID, namespace: rp.namespace}, nil
	}
}

func describeListeners(listeners []langgraphapi.Listener) string {
	var lines []string
	for _, l := range listeners[:min(len(listeners), listenersShown)] {
		lines = append(lines, fmt.Sprintf("  %s  cluster %s  namespaces: %s", l.ID, l.ComputeID, strings.Join(l.ComputeConfig.K8sNamespaces, ", ")))
	}
	if extra := len(listeners) - listenersShown; extra > 0 {
		lines = append(lines, fmt.Sprintf("  ... and %d more", extra))
	}
	if len(listeners) == langgraphapi.MaxPageSize {
		lines = append(lines, fmt.Sprintf("  (only the first %d listeners were read)", langgraphapi.MaxPageSize))
	}
	return strings.Join(lines, "\n")
}

func (r *deployRun) wait(ctx context.Context, deploymentID string, w deployWait) (status, revisionID string, err error) {
	revisions, err := r.hc.ListRevisions(ctx, deploymentID, 1)
	if err != nil || len(revisions) == 0 {
		return "", "", err
	}
	revisionID = revisions[0].ID
	start := time.Now()
	deadline := start.Add(w.timeout)
	var logOffset string
	for time.Now().Before(deadline) {
		rev, err := r.hc.GetRevision(ctx, deploymentID, revisionID)
		if err != nil {
			return status, revisionID, err
		}
		if rev.Status != status {
			status = rev.Status
			r.p.Info("%s (%s)", status, formatElapsed(time.Since(start)))
			if slices.Contains(deployTerminalStatuses, status) {
				return status, revisionID, nil
			}
		}
		if r.verbose && w.remote && (status == "AWAITING_BUILD" || status == "BUILDING") {
			logOffset = r.streamBuildLogs(ctx, deploymentID, revisionID, logOffset)
		}
		timer := time.NewTimer(w.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return status, revisionID, ctx.Err()
		case <-timer.C:
		}
	}
	return status, revisionID, nil
}

func (r *deployRun) streamBuildLogs(ctx context.Context, deploymentID, revisionID, offset string) string {
	resp, err := r.hc.BuildLogs(ctx, deploymentID, revisionID, langgraphapi.LogsRequest{Order: "asc", Limit: 50, Offset: offset})
	if err != nil {
		return offset
	}
	for _, entry := range resp.Logs {
		if entry.Message != "" {
			r.p.Log(entry.Message)
		}
	}
	return cmp.Or(resp.NextOffset, offset)
}

func (r *deployRun) printBuildLogTail(ctx context.Context, deploymentID, revisionID string) {
	r.p.Info("Last build log lines:")
	resp, err := r.hc.BuildLogs(ctx, deploymentID, revisionID, langgraphapi.LogsRequest{Order: "desc", Limit: 30})
	if err != nil {
		r.p.Info("(failed to fetch build logs: %v)", err)
		return
	}
	for _, entry := range slices.Backward(resp.Logs) {
		if entry.Message != "" {
			r.p.Log(entry.Message)
		}
	}
	r.p.Info("Re-run with --verbose to see full build output.")
}

func formatElapsed(d time.Duration) string {
	secs := int(d.Seconds())
	if secs >= 60 {
		return fmt.Sprintf("%dm %02ds", secs/60, secs%60)
	}
	return fmt.Sprintf("%ds", secs)
}
