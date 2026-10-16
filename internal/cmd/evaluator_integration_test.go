//go:build integration

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-cli/internal/client"
	langsmith "github.com/langchain-ai/langsmith-go"
)

// These tests check that every write command leaves a rule's other settings
// alone. Each one seeds a rule with non-default settings (a webhook, 0.1
// sampling, a filter), runs a command, and reads the rule back.

const (
	cliHelperArgsEnv = "LANGSMITH_CLI_INTEGRATION_ARGS"
	seededFilter     = "eq(is_root, true)"
	seededSampling   = 0.1
)

// TestIntegrationCLIHelperProcess runs the CLI when re-executed by runCLI.
// Commands report some failures through os.Exit, so they run in a subprocess.
func TestIntegrationCLIHelperProcess(t *testing.T) {
	raw := os.Getenv(cliHelperArgsEnv)
	if raw == "" {
		t.Skip("helper process for runCLI")
	}
	var args []string
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	cmd := NewRootCmd("dev", "dev")
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		// Matches cmd/langsmith/main.go, which prints the error to stdout.
		fmt.Println(err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}

type cliResult struct {
	stdout, stderr string
	exitCode       int
}

// runCLI runs `langsmith --format json <args>` in a subprocess with empty stdin.
func runCLI(t *testing.T, args ...string) cliResult {
	t.Helper()
	full := append([]string{"--format", "json"}, args...)
	encoded, _ := json.Marshal(full)
	cmd := exec.Command(os.Args[0], "-test.run=^TestIntegrationCLIHelperProcess$")
	cmd.Env = append(os.Environ(), cliHelperArgsEnv+"="+string(encoded))
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	res := cliResult{stdout: stdout.String(), stderr: stderr.String()}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		res.exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("running langsmith %s: %v", strings.Join(full, " "), err)
	}
	return res
}

func mustRunCLI(t *testing.T, args ...string) string {
	t.Helper()
	res := runCLI(t, args...)
	if res.exitCode != 0 {
		t.Fatalf("langsmith %s failed (exit %d):\n%s", strings.Join(args, " "), res.exitCode, res.stderr)
	}
	return res.stdout
}

func runCLIExpectFailure(t *testing.T, wantErr string, args ...string) cliResult {
	t.Helper()
	res := runCLI(t, args...)
	if res.exitCode == 0 {
		t.Fatalf("expected langsmith %s to fail, it succeeded:\n%s", strings.Join(args, " "), res.stdout)
	}
	if output := res.stdout + res.stderr; !strings.Contains(output, wantErr) {
		t.Fatalf("expected langsmith %s to fail with %q, got:\n%s", strings.Join(args, " "), wantErr, output)
	}
	return res
}

func integrationClient(t *testing.T) (*client.Client, context.Context) {
	t.Helper()
	requireIntegrationEnv(t)
	c, err := getClient()
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}
	return c, context.Background()
}

// createTestDataset creates an empty dataset that is deleted after the test.
func createTestDataset(t *testing.T, c *client.Client, ctx context.Context) (id, name string) {
	t.Helper()
	name = randomHandle("cli-it-evaluator")
	ds, err := c.SDK.Datasets.New(ctx, langsmith.DatasetNewParams{
		Name:        langsmith.F(name),
		Description: langsmith.F("langsmith-cli integration test (auto-deletes)"),
	})
	if err != nil {
		t.Fatalf("creating dataset: %v", err)
	}
	t.Cleanup(func() {
		if _, err := c.SDK.Datasets.Delete(context.Background(), ds.ID); err != nil {
			t.Logf("cleanup: deleting dataset %s: %v", ds.ID, err)
		}
	})
	return ds.ID, name
}

func codeEvaluatorPayload(score int) map[string]any {
	return map[string]any{"code_evaluators": []any{map[string]any{
		"code":     fmt.Sprintf("def perform_eval(run, example):\n    return {\"score\": %d}\n", score),
		"language": "python",
	}}}
}

