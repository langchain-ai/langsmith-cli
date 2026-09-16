package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestExampleUpdate(t *testing.T) {
	calls := 0
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		calls++
		if r.Method != "PATCH" || r.URL.Path != "/api/v1/examples/"+"11111111-1111-4111-8111-111111111111" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		if len(body) != 1 || body["outputs"] == nil {
			t.Errorf("must only patch outputs: %#v", body)
		}
		fmt.Fprint(w, `{}`)
	})
	defer setupTestEnv(t, ts.URL)()
	c := newExampleUpdateCmd()
	c.SetArgs([]string{"11111111-1111-4111-8111-111111111111", "--outputs", `{"answer":"corrected"}`})
	captureStdout(t, func() {
		if err := c.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	if calls != 1 {
		t.Fatal(calls)
	}
	for _, value := range []string{"null", "[]", "invalid"} {
		c := newExampleUpdateCmd()
		c.SetArgs([]string{"11111111-1111-4111-8111-111111111111", "--outputs", value})
		if c.Execute() == nil {
			t.Fatal("accepted invalid object", value)
		}
	}
	if calls != 1 {
		t.Fatal("invalid input reached network")
	}
}
