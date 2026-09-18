package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

const presetTestID = "11111111-1111-4111-8111-111111111111"
const presetTestJSON = `{"id":"11111111-1111-4111-8111-111111111111","name":"judge","available_in_evaluators":true,"settings":{"lc":1,"type":"constructor","id":["langchain","chat_models","openai","ChatOpenAI"],"kwargs":{"model":"gpt-4.1-mini","api_key":{"lc":1,"type":"secret","id":["OPENAI_API_KEY"]},"extra_headers":{"Authorization":"DO-NOT-PRINT"}}},"oauth_client_secret":"DO-NOT-PRINT"}`

func TestModelPresetRead(t *testing.T) {
	for _, mode := range []string{"list", "get", "empty", "error"} {
		t.Run(mode, func(t *testing.T) {
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				want := "/api/v1/playground-settings"
				if mode == "get" {
					want += "/" + presetTestID
				}
				if r.Method != "GET" || r.URL.Path != want {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				switch mode {
				case "get":
					fmt.Fprint(w, presetTestJSON)
				case "empty":
					fmt.Fprint(w, `[]`)
				case "error":
					w.WriteHeader(403)
					fmt.Fprint(w, `{"detail":"DO-NOT-PRINT"}`)
				default:
					fmt.Fprintf(w, "[%s]", presetTestJSON)
				}
			})
			defer setupTestEnv(t, ts.URL)()
			flagOutputFormat = "json"
			cmd := newModelCmd()
			args := []string{"preset", "list"}
			if mode == "get" {
				args = []string{"preset", "get", presetTestID}
			}
			cmd.SetArgs(args)
			var err error
			out := captureStdout(t, func() { err = cmd.Execute() })
			if mode == "error" {
				if err == nil || strings.Contains(err.Error(), "DO-NOT-PRINT") {
					t.Fatal("unsafe or missing error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !json.Valid([]byte(out)) || strings.Contains(out, "DO-NOT-PRINT") || strings.Contains(out, "OPENAI_API_KEY") {
				t.Fatalf("unsafe output: %s", out)
			}
			var response map[string]any
			_ = json.Unmarshal([]byte(out), &response)
			if mode == "empty" {
				if response["message"] == nil || response["next_steps"] == nil {
					t.Fatalf("missing empty guidance: %s", out)
				}
			} else if response["message"] != nil {
				t.Fatalf("nonempty result described as empty: %s", out)
			}
		})
	}
}

func TestModelPresetEvaluatorPayload(t *testing.T) {
	var p modelPreset
	_ = json.Unmarshal([]byte(presetTestJSON), &p)
	model, err := p.evaluatorModel()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := buildLLMEvaluatorPayloadWithModel("judge", llmEvaluatorTarget{projectID: presetTestID}, 1, "", "org/prompt", "", "", model, nil)
	if err != nil {
		t.Fatal(err)
	}
	structured := payload["evaluators"].([]map[string]any)[0]["structured"].(map[string]any)
	if structured["model"].(map[string]any)["type"] != "constructor" {
		t.Fatal("model missing")
	}
	for _, mode := range []string{"disabled", "oauth", "malformed"} {
		t.Run(mode, func(t *testing.T) {
			var invalid modelPreset
			_ = json.Unmarshal([]byte(presetTestJSON), &invalid)
			switch mode {
			case "disabled":
				v := false
				invalid.Evaluators = &v
			case "oauth":
				invalid.OAuth = true
			case "malformed":
				invalid.Settings = map[string]any{}
			}
			if _, err := invalid.evaluatorModel(); err == nil {
				t.Fatal("unsupported preset accepted")
			}
		})
	}
}

func TestEvaluatorPresetFlagContract(t *testing.T) {
	cmd := newEvaluatorCreateLLMCmd()
	if err := cmd.ParseFlags([]string{"--model-config", "model.json", "--model-preset", presetTestID}); err != nil {
		t.Fatal(err)
	}
	if err := cmd.ValidateFlagGroups(); err == nil {
		t.Fatal("accepted both model sources")
	}
}

func TestEvaluatorPresetDryRun(t *testing.T) {
	for _, flag := range []string{"--model-id", "--model-preset"} {
		t.Run(flag, func(t *testing.T) { testEvaluatorModelDryRun(t, flag) })
	}
}

func testEvaluatorModelDryRun(t *testing.T, flag string) {
	calls := 0
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/api/v1/playground-settings/"+presetTestID {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, presetTestJSON)
	})
	defer setupTestEnv(t, ts.URL)()
	flagOutputFormat = "json"
	cmd := newEvaluatorCreateLLMCmd()
	cmd.SetArgs([]string{"--name", "judge", "--project-id", presetTestID, "--hub-ref", "org/prompt", flag, presetTestID, "--dry-run"})
	var err error
	out := captureStdout(t, func() { err = cmd.Execute() })
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || !strings.Contains(out, "gpt-4.1-mini") || strings.Contains(out, "DO-NOT-PRINT") {
		t.Fatalf("unexpected dry run: %s", out)
	}
	var result map[string]any
	_ = json.Unmarshal([]byte(out), &result)
	key := "model"
	if flag == "--model-preset" {
		key = "model_preset"
	}
	if result[key] == nil {
		t.Fatalf("missing selected model: %s", out)
	}
}

func TestModelCanonicalCommands(t *testing.T) {
	for _, mode := range []string{"list", "get", "empty"} {
		t.Run(mode, func(t *testing.T) {
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if mode == "get" {
					fmt.Fprint(w, presetTestJSON)
				} else if mode == "empty" {
					fmt.Fprint(w, `[]`)
				} else {
					fmt.Fprintf(w, "[%s]", presetTestJSON)
				}
			})
			defer setupTestEnv(t, ts.URL)()
			flagOutputFormat = "json"
			cmd := newModelCmd()
			args := []string{"list"}
			key := "models"
			if mode == "get" {
				args = []string{"get", presetTestID}
				key = "model"
			}
			cmd.SetArgs(args)
			var err error
			out := captureStdout(t, func() { err = cmd.Execute() })
			if err != nil {
				t.Fatal(err)
			}
			var result map[string]any
			if json.Unmarshal([]byte(out), &result) != nil || result[key] == nil || strings.Contains(out, "DO-NOT-PRINT") {
				t.Fatalf("invalid safe envelope: %s", out)
			}
			if mode == "empty" && len(result[key].([]any)) != 0 {
				t.Fatal("expected empty array")
			}
		})
	}
}

func TestModelSourceFlagsExclusive(t *testing.T) {
	for _, flags := range [][]string{
		{"--model-id", presetTestID, "--model-config", "model.json"},
		{"--model-id", presetTestID, "--model-preset", presetTestID},
		{},
	} {
		cmd := newEvaluatorCreateLLMCmd()
		if err := cmd.ParseFlags(flags); err != nil {
			t.Fatal(err)
		}
		if err := cmd.ValidateFlagGroups(); err == nil {
			t.Fatalf("accepted invalid sources: %v", flags)
		}
	}
}
