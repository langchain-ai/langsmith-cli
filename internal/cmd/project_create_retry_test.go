package cmd

import (
	"fmt"
	"net/http"
	"testing"
)

func TestProjectCreateDoesNotRetry(t *testing.T) {
	calls := 0
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		fmt.Fprint(w, `{}`)
	})
	defer setupTestEnv(t, ts.URL)()
	cmd := newProjectCreateCmd()
	cmd.SetArgs([]string{"--name", "retry-test"})
	var err error
	out := captureStdout(t, func() { err = cmd.Execute() })
	if err == nil || calls != 1 || out != "" {
		t.Fatalf("err=%v calls=%d out=%s", err, calls, out)
	}
}
