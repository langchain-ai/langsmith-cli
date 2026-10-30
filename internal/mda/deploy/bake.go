package deploy

// Bake a deployment's sandbox recipe snapshot from `setup.sh` + bake bases
// (`deploy/bake.rs`).
//
// Fail closed: non-zero setup exit or a snapshot that never becomes `ready`
// aborts deploy before the recipe pointer is persisted.

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/langchain-ai/langsmith-cli/internal/mda/compile"
	"github.com/langchain-ai/langsmith-cli/internal/mda/deployment"
	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
)

// defaultDockerFsCapacityBytes is the default filesystem capacity when
// building a snapshot from a Docker image.
const defaultDockerFsCapacityBytes uint64 = 1 << 30

// dockerSnapshotTimeout bounds create-from-docker-image readiness.
const dockerSnapshotTimeout = 300 * time.Second

// builderTimeoutSecs is the builder box create wait.
const builderTimeoutSecs = 180

// builderIdleTTLSecs is the builder idle TTL — short; bake should finish well
// before this.
const builderIdleTTLSecs = 900

// setupTimeoutSecs is the setup execute timeout.
const setupTimeoutSecs = 600

// RecipeSpec is the inputs that identify and provision a recipe snapshot.
type RecipeSpec struct {
	// SetupScript is the contents of `sandbox/setup.sh` (may be empty).
	SetupScript string
	// SnapshotName is the author bake base: a LangSmith snapshot name.
	SnapshotName *string
	// SnapshotID is the author bake base: a LangSmith snapshot id.
	SnapshotID *string
	// DockerImage is the author bake base: a Docker image.
	DockerImage *string
	// Registry is the secret-safe private registry declaration.
	Registry *compile.SandboxRegistryManifest
	// FsCapacityBytes is the filesystem capacity for a snapshot built from
	// DockerImage.
	FsCapacityBytes *uint64
}

func (s *RecipeSpec) dockerFsCapacityBytes() uint64 {
	if s.FsCapacityBytes != nil {
		return *s.FsCapacityBytes
	}
	return defaultDockerFsCapacityBytes
}

// BakePhase is a coarse stage of a bake, reported so a multi-minute cold bake
// shows which step is running instead of one static spinner line.
type BakePhase int

// The phases.
const (
	// BakePhaseCheckingRecipe: looking for a ready recipe snapshot for the
	// current digest.
	BakePhaseCheckingRecipe BakePhase = iota
	// BakePhaseStoringRegistry: storing the private registry credential on
	// Host.
	BakePhaseStoringRegistry
	// BakePhaseBuildingFromImage: building a snapshot from the author's Docker
	// image.
	BakePhaseBuildingFromImage
	// BakePhaseStartingBuilder: creating the builder sandbox that `setup.sh`
	// runs on.
	BakePhaseStartingBuilder
	// BakePhaseRunningSetup: executing `setup.sh` on the builder.
	BakePhaseRunningSetup
	// BakePhaseCapturingSnapshot: capturing the builder filesystem as the
	// recipe snapshot.
	BakePhaseCapturingSnapshot
	// BakePhaseCleaningUp: tearing down temporary bake resources.
	BakePhaseCleaningUp
)

// Label is a short, lower-case description for the CLI sub-line.
func (p BakePhase) Label() string {
	switch p {
	case BakePhaseCheckingRecipe:
		return "checking for a ready recipe snapshot"
	case BakePhaseStoringRegistry:
		return "storing registry credentials"
	case BakePhaseBuildingFromImage:
		return "building snapshot from Docker image"
	case BakePhaseStartingBuilder:
		return "starting builder sandbox"
	case BakePhaseRunningSetup:
		return "running setup.sh"
	case BakePhaseCapturingSnapshot:
		return "capturing recipe snapshot"
	default:
		return "cleaning up"
	}
}

