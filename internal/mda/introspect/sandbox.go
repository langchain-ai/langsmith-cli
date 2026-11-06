package introspect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

// The source-reading half of `compile/sandbox_recipe.rs`: the bake bases a
// sandbox declaration selects, and whether a project uses sandboxes at all.
// Writing `sandbox-recipe.json` stays in `compile`.

// SandboxRegistryManifest is a secret-safe private registry declaration read
// from the authoring API.
type SandboxRegistryManifest struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	// PasswordEnv is the environment variable `langsmith deploy` reads.
	PasswordEnv string `json:"passwordEnv"`
}

// DigestInput is the non-secret registry identity included in the recipe
// digest.
func (m SandboxRegistryManifest) DigestInput() string {
	return m.URL + "\x00" + m.Username + "\x00" + m.PasswordEnv
}

// BakeBases are the author bake bases a sandbox declaration selects.
type BakeBases struct {
	SnapshotName *string
	SnapshotID   *string
	DockerImage  *string
	Registry     *SandboxRegistryManifest
	// FsCapacityBytes sizes the snapshot built from DockerImage.
	FsCapacityBytes *uint64
}

// IsEmpty reports whether no bake base is selected.
func (b BakeBases) IsEmpty() bool {
	return b.SnapshotName == nil && b.SnapshotID == nil && b.DockerImage == nil && b.Registry == nil
}

// ProjectUsesSandbox reports whether the project needs LangSmith sandboxes: a
// sandbox declaration, or a factory whose `setup.sh` bakes a recipe without one.
//
// Matches what compile's recipe bakes, so preflights that run before compile
// see the same projects the bake does.
func ProjectUsesSandbox(ctx context.Context, p *project.Project) (bool, error) {
	if _, ok := FindSandboxEntry(p); ok {
		return true, nil
	}
	setup, ok, err := ReadSetupScript(p)
	if err != nil {
		return false, err
	}
	if !ok || strings.TrimSpace(setup) == "" {
		return false, nil
	}
	agent, err := ReadAgentStaticConfig(ctx, p)
	if err != nil {
		return false, err
	}
	return agent.IsFactory, nil
}

// sandboxOptions is the exported sandbox's bake options, as the helper read
// them off the module at entry.
type sandboxOptions struct {
	entry    string
	language project.Language
	fields   sandboxFields
}

// readSandboxOptions reads the sandbox declaration at [FindSandboxEntry].
func readSandboxOptions(ctx context.Context, p *project.Project) (sandboxOptions, error) {
	doc, err := projectDocument(ctx, p)
	if err != nil {
		return sandboxOptions{}, err
	}
	if doc.Sandbox == nil {
		return sandboxOptions{}, fmt.Errorf("sandbox: %w", errNoHelperValue)
	}
	entry := doc.Sandbox.label()
	fields, err := doc.Sandbox.get()
	if err != nil {
		return sandboxOptions{}, err
	}
	options := sandboxOptions{entry: entry, language: p.Language, fields: fields}
	if !fields.Declared {
		return sandboxOptions{}, fmt.Errorf("%s must export a named `sandbox` declaration", entry)
	}
	if !fields.IsSandbox {
		return sandboxOptions{}, fmt.Errorf("%s must export `sandbox = %s(...)`",
			entry, p.Language.Ident("defineSandbox", "define_sandbox"))
	}
	return options, nil
}

// selected returns the selected keys as one JSON object, failing on the first
// of them that is not plain JSON data.
func (o sandboxOptions) selected(keys ...string) (map[string]any, error) {
	config := map[string]any{}
	for _, key := range keys {
		if problem, ok := o.fields.Errors[key]; ok {
			return nil, o.notStatic(problem)
		}
		raw, ok := o.fields.Options[key]
		if !ok {
			continue
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		var value any
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("failed to decode sandbox `%s` from %s: %w", key, o.entry, err)
		}
		config[key] = value
	}
	return config, nil
}

// notStatic is the error for an option that is not plain JSON data, worded as
// the Rust static extractors worded theirs.
func (o sandboxOptions) notStatic(problem string) error {
	if o.language == project.Python {
		return fmt.Errorf("%s sandbox is not static: %s", o.entry, problem)
	}
	return fmt.Errorf("sandbox %s must be statically serializable: %s", o.entry, problem)
}

// ExtractSandboxBases reads the bake bases the project's sandbox declaration
// ([FindSandboxEntry]) selects.
func ExtractSandboxBases(ctx context.Context, p *project.Project) (BakeBases, error) {
	options, err := readSandboxOptions(ctx, p)
	if err != nil {
		return BakeBases{}, err
	}
	keys := []string{"snapshotName", "snapshotId", "dockerImage", "registry"}
	if p.Language == project.Python {
		keys = []string{"snapshot_name", "snapshot_id", "docker_image", "registry"}
	}
	config, err := options.selected(keys...)
	if err != nil {
		return BakeBases{}, err
	}
	bases, err := basesFromConfig(config, p.Language)
	if err != nil {
		return BakeBases{}, err
	}
	if bases.DockerImage != nil {
		bases.FsCapacityBytes, err = extractDockerFsCapacityBytes(options)
		if err != nil {
			return BakeBases{}, err
		}
	}
	return bases, nil
}

