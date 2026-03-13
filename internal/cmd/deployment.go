package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/deployment"
	"github.com/spf13/cobra"
)

// API key env var names for deployment commands
var deploymentAPIKeyEnvNames = []string{
	"LANGGRAPH_HOST_API_KEY",
	"LANGSMITH_API_KEY",
	"LANGCHAIN_API_KEY",
}

// getDeploymentAPIKey resolves the API key for deployment commands.
func getDeploymentAPIKey(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	for _, envName := range deploymentAPIKeyEnvNames {
		if v := os.Getenv(envName); v != "" {
			return v
		}
	}
	return ""
}

func newDeploymentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deployment",
		Short: "Build, deploy, and manage LangGraph API servers",
		Long: `Build, deploy, and manage LangGraph API servers.

Commands for building Docker images, deploying to LangSmith,
and managing deployments.

Examples:
  langsmith deployment up
  langsmith deployment build --tag my-image
  langsmith deployment dev
  langsmith deployment deploy --name my-deployment
  langsmith deployment deploy list
  langsmith deployment deploy delete DEPLOYMENT_ID
  langsmith deployment dockerfile Dockerfile
  langsmith deployment new ./my-project --template react-agent-python`,
	}

	cmd.AddCommand(newDeploymentUpCmd())
	cmd.AddCommand(newDeploymentBuildCmd())
	cmd.AddCommand(newDeploymentDevCmd())
	cmd.AddCommand(newDeploymentDeployCmd())
	cmd.AddCommand(newDeploymentDockerfileCmd())
	cmd.AddCommand(newDeploymentNewCmd())

	return cmd
}

// ===================== UP =====================

func newDeploymentUpCmd() *cobra.Command {
	var (
		config            string
		dockerCompose     string
		port              int
		recreate          bool
		pull              bool
		watch             bool
		wait              bool
		verbose           bool
		debuggerPort      int
		debuggerBaseURL   string
		postgresURI       string
		apiVersion        string
		engineRuntimeMode string
		image             string
		baseImage         string
	)

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Launch LangGraph API server",
		Long: `Launch LangGraph API server using Docker Compose.

Sets up Redis, PostgreSQL (with pgvector), and the LangGraph API server.
Optionally includes a debugger UI.

Requires Docker and Docker Compose to be installed.

Examples:
  langsmith deployment up
  langsmith deployment up --port 8000 --watch
  langsmith deployment up --postgres-uri "postgres://user:pass@host:5432/db"
  langsmith deployment up --image my-prebuilt-image`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(os.Stderr, "Starting LangGraph API server...")
			fmt.Fprintln(os.Stderr, `For local dev, requires env var LANGSMITH_API_KEY with access to LangSmith Deployment.
For production use, requires a license key in env var LANGGRAPH_CLOUD_LICENSE_KEY.`)

			// Check Docker capabilities
			caps, err := deployment.CheckCapabilities()
			if err != nil {
				exitErrorf("Docker check failed: %v", err)
			}

			// Load and validate config
			configPath := resolveConfigPath(config)
			cfg, err := deployment.ValidateConfigFile(configPath)
			if err != nil {
				exitErrorf("Invalid config: %v", err)
			}

			deployment.WarnNonWolfiDistro(cfg)

			// Pull base image if requested
			if pull {
				tag := deployment.DockerTag(baseImage, cfg, apiVersion, engineRuntimeMode)
				fmt.Fprintf(os.Stderr, "Pulling %s...\n", tag)
				_ = deployment.RunCommandVerbose(verbose, "docker", "pull", tag)
			}

			// Build compose args
			opts := deployment.ComposeOptions{
				Port:              port,
				DebuggerPort:      debuggerPort,
				DebuggerBaseURL:   debuggerBaseURL,
				PostgresURI:       postgresURI,
				Image:             image,
				BaseImage:         baseImage,
				APIVersion:        apiVersion,
				EngineRuntimeMode: engineRuntimeMode,
			}

			composeArgs, stdin, err := deployment.ConfigToCompose(
				caps, configPath, cfg, opts, watch, dockerCompose, apiVersion, engineRuntimeMode,
			)
			if err != nil {
				exitErrorf("Building compose config: %v", err)
			}

			// Build docker compose command
			composeCmd := dockerComposeCommand(caps)
			allArgs := append(composeCmd[1:], composeArgs...)
			allArgs = append(allArgs, "up", "--remove-orphans")

			if recreate {
				allArgs = append(allArgs, "--force-recreate", "--renew-anon-volumes")
				// Try to remove old volume
				_ = deployment.RunCommandVerbose(false, "docker", "volume", "rm", "langgraph-data")
			}
			if watch {
				allArgs = append(allArgs, "--watch")
			}
			if wait {
				allArgs = append(allArgs, "--wait")
			} else {
				allArgs = append(allArgs, "--abort-on-container-exit")
			}

			if verbose {
				fmt.Fprintf(os.Stderr, "+ %s %s\n", composeCmd[0], strings.Join(allArgs, " "))
			}

			err = deployment.RunCommandWithInputVerbose(true, stdin, composeCmd[0], allArgs...)
			if err != nil {
				exitErrorf("Docker compose failed: %v", err)
			}
		},
	}

	cmd.Flags().StringVarP(&config, "config", "c", deployment.DefaultConfig, "Path to langgraph.json config file")
	cmd.Flags().StringVarP(&dockerCompose, "docker-compose", "d", "", "Path to additional docker-compose.yml")
	cmd.Flags().IntVarP(&port, "port", "p", deployment.DefaultPort, "Port to expose")
	cmd.Flags().BoolVar(&recreate, "recreate", false, "Recreate containers even if unchanged")
	cmd.Flags().BoolVar(&pull, "pull", true, "Pull latest images")
	cmd.Flags().BoolVar(&watch, "watch", false, "Restart on file changes")
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait for services to start before returning")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Show more output")
	cmd.Flags().IntVar(&debuggerPort, "debugger-port", 0, "Port for the debugger UI")
	cmd.Flags().StringVar(&debuggerBaseURL, "debugger-base-url", "", "URL for debugger to access LangGraph API")
	cmd.Flags().StringVar(&postgresURI, "postgres-uri", "", "Postgres URI (defaults to launching a local database)")
	cmd.Flags().StringVar(&apiVersion, "api-version", "", "API server version for the base image")
	cmd.Flags().StringVar(&engineRuntimeMode, "engine-runtime-mode", "combined_queue_worker", "Runtime mode: combined_queue_worker or distributed")
	cmd.Flags().StringVar(&image, "image", "", "Pre-built Docker image to use (skips building)")
	cmd.Flags().StringVar(&baseImage, "base-image", "", "Base image for the LangGraph API server")

	return cmd
}