// BakeOutcome is the result of [bakeRecipe].
type BakeOutcome struct {
	// RecipeName is the deterministic recipe snapshot name
	// (`{deployment}--setup-{digest}`).
	//
	// Bookkeeping identity: name matching and stale-recipe cleanup. Not what
	// the runtime resolves — use RecipeReference for that.
	RecipeName string
	// RecipeReference is the reference the runtime clones from, tagged when
	// Host did not tag the snapshot `latest`. See
	// [deployment.RecipeSnapshotReference].
	RecipeReference string
	// Skipped is true when a ready snapshot with that name already existed.
	Skipped bool
}

// bakeRecipe bakes or reuses the recipe snapshot for deploymentName.
//
// envVars are non-reserved project secrets forwarded onto the builder box so
// `setup.sh` can reach private registries / APIs without embedding secrets in
// the script.
//
// progress is called as the bake enters each [BakePhase].
func bakeRecipe(ctx context.Context, client *hostapi.Client, deploymentName string, spec *RecipeSpec, envVars map[string]string, progress func(BakePhase)) (BakeOutcome, error) {
	var registryDigest *string
	if spec.Registry != nil {
		digest := spec.Registry.DigestInput()
		registryDigest = &digest
	}
	digest := deployment.RecipeInputs{
		Setup:           []byte(spec.SetupScript),
		SnapshotName:    spec.SnapshotName,
		SnapshotID:      spec.SnapshotID,
		DockerImage:     spec.DockerImage,
		Registry:        registryDigest,
		FsCapacityBytes: spec.FsCapacityBytes,
	}.Digest()
	recipeName := deployment.RecipeSnapshotName(deploymentName, digest)

	progress(BakePhaseCheckingRecipe)
	existing, err := client.FindReadySnapshotByName(ctx, recipeName)
	if err != nil {
		return BakeOutcome{}, err
	}
	if existing != nil {
		return BakeOutcome{
			RecipeName:      recipeName,
			RecipeReference: deployment.RecipeSnapshotReference(recipeName, existing.Tags),
			Skipped:         true,
		}, nil
	}

	registryID, err := resolveRegistryID(ctx, client, deploymentName, spec, envVars, progress)
	if err != nil {
		return BakeOutcome{}, err
	}

	// Docker image with an empty setup: the docker-built snapshot *is* the
	// recipe.
	if spec.DockerImage != nil && strings.TrimSpace(spec.SetupScript) == "" && spec.SnapshotName == nil && spec.SnapshotID == nil {
		progress(BakePhaseBuildingFromImage)
		snapshot, err := client.CreateSnapshotFromDockerImage(ctx, recipeName, *spec.DockerImage,
			spec.dockerFsCapacityBytes(), dockerSnapshotTimeout, registryID)
		if err != nil {
			return BakeOutcome{}, err
		}
		return BakeOutcome{
			RecipeName:      recipeName,
			RecipeReference: deployment.RecipeSnapshotReference(recipeName, snapshot.Tags),
		}, nil
	}

	baseSnapshotName, baseSnapshotID, intermediate, err := resolveBuilderBase(ctx, client, deploymentName, digest, spec, registryID, progress)
	if err != nil {
		return BakeOutcome{}, err
	}

	builderName := builderSandboxName(deploymentName, digest)
	// Drop a leftover builder from a previous interrupted bake.
	if err := client.DeleteSandboxBox(ctx, builderName); err != nil {
		slog.Debug("leftover bake builder delete failed", "builder", builderName, "error", err)
	}

	progress(BakePhaseStartingBuilder)
	box, err := client.CreateSandboxBox(ctx, builderName, hostapi.CreateSandboxBoxOptions{
		SnapshotName:   baseSnapshotName,
		SnapshotID:     baseSnapshotID,
		EnvVars:        envVars,
		TimeoutSecs:    builderTimeoutSecs,
		IdleTTLSeconds: builderIdleTTLSecs,
	})
	if err != nil {
		return BakeOutcome{}, fmt.Errorf("failed to create bake builder '%s': %w", builderName, err)
	}

	tags, bakeErr := func() ([]string, error) {
		if strings.TrimSpace(spec.SetupScript) != "" {
			if box.DataplaneURL == "" {
				return nil, fmt.Errorf("bake builder is missing dataplane_url")
			}
			progress(BakePhaseRunningSetup)
			if err := runSetupScript(ctx, client, box.DataplaneURL, spec.SetupScript); err != nil {
				return nil, err
			}
		}
		progress(BakePhaseCapturingSnapshot)
		captured, err := client.CaptureSnapshot(ctx, builderName, recipeName, hostapi.DefaultSnapshotReadyTimeout)
		if err != nil {
			return nil, err
		}
		return captured.Tags, nil
	}()

	// Always tear down the builder; the intermediate docker base too, whether
	// the bake succeeded or not.
	progress(BakePhaseCleaningUp)
	if err := client.DeleteSandboxBox(context.WithoutCancel(ctx), builderName); err != nil {
		slog.Debug("bake builder delete failed", "builder", builderName, "error", err)
	}
	if intermediate != "" {
		if err := deleteSnapshotByName(context.WithoutCancel(ctx), client, intermediate); err != nil {
			slog.Debug("intermediate bake snapshot delete failed", "snapshot", intermediate, "error", err)
		}
	}
	if bakeErr != nil {
		return BakeOutcome{}, bakeErr
	}

	return BakeOutcome{
		RecipeName:      recipeName,
		RecipeReference: deployment.RecipeSnapshotReference(recipeName, tags),
	}, nil
}

