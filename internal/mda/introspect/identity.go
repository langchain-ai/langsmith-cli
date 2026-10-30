package introspect

import (
	"context"
	"fmt"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

// The project's identity declaration (`compile/identity.rs`): whether it needs
// `MDA_INGRESS_SECRET` at runtime (`backend` HTTP ingress), and a summary of its
// isolation scope for CLI reporting.
//
// The helper imports the identity module and reads the exported `identity`
// value, so comments, unrelated strings and import aliases cannot mislead it.

// IdentityScopeAxes are the resolved isolation axes of a `defineIdentity` /
// `define_identity` declaration.
//
// Threads are always user-owned and durable memory has its own declaration, so
// neither is part of the summary.
type IdentityScopeAxes struct {
	Credentials string `json:"credentials"`
}

// IdentityAuthMode is how callers authenticate over HTTP.
type IdentityAuthMode int

// The HTTP ingress auth modes.
const (
	// IdentityAuthBackend is reserved ingress headers from a backend the author
	// operates.
	IdentityAuthBackend IdentityAuthMode = iota
	// IdentityAuthLangSmithAPIKey is a caller presenting a LangSmith workspace
	// API key.
	IdentityAuthLangSmithAPIKey
	// IdentityAuthValidatedToken is a caller presenting a token MDA verifies
	// (IdP providers).
	IdentityAuthValidatedToken
)

// Label is the mode's snake_case name.
func (m IdentityAuthMode) Label() string {
	switch m {
	case IdentityAuthBackend:
		return "backend"
	case IdentityAuthLangSmithAPIKey:
		return "langsmith_api_key"
	default:
		return "validated_token"
	}
}

// IdentitySummary is the root identity declaration of a project.
type IdentitySummary struct {
	// Path is relative to the project root (`identity.ts`, `identity.py`, …).
	Path string
	// Auth is the HTTP ingress auth mode, nil when the module exports no
	// identity declaration.
	Auth *IdentityAuthMode
	// AuthOmitted reports a declaration that omits the required `auth` option.
	// Both SDKs refuse one when the module is imported, so a summary read by
	// importing it never has it set.
	AuthOmitted bool
	// Scope is the declaration's credentials scope, nil when unknown.
	Scope *IdentityScopeAxes
	// DeclaresMemory reports the retired `scope.memory` axis. Memory moved to
	// `memory.{ts,py}`, so its presence is a migration error.
	DeclaresMemory bool
}

// IngressSecretEnv is the env var the managed auth handler reads for `backend`
// ingress.
const IngressSecretEnv = "MDA_INGRESS_SECRET"

// ReadIdentitySummary reads the project's identity declaration.
//
// Returns nil when there is no identity module. A module that fails to import
// is an error naming it.
func ReadIdentitySummary(ctx context.Context, p *project.Project) (*IdentitySummary, error) {
	rel, ok := FindIdentityEntry(p)
	if !ok {
		return nil, nil
	}
	doc, err := projectDocument(ctx, p)
	if err != nil {
		return nil, err
	}
	if doc.Identity == nil {
		return nil, fmt.Errorf("%s: %w", rel, errNoHelperValue)
	}
	fields, err := doc.Identity.get()
	if err != nil {
		return nil, err
	}
	summary := &IdentitySummary{
		Path:           rel,
		AuthOmitted:    fields.AuthOmitted,
		Scope:          fields.Scope,
		DeclaresMemory: fields.DeclaresMemory,
	}
	if fields.Auth != nil {
		var mode IdentityAuthMode
		switch *fields.Auth {
		case "backend":
			mode = IdentityAuthBackend
		case "langsmith_api_key":
			mode = IdentityAuthLangSmithAPIKey
		case "validated_token":
			mode = IdentityAuthValidatedToken
		default:
			return summary, nil
		}
		summary.Auth = &mode
	}
	return summary, nil
}

// IdentityRequiresIngressSecret reports whether the project's identity
// declaration needs [IngressSecretEnv].
//
// False when the project has no identity module or its `identity` export does
// not use `backend` ingress. An identity module that does not import is an
// error: silently reading it as "no secret required" would deploy an ingress
// the author cannot authenticate against.
func IdentityRequiresIngressSecret(ctx context.Context, p *project.Project) (bool, error) {
	summary, err := ReadIdentitySummary(ctx, p)
	if err != nil {
		return false, err
	}
	return summary != nil && summary.Auth != nil && *summary.Auth == IdentityAuthBackend, nil
}
