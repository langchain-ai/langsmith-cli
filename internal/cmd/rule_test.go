package cmd

import (
	"fmt"
	"net/http"
	"testing"
)

func TestRuleListScope(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/v1/runs/rules" || r.URL.Query().Get("session_id") != deleteTestProjectID {
			t.Errorf("wrong scope: %s", r.URL.String())
		}
		fmt.Fprint(w, `[]`)
	})
	defer setupTestEnv(t, ts.URL)()
	c := newRuleCmd()
	c.SetArgs([]string{"list", "--project-id", deleteTestProjectID})
	captureStdout(t, func() {
		if err := c.Execute(); err != nil {
			t.Fatal(err)
		}
	})
}