// ===================== BUILD =====================

func newDeploymentBuildCmd() *cobra.Command {
	var (
		config            string
		tag               string
		pull              bool
		baseImage         string
		apiVersion        string
		engineRuntimeMode string
	)

	cmd := &cobra.Command{
		Use:   "build [flags] [-- DOCKER_BUILD_ARGS...]",
		Short: "Build LangGraph API server Docker image",
		Long: `Build a Docker image for the LangGraph API server.

Generates a Dockerfile from the config and builds it using Docker.

Examples:
  langsmith deployment build --tag my-image
  langsmith deployment build --tag my-image --base-image langchain/langgraph-api:0.2.18
  langsmith deployment build --tag my-image -- --platform linux/amd64`,
		Run: func(cmd *cobra.Command, args []string) {
			if tag == "" {
				exitError("--tag is required")
			}

			// Check Docker
			if _, err := exec.LookPath("docker"); err != nil {
				exitError("Docker not installed")
			}

			// Load and validate config
			configPath := resolveConfigPath(config)
			cfg, err := deployment.ValidateConfigFile(configPath)
			if err != nil {
				exitErrorf("Invalid config: %v", err)
			}

			deployment.WarnNonWolfiDistro(cfg)

			configDir, _ := filepath.Abs(filepath.Dir(configPath))
			fromImage := deployment.DockerTag(baseImage, cfg, apiVersion, engineRuntimeMode)

			fmt.Fprintf(os.Stderr, "Building image %s from %s...\n", tag, fromImage)

			// Pull base image if requested
			if pull {
				fmt.Fprintf(os.Stderr, "Pulling %s...\n", fromImage)
				_ = deployment.RunCommandVerbose(true, "docker", "pull", fromImage)
			}

			// Generate Dockerfile
			dockerfile, err := deployment.ConfigToDocker(cfg, configDir, baseImage, apiVersion, engineRuntimeMode)
			if err != nil {
				exitErrorf("Generating Dockerfile: %v", err)
			}

			// Build Docker args
			buildArgs := []string{
				"build",
				"-t", tag,
				"-f", "-",
			}

			// Add build contexts for parent deps
			if deps, ok := cfg["dependencies"].([]any); ok {
				for i, dep := range deps {
					if depStr, ok := dep.(string); ok && strings.HasPrefix(depStr, "../") {
						absPath, _ := filepath.Abs(filepath.Join(configDir, depStr))
						buildArgs = append(buildArgs, "--build-context", fmt.Sprintf("cli_%d=%s", i, absPath))
					}
				}
			}

			// Add any extra Docker build args passed after --
			buildArgs = append(buildArgs, args...)
			buildArgs = append(buildArgs, configDir)

			fmt.Fprintf(os.Stderr, "+ docker %s\n", strings.Join(buildArgs, " "))

			err = deployment.RunCommandWithInputVerbose(true, dockerfile, "docker", buildArgs...)
			if err != nil {
				exitErrorf("Docker build failed: %v", err)
			}

			fmt.Fprintf(os.Stderr, "Successfully built %s\n", tag)
		},
	}

	cmd.Flags().StringVarP(&config, "config", "c", deployment.DefaultConfig, "Path to langgraph.json config file")
	cmd.Flags().StringVarP(&tag, "tag", "t", "", "Tag for the Docker image (required)")
	cmd.Flags().BoolVar(&pull, "pull", true, "Pull latest base image")
	cmd.Flags().StringVar(&baseImage, "base-image", "", "Base image for the LangGraph API server")
	cmd.Flags().StringVar(&apiVersion, "api-version", "", "API server version for the base image")
	cmd.Flags().StringVar(&engineRuntimeMode, "engine-runtime-mode", "combined_queue_worker", "Runtime mode: combined_queue_worker or distributed")

	return cmd
}

