package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	langsmith "github.com/langchain-ai/langsmith-go"
)

func TestExampleReadFilters(t *testing.T) {
	f := exampleReadFilters{asOf: "prod", splits: []string{"test", "refunds"}, metadata: `{"reviewed":true}`, filter: `exists(metadata,"source")`, search: []string{"refund"}}
	p := langsmith.ExampleListParams{}
	if err := f.apply(&p); err != nil {
		t.Fatal(err)
	}
	q := p.URLQuery()
	if q.Get("as_of") != "prod" || len(q["splits"]) != 2 || q.Get("metadata") != `{"reviewed":true}` || q.Get("filter") != f.filter || q.Get("full_text_contains") != "refund" {
		t.Fatal(q)
	}
	for _, splits := range [][]string{{" "}, {"test", "test"}, {" test"}} {
		if validateExampleSplits(splits) == nil {
			t.Fatal("invalid split accepted")
		}
	}
}

func TestDatasetVersionRequests(t *testing.T) {
	for _, action := range []string{"list", "get", "diff", "tag"} {
		t.Run(action, func(t *testing.T) {
			writes := 0
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.URL.Path == "/api/v1/datasets/"+workflowDataset:
					fmt.Fprintf(w, `{"id":%q,"name":"test"}`, workflowDataset)
				case strings.HasSuffix(r.URL.Path, "/versions/diff"):
					if r.URL.Query().Get("from_version") != "prod" || r.URL.Query().Get("to_version") != "latest" {
						t.Error(r.URL)
					}
					fmt.Fprint(w, `{"examples_added":[],"examples_modified":[],"examples_removed":[]}`)
				case strings.HasSuffix(r.URL.Path, "/versions"):
					if r.URL.Query().Get("limit") != "2" {
						t.Error(r.URL)
					}
					fmt.Fprintf(w, `[{"as_of":%q,"tags":["prod"]}]`, workflowTime)
				case r.Method != "GET":
					writes++
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
						return
					}
					if body["as_of"] != workflowTime || body["tag"] != "reviewed" {
						t.Error(body)
					}
					fmt.Fprintf(w, `{"as_of":%q,"tags":["reviewed"]}`, workflowTime)
				default:
					if r.URL.Query().Get("tag") != "prod" && !(action == "tag" && writes == 1 && r.URL.Query().Get("tag") == "reviewed") {
						t.Error(r.URL)
					}
					fmt.Fprintf(w, `{"as_of":%q,"tags":["prod"]}`, workflowTime)
				}
			})
			defer setupTestEnv(t, ts.URL)()
			cmd := newDatasetVersionAction(action)
			args := []string{"--dataset", workflowDataset}
			switch action {
			case "list":
				args = append(args, "--limit", "2")
			case "get":
				args = append(args, "--as-of", "prod")
			case "diff":
				args = append(args, "--from", "prod", "--to", "latest")
			case "tag":
				args = append(args, "--as-of", "prod", "--tag", "reviewed")
			}
			cmd.SetArgs(args)
			out := captureStdout(t, func() {
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
			})
			if !json.Valid([]byte(out)) || !strings.Contains(out, workflowDataset) {
				t.Fatal(out)
			}
			if action == "tag" && writes != 1 {
				t.Fatal(writes)
			}
		})
	}
}