func llmEvaluatorPayload() map[string]any {
	return map[string]any{"evaluators": []any{map[string]any{"structured": map[string]any{
		"model":           testModelConfig(),
		"prompt":          [][]string{{"system", "Grade the answer."}, {"human", "{{input}} {{output}}"}},
		"schema":          map[string]any{"name": "eval", "description": "", "parameters": testSchema()},
		"template_format": "mustache",
	}}}}
}

func testModelConfig() map[string]any {
	return map[string]any{
		"lc": 1, "type": "constructor",
		"id":     []string{"langchain", "chat_models", "openai", "ChatOpenAI"},
		"kwargs": map[string]any{"model": "gpt-4o-mini", "temperature": 0},
	}
}

func testSchema() map[string]any {
	return map[string]any{
		"title": "eval", "description": "Whether the answer is correct", "type": "object",
		"properties": map[string]any{"correct": map[string]any{"type": "boolean", "description": "Correct"}},
		"required":   []string{"correct"},
	}
}

// seededRule describes the non-default settings a seeded rule starts with.
type seededRule struct {
	id, name, webhookURL string
}

// seedRule creates a rule on the dataset with a webhook, 0.1 sampling, and a
// filter, plus the given action payload, and deletes it after the test.
func seedRule(t *testing.T, c *client.Client, ctx context.Context, datasetID, name string, action map[string]any) seededRule {
	t.Helper()
	hook := "https://example.com/langsmith-cli-it/" + randomHandle("hook")
	body := map[string]any{
		"display_name":  name,
		"dataset_id":    datasetID,
		"sampling_rate": seededSampling,
		"is_enabled":    true,
		"filter":        seededFilter,
		"webhooks":      []any{map[string]any{"url": hook}},
	}
	for k, v := range action {
		body[k] = v
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := c.RawPost(ctx, "/api/v1/runs/rules", body, &created); err != nil {
		skipWithoutModelCredentials(t, err.Error())
		t.Fatalf("seeding rule %q: %v", name, err)
	}
	t.Cleanup(func() {
		_ = c.RawDelete(context.Background(), "/api/v1/runs/rules/"+created.ID, nil)
	})
	return seededRule{id: created.ID, name: name, webhookURL: hook}
}

func fetchRule(t *testing.T, c *client.Client, ctx context.Context, id string) *langsmith.Evaluator {
	t.Helper()
	rule, err := getRule(ctx, c, id)
	if err != nil {
		t.Fatalf("fetching rule %s: %v", id, err)
	}
	return rule
}

func ruleExists(t *testing.T, c *client.Client, ctx context.Context, id string) bool {
	t.Helper()
	rules, err := c.SDK.Evaluators.List(ctx, langsmith.EvaluatorListParams{ID: langsmith.F([]string{id})})
	if err != nil {
		t.Fatalf("listing rule %s: %v", id, err)
	}
	return slices.ContainsFunc(*rules, func(r langsmith.Evaluator) bool { return r.ID == id })
}

// assertSeededSettings checks that the rule still has every seeded setting.
func assertSeededSettings(t *testing.T, c *client.Client, ctx context.Context, seed seededRule, wantSampling float64) *langsmith.Evaluator {
	t.Helper()
	rule := fetchRule(t, c, ctx, seed.id)
	if rule.DisplayName != seed.name {
		t.Errorf("display_name = %q, want %q", rule.DisplayName, seed.name)
	}
	if rule.SamplingRate != wantSampling {
		t.Errorf("sampling_rate = %v, want %v", rule.SamplingRate, wantSampling)
	}
	if rule.Filter != seededFilter {
		t.Errorf("filter = %q, want %q", rule.Filter, seededFilter)
	}
	if !rule.IsEnabled {
		t.Error("is_enabled = false, want true")
	}
	if len(rule.Webhooks) != 1 || rule.Webhooks[0].URL != seed.webhookURL {
		t.Errorf("webhooks = %+v, want one webhook to %s", rule.Webhooks, seed.webhookURL)
	}
	return rule
}

func writeEvaluatorFile(t *testing.T, score int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "eval.py")
	src := fmt.Sprintf("def grade(run, example):\n    return {\"score\": %d}\n", score)
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeLLMFiles(t *testing.T) (prompt, schema, model string) {
	t.Helper()
	dir := t.TempDir()
	files := map[string]any{
		"prompt.json": [][]string{{"system", "Grade the answer strictly."}, {"human", "{{input}} {{output}}"}},
		"schema.json": testSchema(),
		"model.json":  testModelConfig(),
	}
	for name, content := range files {
		data, _ := json.Marshal(content)
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "prompt.json"), filepath.Join(dir, "schema.json"), filepath.Join(dir, "model.json")
}

func TestIntegrationEvaluatorUploadKeepsRuleSettings(t *testing.T) {
	c, ctx := integrationClient(t)
	datasetID, datasetName := createTestDataset(t, c, ctx)
	seed := seedRule(t, c, ctx, datasetID, randomHandle("upload"), codeEvaluatorPayload(0))

	// Without --replace the existing rule is refused and left alone.
	runCLIExpectFailure(t, "already exists",
		"evaluator", "upload", writeEvaluatorFile(t, 1), "--name", seed.name, "--function", "grade", "--dataset", datasetName)
	assertSeededSettings(t, c, ctx, seed, seededSampling)

	mustRunCLI(t, "evaluator", "upload", writeEvaluatorFile(t, 1),
		"--name", seed.name, "--function", "grade", "--dataset", datasetName, "--replace", "--yes")
	rule := assertSeededSettings(t, c, ctx, seed, seededSampling)
	if len(rule.CodeEvaluators) != 1 || !strings.Contains(rule.CodeEvaluators[0].Code, `"score": 1`) {
		t.Errorf("expected the uploaded code, got %+v", rule.CodeEvaluators)
	}

	// An explicit --sampling-rate still changes the rate and nothing else.
	mustRunCLI(t, "evaluator", "upload", writeEvaluatorFile(t, 2),
		"--name", seed.name, "--function", "grade", "--dataset", datasetName, "--replace", "--yes", "--sampling-rate", "0.5")
	assertSeededSettings(t, c, ctx, seed, 0.5)
}

func TestIntegrationEvaluatorUploadCreatesRule(t *testing.T) {
	c, ctx := integrationClient(t)
	_, datasetName := createTestDataset(t, c, ctx)
	name := randomHandle("upload-new")

	out := mustRunCLI(t, "evaluator", "upload", writeEvaluatorFile(t, 1),
		"--name", name, "--function", "grade", "--dataset", datasetName, "--sampling-rate", "0.25")
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parsing upload output: %v\n%s", err, out)
	}
	ruleID, _ := result["rule_id"].(string)
	t.Cleanup(func() { _ = c.RawDelete(context.Background(), "/api/v1/runs/rules/"+ruleID, nil) })

	rule := fetchRule(t, c, ctx, ruleID)
	if rule.DisplayName != name || rule.SamplingRate != 0.25 || !rule.IsEnabled || len(rule.CodeEvaluators) != 1 {
		t.Errorf("unexpected new rule: name=%q sampling=%v enabled=%v code=%d", rule.DisplayName, rule.SamplingRate, rule.IsEnabled, len(rule.CodeEvaluators))
	}
}

