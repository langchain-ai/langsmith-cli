package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestInsightsProviderSelection(t *testing.T) {
	oldTerminal, oldFormat := inputIsTerminal, flagOutputFormat
	t.Cleanup(func() { inputIsTerminal = oldTerminal; flagOutputFormat = oldFormat })
	for _, tc := range []struct {
		name, input, explicit, format, want string
		terminal, wantError, wantPrompt     bool
	}{
		{"openai", "1\n", "", "pretty", "openai", true, false, true},
		{"anthropic", "Anthropic\n", "", "pretty", "anthropic", true, false, true},
		{"explicit", "", "openai", "json", "openai", false, false, false},
		{"nonterminal", "1\n", "", "pretty", "", false, true, false},
		{"json", "1\n", "", "json", "", true, true, false},
		{"EOF", "", "", "pretty", "", true, true, true},
		{"blank", "\n", "", "pretty", "", true, true, true},
		{"invalid", "3\n", "", "pretty", "", true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inputIsTerminal = func(io.Reader) bool { return tc.terminal }
			flagOutputFormat = tc.format
			cmd := newInsightsCreateCmd()
			cmd.SetIn(strings.NewReader(tc.input))
			var prompt bytes.Buffer
			cmd.SetErr(&prompt)
			got, err := chooseInsightsProvider(cmd, tc.explicit)
			if got != tc.want || (err != nil) != tc.wantError {
				t.Fatalf("got=%q err=%v", got, err)
			}
			if (prompt.Len() > 0) != tc.wantPrompt {
				t.Fatalf("unexpected prompt: %s", prompt.String())
			}
		})
	}
}

func TestInsightsCreatePrettyGuidance(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"report-id","name":"Test report","status":"queued"}`))
	})
	defer setupTestEnv(t, ts.URL)()
	setupProjectDefaultConfig(t, ts.URL)
	cmd := newInsightsCreateCmd()
	cmd.SetArgs([]string{"--project-id", deleteTestProjectID, "--last-n-hours", "24", "--sample", "6", "--model", "openai"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Report queued. Results are not ready yet.", "Report ID: report-id", "insights get report-id", "insights runs report-id", "--project-id " + deleteTestProjectID} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %s", want, out.String())
		}
	}
}

func TestInsightsCreateParamsValidation(t *testing.T) {
	valid := insightsCreateOptions{model: "openai", sample: 20, lastNHours: 24}
	for _, tc := range []struct {
		name   string
		change func(*insightsCreateOptions)
	}{
		{"model", func(o *insightsCreateOptions) { o.model = "invalid" }},
		{"sample", func(o *insightsCreateOptions) { o.sample = 0 }},
		{"negative sample", func(o *insightsCreateOptions) { o.sample = -1 }},
		{"negative hours", func(o *insightsCreateOptions) { o.lastNHours = -1 }},
		{"missing window", func(o *insightsCreateOptions) { o.lastNHours = 0 }},
		{"conflicting window", func(o *insightsCreateOptions) { o.start = "2026-09-01T00:00:00Z" }},
		{"end only", func(o *insightsCreateOptions) { o.end = "2026-09-02T00:00:00Z" }},
		{"invalid start", func(o *insightsCreateOptions) { o.lastNHours = 0; o.start = "yesterday" }},
		{"reversed window", func(o *insightsCreateOptions) {
			o.lastNHours = 0
			o.start = "2026-09-02T00:00:00Z"
			o.end = "2026-09-01T00:00:00Z"
		}},
		{"context type", func(o *insightsCreateOptions) { o.userContext = `{"goal":1}` }},
		{"empty context", func(o *insightsCreateOptions) { o.userContext = `{"goal":" "}` }},
		{"null context", func(o *insightsCreateOptions) { o.userContext = `null` }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := valid
			tc.change(&opts)
			if _, err := insightsCreateParams(opts); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestInsightsCreateParamsAbsoluteWindow(t *testing.T) {
	params, err := insightsCreateParams(insightsCreateOptions{model: "anthropic", sample: 5,
		start: "2026-09-01T00:00:00Z", end: "2026-09-02T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatal(err)
	}
	if body["start_time"] == nil || body["end_time"] == nil || body["last_n_hours"] != nil {
		t.Fatalf("unexpected window body: %s", b)
	}
}

func TestInsightsCreateCmdRequestAndQueuedStatus(t *testing.T) {
	calls := 0
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/sessions/"+deleteTestProjectID+"/insights" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("x-tenant-id") != "demo-workspace" {
			t.Error("workspace header missing")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			http.Error(w, "bad body", 400)
			return
		}
		if body["sample"] != float64(20) || body["last_n_hours"] != float64(24) || body["model"] != "openai" {
			t.Errorf("unexpected request body: %#v", body)
		}
		if body["is_scheduled"] != nil || body["validate_model_secrets"] != nil || body["config_id"] != nil {
			t.Error("must not schedule, bypass validation, or apply a saved config implicitly")
		}
		context, ok := body["user_context"].(map[string]any)
		if !ok || context["Business goal"] != "Resolve refunds" {
			t.Errorf("context not preserved: %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"job-id","name":"demo-insights","status":"queued","error":null}`))
	})
	defer setupTestEnv(t, ts.URL)()
	flagWorkspaceID = "demo-workspace"
	flagOutputFormat = "json"
	cmd := newInsightsCreateCmd()
	cmd.SetArgs([]string{"--project-id", deleteTestProjectID, "--last-n-hours", "24", "--sample", "20", "--model", "openai", "--user-context", `{"Business goal":"Resolve refunds"}`})
	var err error
	stdout := captureStdout(t, func() { err = cmd.Execute() })
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || result["status"] != "queued" || result["project_id"] != deleteTestProjectID {
		t.Fatalf("unexpected result: calls=%d output=%s", calls, stdout)
	}
}

func TestInsightsCreateCmdDoesNotRetry429(t *testing.T) {
	calls := 0
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"detail":"Too many concurrent Insights jobs"}`))
	})
	defer setupTestEnv(t, ts.URL)()
	cmd := newInsightsCreateCmd()
	cmd.SetArgs([]string{"--project-id", deleteTestProjectID, "--last-n-hours", "24", "--sample", "20", "--model", "openai"})
	err := cmd.Execute()
	if calls != 1 || err == nil || !strings.Contains(err.Error(), "creating Insights job") {
		t.Fatalf("expected one request and contextual error: calls=%d err=%v", calls, err)
	}
}
