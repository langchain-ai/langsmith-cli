package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/codegen"
	"github.com/langchain-ai/langsmith-cli/internal/gen"
	"github.com/langchain-ai/langsmith-cli/internal/genrt"
	"github.com/langchain-ai/langsmith-cli/internal/overrides"
	"github.com/langchain-ai/langsmith-cli/internal/structured"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

const queueID = "11111111-1111-1111-1111-111111111111"

type recorded struct {
	Method string
	Path   string
	Query  map[string][]string
	Body   map[string]any
}

// fakeAPI serves canned responses keyed by "METHOD path" and records requests.
func fakeAPI(t *testing.T, routes map[string]func(w http.ResponseWriter, r *http.Request)) (string, *[]recorded) {
	t.Helper()
	var calls []recorded
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		rec := recorded{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query()}
		if data, _ := io.ReadAll(r.Body); len(data) > 0 {
			_ = json.Unmarshal(data, &rec.Body)
		}
		calls = append(calls, rec)
		handler, ok := routes[r.Method+" "+r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"detail":"no route"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		handler(w, r)
	})
	return ts.URL, &calls
}

func reply(status int, body string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func runGenerated(t *testing.T, url string, args ...string) (string, error) {
	t.Helper()
	return executeCommand(t, append([]string{"--api-url", url, "--api-key", "test-key"}, args...)...)
}

func itemsPath(suffix string) string {
	return "/api/v1/platform/annotation-queues/" + queueID + "/items" + suffix
}

func TestGeneratedListReturnsPageEnvelope(t *testing.T) {
	url, calls := fakeAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"GET " + itemsPath(""): reply(200, `{"items":[{"id":"i1","item_type":"RUN","run_id":"r1"}],"next_cursor":"c2"}`),
	})

	out, err := runGenerated(t, url, "annotation-queue", "item", "list",
		"--queue-id", queueID, "--status", "needs_my_review", "--limit", "1", "--cursor", "c1", "--format", "json")

	require.NoError(t, err)
	require.JSONEq(t, `{"items":[{"id":"i1","item_type":"RUN","run_id":"r1"}],"next_cursor":"c2","has_more":true}`, out)
	q := (*calls)[0].Query
	require.Equal(t, []string{"needs_my_review"}, q["status"])
	require.Equal(t, []string{"1"}, q["page_size"])
	require.Equal(t, []string{"c1"}, q["cursor"])
}

func TestGeneratedListLastPageAndJQ(t *testing.T) {
	url, _ := fakeAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"GET " + itemsPath(""): reply(200, `{"items":[{"id":"i1"},{"id":"i2"}],"next_cursor":""}`),
	})

	out, err := runGenerated(t, url, "annotation-queue", "item", "list",
		"--queue-id", queueID, "--status", "archived", "--jq", "[.has_more, (.items | map(.id))]")

	require.NoError(t, err)
	require.JSONEq(t, `[false, ["i1","i2"]]`, out)
}

func TestGeneratedListPrettyShowsContinuation(t *testing.T) {
	url, _ := fakeAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"GET " + itemsPath(""): reply(200, `{"items":[{"id":"i1","item_type":"RUN"}],"next_cursor":"c2"}`),
	})

	out, err := runGenerated(t, url, "annotation-queue", "item", "list", "--queue-id", queueID, "--status", "archived")

	require.NoError(t, err)
	require.Contains(t, out, "i1")
	require.Contains(t, out, "RUN")
	require.Contains(t, out, "More results: rerun with --cursor c2")
}

func TestGeneratedWriteSendsTypedAndJSONFields(t *testing.T) {
	url, calls := fakeAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"POST " + itemsPath(""): reply(201, `{"items":[{"id":"i9","item_type":"RUN","run_id":"r1"}]}`),
	})

	out, err := runGenerated(t, url, "annotation-queue", "item", "create", "--queue-id", queueID,
		"--items-json", `[{"item_type":"RUN","run_id":"r1"}]`, "--extend-trace-retention", "--format", "json")

	require.NoError(t, err)
	require.JSONEq(t, `{"items":[{"id":"i9","item_type":"RUN","run_id":"r1"}]}`, out)
	call := (*calls)[0]
	require.Equal(t, []string{"true"}, call.Query["extend_trace_retention"])
	require.Equal(t, []any{map[string]any{"item_type": "RUN", "run_id": "r1"}}, call.Body["items"])
}