// resolveRegistryID stores the declared private registry's credential on
// Host and returns its id, "" when the spec declares no registry.
func resolveRegistryID(ctx context.Context, client *hostapi.Client, deploymentName string, spec *RecipeSpec, envVars map[string]string, progress func(BakePhase)) (string, error) {
	registry := spec.Registry
	if registry == nil {
		return "", nil
	}
	if spec.DockerImage == nil || strings.TrimSpace(*spec.DockerImage) == "" {
		return "", fmt.Errorf("registry requires dockerImage / docker_image on the sandbox declaration")
	}
	password := envVars[registry.PasswordEnv]
	if password == "" {
		password = os.Getenv(registry.PasswordEnv)
	}
	if password == "" {
		return "", fmt.Errorf("sandbox registry password is missing; set %s in the project .env or process environment", registry.PasswordEnv)
	}
	progress(BakePhaseStoringRegistry)
	name := deployment.SandboxRegistryName(deploymentName)
	reconciled, err := client.ReconcileRegistry(ctx, name, registry.URL, registry.Username, password)
	if err != nil {
		return "", err
	}
	return reconciled.ID, nil
}

// resolveBuilderBase picks the snapshot the builder boots from: by name, by
// id, or a snapshot built from the Docker image, which is also returned as
// the intermediate snapshot to delete afterwards ("" for none).
func resolveBuilderBase(ctx context.Context, client *hostapi.Client, deploymentName, digest string, spec *RecipeSpec, registryID string, progress func(BakePhase)) (snapshotName, snapshotID, intermediate string, err error) {
	if spec.DockerImage != nil {
		progress(BakePhaseBuildingFromImage)
		baseName := deployment.IntermediateSnapshotName(deploymentName, "dockerbase", digest)
		snapshot, err := client.CreateSnapshotFromDockerImage(ctx, baseName, *spec.DockerImage,
			spec.dockerFsCapacityBytes(), dockerSnapshotTimeout, registryID)
		if err != nil {
			return "", "", "", err
		}
		// Boot the builder by id only: the sandbox API rejects a create
		// request that names a snapshot both by id and by name.
		return "", snapshot.ID, baseName, nil
	}
	return deref(spec.SnapshotName), deref(spec.SnapshotID), "", nil
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func runSetupScript(ctx context.Context, client *hostapi.Client, dataplaneURL, script string) error {
	encoded := base64.StdEncoding.EncodeToString([]byte(script))
	// The base64 alphabet is single-quote safe.
	command := fmt.Sprintf("echo '%s' | base64 -d > /tmp/mda-setup.sh && bash -e /tmp/mda-setup.sh", encoded)
	result, err := client.ExecuteSandboxCommand(ctx, dataplaneURL, command, setupTimeoutSecs)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("sandbox setup.sh exited with code %d%s", result.ExitCode, formatSetupOutput(result))
	}
	return nil
}

