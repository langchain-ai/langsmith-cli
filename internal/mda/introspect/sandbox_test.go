package introspect

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

const tsSandboxImport = "import { defineSandbox } from \"managed-deepagents\";\n"
const pySandboxImport = "from managed_deepagents import define_sandbox\n"

func str(value string) *string { return &value }

func TestBasesFromConfig(t *testing.T) {
	bases, err := basesFromConfig(map[string]any{"idleTtlSeconds": json.Number("600"), "snapshotName": "base-snap"}, project.TypeScript)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(bases, BakeBases{SnapshotName: str("base-snap")}) {
		t.Fatalf("got %+v", bases)
	}

	bases, err = basesFromConfig(map[string]any{
		"docker_image": "python:3.12-slim",
		"registry":     map[string]any{"url": "ghcr.io", "username": "octocat", "password_env": "GHCR_TOKEN"},
	}, project.Python)
	if err != nil {
		t.Fatal(err)
	}
	want := BakeBases{
		DockerImage: str("python:3.12-slim"),
		Registry:    &SandboxRegistryManifest{URL: "ghcr.io", Username: "octocat", PasswordEnv: "GHCR_TOKEN"},
	}
	if !reflect.DeepEqual(bases, want) {
		t.Fatalf("got %+v", bases)
	}

	bases, err = basesFromConfig(map[string]any{
		"dockerImage": "ghcr.io/acme/base:1",
		"registry":    map[string]any{"url": "ghcr.io", "username": "octocat", "passwordEnv": "GHCR_TOKEN"},
	}, project.TypeScript)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(bases.Registry, want.Registry) {
		t.Fatalf("got %+v", bases.Registry)
	}

	_, err = basesFromConfig(map[string]any{"registry": map[string]any{"url": "ghcr.io"}}, project.Python)
	requireErrorContains(t, err, "sandbox registry.username must be a non-empty string")
	_, err = basesFromConfig(map[string]any{
		"registry": map[string]any{"url": "ghcr.io", "username": "u", "passwordEnv": "P"},
	}, project.TypeScript)
	requireErrorContains(t, err, "sandbox registry requires dockerImage")
}

func TestFsCapacityBytesFromNumbers(t *testing.T) {
	for language, config := range map[project.Language]map[string]any{
		project.Python:     {"fs_capacity_bytes": json.Number("2147483648")},
		project.TypeScript: {"fsCapacityBytes": json.Number("2147483648.0")},
	} {
		got, err := fsCapacityBytes(config, language)
		if err != nil || got == nil || *got != 2<<30 {
			t.Fatalf("%v: got %v, %v", language, got, err)
		}
	}
	for _, bad := range []any{json.Number("0"), json.Number("1.5"), "2GiB"} {
		_, err := fsCapacityBytes(map[string]any{"fs_capacity_bytes": bad}, project.Python)
		requireErrorContains(t, err, "sandbox fs_capacity_bytes must be a positive integer literal")
	}
}

func TestDockerSandboxRecordsFsCapacityBytes(t *testing.T) {
	ts := typescriptProject(t, map[string]string{
		"agent.ts":         tsAgent,
		"sandbox/index.ts": tsSandboxImport + "export const sandbox = defineSandbox({ dockerImage: \"python:3.12\", fsCapacityBytes: 2_147_483_648 });\n",
	})
	py := pythonProject(t, map[string]string{
		"agent.py":            pyAgent,
		"sandbox/__init__.py": pySandboxImport + "sandbox = define_sandbox(docker_image='python:3.12', fs_capacity_bytes=2 * 1024**3)\n",
	})
	for _, p := range []*project.Project{ts, py} {
		bases, err := ExtractSandboxBases(t.Context(), p)
		if err != nil {
			t.Fatal(err)
		}
		if bases.FsCapacityBytes == nil || *bases.FsCapacityBytes != 2<<30 || *bases.DockerImage != "python:3.12" {
			t.Fatalf("got %+v", bases)
		}
	}
}