func TestIntegrationEvaluatorCreateLLMKeepsRuleSettings(t *testing.T) {
	c, ctx := integrationClient(t)
	datasetID, datasetName := createTestDataset(t, c, ctx)
	seed := seedRule(t, c, ctx, datasetID, randomHandle("llm"), llmEvaluatorPayload())
	prompt, schema, model := writeLLMFiles(t)
	args := []string{"evaluator", "create-llm", "--name", seed.name, "--dataset", datasetName,
		"--prompt", prompt, "--schema", schema, "--model-config", model}

	runCLIExpectFailure(t, "already exists", args...)
	assertSeededSettings(t, c, ctx, seed, seededSampling)

	mustRunCLI(t, append(args, "--replace", "--yes")...)
	assertSeededSettings(t, c, ctx, seed, seededSampling)

	mustRunCLI(t, append(args, "--replace", "--yes", "--sampling-rate", "0.5")...)
	assertSeededSettings(t, c, ctx, seed, 0.5)
}

func TestIntegrationEvaluatorCreateLLMCreatesRule(t *testing.T) {
	c, ctx := integrationClient(t)
	_, datasetName := createTestDataset(t, c, ctx)
	prompt, schema, model := writeLLMFiles(t)
	name := randomHandle("llm-new")

	args := []string{"evaluator", "create-llm", "--name", name, "--dataset", datasetName,
		"--prompt", prompt, "--schema", schema, "--model-config", model, "--sampling-rate", "0.25"}
	res := runCLI(t, args...)
	if res.exitCode != 0 {
		skipWithoutModelCredentials(t, res.stdout+res.stderr)
		t.Fatalf("langsmith %s failed (exit %d):\n%s%s", strings.Join(args, " "), res.exitCode, res.stdout, res.stderr)
	}
	out := res.stdout
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parsing create-llm output: %v\n%s", err, out)
	}
	ruleID, _ := result["rule_id"].(string)
	t.Cleanup(func() { _ = c.RawDelete(context.Background(), "/api/v1/runs/rules/"+ruleID, nil) })

	rule := fetchRule(t, c, ctx, ruleID)
	if rule.DisplayName != name || rule.SamplingRate != 0.25 || !rule.IsEnabled || !isEvaluatorRule(*rule) {
		t.Errorf("unexpected new rule: name=%q sampling=%v enabled=%v", rule.DisplayName, rule.SamplingRate, rule.IsEnabled)
	}
}