// ===================== DEV =====================

func newDeploymentDevCmd() *cobra.Command {
	var (
		host           string
		port           int
		noReload       bool
		config         string
		nJobsPerWorker int
		noBrowser      bool
		debugPort      int
		studioURL      string
		allowBlocking  bool
		serverLogLevel string
	)

	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Run LangGraph API server in development mode",
		Long: `Run a LangGraph API server in lightweight development mode with hot reloading.

This requires the Python 'langgraph-cli[inmem]' package. The command will:
  1. Use an existing 'langgraph' CLI if found in PATH (no internet needed)
  2. Otherwise, use 'uvx' to run it (cached — no download if already used before)

For a full Docker-based server, use 'langsmith deployment up' instead.

Examples:
  langsmith deployment dev
  langsmith deployment dev --port 8000 --no-browser
  langsmith deployment dev --config langgraph.json`,
		Run: func(cmd *cobra.Command, args []string) {
			// Build the args list for 'langgraph dev'
			devArgs := []string{"dev"}
			devArgs = append(devArgs, "--host", host)
			devArgs = append(devArgs, "--port", fmt.Sprintf("%d", port))
			if noReload {
				devArgs = append(devArgs, "--no-reload")
			}
			if config != "" && config != deployment.DefaultConfig {
				devArgs = append(devArgs, "--config", config)
			}
			if nJobsPerWorker > 0 {
				devArgs = append(devArgs, "--n-jobs-per-worker", fmt.Sprintf("%d", nJobsPerWorker))
			}
			if noBrowser {
				devArgs = append(devArgs, "--no-browser")
			}
			if debugPort > 0 {
				devArgs = append(devArgs, "--debug-port", fmt.Sprintf("%d", debugPort))
			}
			if studioURL != "" {
				devArgs = append(devArgs, "--studio-url", studioURL)
			}
			if allowBlocking {
				devArgs = append(devArgs, "--allow-blocking")
			}
			if serverLogLevel != "" {
				devArgs = append(devArgs, "--server-log-level", serverLogLevel)
			}

			// Strategy 1: use 'langgraph' if already in PATH
			if lgPath, err := exec.LookPath("langgraph"); err == nil {
				fmt.Fprintf(os.Stderr, "Using langgraph at %s\n", lgPath)
				err := deployment.RunCommandWithInputVerbose(false, "", lgPath, devArgs...)
				if err != nil {
					exitErrorf("dev server failed: %v", err)
				}
				return
			}

			// Strategy 2: use 'uvx' (uv tool run) which caches environments
			if uvxPath, err := exec.LookPath("uvx"); err == nil {
				uvxArgs := []string{"--from", "langgraph-cli[inmem]", "langgraph"}
				uvxArgs = append(uvxArgs, devArgs...)
				fmt.Fprintln(os.Stderr, "Running via uvx (will use cache if available)...")
				err := deployment.RunCommandWithInputVerbose(false, "", uvxPath, uvxArgs...)
				if err != nil {
					exitErrorf("dev server failed: %v", err)
				}
				return
			}

			// Strategy 2b: try 'uv tool run' if 'uvx' alias doesn't exist
			if uvPath, err := exec.LookPath("uv"); err == nil {
				uvArgs := []string{"tool", "run", "--from", "langgraph-cli[inmem]", "langgraph"}
				uvArgs = append(uvArgs, devArgs...)
				fmt.Fprintln(os.Stderr, "Running via uv tool run (will use cache if available)...")
				err := deployment.RunCommandWithInputVerbose(false, "", uvPath, uvArgs...)
				if err != nil {
					exitErrorf("dev server failed: %v", err)
				}
				return
			}

			exitError(`Could not find 'langgraph' or 'uvx'/'uv' in PATH.

Install the LangGraph dev server using one of:
  uv tool install 'langgraph-cli[inmem]'    # then 'langgraph' is always available
  pip install 'langgraph-cli[inmem]'         # adds 'langgraph' to PATH
  uvx --from 'langgraph-cli[inmem]' langgraph dev  # one-off, cached after first run`)
		},
	}

	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "Host to bind to")
	cmd.Flags().IntVarP(&port, "port", "p", deployment.DefaultPort, "Port to expose")
	cmd.Flags().BoolVar(&noReload, "no-reload", false, "Disable auto-reload")
	cmd.Flags().StringVarP(&config, "config", "c", deployment.DefaultConfig, "Path to langgraph.json config file")
	cmd.Flags().IntVar(&nJobsPerWorker, "n-jobs-per-worker", 0, "Number of jobs per worker")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Don't open browser on startup")
	cmd.Flags().IntVar(&debugPort, "debug-port", 0, "Port for the debugger")
	cmd.Flags().StringVar(&studioURL, "studio-url", "", "URL for LangGraph Studio")
	cmd.Flags().BoolVar(&allowBlocking, "allow-blocking", false, "Allow blocking operations")
	cmd.Flags().StringVar(&serverLogLevel, "server-log-level", "", "Server log level")

	return cmd
}

