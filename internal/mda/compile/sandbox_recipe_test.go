package compile

import (
	"reflect"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/mda/deployment"
)

const tsSandbox = "import { defineSandbox } from \"managed-deepagents\";\n"

func TestARecipeRecordsTheBakeBasesAndSetupDigest(t *testing.T) {
	f := typescriptProject(t, map[string]string{
		"agent.ts": tsAgent,
		"sandbox/index.ts": tsSandbox + "export const sandbox = defineSandbox({\n" +
			"  dockerImage: \"ghcr.io/acme/base:1\",\n  fsCapacityBytes: 2147483648,\n" +
			"  registry: { url: \"ghcr.io\", username: \"octocat\", passwordEnv: \"GHCR_TOKEN\" },\n});\n",
		"sandbox/setup.sh": "echo ready\n",
	})

	manifest, err := writeSandboxRecipeManifest(t.Context(), f.project, f.outDir, false)
	if err != nil {
		t.Fatal(err)
	}

	image, registry := "ghcr.io/acme/base:1", "ghcr.io\x00octocat\x00GHCR_TOKEN"
	capacity := uint64(2 << 30)
	digest := deployment.RecipeDigest([]byte("echo ready\n"), nil, nil, &image, &registry, &capacity)
	preview := deployment.RecipeDigest([]byte("echo ready\n"), nil, nil, nil, nil, nil)
	want := "{\n  \"hasSetupScript\": true,\n  \"setupShaPreview\": \"" + preview + "\",\n" +
		"  \"snapshotName\": null,\n  \"snapshotId\": null,\n  \"dockerImage\": \"ghcr.io/acme/base:1\",\n" +
		"  \"registry\": {\n    \"url\": \"ghcr.io\",\n    \"username\": \"octocat\",\n    \"passwordEnv\": \"GHCR_TOKEN\"\n  },\n" +
		"  \"fsCapacityBytes\": 2147483648,\n  \"digest\": \"" + digest + "\"\n}\n"
	if got := f.read(t, SandboxRecipeFile); got != want {
		t.Fatalf("%s:\n%s", SandboxRecipeFile, got)
	}
	loaded, err := LoadSandboxRecipeManifest(f.outDir)
	if err != nil || !reflect.DeepEqual(loaded, manifest) {
		t.Fatalf("loaded %+v, %v; wrote %+v", loaded, err, manifest)
	}
}

func TestAFactorySetupScriptWithoutADeclarationBakesOnTheDefaultImage(t *testing.T) {
	f := pythonProject(t, map[string]string{"agent.py": pyAgent, "sandbox/setup.sh": "echo ready\n"})

	manifest, err := writeSandboxRecipeManifest(t.Context(), f.project, f.outDir, true)
	if err != nil {
		t.Fatal(err)
	}

	digest := deployment.RecipeDigest([]byte("echo ready\n"), nil, nil, nil, nil, nil)
	want := &SandboxRecipeManifest{HasSetupScript: true, SetupShaPreview: &digest, Digest: digest}
	if !reflect.DeepEqual(manifest, want) {
		t.Fatalf("got %+v", manifest)
	}
	if got := f.read(t, SandboxRecipeFile); got != "{\n  \"hasSetupScript\": true,\n  \"setupShaPreview\": \""+digest+"\",\n"+
		"  \"snapshotName\": null,\n  \"snapshotId\": null,\n  \"dockerImage\": null,\n  \"registry\": null,\n"+
		"  \"digest\": \""+digest+"\"\n}\n" {
		t.Fatalf("%s:\n%s", SandboxRecipeFile, got)
	}
}

func TestNothingToBakeWritesNoRecipeAndRemovesAStaleOne(t *testing.T) {
	for _, tc := range []struct {
		name    string
		project func(*testing.T) fixture
		factory bool
	}{
		// A static agent's `setup.sh` without a declaration is not baked.
		{"static setup script", func(t *testing.T) fixture {
			return pythonProject(t, map[string]string{"agent.py": pyAgent, "sandbox/setup.sh": "echo ready\n"})
		}, false},
		// A factory without a setup script has nothing to bake.
		{"factory without setup", func(t *testing.T) fixture {
			return pythonProject(t, map[string]string{"agent.py": pyAgent, "sandbox/setup.sh": "  \n"})
		}, true},
		// A declaration without bases or setup script either.
		{"plain declaration", func(t *testing.T) fixture {
			return typescriptProject(t, map[string]string{
				"agent.ts":         tsAgent,
				"sandbox/index.ts": tsSandbox + "export const sandbox = defineSandbox({ idleTtlSeconds: 600 });\n",
			})
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.project(t)
			writeFile(t, f.outDir, SandboxRecipeFile, "stale")

			manifest, err := writeSandboxRecipeManifest(t.Context(), f.project, f.outDir, tc.factory)

			if err != nil || manifest != nil {
				t.Fatalf("got %+v, %v", manifest, err)
			}
			if f.exists(SandboxRecipeFile) {
				t.Fatal("the stale recipe survived")
			}
			if loaded, err := LoadSandboxRecipeManifest(f.outDir); err != nil || loaded != nil {
				t.Fatalf("loaded %+v, %v", loaded, err)
			}
		})
	}
}

func TestCompileRejectsTheRemovedSandboxScope(t *testing.T) {
	f := pythonProject(t, map[string]string{
		"agent.py":            pyAgent,
		"sandbox/__init__.py": "from managed_deepagents import define_sandbox\nsandbox = define_sandbox()\nsandbox.options['scope'] = 'thread'\n",
	})

	_, err := f.deploy(t)

	requireErrorContains(t, err, "passes the removed sandbox `scope` option")
}