func TestRuntimeOnlySandboxOptionsNeedNotBeData(t *testing.T) {
	p := typescriptProject(t, map[string]string{
		"agent.ts": tsAgent,
		"sandbox/index.ts": tsSandboxImport + "export const sandbox = defineSandbox({\n" +
			"  defaultTimeout: Number(process.env.UNSET_TIMEOUT), proxyConfig: { resolve: () => 1 }, snapshotName: \"base\",\n" +
			"});\n",
	})
	bases, err := ExtractSandboxBases(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(bases, BakeBases{SnapshotName: str("base")}) {
		t.Fatalf("got %+v", bases)
	}
}

func TestBakeOptionsMustBePlainData(t *testing.T) {
	ts := typescriptProject(t, map[string]string{
		"agent.ts":         tsAgent,
		"sandbox/index.ts": tsSandboxImport + "export const sandbox = defineSandbox({ dockerImage: () => \"python\" });\n",
	})
	_, err := ExtractSandboxBases(t.Context(), ts)
	requireErrorPrefix(t, err, "sandbox sandbox/index.ts must be statically serializable: "+
		"property `dockerImage` is not static: a function is not plain JSON data")

	py := pythonProject(t, map[string]string{
		"agent.py":            pyAgent,
		"sandbox/__init__.py": pySandboxImport + "sandbox = define_sandbox(registry={'url': object()}, docker_image='x')\n",
	})
	_, err = ExtractSandboxBases(t.Context(), py)
	requireErrorPrefix(t, err, "sandbox/__init__.py sandbox is not static: property `registry` is not static: "+
		"property `url` is not static: an instance of object is not plain JSON data")

	capacity := typescriptProject(t, map[string]string{
		"agent.ts":         tsAgent,
		"sandbox/index.ts": tsSandboxImport + "export const sandbox = defineSandbox({ dockerImage: \"python\", fsCapacityBytes: 2n ** 31n });\n",
	})
	_, err = ExtractSandboxBases(t.Context(), capacity)
	requireErrorPrefix(t, err, "sandbox fsCapacityBytes sizes the dockerImage snapshot, so it must be an "+
		"integer literal or a top-level constant: sandbox sandbox/index.ts must be statically serializable: "+
		"property `fsCapacityBytes` is not static: a bigint is not plain JSON data")
}

func TestSandboxModulesMustExportASandbox(t *testing.T) {
	missing := typescriptProject(t, map[string]string{"agent.ts": tsAgent, "sandbox/index.ts": "export const box = 1;\n"})
	_, err := ExtractSandboxBases(t.Context(), missing)
	requireErrorPrefix(t, err, "sandbox/index.ts must export a named `sandbox` declaration")

	other := pythonProject(t, map[string]string{"agent.py": pyAgent, "sandbox/__init__.py": "sandbox = {'docker_image': 'x'}\n"})
	_, err = ExtractSandboxBases(t.Context(), other)
	requireErrorPrefix(t, err, "sandbox/__init__.py must export `sandbox = define_sandbox(...)`")
}

func TestRejectRemovedSandboxScope(t *testing.T) {
	p := typescriptProject(t, map[string]string{
		"agent.ts":         tsAgent,
		"sandbox/index.ts": "export const sandbox = { kind: \"sandbox\", options: { scope: \"thread\", idleTtlSeconds: 600 } };\n",
	})
	err := RejectRemovedSandboxScope(t.Context(), p)
	requireErrorContains(t, err, "sandbox/index.ts")
	requireErrorContains(t, err, "removed sandbox `scope` option")

	accepted := typescriptProject(t, map[string]string{
		"agent.ts":         tsAgent,
		"sandbox/index.ts": tsSandboxImport + "export const sandbox = defineSandbox({ idleTtlSeconds: 600 });\n",
	})
	if err := RejectRemovedSandboxScope(t.Context(), accepted); err != nil {
		t.Fatal(err)
	}

	// The SDK refuses `scope` itself; the import error surfaces from the bases.
	sdk := typescriptProject(t, map[string]string{
		"agent.ts":         tsAgent,
		"sandbox/index.ts": tsSandboxImport + "export const sandbox = defineSandbox({ scope: \"thread\" });\n",
	})
	if err := RejectRemovedSandboxScope(t.Context(), sdk); err != nil {
		t.Fatal(err)
	}
	_, err = ExtractSandboxBases(t.Context(), sdk)
	requireErrorPrefix(t, err, "failed to import sandbox/index.ts: TypeError: sandbox scope was removed")
}

func TestProjectUsesSandbox(t *testing.T) {
	setup := map[string]string{"sandbox/setup.sh": "echo ready\n"}
	factory := typescriptProject(t, merge(setup, map[string]string{
		"agent.ts": tsImport + "export const agent = defineAgent(\"a\", () => defineAgent({}));\n",
	}))
	fixed := typescriptProject(t, merge(setup, map[string]string{"agent.ts": tsAgent}))
	declared := pythonProject(t, map[string]string{"agent.py": pyAgent, "sandbox/__init__.py": ""})
	for p, want := range map[*project.Project]bool{factory: true, fixed: false, declared: true} {
		got, err := ProjectUsesSandbox(t.Context(), p)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s: got %v, want %v", p.Root, got, want)
		}
	}
}

func merge(maps ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}