func TestIntegrationEvaluatorRuleGetAndList(t *testing.T) {
	c, ctx := integrationClient(t)
	datasetID, datasetName := createTestDataset(t, c, ctx)
	seed := seedRule(t, c, ctx, datasetID, randomHandle("show"), codeEvaluatorPayload(0))

	out := mustRunCLI(t, "evaluator", "rule", "get", seed.id)
	if strings.Contains(out, "langsmith-cli-it/") {
		t.Errorf("rule get should mask the webhook path, got:\n%s", out)
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(out), &detail); err != nil {
		t.Fatalf("parsing rule get output: %v\n%s", err, out)
	}
	if got := mustJSON(t, detail["actions"]); got != `["evaluator","webhook"]` {
		t.Errorf("rule get actions = %s", got)
	}
	if got := mustJSON(t, detail["webhooks"]); got != `[{"url":"https://example.com/..."}]` {
		t.Errorf("rule get webhooks = %s", got)
	}
	if detail["filter"] != seededFilter || detail["sampling_rate"] != seededSampling {
		t.Errorf("rule get filter/sampling = %v / %v", detail["filter"], detail["sampling_rate"])
	}

	out = mustRunCLI(t, "evaluator", "rule", "list", "--dataset", datasetName)
	var rules []map[string]any
	if err := json.Unmarshal([]byte(out), &rules); err != nil {
		t.Fatalf("parsing rule list output: %v\n%s", err, out)
	}
	if len(rules) != 1 || rules[0]["rule_id"] != seed.id {
		t.Fatalf("rule list = %v, want only %s", rules, seed.id)
	}
	if got := mustJSON(t, rules[0]["actions"]); got != `["evaluator","webhook"]` {
		t.Errorf("rule list actions = %s", got)
	}
	assertSeededSettings(t, c, ctx, seed, seededSampling)
}