func TestDatasetVersionPrecisionAndVerification(t *testing.T) {
	const precise = "2026-09-16T20:28:39.895276Z"
	for _, mode := range []string{"latest", "timestamp", "tag", "dry-run", "mismatch", "missing", "write-error"} {
		t.Run(mode, func(t *testing.T) {
			writes, reads := 0, 0
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api/v1/datasets/"+workflowDataset {
					fmt.Fprintf(w, `{"id":%q}`, workflowDataset)
					return
				}
				if r.Method != http.MethodGet {
					writes++
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if body["as_of"] != precise || body["tag"] != "reviewed" {
						t.Errorf("lost tag timestamp precision: %v", body)
					}
					if mode == "write-error" {
						w.WriteHeader(http.StatusInternalServerError)
					}
				} else {
					reads++
					q := r.URL.Query()
					if reads == 1 {
						if mode == "latest" {
							if q.Get("tag") != "latest" {
								t.Error(q)
							}
						} else if q.Get("as_of") != precise || q.Get("tag") != "" {
							t.Errorf("lost query timestamp precision: %v", q)
						}
					} else {
						if q.Get("tag") != "reviewed" {
							t.Error(q)
						}
						if mode == "missing" {
							w.WriteHeader(http.StatusNotFound)
							fmt.Fprint(w, `{}`)
							return
						}
						if mode == "mismatch" {
							fmt.Fprint(w, `{"as_of":"2026-09-16T20:28:39Z"}`)
							return
						}
					}
				}
				fmt.Fprintf(w, `{"as_of":%q}`, precise)
			})
			defer setupTestEnv(t, ts.URL)()
			action := "tag"
			args := []string{"--dataset", workflowDataset}
			if mode == "latest" || mode == "timestamp" {
				action = "get"
			} else {
				args = append(args, "--tag", "reviewed")
			}
			if mode != "latest" {
				args = append(args, "--as-of", precise)
			}
			if mode == "dry-run" {
				args = append(args, "--dry-run")
			}
			cmd := newDatasetVersionAction(action)
			cmd.SetArgs(args)
			var err error
			out := captureStdout(t, func() { err = cmd.Execute() })
			wantError := mode == "missing" || mode == "mismatch" || mode == "write-error"
			if (err != nil) != wantError {
				t.Fatalf("output=%s err=%v", out, err)
			}
			if wantError {
				if diagnostic, ok := err.(commandDiagnostic); !ok || diagnostic.code != "dataset_write_unverified" {
					t.Fatalf("expected unverified diagnostic: %v", err)
				}
				if strings.Contains(out, `"updated"`) {
					t.Fatal("unverified write reported success")
				}
			} else if !strings.Contains(out, precise) {
				t.Fatal(out)
			}
			wantWrites := 1
			if action == "get" || mode == "dry-run" {
				wantWrites = 0
			}
			if writes != wantWrites {
				t.Fatalf("writes=%d want=%d", writes, wantWrites)
			}
			if mode == "tag" && reads != 2 {
				t.Fatalf("missing read-back: %d reads", reads)
			}
		})
	}
}

func TestExampleBulkEmptyObjectPreview(t *testing.T) {
	for _, field := range []string{"inputs", "outputs", "metadata"} {
		t.Run(field, func(t *testing.T) {
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Error("dry-run attempted write")
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"id":%q,"dataset_id":%q,"modified_at":%q}`, workflowDataset, workflowDataset, workflowTime)
			})
			defer setupTestEnv(t, ts.URL)()
			file := workflowFile(t, fmt.Sprintf(`[{"id":%q,"expected_modified_at":%q,%q:{}}]`, workflowExample, workflowTime, field))
			cmd := newExampleUpdateBulkCmd()
			cmd.SetArgs([]string{"--dataset", workflowDataset, "--file", file, "--dry-run"})
			out := captureStdout(t, func() {
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
			})
			var result struct{ Edits []map[string]any }
			if err := json.Unmarshal([]byte(out), &result); err != nil || len(result.Edits) != 1 {
				t.Fatal(out)
			}
			edit := result.Edits[0]
			value, ok := edit[field].(map[string]any)
			if !ok || len(value) != 0 || len(edit) != 3 {
				t.Fatalf("preview must preserve only the explicit empty field: %v", edit)
			}
		})
	}
}

func TestDatasetListEmptyJSON(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	defer setupTestEnv(t, ts.URL)()
	flagOutputFormat = "json"
	cmd := newDatasetListCmd()
	out := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("expected empty array, got %q", out)
	}
}

func TestExampleBulkPreflightAndApply(t *testing.T) {
	for _, mode := range []string{"dry-run", "apply", "stale", "wrong-dataset", "partial"} {
		t.Run(mode, func(t *testing.T) {
			writes := 0
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.Contains(r.URL.Path, "/datasets/") {
					fmt.Fprintf(w, `{"id":%q}`, workflowDataset)
					return
				}
				if r.Method == "GET" {
					ds, modified := workflowDataset, workflowTime
					if mode == "wrong-dataset" {
						ds = workflowExample
					}
					if mode == "stale" {
						modified = "2026-09-17T00:00:00Z"
					}
					fmt.Fprintf(w, `{"id":%q,"dataset_id":%q,"modified_at":%q}`, workflowExample, ds, modified)
					return
				}
				writes++
				var body map[string]any
				d := json.NewDecoder(r.Body)
				d.UseNumber()
				if err := d.Decode(&body); err != nil {
					t.Error(err)
					return
				}
				if body["outputs"].(map[string]any)["number"] != json.Number("9007199254740993") {
					t.Error(body)
				}
				if mode == "partial" {
					w.WriteHeader(500)
					fmt.Fprint(w, `{"detail":"secret"}`)
					return
				}
				fmt.Fprint(w, `{}`)
			})
			defer setupTestEnv(t, ts.URL)()
			file := workflowFile(t, fmt.Sprintf(`[{"id":%q,"expected_modified_at":%q,"outputs":{"number":9007199254740993},"splits":[]}]`, workflowExample, workflowTime))
			flag := "--apply"
			if mode == "dry-run" {
				flag = "--dry-run"
			}
			cmd := newExampleUpdateBulkCmd()
			cmd.SetArgs([]string{"--dataset", workflowDataset, "--file", file, flag})
			var err error
			out := captureStdout(t, func() { err = cmd.Execute() })
			wantError := mode == "stale" || mode == "wrong-dataset" || mode == "partial"
			if (err != nil) != wantError {
				t.Fatal(err)
			}
			wantWrites := 0
			if mode == "apply" || mode == "partial" {
				wantWrites = 1
			}
			if writes != wantWrites {
				t.Fatalf("writes=%d want=%d", writes, wantWrites)
			}
			if strings.Contains(out, "secret") {
				t.Fatal("unsafe error")
			}
			if mode == "partial" && !strings.Contains(out, "unverified") {
				t.Fatal(out)
			}
		})
	}
}

func TestDatasetEditFileValidation(t *testing.T) {
	for _, data := range []string{`[{"id":"a","id":"b"}]`, `[{"unknown":true}]`, `[] {}`, `[{"outputs":{"a":1,"a":2}}]`} {
		var edits []exampleEdit
		if readDatasetEditFile(workflowFile(t, data), &edits) == nil {
			t.Fatal("accepted", data)
		}
	}
}

func TestExampleUpdateMetadataAndSplits(t *testing.T) {
	for _, clear := range []bool{false, true} {
		t.Run(fmt.Sprint(clear), func(t *testing.T) {
			calls := 0
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "PATCH" {
					t.Error(r.Method)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if len(body) != 2 || body["metadata"].(map[string]any)["reviewed"] != true {
					t.Error(body)
				}
				splits := body["split"].([]any)
				if clear && len(splits) != 0 || !clear && len(splits) != 2 {
					t.Error(splits)
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{}`)
			})
			defer setupTestEnv(t, ts.URL)()
			cmd := newExampleUpdateCmd()
			args := []string{workflowExample, "--metadata", `{"reviewed":true}`}
			if clear {
				args = append(args, "--clear-splits")
			} else {
				args = append(args, "--split", "test", "--split", "refunds")
			}
			cmd.SetArgs(args)
			captureStdout(t, func() {
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
			})
			if calls != 1 {
				t.Fatal(calls)
			}
		})
	}
}

