package introspect

import (
	"testing"
)

const tsAgent = tsImport + "export const agent = defineDeepAgent({ name: \"agent\" });\n"
const pyAgent = pyImport + "agent = define_deep_agent(name='agent')\n"

func identityAuth(t *testing.T, summary *IdentitySummary) *IdentityAuthMode {
	t.Helper()
	if summary == nil {
		t.Fatal("expected an identity summary")
	}
	return summary.Auth
}

func TestTypescriptIdentityAuthModes(t *testing.T) {
	for source, want := range map[string]IdentityAuthMode{
		`defineIdentity({ auth: "backend" })`:                              IdentityAuthBackend,
		`defineIdentity({ auth: auth.langsmithApiKey() })`:                 IdentityAuthLangSmithAPIKey,
		`defineIdentity({ auth: auth.supabase({ projectRef: "abc" }) })`:   IdentityAuthValidatedToken,
		`defineIdentity({ auth: [auth.supabase({ projectRef: "abc" })] })`: IdentityAuthValidatedToken,
	} {
		p := typescriptProject(t, map[string]string{
			"agent.ts": tsAgent,
			"identity.ts": "import { auth, defineIdentity } from \"managed-deepagents\";\n" +
				"// Previously: auth: \"backend\"\n" +
				"export const identity = " + source + ";\n",
		})
		summary, err := ReadIdentitySummary(t.Context(), p)
		if err != nil {
			t.Fatal(err)
		}
		if got := identityAuth(t, summary); got == nil || *got != want {
			t.Fatalf("%s: got %v, want %v", source, got, want)
		}
		if summary.Path != "identity.ts" || summary.Scope == nil || summary.Scope.Credentials != "user" ||
			summary.AuthOmitted || summary.DeclaresMemory {
			t.Fatalf("%s: got %+v", source, summary)
		}
		requires, err := IdentityRequiresIngressSecret(t.Context(), p)
		if err != nil {
			t.Fatal(err)
		}
		if requires != (want == IdentityAuthBackend) {
			t.Fatalf("%s: requires ingress secret = %v", source, requires)
		}
	}
}

func TestPythonIdentityFollowsImportAliasesAndNamespaces(t *testing.T) {
	for _, source := range []string{
		"from managed_deepagents import define_identity as di\nidentity = di(auth=\"backend\")\n",
		"import managed_deepagents as mda\nidentity = mda.define_identity(auth=\"backend\")\n",
		"from managed_deepagents import define_identity\nidentity = helper = None\nidentity = define_identity(auth=\"backend\")\n",
	} {
		p := pythonProject(t, map[string]string{"agent.py": pyAgent, "identity.py": source})
		requires, err := IdentityRequiresIngressSecret(t.Context(), p)
		if err != nil {
			t.Fatal(err)
		}
		if !requires {
			t.Fatalf("%s: expected backend ingress", source)
		}
	}

	p := pythonProject(t, map[string]string{
		"agent.py":    pyAgent,
		"identity.py": "from managed_deepagents import auth, define_identity\nidentity = define_identity(auth=[auth.supabase(project_ref='abc')])\n",
	})
	summary, err := ReadIdentitySummary(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	if got := identityAuth(t, summary); got == nil || *got != IdentityAuthValidatedToken {
		t.Fatalf("got %v", got)
	}
}

func TestProjectWithoutIdentityDoesNotRequireSecret(t *testing.T) {
	p := pythonProject(t, map[string]string{"agent.py": pyAgent})
	summary, err := ReadIdentitySummary(t.Context(), p)
	if err != nil || summary != nil {
		t.Fatalf("got %+v, %v", summary, err)
	}
	requires, err := IdentityRequiresIngressSecret(t.Context(), p)
	if err != nil || requires {
		t.Fatalf("got %v, %v", requires, err)
	}
}

func TestIdentityModuleWithoutADeclarationHasNoAuth(t *testing.T) {
	p := typescriptProject(t, map[string]string{
		"agent.ts":    tsAgent,
		"identity.ts": "// no identity export\nexport const x = 1;\n",
	})
	summary, err := ReadIdentitySummary(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	if identityAuth(t, summary) != nil || summary.Scope != nil {
		t.Fatalf("got %+v", summary)
	}
}

func TestIdentityThatOmitsAuthIsReportedByTheSDK(t *testing.T) {
	p := typescriptProject(t, map[string]string{
		"agent.ts":    tsAgent,
		"identity.ts": "import { defineIdentity } from \"managed-deepagents\";\nexport const identity = defineIdentity({});\n",
	})
	_, err := IdentityRequiresIngressSecret(t.Context(), p)
	requireErrorPrefix(t, err, "failed to import identity.ts: Error: defineIdentity(...) requires `auth`.")
}

func TestIdentitySyntaxErrorIsReportedRatherThanReadAsNoSecret(t *testing.T) {
	p := pythonProject(t, map[string]string{
		"agent.py":    pyAgent,
		"identity.py": "identity = define_identity(auth=\"backend\"\n",
	})
	_, err := IdentityRequiresIngressSecret(t.Context(), p)
	requireErrorPrefix(t, err, "failed to parse identity.py:")
}

func TestIdentityReportsTheRetiredMemoryAxis(t *testing.T) {
	p := typescriptProject(t, map[string]string{
		"agent.ts": tsAgent,
		"identity.ts": "export const identity = { kind: \"identity\", config: " +
			"{ ingress: { http: \"backend\" }, scope: { credentials: \"user\", memory: \"agent\" } } };\n",
	})
	summary, err := ReadIdentitySummary(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	if !summary.DeclaresMemory {
		t.Fatalf("got %+v", summary)
	}
}