func TestIntegrationEvaluatorRuleDelete(t *testing.T) {
	c, ctx := integrationClient(t)
	datasetID, datasetName := createTestDataset(t, c, ctx)
	name := randomHandle("delete")
	first := seedRule(t, c, ctx, datasetID, name, codeEvaluatorPayload(0))
	second := seedRule(t, c, ctx, datasetID, name, codeEvaluatorPayload(1))
	webhookOnly := seedRule(t, c, ctx, datasetID, randomHandle("webhook-only"), nil)

	runCLIExpectFailure(t, "pass a rule ID", "evaluator", "rule", "delete", name, "--yes")
	runCLIExpectFailure(t, "2 rules are named", "evaluator", "rule", "delete", name, "--dataset", datasetName, "--yes")
	runCLIExpectFailure(t, "no evaluator rule named", "evaluator", "rule", "delete", randomHandle("missing"), "--dataset", datasetName, "--yes")
	runCLIExpectFailure(t, "not an evaluator rule", "evaluator", "rule", "delete", webhookOnly.id, "--yes")
	for _, seed := range []seededRule{first, second, webhookOnly} {
		assertSeededSettings(t, c, ctx, seed, seededSampling)
	}

	// Without --yes the prompt names the webhook that would go with the rule.
	res := runCLIExpectFailure(t, "aborted", "evaluator", "rule", "delete", first.id)
	if !strings.Contains(res.stderr, "1 webhook(s)") {
		t.Errorf("confirmation should mention the webhook, got:\n%s", res.stderr)
	}
	assertSeededSettings(t, c, ctx, first, seededSampling)

	evaluatorID := fetchRule(t, c, ctx, first.id).EvaluatorID
	mustRunCLI(t, "evaluator", "rule", "delete", first.id, "--yes")
	if ruleExists(t, c, ctx, first.id) {
		t.Errorf("rule %s still exists after delete", first.id)
	}
	if evaluatorID != "" {
		mustRunCLI(t, "evaluator", "get", evaluatorID)
	}
	assertSeededSettings(t, c, ctx, second, seededSampling)
	assertSeededSettings(t, c, ctx, webhookOnly, seededSampling)
}

func TestIntegrationEvaluatorDeleteWithRules(t *testing.T) {
	c, ctx := integrationClient(t)
	datasetID, _ := createTestDataset(t, c, ctx)
	seed := seedRule(t, c, ctx, datasetID, randomHandle("evaluator-delete"), codeEvaluatorPayload(0))
	evaluatorID := fetchRule(t, c, ctx, seed.id).EvaluatorID
	if evaluatorID == "" {
		t.Skip("the seeded rule has no evaluator_id; evaluator delete needs one")
	}

	runCLIExpectFailure(t, "--delete-rules", "evaluator", "delete", evaluatorID)
	assertSeededSettings(t, c, ctx, seed, seededSampling)

	res := runCLIExpectFailure(t, "aborted", "evaluator", "delete", evaluatorID, "--delete-rules")
	if !strings.Contains(res.stderr, "1 webhook(s)") {
		t.Errorf("confirmation should list the rule's webhook, got:\n%s", res.stderr)
	}
	assertSeededSettings(t, c, ctx, seed, seededSampling)

	mustRunCLI(t, "evaluator", "delete", evaluatorID, "--delete-rules", "--yes")
	if ruleExists(t, c, ctx, seed.id) {
		t.Errorf("rule %s still exists after evaluator delete --delete-rules", seed.id)
	}
}

