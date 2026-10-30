package compile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/mda/deployment"
	"github.com/langchain-ai/langsmith-cli/internal/mda/introspect"
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

// Emit `.mda/build/sandbox-recipe.json` for the deploy bake.
//
// Digests cover `setup.sh` bytes plus optional author bake bases read from
// `defineSandbox` / `define_sandbox` ([introspect.ExtractSandboxBases]).

// SandboxRecipeFile is the build artifact next to `langgraph.json`.
const SandboxRecipeFile = "sandbox-recipe.json"

// SandboxRegistryManifest is a secret-safe private registry declaration
// compiled from the authoring API.
type SandboxRegistryManifest = introspect.SandboxRegistryManifest

// SandboxRecipeManifest is the author bake bases and setup metadata written at
// compile time.
type SandboxRecipeManifest struct {
	HasSetupScript bool `json:"hasSetupScript"`
	// SetupShaPreview is the first 16 hex chars of the setup SHA (preview
	// only; the full digest is Digest).
	SetupShaPreview *string `json:"setupShaPreview"`
	SnapshotName    *string `json:"snapshotName"`
	SnapshotID      *string `json:"snapshotId"`
	DockerImage     *string `json:"dockerImage"`
	// Registry is the private registry declaration. It contains only an
	// env-var name, not a secret.
	Registry *SandboxRegistryManifest `json:"registry"`
	// FsCapacityBytes is the filesystem size for a snapshot built from
	// DockerImage.
	FsCapacityBytes *uint64 `json:"fsCapacityBytes,omitempty"`
	// Digest is [deployment.RecipeDigest] over setup bytes + bake bases.
	Digest string `json:"digest"`
}

// json renders the manifest the way `serde_json::to_string_pretty` renders
// the Rust struct.
func (m *SandboxRecipeManifest) json() string {
	optional := func(value *string) any {
		if value == nil {
			return nil
		}
		return *value
	}
	object := jsonObjectOf(
		"hasSetupScript", m.HasSetupScript,
		"setupShaPreview", optional(m.SetupShaPreview),
		"snapshotName", optional(m.SnapshotName),
		"snapshotId", optional(m.SnapshotID),
		"dockerImage", optional(m.DockerImage),
	)
	if m.Registry == nil {
		object.insert("registry", nil)
	} else {
		object.insert("registry", jsonObjectOf(
			"url", m.Registry.URL,
			"username", m.Registry.Username,
			"passwordEnv", m.Registry.PasswordEnv,
		))
	}
	if m.FsCapacityBytes != nil {
		object.insert("fsCapacityBytes", jsonNumber(strconv.FormatUint(*m.FsCapacityBytes, 10)))
	}
	object.insert("digest", m.Digest)
	return prettyJSON(object)
}

// writeSandboxRecipeManifest probes the sandbox declaration and writes
// `sandbox-recipe.json` when there is something to bake.
//
// A factory may select its sandbox per run without a static declaration, so
// its `setup.sh` alone is a recipe, baked on Host's default image.
//
// Returns nil (and removes a stale file) when there is nothing to bake.
func writeSandboxRecipeManifest(ctx context.Context, p *project.Project, outDir string, factory bool) (*SandboxRecipeManifest, error) {
	path := filepath.Join(outDir, SandboxRecipeFile)
	setup, _, err := introspect.ReadSetupScript(p)
	if err != nil {
		return nil, err
	}
	hasSetupScript := strings.TrimSpace(setup) != ""
	var bases introspect.BakeBases
	if _, ok := introspect.FindSandboxEntry(p); ok {
		bases, err = introspect.ExtractSandboxBases(ctx, p)
		if err != nil {
			return nil, err
		}
	} else if !factory || !hasSetupScript {
		removeStaleFile(path)
		return nil, nil
	}
	if !hasSetupScript && bases.IsEmpty() {
		removeStaleFile(path)
		return nil, nil
	}
	var registryDigest *string
	if bases.Registry != nil {
		input := bases.Registry.DigestInput()
		registryDigest = &input
	}
	digest := deployment.RecipeInputs{
		Setup:           []byte(setup),
		SnapshotName:    bases.SnapshotName,
		SnapshotID:      bases.SnapshotID,
		DockerImage:     bases.DockerImage,
		Registry:        registryDigest,
		FsCapacityBytes: bases.FsCapacityBytes,
	}.Digest()
	var setupShaPreview *string
	if hasSetupScript {
		preview := deployment.RecipeInputs{Setup: []byte(setup)}.Digest()
		setupShaPreview = &preview
	}

	manifest := &SandboxRecipeManifest{
		HasSetupScript:  hasSetupScript,
		SetupShaPreview: setupShaPreview,
		SnapshotName:    bases.SnapshotName,
		SnapshotID:      bases.SnapshotID,
		DockerImage:     bases.DockerImage,
		Registry:        bases.Registry,
		FsCapacityBytes: bases.FsCapacityBytes,
		Digest:          digest,
	}
	if _, err := writeIfChanged(path, []byte(manifest.json()+"\n")); err != nil {
		return nil, fmt.Errorf("failed to write %s: %w", path, err)
	}
	return manifest, nil
}

// removeStaleFile removes a stale artifact, as the Rust CLI did, which ignored
// a failure: the next build overwrites it.
func removeStaleFile(path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		slog.Debug("failed to remove a stale build artifact", "path", path, "error", err)
	}
}

// LoadSandboxRecipeManifest loads a previously written recipe manifest from
// the build directory; nil when there is none.
func LoadSandboxRecipeManifest(outDir string) (*SandboxRecipeManifest, error) {
	path := filepath.Join(outDir, SandboxRecipeFile)
	if !isFile(path) {
		return nil, nil
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	var manifest SandboxRecipeManifest
	if err := json.Unmarshal(text, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return &manifest, nil
}