func TestGeneratedUpdateParsesTimestamps(t *testing.T) {
	url, calls := fakeAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"PATCH " + itemsPath("/i1"): reply(200, `{"id":"i1"}`),
	})

	_, err := runGenerated(t, url, "annotation-queue", "item", "update", "--queue-id", queueID, "--item-id", "i1",
		"--added-at", "2026-01-02T03:04:05Z", "--format", "json")

	require.NoError(t, err)
	require.Equal(t, "2026-01-02T03:04:05Z", (*calls)[0].Body["added_at"])

	_, err = runGenerated(t, url, "annotation-queue", "item", "update", "--queue-id", queueID, "--item-id", "i1",
		"--added-at", "yesterday")
	require.Equal(t, structured.CategoryInvalidInput, structured.Classify(err).Category)
}

func TestGeneratedDestructiveWriteNeedsYes(t *testing.T) {
	url, calls := fakeAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"POST " + itemsPath("/delete"): reply(200, `{"status":"ok"}`),
	})
	args := []string{"annotation-queue", "item", "delete-all", "--queue-id", queueID, "--item-ids", "a,b", "--format", "json"}

	_, err := runGenerated(t, url, args...)
	require.Equal(t, structured.CategoryInvalidInput, structured.Classify(err).Category)
	require.Contains(t, err.Error(), "--yes")
	require.Empty(t, *calls)

	out, err := runGenerated(t, url, append(args, "--yes")...)
	require.NoError(t, err)
	require.JSONEq(t, `{"status":"ok"}`, out)
	require.Equal(t, []any{"a", "b"}, (*calls)[0].Body["item_ids"])
}

func TestGeneratedValidationFailures(t *testing.T) {
	url, calls := fakeAPI(t, nil)

	_, err := runGenerated(t, url, "annotation-queue", "item", "list", "--status", "archived")
	se := structured.Classify(err)
	require.Equal(t, structured.CategoryInvalidInput, se.Category)
	require.Contains(t, se.Message, "--queue-id")

	_, err = runGenerated(t, url, "annotation-queue", "item", "list", "--queue-id", queueID, "--status", "later")
	require.Contains(t, structured.Classify(err).Message, "must be one of needs_my_review")

	_, err = runGenerated(t, url, "annotation-queue", "item", "create", "--queue-id", queueID, "--items-json", "[oops")
	require.Contains(t, structured.Classify(err).Message, "invalid JSON in --items-json")
	require.Empty(t, *calls)
}

func TestGeneratedAPIFailureIsStructured(t *testing.T) {
	url, _ := fakeAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"GET " + itemsPath("/i1/placement"): func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-Request-Id", "req-7")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"title":"Not Found","detail":"item not found","status":404}`))
		},
	})

	_, err := runGenerated(t, url, "annotation-queue", "item", "get-placement", "--queue-id", queueID, "--item-id", "i1")

	se := structured.Classify(err)
	require.Equal(t, structured.CategoryNotFound, se.Category)
	require.Equal(t, "item not found", se.Message)
	require.Equal(t, "req-7", se.RequestID)
	require.Equal(t, 5, se.ExitCode())
}

func TestOverrideResolvesQueueName(t *testing.T) {
	queues := `[{"id":"` + queueID + `","name":"checkout","queue_type":"single","tenant_id":"t"}]`
	url, calls := fakeAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"GET /api/v1/annotation-queues": reply(200, queues),
		"GET " + itemsPath(""):          reply(200, `{"items":[],"next_cursor":""}`),
	})

	out, err := runGenerated(t, url, "annotation-queue", "item", "list", "--queue", "checkout", "--status", "archived", "--format", "json")

	require.NoError(t, err)
	require.JSONEq(t, `{"items":[],"next_cursor":null,"has_more":false}`, out)
	require.Equal(t, []string{"checkout"}, (*calls)[0].Query["name"])
	require.Equal(t, itemsPath(""), (*calls)[1].Path)
}