// ===================== DEPLOY =====================

func newDeploymentDeployCmd() *cobra.Command {
	var (
		config  string
		tag     string
		apiKey  string
		hostURL string
		name    string
		wait    bool
		verbose bool
	)

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy to LangSmith or manage deployments",
		Long: `Deploy a LangGraph API server to LangSmith, or manage existing deployments.

When invoked without a subcommand, builds and pushes a Docker image
to LangSmith and creates/updates a deployment.

Subcommands:
  list    List existing deployments
  delete  Delete a deployment
  logs    Fetch deployment logs

Examples:
  langsmith deployment deploy --name my-deployment
  langsmith deployment deploy list
  langsmith deployment deploy delete DEPLOYMENT_ID
  langsmith deployment deploy logs --name my-deployment`,
		Run: func(cmd *cobra.Command, args []string) {
			// Full deploy flow
			key := getDeploymentAPIKey(apiKey)
			if key == "" {
				exitError("API key required. Set LANGSMITH_API_KEY or use --api-key")
			}

			// Load config
			configPath := resolveConfigPath(config)
			cfg, err := deployment.ValidateConfigFile(configPath)
			if err != nil {
				exitErrorf("Invalid config: %v", err)
			}

			deployment.WarnNonWolfiDistro(cfg)

			// Resolve deployment name
			deployName := name
			if deployName == "" {
				deployName = os.Getenv("LANGSMITH_DEPLOYMENT_NAME")
			}
			if deployName == "" {
				// Default to current directory name
				cwd, _ := os.Getwd()
				deployName = filepath.Base(cwd)
			}

			client := deployment.NewHostBackendClient(hostURL, key, "")

			// Create or find deployment
			fmt.Fprintf(os.Stderr, "Finding or creating deployment '%s'...\n", deployName)
			dep, err := client.CreateDeployment(map[string]any{
				"name": deployName,
			})
			if err != nil {
				// Maybe it already exists, try to find it
				existing, listErr := client.ListDeployments(deployName)
				if listErr != nil {
					exitErrorf("Creating deployment: %v", err)
				}
				resources, _ := existing["resources"].([]any)
				found := false
				for _, r := range resources {
					d, ok := r.(map[string]any)
					if !ok {
						continue
					}
					if d["name"] == deployName {
						dep = d
						found = true
						break
					}
				}
				if !found {
					exitErrorf("Creating deployment: %v", err)
				}
			}

			deploymentID, _ := dep["id"].(string)
			fmt.Fprintf(os.Stderr, "Deployment ID: %s\n", deploymentID)

			// Build the image
			configDir, _ := filepath.Abs(filepath.Dir(configPath))
			imageTag := tag
			if imageTag == "" {
				imageTag = fmt.Sprintf("langgraph-deploy-%s:latest", deployName)
			}

			fmt.Fprintf(os.Stderr, "Building image %s...\n", imageTag)
			dockerfile, err := deployment.ConfigToDocker(cfg, configDir, "", "", "combined_queue_worker")
			if err != nil {
				exitErrorf("Generating Dockerfile: %v", err)
			}

			buildArgs := []string{"build", "-t", imageTag, "-f", "-"}

			// Add build contexts for parent deps
			if deps, ok := cfg["dependencies"].([]any); ok {
				for i, dep := range deps {
					if depStr, ok := dep.(string); ok && strings.HasPrefix(depStr, "../") {
						absPath, _ := filepath.Abs(filepath.Join(configDir, depStr))
						buildArgs = append(buildArgs, "--build-context", fmt.Sprintf("cli_%d=%s", i, absPath))
					}
				}
			}
			buildArgs = append(buildArgs, configDir)

			err = deployment.RunCommandWithInputVerbose(verbose, dockerfile, "docker", buildArgs...)
			if err != nil {
				exitErrorf("Docker build failed: %v", err)
			}

			// Request push token
			fmt.Fprintln(os.Stderr, "Requesting push token...")
			tokenResp, err := client.RequestPushToken(deploymentID)
			if err != nil {
				exitErrorf("Requesting push token: %v", err)
			}

			registryHost, _ := tokenResp["registry_host"].(string)
			token, _ := tokenResp["token"].(string)
			imageURI, _ := tokenResp["image_uri"].(string)

			if registryHost == "" || token == "" || imageURI == "" {
				exitError("Invalid push token response")
			}

			// Create temp Docker config with push token
			tmpDir, err := os.MkdirTemp("", "langsmith-deploy-*")
			if err != nil {
				exitErrorf("Creating temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			authB64 := base64.StdEncoding.EncodeToString([]byte("oauth2accesstoken:" + token))
			dockerConfig := map[string]any{
				"auths": map[string]any{
					registryHost: map[string]any{
						"auth": authB64,
					},
				},
			}
			configJSON, _ := json.Marshal(dockerConfig)
			if err := os.WriteFile(filepath.Join(tmpDir, "config.json"), configJSON, 0600); err != nil {
				exitErrorf("Writing docker config: %v", err)
			}

			// Tag and push
			fmt.Fprintf(os.Stderr, "Pushing to %s...\n", imageURI)
			_, _, err = deployment.RunCommand("docker", "tag", imageTag, imageURI)
			if err != nil {
				exitErrorf("Tagging image: %v", err)
			}

			err = deployment.RunCommandVerbose(verbose, "docker", "--config", tmpDir, "push", imageURI)
			if err != nil {
				exitErrorf("Pushing image: %v", err)
			}

			// Update deployment with secrets
			secrets := deployment.SecretsFromEnv(cfg)
			fmt.Fprintln(os.Stderr, "Updating deployment...")
			var secretMaps []map[string]string
			for _, s := range secrets {
				secretMaps = append(secretMaps, s)
			}
			_, err = client.UpdateDeployment(deploymentID, imageURI, secretMaps)
			if err != nil {
				exitErrorf("Updating deployment: %v", err)
			}

			// Wait for deployment
			if wait {
				fmt.Fprintln(os.Stderr, "Waiting for deployment...")
				terminalStatuses := map[string]bool{
					"DEPLOYED":      true,
					"CREATE_FAILED": true,
					"BUILD_FAILED":  true,
					"DEPLOY_FAILED": true,
					"SKIPPED":       true,
				}

				timeout := time.After(5 * time.Minute)
				ticker := time.NewTicker(1 * time.Second)
				defer ticker.Stop()

				for {
					select {
					case <-timeout:
						exitError("Deployment timed out after 5 minutes")
					case <-ticker.C:
						revisions, err := client.ListRevisions(deploymentID, 1)
						if err != nil {
							continue
						}
						resources, _ := revisions["resources"].([]any)
						if len(resources) == 0 {
							continue
						}
						rev, _ := resources[0].(map[string]any)
						status, _ := rev["status"].(string)
						fmt.Fprintf(os.Stderr, "  Status: %s\n", status)

						if terminalStatuses[status] {
							if status == "DEPLOYED" {
								// Print URL if available
								depInfo, _ := client.GetDeployment(deploymentID)
								if depInfo != nil {
									if sc, ok := depInfo["source_config"].(map[string]any); ok {
										if url, ok := sc["custom_url"].(string); ok && url != "" {
											fmt.Fprintf(os.Stderr, "Deployment URL: %s\n", url)
										}
									}
								}
								fmt.Fprintln(os.Stderr, "Deployment successful!")
							} else {
								exitErrorf("Deployment failed with status: %s", status)
							}
							goto done
						}
					}
				}
			done:
			}

			fmt.Fprintln(os.Stderr, "Done!")
		},
	}

	cmd.Flags().StringVarP(&config, "config", "c", deployment.DefaultConfig, "Path to langgraph.json config file")
	cmd.Flags().StringVarP(&tag, "tag", "t", "", "Tag for the Docker image")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key [env: LANGSMITH_API_KEY]")
	cmd.Flags().StringVar(&hostURL, "host-url", "https://api.smith.langchain.com", "LangSmith host URL")
	cmd.Flags().StringVar(&name, "name", "", "Deployment name")
	cmd.Flags().BoolVar(&wait, "wait", true, "Wait for deployment to complete")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Show more output")

	cmd.AddCommand(newDeploymentDeployListCmd())
	cmd.AddCommand(newDeploymentDeployDeleteCmd())
	cmd.AddCommand(newDeploymentDeployLogsCmd())

	return cmd
}

func newDeploymentDeployListCmd() *cobra.Command {
	var (
		apiKey       string
		hostURL      string
		nameContains string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List LangSmith Deployments",
		Run: func(cmd *cobra.Command, args []string) {
			key := getDeploymentAPIKey(apiKey)
			if key == "" {
				exitError("API key required. Set LANGSMITH_API_KEY or use --api-key")
			}

			client := deployment.NewHostBackendClient(hostURL, key, "")
			result, err := client.ListDeployments(nameContains)
			if err != nil {
				exitErrorf("Listing deployments: %v", err)
			}

			resources, _ := result["resources"].([]any)
			if len(resources) == 0 {
				cmd.Println("No deployments found.")
				return
			}

			var deployments []map[string]any
			for _, r := range resources {
				if d, ok := r.(map[string]any); ok {
					deployments = append(deployments, d)
				}
			}

			cmd.Println(deployment.FormatDeploymentsTable(deployments))
		},
	}

	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key [env: LANGSMITH_API_KEY]")
	cmd.Flags().StringVar(&hostURL, "host-url", "https://api.smith.langchain.com", "LangSmith host URL")
	cmd.Flags().StringVar(&nameContains, "name-contains", "", "Filter by name substring")

	return cmd
}

func newDeploymentDeployDeleteCmd() *cobra.Command {
	var (
		apiKey  string
		hostURL string
		force   bool
	)

	cmd := &cobra.Command{
		Use:   "delete DEPLOYMENT_ID",
		Short: "Delete a LangSmith Deployment",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			deploymentID := args[0]

			key := getDeploymentAPIKey(apiKey)
			if key == "" {
				exitError("API key required. Set LANGSMITH_API_KEY or use --api-key")
			}

			if !force {
				fmt.Fprintf(os.Stderr, "Are you sure you want to delete deployment ID %s? (Y/n): ", deploymentID)
				var confirm string
				fmt.Scanln(&confirm)
				confirm = strings.TrimSpace(strings.ToLower(confirm))
				if confirm != "" && confirm != "y" && confirm != "yes" {
					fmt.Fprintln(os.Stderr, "Aborted!")
					os.Exit(1)
				}
			}

			client := deployment.NewHostBackendClient(hostURL, key, "")
			err := client.DeleteDeployment(deploymentID)
			if err != nil {
				exitErrorf("Deleting deployment: %v", err)
			}

			cmd.Printf("Deleted deployment %s.\n", deploymentID)
		},
	}

	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key [env: LANGSMITH_API_KEY]")
	cmd.Flags().StringVar(&hostURL, "host-url", "https://api.smith.langchain.com", "LangSmith host URL")
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")

	return cmd
}

