package cmd

import "testing"

func TestDeniedProxyPathPattern(t *testing.T) {
	denied := []string{
		"/api/v1/api-key",
		"/api/v1/api-keys/abc",
		"/api/v1/orgs/current/members",
		"/api/v1/orgs/current/roles",
		"/api/v1/workspaces/current/users/info",
		"/api/v1/workspaces/current/identities",
		"/scim/v2/Users",
	}
	for _, path := range denied {
		if !deniedProxyPathPattern.MatchString(path) {
			t.Errorf("expected %q to be denied", path)
		}
	}

	allowed := []string{
		"/api/v1/sessions",
		"/api/v1/datasets",
		"/api/v1/feedback",
		"/v2/runs/query",
		"/api/v1/annotation-queues/abc/runs",
	}
	for _, path := range allowed {
		if deniedProxyPathPattern.MatchString(path) {
			t.Errorf("expected %q to be allowed", path)
		}
	}
}