func TestExampleBulkMixedOutcomes(t *testing.T) {
	writes := 0
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/datasets/") {
			fmt.Fprintf(w, `{"id":%q}`, workflowDataset)
			return
		}
		if r.Method == "GET" {
			fmt.Fprintf(w, `{"dataset_id":%q,"modified_at":%q}`, workflowDataset, workflowTime)
			return
		}
		writes++
		if writes == 2 {
			w.WriteHeader(500)
		}
		fmt.Fprint(w, `{}`)
	})
	defer setupTestEnv(t, ts.URL)()
	file := workflowFile(t, fmt.Sprintf(`[
		{"id":%q,"expected_modified_at":%q,"outputs":{}},
		{"id":"33333333-3333-4333-8333-333333333333","expected_modified_at":%q,"outputs":{}}
	]`, workflowExample, workflowTime, workflowTime))
	cmd := newExampleUpdateBulkCmd()
	cmd.SetArgs([]string{"--dataset", workflowDataset, "--file", file, "--apply"})
	var err error
	out := captureStdout(t, func() { err = cmd.Execute() })
	var result struct {
		Updated, Unverified int
		Results             []map[string]any
	}
	if json.Unmarshal([]byte(out), &result) != nil || err == nil || writes != 2 || result.Updated != 1 || result.Unverified != 1 || len(result.Results) != 2 {
		t.Fatalf("out=%s writes=%d err=%v", out, writes, err)
	}
}

func TestDatasetConfigureDryRunAndApply(t *testing.T) {
	for _, mode := range []string{"--dry-run", "--apply"} {
		t.Run(mode, func(t *testing.T) {
			writes := 0
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == "PATCH" {
					writes++
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
						return
					}
					if len(body) != 2 || body["inputs_schema_definition"] == nil || body["transformations"] == nil {
						t.Error(body)
					}
					fmt.Fprint(w, `{}`)
					return
				}
				fmt.Fprintf(w, `{"id":%q}`, workflowDataset)
			})
			defer setupTestEnv(t, ts.URL)()
			file := workflowFile(t, `{"inputs_schema_definition":{"type":"object"},"transformations":[]}`)
			cmd := newDatasetConfigureCmd()
			cmd.SetArgs([]string{"--dataset", workflowDataset, "--file", file, mode})
			captureStdout(t, func() {
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
			})
			if (writes == 1) != (mode == "--apply") {
				t.Fatal(writes)
			}
		})
	}
}