func newDeploymentDeployLogsCmd() *cobra.Command {
	var (
		apiKey       string
		hostURL      string
		name         string
		deploymentID string
		logType      string
		revisionID   string
		level        string
		limit        int
		query        string
		startTime    string
		endTime      string
		follow       bool
	)

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Fetch deployment logs",
		Long: `Fetch build or deploy logs for a LangSmith deployment.

Examples:
  langsmith deployment deploy logs --name my-deployment
  langsmith deployment deploy logs --deployment-id DEP_ID --type build
  langsmith deployment deploy logs --name my-deployment --follow`,
		Run: func(cmd *cobra.Command, args []string) {
			key := getDeploymentAPIKey(apiKey)
			if key == "" {
				exitError("API key required. Set LANGSMITH_API_KEY or use --api-key")
			}

			client := deployment.NewHostBackendClient(hostURL, key, "")

			depID, err := deployment.ResolveDeploymentID(client, deploymentID, name)
			if err != nil {
				exitErrorf("%v", err)
			}

			// Get deployment info for project_id
			depInfo, err := client.GetDeployment(depID)
			if err != nil {
				exitErrorf("Getting deployment: %v", err)
			}
			projectID, _ := depInfo["project_id"].(string)
			if projectID == "" {
				exitError("Deployment has no associated project")
			}

			// Resolve revision ID if not specified
			revID := revisionID
			if revID == "" {
				revisions, err := client.ListRevisions(depID, 1)
				if err == nil {
					if resources, ok := revisions["resources"].([]any); ok && len(resources) > 0 {
						if rev, ok := resources[0].(map[string]any); ok {
							revID, _ = rev["id"].(string)
						}
					}
				}
			}

			// Build payload
			payload := map[string]any{
				"limit": limit,
			}
			if level != "" {
				payload["level"] = level
			}
			if query != "" {
				payload["query"] = query
			}
			if startTime != "" {
				payload["start_time"] = startTime
			}
			if endTime != "" {
				payload["end_time"] = endTime
			}

			fetchAndPrintLogs := func() {
				var result map[string]any
				var fetchErr error

				if logType == "build" {
					if revID == "" {
						exitError("Revision ID required for build logs. Use --revision-id or ensure deployment has revisions.")
					}
					result, fetchErr = client.GetBuildLogs(projectID, revID, payload)
				} else {
					result, fetchErr = client.GetDeployLogs(projectID, revID, payload)
				}

				if fetchErr != nil {
					exitErrorf("Fetching logs: %v", fetchErr)
				}

				if result == nil {
					fmt.Fprintln(os.Stderr, "No logs found.")
					return
				}

				// Print log entries
				entries, _ := result["entries"].([]any)
				for _, e := range entries {
					entry, ok := e.(map[string]any)
					if !ok {
						continue
					}
					formatted := deployment.FormatLogEntry(entry)
					lvl, _ := entry["level"].(string)
					color := deployment.LevelColor(lvl)
					if color != "" {
						fmt.Printf("%s%s%s\n", color, formatted, deployment.ColorReset)
					} else {
						fmt.Println(formatted)
					}
				}
			}

			fetchAndPrintLogs()

			if follow {
				ticker := time.NewTicker(2 * time.Second)
				defer ticker.Stop()
				for range ticker.C {
					fetchAndPrintLogs()
				}
			}
		},
	}

	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key [env: LANGSMITH_API_KEY]")
	cmd.Flags().StringVar(&hostURL, "host-url", "https://api.smith.langchain.com", "LangSmith host URL")
	cmd.Flags().StringVar(&name, "name", "", "Deployment name")
	cmd.Flags().StringVar(&deploymentID, "deployment-id", "", "Deployment ID")
	cmd.Flags().StringVar(&logType, "type", "deploy", "Log type: deploy or build")
	cmd.Flags().StringVar(&revisionID, "revision-id", "", "Revision ID")
	cmd.Flags().StringVar(&level, "level", "", "Log level filter")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of log entries")
	cmd.Flags().StringVar(&query, "query", "", "Search query")
	cmd.Flags().StringVar(&startTime, "start-time", "", "Start time filter (ISO 8601)")
	cmd.Flags().StringVar(&endTime, "end-time", "", "End time filter (ISO 8601)")
	cmd.Flags().BoolVar(&follow, "follow", false, "Follow logs in real-time")

	return cmd
}