// extractDockerFsCapacityBytes reads the declared size of the snapshot built
// from `docker_image`.
//
// Read only for a `docker_image` base so other sandboxes may still compute it.
func extractDockerFsCapacityBytes(options sandboxOptions) (*uint64, error) {
	key := options.language.Ident("fsCapacityBytes", "fs_capacity_bytes")
	config, err := options.selected(key)
	if err != nil {
		return nil, fmt.Errorf(
			"sandbox %s sizes the %s snapshot, so it must be an integer literal or a top-level constant: %w",
			key, options.language.Ident("dockerImage", "docker_image"), err)
	}
	return fsCapacityBytes(config, options.language)
}

func fsCapacityBytes(config map[string]any, language project.Language) (*uint64, error) {
	key := language.Ident("fsCapacityBytes", "fs_capacity_bytes")
	value, ok := config[key]
	if !ok || value == nil {
		return nil, nil
	}
	invalid := fmt.Errorf("sandbox %s must be a positive integer literal", key)
	number, ok := value.(json.Number)
	if !ok {
		return nil, invalid
	}
	if whole, err := number.Int64(); err == nil {
		if whole <= 0 {
			return nil, invalid
		}
		bytes := uint64(whole)
		return &bytes, nil
	}
	// TypeScript numbers may arrive as floats (`2147483648.0`).
	float, err := number.Float64()
	if err != nil || float <= 0 || math.Trunc(float) != float || float > math.MaxUint64 {
		return nil, invalid
	}
	bytes := uint64(float)
	return &bytes, nil
}

// RejectRemovedSandboxScope fails when the sandbox declaration still passes
// the removed `scope` option, which `defineSandbox` / `define_sandbox` reject
// at startup.
func RejectRemovedSandboxScope(ctx context.Context, p *project.Project) error {
	if _, ok := FindSandboxEntry(p); !ok {
		return nil
	}
	// A declaration that cannot be read is reported by recipe extraction
	// ([ExtractSandboxBases]) later in the compile.
	options, err := readSandboxOptions(ctx, p)
	_, hasScope := options.fields.Options["scope"]
	_, scopeUnreadable := options.fields.Errors["scope"]
	if err == nil && (hasScope || scopeUnreadable) {
		return fmt.Errorf(
			"%s passes the removed sandbox `scope` option. Sandbox reuse is always one box per "+
				"thread: remove `scope` from the sandbox declaration.", options.entry)
	}
	return nil
}

func basesFromConfig(config map[string]any, language project.Language) (BakeBases, error) {
	text := func(object map[string]any, key string) *string {
		value, ok := object[key].(string)
		if !ok {
			return nil
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return nil
		}
		return &value
	}
	// Accept either spelling when present (defensive for mixed extracts).
	pick := func(camel, snake string) *string {
		preferred, other := camel, snake
		if language == project.Python {
			preferred, other = snake, camel
		}
		if value := text(config, preferred); value != nil {
			return value
		}
		return text(config, other)
	}
	var bases BakeBases
	if raw, ok := config["registry"]; ok {
		object, ok := raw.(map[string]any)
		if !ok {
			return BakeBases{}, fmt.Errorf("sandbox registry must be an object")
		}
		field := func(camel, snake string) *string {
			preferred, other := camel, snake
			if language == project.Python {
				preferred, other = snake, camel
			}
			if _, ok := object[preferred]; ok {
				return text(object, preferred)
			}
			return text(object, other)
		}
		required := func(value *string, name string) (string, error) {
			if value == nil {
				return "", fmt.Errorf("sandbox registry.%s must be a non-empty string", name)
			}
			return *value, nil
		}
		url, err := required(field("url", "url"), "url")
		if err != nil {
			return BakeBases{}, err
		}
		username, err := required(field("username", "username"), "username")
		if err != nil {
			return BakeBases{}, err
		}
		passwordEnv, err := required(field("passwordEnv", "password_env"),
			language.Ident("passwordEnv", "password_env"))
		if err != nil {
			return BakeBases{}, err
		}
		bases.Registry = &SandboxRegistryManifest{URL: url, Username: username, PasswordEnv: passwordEnv}
	}
	bases.SnapshotName = pick("snapshotName", "snapshot_name")
	bases.SnapshotID = pick("snapshotId", "snapshot_id")
	bases.DockerImage = pick("dockerImage", "docker_image")
	if bases.Registry != nil && bases.DockerImage == nil {
		return BakeBases{}, fmt.Errorf("sandbox registry requires %s", language.Ident("dockerImage", "docker_image"))
	}
	return bases, nil
}