// createTestProject creates a project with two root runs that share a name.
// Projects and datasets are the only targets trace export reads, and only a
// project holds traces.
func createTestProject(t *testing.T, c *client.Client, ctx context.Context) (id, name string) {
	t.Helper()
	name = randomHandle("cli-it-export")
	session, err := c.SDK.Sessions.New(ctx, langsmith.SessionNewParams{
		Name:        langsmith.F(name),
		Description: langsmith.F("langsmith-cli integration test (auto-deletes)"),
	})
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}
	t.Cleanup(func() { deleteTestProjects(t, c, session.ID, name) })

	now := time.Now().UTC()
	var runs []langsmith.RunIngestParam
	for i := range 2 {
		runID := uuid.NewString()
		start := now.Add(time.Duration(i) * time.Second)
		dotted := fmt.Sprintf("%s%06dZ%s", start.Format("20060102T150405"), start.Nanosecond()/1000, runID)
		runs = append(runs, langsmith.RunIngestParam{
			ID:          langsmith.F(runID),
			TraceID:     langsmith.F(runID),
			DottedOrder: langsmith.F(dotted),
			Name:        langsmith.F("same-name"),
			RunType:     langsmith.F(langsmith.RunIngestRunTypeChain),
			SessionID:   langsmith.F(session.ID),
			StartTime:   langsmith.F(start.Format(time.RFC3339Nano)),
			EndTime:     langsmith.F(start.Add(100 * time.Millisecond).Format(time.RFC3339Nano)),
			Inputs:      langsmith.F(map[string]any{"i": i}),
			Outputs:     langsmith.F(map[string]any{"o": i}),
		})
	}
	if _, err := c.SDK.Runs.IngestBatch(ctx, langsmith.RunIngestBatchParams{Post: langsmith.F(runs)}); err != nil {
		t.Fatalf("ingesting runs: %v", err)
	}
	return session.ID, name
}

// deleteTestProjects deletes the project and any -local, -development, or
// -staging sibling that run ingestion created for it.
func deleteTestProjects(t *testing.T, c *client.Client, id, name string) {
	ctx := context.Background()
	if err := c.SDK.Sessions.Delete(ctx, id); err != nil {
		t.Logf("cleanup: deleting project %s: %v", name, err)
	}
	for _, suffix := range []string{"-local", "-development", "-staging"} {
		sid, err := resolveSessionID(ctx, c, name+suffix, "", "cleanup")
		if err != nil {
			continue
		}
		if err := c.SDK.Sessions.Delete(ctx, sid); err != nil {
			t.Logf("cleanup: deleting project %s%s: %v", name, suffix, err)
		}
	}
}

func TestIntegrationTraceExportFilenameCollision(t *testing.T) {
	c, ctx := integrationClient(t)
	projectID, _ := createTestProject(t, c, ctx)

	// Ingestion is asynchronous; wait until both traces export.
	okDir := t.TempDir()
	deadline := time.Now().Add(2 * time.Minute)
	for {
		out := mustRunCLI(t, "trace", "export", okDir, "--project-id", projectID)
		var result map[string]any
		_ = json.Unmarshal([]byte(out), &result)
		if result["count"] == float64(2) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("traces did not become queryable in time; last export: %s", out)
		}
		time.Sleep(5 * time.Second)
	}
	entries, err := os.ReadDir(okDir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("expected 2 files with the default pattern, got %d (%v)", len(entries), err)
	}

	clashDir := t.TempDir()
	runCLIExpectFailure(t, "filename pattern resolves multiple traces",
		"trace", "export", clashDir, "--project-id", projectID, "--filename-pattern", "{name}.jsonl")
	if entries, _ := os.ReadDir(clashDir); len(entries) != 0 {
		t.Errorf("expected no files written on a filename collision, got %d", len(entries))
	}
}

// skipWithoutModelCredentials skips LLM evaluator tests in workspaces that have
// no model provider key, where the API rejects every LLM evaluator.
func skipWithoutModelCredentials(t *testing.T, output string) {
	t.Helper()
	if strings.Contains(output, "Missing credentials") {
		t.Skip("workspace has no model provider credentials for LLM evaluators")
	}
}