// ===================== DOCKERFILE =====================

func newDeploymentDockerfileCmd() *cobra.Command {
	var (
		config            string
		addDockerCompose  bool
		baseImage         string
		apiVersion        string
		engineRuntimeMode string
	)

	cmd := &cobra.Command{
		Use:   "dockerfile SAVE_PATH",
		Short: "Generate a Dockerfile for the LangGraph API server",
		Long: `Generate a Dockerfile (and optionally Docker Compose files) for the LangGraph API server.

Examples:
  langsmith deployment dockerfile Dockerfile
  langsmith deployment dockerfile Dockerfile --add-docker-compose
  langsmith deployment dockerfile Dockerfile --api-version 0.2.74
  langsmith deployment dockerfile Dockerfile --engine-runtime-mode distributed`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			savePath := args[0]

			configPath := resolveConfigPath(config)
			cfg, err := deployment.ValidateConfigFile(configPath)
			if err != nil {
				exitErrorf("Invalid config: %v", err)
			}

			deployment.WarnNonWolfiDistro(cfg)

			configDir, _ := filepath.Abs(filepath.Dir(configPath))
			dockerfile, err := deployment.ConfigToDocker(cfg, configDir, baseImage, apiVersion, engineRuntimeMode)
			if err != nil {
				exitErrorf("Generating Dockerfile: %v", err)
			}

			// Write Dockerfile
			if err := os.WriteFile(savePath, []byte(dockerfile+"\n"), 0644); err != nil {
				exitErrorf("Writing Dockerfile: %v", err)
			}
			fmt.Fprintf(os.Stderr, "Created: Dockerfile at %s\n", savePath)

			if addDockerCompose {
				saveDir := filepath.Dir(savePath)
				if saveDir == "" {
					saveDir = "."
				}

				// Generate .dockerignore
				dockerIgnore := ".git\n__pycache__\n*.pyc\n.env\nnode_modules\n"
				ignorePath := filepath.Join(saveDir, ".dockerignore")
				if err := os.WriteFile(ignorePath, []byte(dockerIgnore), 0644); err != nil {
					exitErrorf("Writing .dockerignore: %v", err)
				}
				fmt.Fprintf(os.Stderr, "Created: .dockerignore\n")

				// Generate docker-compose.yml
				caps := &deployment.DockerCapabilities{
					VersionDocker:            deployment.Version{Major: 25, Minor: 0, Patch: 0},
					VersionCompose:           deployment.Version{Major: 2, Minor: 27, Patch: 0},
					HealthcheckStartInterval: true,
					ComposeType:              deployment.ComposePlugin,
				}
				composeStr := deployment.Compose(caps, deployment.ComposeOptions{
					Port:              deployment.DefaultPort,
					EngineRuntimeMode: engineRuntimeMode,
				})
				composePath := filepath.Join(saveDir, "docker-compose.yml")
				if err := os.WriteFile(composePath, []byte(composeStr), 0644); err != nil {
					exitErrorf("Writing docker-compose.yml: %v", err)
				}
				fmt.Fprintf(os.Stderr, "Created: docker-compose.yml\n")

				// Generate .env if it doesn't exist
				envPath := filepath.Join(saveDir, ".env")
				if _, err := os.Stat(envPath); os.IsNotExist(err) {
					if err := os.WriteFile(envPath, []byte("# Add your environment variables here\n"), 0644); err != nil {
						exitErrorf("Writing .env: %v", err)
					}
					fmt.Fprintf(os.Stderr, "Created: .env\n")
				} else {
					fmt.Fprintf(os.Stderr, "Skipped: .env (already exists)\n")
				}

				fmt.Fprintf(os.Stderr, "Files generated successfully!\n")
			}
		},
	}

	cmd.Flags().StringVarP(&config, "config", "c", deployment.DefaultConfig, "Path to langgraph.json config file")
	cmd.Flags().BoolVar(&addDockerCompose, "add-docker-compose", false, "Also generate docker-compose.yml and related files")
	cmd.Flags().StringVar(&baseImage, "base-image", "", "Base image for the LangGraph API server")
	cmd.Flags().StringVar(&apiVersion, "api-version", "", "API server version for the base image")
	cmd.Flags().StringVar(&engineRuntimeMode, "engine-runtime-mode", "combined_queue_worker", "Runtime mode: combined_queue_worker or distributed")

	return cmd
}