func TestOverrideReportsAmbiguousAndMissingNames(t *testing.T) {
	two := `[{"id":"q1","name":"dup","queue_type":"single","tenant_id":"t"},{"id":"q2","name":"dup","queue_type":"single","tenant_id":"t"}]`
	url, _ := fakeAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"GET /api/v1/annotation-queues": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("name") == "dup" {
				_, _ = w.Write([]byte(two))
				return
			}
			_, _ = w.Write([]byte(`[]`))
		},
	})

	_, err := runGenerated(t, url, "annotation-queue", "item", "list", "--queue", "dup", "--status", "archived")
	se := structured.Classify(err)
	require.Equal(t, structured.CategoryInvalidInput, se.Category)
	require.Contains(t, se.Hint, "q1, q2")

	_, err = runGenerated(t, url, "annotation-queue", "item", "list", "--queue", "nope", "--status", "archived")
	require.Equal(t, structured.CategoryNotFound, structured.Classify(err).Category)

	_, err = runGenerated(t, url, "annotation-queue", "item", "list", "--queue", "dup", "--queue-id", queueID, "--status", "archived")
	require.Contains(t, err.Error(), "not both")
}

func TestGeneratedCommandsCarryOperationMetadata(t *testing.T) {
	root := NewRootCmd("test", "test")
	for _, op := range gen.Operations() {
		cmd, _, err := root.Find(op.Path)
		require.NoError(t, err, op.ID)
		require.Equal(t, op.ID, cmd.Annotations[genrt.AnnotationOperationID])
		require.Equal(t, op.Risk, cmd.Annotations[genrt.AnnotationRisk])
		require.NotNil(t, cmd.Flags().Lookup("jq"), op.ID)
		if op.Risk == "DESTRUCTIVE_WRITE" {
			require.NotNil(t, cmd.Flags().Lookup("yes"), op.ID)
		}
	}
}

func TestOverridesMatchCatalog(t *testing.T) {
	data, err := os.ReadFile("../gen/catalog.json")
	require.NoError(t, err)
	catalog, err := codegen.ParseCatalog(data)
	require.NoError(t, err)

	declared := map[string]bool{}
	for _, op := range catalog.Exposed() {
		if op.Override != nil {
			declared[op.ID] = true
		}
	}
	implemented := map[string]bool{}
	for id := range overrides.All() {
		implemented[id] = true
	}
	require.Equal(t, declared, implemented, "overlay `override:` entries and internal/overrides must match")
}

// TestGeneratedCommandSurface snapshots the generated command surface so a
// renamed flag or changed risk tier shows up as a reviewable diff.
// Run with UPDATE_GOLDEN=1 to accept changes.
func TestGeneratedCommandSurface(t *testing.T) {
	root := NewRootCmd("test", "test")
	var b strings.Builder
	for _, op := range gen.Operations() {
		cmd, _, err := root.Find(op.Path)
		require.NoError(t, err)
		fmt.Fprintf(&b, "%s  [%s]  %s\n", strings.Join(op.Path, " "), op.Risk, op.ID)
		var flags []string
		cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
			req := ""
			if strings.Contains(f.Usage, "(required)") {
				req = " required"
			}
			flags = append(flags, fmt.Sprintf("    --%s %s%s", f.Name, f.Value.Type(), req))
		})
		sort.Strings(flags)
		b.WriteString(strings.Join(flags, "\n") + "\n")
	}
	golden := "testdata/generated_surface.golden"
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		require.NoError(t, os.WriteFile(golden, []byte(b.String()), 0o644))
	}
	want, err := os.ReadFile(golden)
	require.NoError(t, err, "run with UPDATE_GOLDEN=1 to create %s", golden)
	require.Equal(t, string(want), b.String())
}

const (
	ruleID    = "22222222-2222-2222-2222-222222222222"
	projectID = "33333333-3333-3333-3333-333333333333"
)

var projectsRoute = map[string]func(http.ResponseWriter, *http.Request){
	"GET /api/v1/sessions": func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("name") != "checkout" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		_, _ = w.Write([]byte(`[{"id":"` + projectID + `","name":"checkout"}]`))
	},
}