func formatSetupOutput(result hostapi.SandboxCommandResult) string {
	if stderr := strings.TrimSpace(result.Stderr); stderr != "" {
		return "\n" + truncateForError(stderr)
	}
	if stdout := strings.TrimSpace(result.Stdout); stdout != "" {
		return "\n" + truncateForError(stdout)
	}
	return ""
}

func truncateForError(text string) string {
	const maxLength = 2_000
	if len(text) <= maxLength {
		return text
	}
	end := maxLength
	for end > 0 && !utf8.RuneStart(text[end]) {
		end--
	}
	return text[:end] + "…"
}

func builderSandboxName(deploymentName, digest string) string {
	suffix := deployment.SandboxNameSeparator + "bake-" + digest
	component := deployment.SandboxDeploymentComponent(deploymentName, deployment.HostSandboxNameLimit-len(suffix))
	return component + suffix
}

func deleteSnapshotByName(ctx context.Context, client *hostapi.Client, name string) error {
	matches, err := client.ListSnapshotsWithPrefix(ctx, name)
	if err != nil {
		return err
	}
	for _, snapshot := range matches {
		if snapshot.Name == name {
			if _, err := client.DeleteSnapshot(ctx, snapshot.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// StaleRecipeCleanup is the sweep of `{deployment}--setup-*` snapshots other
// than the live recipe.
type StaleRecipeCleanup struct {
	// InUse are previous recipes LangSmith refused to delete because boxes
	// still use them.
	InUse []string
}

// InUseNote is the note when previous recipes were left in place; false if
// every stale snapshot was deleted (or none existed).
func (c StaleRecipeCleanup) InUseNote() (string, bool) {
	if len(c.InUse) == 0 {
		return "", false
	}
	quoted := make([]string, len(c.InUse))
	for index, name := range c.InUse {
		quoted[index] = "'" + name + "'"
	}
	names := strings.Join(quoted, ", ")
	count, pronoun, subject := "1 previous recipe snapshot", "it", "It"
	if len(c.InUse) != 1 {
		count, pronoun, subject = fmt.Sprintf("%d previous recipe snapshots", len(c.InUse)), "them", "They"
	}
	return fmt.Sprintf("Left %s in place because live sandboxes still use %s (%s). %s will be removed on a later deploy once those sandboxes are gone.",
		count, pronoun, names, subject), true
}

// deleteStaleRecipeSnapshots deletes `{deployment}--setup-*` snapshots except
// the active recipe keepName ("" for none), plus temporary
// `{deployment}--dockerbase-*` / `--sourcebase-*` bases that an earlier bake
// could not remove while its builder was still tearing down.
//
// A snapshot still referenced by live boxes is skipped, not a failure. The
// next deploy retries the sweep after those boxes are reaped.
func deleteStaleRecipeSnapshots(ctx context.Context, client *hostapi.Client, deploymentName, keepName string) (StaleRecipeCleanup, error) {
	found, err := client.ListSnapshotsWithPrefix(ctx, deployment.BakeSnapshotListPrefix(deploymentName))
	if err != nil {
		return StaleRecipeCleanup{}, err
	}
	var (
		cleanup  StaleRecipeCleanup
		failures []string
	)
	for _, snapshot := range found {
		name := snapshot.Name
		if name == "" {
			continue
		}
		intermediate := deployment.IsDeploymentIntermediateSnapshot(name, deploymentName)
		if (keepName != "" && name == keepName) || !(intermediate || deployment.IsDeploymentRecipeSnapshot(name, deploymentName)) {
			continue
		}
		outcome, err := client.DeleteSnapshot(ctx, snapshot.ID)
		switch {
		case err != nil:
			failures = append(failures, fmt.Sprintf("'%s': %v", name, err))
		// A builder that is still shutting down holds its base; the next sweep
		// removes it. Not worth a note: users never reference it.
		case outcome == hostapi.SnapshotDeleteInUse && !intermediate:
			cleanup.InUse = append(cleanup.InUse, name)
		}
	}
	if len(failures) > 0 {
		return StaleRecipeCleanup{}, fmt.Errorf("failed to delete %d stale snapshot(s): %s", len(failures), strings.Join(failures, "; "))
	}
	return cleanup, nil
}