// ===================== NEW =====================

func newDeploymentNewCmd() *cobra.Command {
	var template string

	cmd := &cobra.Command{
		Use:   "new [PATH]",
		Short: "Create a new LangGraph project from a template",
		Long: `Create a new LangGraph project from a template.

Available templates:
` + deployment.TemplateHelpString() + `

Examples:
  langsmith deployment new ./my-project --template react-agent-python
  langsmith deployment new --template new-langgraph-project-python`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			path := "."
			if len(args) > 0 {
				path = args[0]
			}

			if template == "" {
				exitError("--template is required. Use --help to see available templates.")
			}

			if err := deployment.CreateNew(path, template); err != nil {
				exitErrorf("%v", err)
			}
		},
	}

	cmd.Flags().StringVar(&template, "template", "", "Template to use (see --help for options)")

	return cmd
}

// ===================== HELPERS =====================

func resolveConfigPath(config string) string {
	if config == "" {
		config = deployment.DefaultConfig
	}
	absPath, err := filepath.Abs(config)
	if err != nil {
		return config
	}
	return absPath
}

func dockerComposeCommand(caps *deployment.DockerCapabilities) []string {
	if caps.ComposeType == deployment.ComposePlugin {
		return []string{"docker", "compose"}
	}
	return []string{"docker-compose"}
}