func withRoutes(extra map[string]func(http.ResponseWriter, *http.Request)) map[string]func(http.ResponseWriter, *http.Request) {
	out := map[string]func(http.ResponseWriter, *http.Request){}
	for k, v := range projectsRoute {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func TestAutomationRuleCreateResolvesProjectAndSendsBody(t *testing.T) {
	url, calls := fakeAPI(t, withRoutes(map[string]func(http.ResponseWriter, *http.Request){
		"POST /api/v1/runs/rules": reply(200, `{"id":"`+ruleID+`","display_name":"errors to queue"}`),
	}))

	out, err := runGenerated(t, url, "automation-rule", "create", "--project", "checkout",
		"--display-name", "errors to queue", "--sampling-rate", "0.5", "--filter", `eq(error, true)`,
		"--webhooks-json", `[{"url":"https://example.com/hook"}]`, "--format", "json")

	require.NoError(t, err)
	require.JSONEq(t, `{"id":"`+ruleID+`","display_name":"errors to queue"}`, out)
	body := (*calls)[len(*calls)-1].Body
	require.Equal(t, projectID, body["session_id"])
	require.Equal(t, "errors to queue", body["display_name"])
	require.Equal(t, 0.5, body["sampling_rate"])
	require.Equal(t, "eq(error, true)", body["filter"])
	require.Equal(t, []any{map[string]any{"url": "https://example.com/hook"}}, body["webhooks"])
}

func TestAutomationRuleCreateValidation(t *testing.T) {
	url, calls := fakeAPI(t, projectsRoute)

	_, err := runGenerated(t, url, "automation-rule", "create", "--project-id", projectID)
	se := structured.Classify(err)
	require.Equal(t, structured.CategoryInvalidInput, se.Category)
	require.Contains(t, se.Message, "--display-name")
	require.Contains(t, se.Message, "--sampling-rate")

	_, err = runGenerated(t, url, "automation-rule", "create", "--project-id", "not-a-uuid",
		"--display-name", "x", "--sampling-rate", "1")
	require.Contains(t, structured.Classify(err).Message, "must be a project UUID")

	_, err = runGenerated(t, url, "automation-rule", "create", "--project", "nope",
		"--display-name", "x", "--sampling-rate", "1")
	require.Equal(t, structured.CategoryNotFound, structured.Classify(err).Category)
	for _, c := range *calls {
		require.NotEqual(t, "POST", c.Method)
	}
}

func TestAutomationRuleListLogsPaginates(t *testing.T) {
	url, calls := fakeAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"GET /api/v1/runs/rules/" + ruleID + "/logs/v2": reply(200, `{"logs":[{"rule_id":"`+ruleID+`","run_id":"r1"}],"cursor":"next-1"}`),
	})

	out, err := runGenerated(t, url, "automation-rule", "list-logs", "--rule-id", ruleID, "--limit", "1", "--cursor", "c0", "--format", "json")

	require.NoError(t, err)
	require.JSONEq(t, `{"items":[{"rule_id":"`+ruleID+`","run_id":"r1"}],"next_cursor":"next-1","has_more":true}`, out)
	require.Equal(t, []string{"c0"}, (*calls)[0].Query["cursor"])
	require.Equal(t, []string{"1"}, (*calls)[0].Query["limit"])
}

func TestAutomationRuleListFiltersByProject(t *testing.T) {
	url, calls := fakeAPI(t, withRoutes(map[string]func(http.ResponseWriter, *http.Request){
		"GET /api/v1/runs/rules": reply(200, `[{"id":"`+ruleID+`","display_name":"a"}]`),
	}))

	out, err := runGenerated(t, url, "automation-rule", "list", "--project", "checkout", "--jq", ".[].display_name")

	require.NoError(t, err)
	require.Equal(t, "a\n", out)
	require.Equal(t, []string{projectID}, (*calls)[len(*calls)-1].Query["session_id"])
}

func TestAutomationRuleTriggerIsGuarded(t *testing.T) {
	url, calls := fakeAPI(t, map[string]func(http.ResponseWriter, *http.Request){
		"POST /api/v1/runs/rules/" + ruleID + "/trigger": reply(200, `null`),
	})

	_, err := runGenerated(t, url, "automation-rule", "trigger", "--rule-id", ruleID)
	require.Contains(t, err.Error(), "--yes")
	require.Empty(t, *calls)

	_, err = runGenerated(t, url, "automation-rule", "trigger", "--rule-id", ruleID, "--yes", "--format", "json")
	require.NoError(t, err)
	require.Len(t, *calls, 1)
}
