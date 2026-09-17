package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"github.com/spf13/cobra"
)

const traceSelectionVersion = 1
const maxTraceSelection = 1000

type traceExample struct {
	RunID     string         `json:"run_id"`
	TraceID   string         `json:"trace_id"`
	StartTime time.Time      `json:"start_time"`
	Inputs    map[string]any `json:"inputs"`
	Outputs   map[string]any `json:"outputs,omitempty"`
}

type traceSelection struct {
	Version        int                 `json:"version"`
	APIURL         string              `json:"api_url"`
	ProjectID      string              `json:"project_id"`
	DatasetID      string              `json:"dataset_id"`
	ReferenceMode  string              `json:"reference_mode"`
	InputsPointer  string              `json:"inputs_pointer"`
	OutputsPointer string              `json:"outputs_pointer"`
	Examples       []traceExample      `json:"examples"`
	WorkspaceID    *string             `json:"workspace_id,omitempty"`
	SelectionInfo  *traceSelectionInfo `json:"selection_info,omitempty"`
}

type traceSelectionInfo struct {
	Limit    int    `json:"limit"`
	Selected int    `json:"selected"`
	HasMore  bool   `json:"has_more"`
	Scope    string `json:"scope"`
}

// objectAtPointer uses JSON Pointer, not executable expressions or environment access.
func objectAtPointer(value map[string]any, pointer string) (map[string]any, error) {
	var current any = value
	if pointer != "" {
		if !strings.HasPrefix(pointer, "/") {
			return nil, fmt.Errorf("JSON pointer must start with /")
		}
		for _, token := range strings.Split(pointer[1:], "/") {
			for i := 0; i < len(token); i++ {
				if token[i] == '~' {
					if i+1 == len(token) || (token[i+1] != '0' && token[i+1] != '1') {
						return nil, fmt.Errorf("invalid JSON pointer escape")
					}
					i++
				}
			}
			token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
			switch v := current.(type) {
			case map[string]any:
				current = v[token]
			case []any:
				n, err := strconv.Atoi(token)
				if err != nil || n < 0 || n >= len(v) {
					return nil, fmt.Errorf("JSON pointer array index not found")
				}
				current = v[n]
			default:
				return nil, fmt.Errorf("JSON pointer path not found")
			}
		}
	}
	obj, ok := current.(map[string]any)
	if !ok || obj == nil {
		return nil, fmt.Errorf("selected value must be a non-null JSON object")
	}
	return obj, nil
}

func (s traceSelection) validate() error {
	if s.Version != traceSelectionVersion {
		return fmt.Errorf("unsupported selection version")
	}
	for _, id := range []string{s.ProjectID, s.DatasetID} {
		if _, err := uuid.Parse(id); err != nil {
			return fmt.Errorf("invalid selection project/dataset UUID")
		}
	}
	if s.ReferenceMode != "inputs-only" && s.ReferenceMode != "observed" && s.ReferenceMode != "corrected" {
		return fmt.Errorf("reference mode must be inputs-only, observed, or corrected")
	}
	if len(s.Examples) > maxTraceSelection {
		return fmt.Errorf("selection exceeds %d examples", maxTraceSelection)
	}
	seen := map[string]bool{}
	for _, ex := range s.Examples {
		for _, id := range []string{ex.RunID, ex.TraceID} {
			if _, err := uuid.Parse(id); err != nil {
				return fmt.Errorf("invalid source run/trace UUID")
			}
		}
		if seen[ex.RunID] {
			return fmt.Errorf("duplicate run ID in selection: %s", ex.RunID)
		}
		seen[ex.RunID] = true
		if ex.Inputs == nil {
			return fmt.Errorf("run %s has no inputs object", ex.RunID)
		}
		if s.ReferenceMode == "inputs-only" && ex.Outputs != nil {
			return fmt.Errorf("inputs-only selection contains outputs; choose corrected explicitly for edited references")
		}
		if s.ReferenceMode != "inputs-only" && ex.Outputs == nil {
			return fmt.Errorf("reference outputs missing for run %s", ex.RunID)
		}
	}
	return nil
}

func selectionIdentity(s traceSelection, ex traceExample) (string, string) {
	// Content-derived IDs make repeats safe without overwriting corrected examples.
	b, _ := json.Marshal(struct {
		Version                                               int
		APIURL, Project, Dataset, Mode, InputPath, OutputPath string
		Example                                               traceExample
	}{s.Version, s.APIURL, s.ProjectID, s.DatasetID, s.ReferenceMode, s.InputsPointer, s.OutputsPointer, ex})
	hash := sha256.Sum256(b)
	digest := hex.EncodeToString(hash[:])
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("langsmith-cli:trace-example:"+digest)).String(), digest
}

func writeTraceSelection(s traceSelection, path string) error {
	if path == "" {
		return output.OutputJSON(s, "")
	}
	// A preview may contain sensitive inputs. Never overwrite an existing file.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(s)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func readTraceSelection(path string) (traceSelection, error) {
	var s traceSelection
	b, err := readJSONInput(path, 32*1024*1024)
	if err != nil {
		return s, err
	}
	if len(b) > 32*1024*1024 {
		return s, fmt.Errorf("selection exceeds 32 MiB")
	}
	err = decodeTraceSelection(b, &s)
	if err != nil {
		return s, err
	}
	return s, s.validate()
}

func newDatasetSelectionPreviewCmd() *cobra.Command {
	var ff FilterFlags
	var dataset, runIDs, traceID, mode, inputPath, outputPath, path string
	var assertionsFile string
	cmd := &cobra.Command{Use: "add", Short: "Preview trace examples and freeze a selection for import", Args: cobra.NoArgs,
		Long: "Read-only preview. Defaults to inputs-only: observed agent outputs are not trusted references.\nUse --run-ids for individual child steps, --trace-id for one root, or existing trace\nfilters for a bounded root sample. JSON pointers must select objects. Output files\ncontain trace data and are created privately without overwriting existing files.",
		RunE: func(cmd *cobra.Command, args []string) error {
			var assertions []datasetAssertion
			if cmd.Flags().Changed("assertions") {
				if mode != "inputs-only" || cmd.Flags().Changed("outputs-pointer") || (traceID == "" && runIDs == "") || strings.Contains(runIDs, ",") {
					return invalidAssertions("assertions require one explicit trace/run and cannot be combined with observed outputs or output pointers")
				}
				var err error
				assertions, err = readDatasetAssertions(assertionsFile)
				if err != nil {
					return err
				}
			}
			for _, flag := range []string{"trace-id", "run-ids", "trace-ids"} {
				if cmd.Flags().Changed(flag) {
					value, _ := cmd.Flags().GetString(flag)
					if strings.TrimSpace(value) == "" {
						return fmt.Errorf("--%s must not be blank", flag)
					}
				}
			}
			if ff.Limit == 0 {
				ff.Limit = 20
			}
			if ff.Limit < 1 || ff.Limit > maxTraceSelection {
				return fmt.Errorf("--limit must be between 1 and %d", maxTraceSelection)
			}
			if mode != "inputs-only" && mode != "observed" {
				return fmt.Errorf("--reference-mode must be inputs-only or observed; corrected references may be supplied in the reviewed selection")
			}
			if mode == "inputs-only" && outputPath != "" {
				return fmt.Errorf("--outputs-pointer requires observed reference mode")
			}
			if ff.ErrorFlag && ff.NoErrorFlag {
				return fmt.Errorf("--error and --no-error are mutually exclusive")
			}
			var ids []string
			if traceID != "" {
				ids = []string{traceID}
			}
			if runIDs != "" {
				ids = splitTrim(runIDs)
			}
			for _, id := range append(append([]string{}, ids...), splitTrim(ff.TraceIDs)...) {
				if _, err := uuid.Parse(id); err != nil {
					return fmt.Errorf("invalid source UUID %q", id)
				}
			}
			if len(ids) > ff.Limit {
				return fmt.Errorf("explicit ID count exceeds --limit")
			}
			if len(ids) > 0 {
				for _, flag := range []string{"trace-ids", "since", "before", "last-n-minutes", "filter", "metadata", "tags", "name", "error", "no-error", "min-latency", "max-latency", "min-tokens"} {
					if cmd.Flags().Changed(flag) {
						return fmt.Errorf("explicit IDs cannot be combined with --%s", flag)
					}
				}
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			project, err := resolveSessionID(ctx, c, ff.Project, ff.ProjectID, "dataset add")
			if err != nil {
				return err
			}
			ds, err := resolveDataset(ctx, c, dataset)
			if err != nil {
				return err
			}
			var params langsmith.RunQueryParams
			if len(ids) > 0 {
				params = langsmith.RunQueryParams{ID: langsmith.F(ids), Limit: langsmith.F(int64(len(ids)))}
				if traceID != "" {
					params.IsRoot = langsmith.F(true)
				}
			} else {
				params = BuildRunQueryParams(&ff, true, ff.Limit)
			}
			fields := []langsmith.RunQueryParamsSelect{"id", "trace_id", "start_time", "inputs", "total_tokens"}
			if mode == "observed" {
				fields = append(fields, "outputs")
			}
			params.Select = langsmith.F(fields)
			queryLimit := ff.Limit
			if len(ids) == 0 {
				queryLimit++ // Probe one extra eligible root to disclose truncation.
				params.Limit = langsmith.F(int64(min(queryLimit, 100)))
			}
			runs, err := queryRuns(ctx, c, params, project, queryLimit, ff.MinTokens)
			if err != nil {
				return err
			}
			if len(ids) > 0 && len(runs) != len(ids) {
				return fmt.Errorf("not all requested runs were found in the selected project; no selection created")
			}
			hasMore := len(runs) > ff.Limit
			if hasMore {
				runs = runs[:ff.Limit]
			}
			scope := "matching_roots"
			if len(ids) > 0 {
				scope = "explicit_ids"
			}
			s := traceSelection{Version: traceSelectionVersion, APIURL: c.APIURL(), ProjectID: project, DatasetID: ds.ID, ReferenceMode: mode, InputsPointer: inputPath, OutputsPointer: outputPath, Examples: []traceExample{}, WorkspaceID: resultWorkspaceID(), SelectionInfo: &traceSelectionInfo{Limit: ff.Limit, Selected: len(runs), HasMore: hasMore, Scope: scope}}
			if assertions != nil {
				if len(runs) != 1 {
					return invalidAssertions("assertions require exactly one matching run")
				}
				s.ReferenceMode = "corrected"
			}
			for _, run := range runs {
				io, err := preciseTraceIO(run.JSON.RawJSON())
				if err != nil {
					return err
				}
				inputs, err := objectAtPointer(io.Inputs, inputPath)
				if err != nil {
					return fmt.Errorf("run %s inputs: %w", run.ID, err)
				}
				ex := traceExample{RunID: run.ID, TraceID: run.TraceID, StartTime: run.StartTime, Inputs: inputs}
				if assertions != nil {
					ex.Outputs = map[string]any{"assertions": assertions}
				}
				if mode == "observed" {
					ex.Outputs, err = objectAtPointer(io.Outputs, outputPath)
					if err != nil {
						return fmt.Errorf("run %s outputs: %w", run.ID, err)
					}
				}
				s.Examples = append(s.Examples, ex)
			}
			sort.Slice(s.Examples, func(i, j int) bool { return s.Examples[i].RunID < s.Examples[j].RunID })
			if err := s.validate(); err != nil {
				return err
			}
			return writeTraceSelection(s, path)
		},
	}
	addCommonFilterFlags(cmd, &ff, false)
	cmd.Flags().StringVar(&dataset, "dataset", "", "Existing destination dataset name or UUID (required)")
	cmd.Flags().StringVar(&traceID, "trace-id", "", "Explicit root trace UUID (no default time window)")
	cmd.Flags().StringVar(&runIDs, "run-ids", "", "Comma-separated explicit run UUIDs, including child steps")
	cmd.MarkFlagsMutuallyExclusive("trace-id", "run-ids")
	cmd.Flags().StringVar(&mode, "reference-mode", "inputs-only", "inputs-only or observed (explicitly trust the selected outputs)")
	cmd.Flags().StringVar(&assertionsFile, "assertions", "", "Key/comment criteria array: inline JSON, file.json, or @file.json; one trace/run only; replaces reference output, does not evaluate")
	cmd.Flags().StringVar(&inputPath, "inputs-pointer", "", "JSON pointer within run inputs; default is the whole object")
	cmd.Flags().StringVar(&outputPath, "outputs-pointer", "", "JSON pointer within run outputs; default is the whole object")
	cmd.Flags().StringVarP(&path, "output", "o", "", "Save frozen JSON selection to a new private file")
	_ = cmd.MarkFlagRequired("dataset")
	return cmd
}

func newDatasetSelectionApplyCmd() *cobra.Command {
	var file, dataset, project, projectID string
	cmd := &cobra.Command{Use: "add", Short: "Import a reviewed preview selection, skipping identical prior imports", Args: cobra.NoArgs,
		Long: "Import the exact payload in a dataset add --dry-run selection, not a fresh filter query.\nReview inputs and reference mode first. To provide corrected answers, set the file's\nreference_mode to corrected and supply each example's outputs object. Repeating\nthe same selection skips identical examples; changed payloads create new examples.\nExisting examples are never overwritten. Partial failures return per-item results\nand a nonzero exit; retry the same file to recover without duplicating successes.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(file) == "" {
				return fmt.Errorf("provide a nonblank --selection")
			}
			s, err := readTraceSelection(file)
			if err != nil {
				return err
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if s.APIURL != c.APIURL() {
				return fmt.Errorf("selection API URL differs from the configured API URL")
			}
			if s.WorkspaceID != nil && *s.WorkspaceID != GetWorkspaceID() {
				return fmt.Errorf("selection workspace does not match the selected workspace; select the original workspace explicitly")
			}
			pid, err := resolveSessionID(ctx, c, project, projectID, "dataset add")
			if err != nil {
				return err
			}
			ds, err := resolveDataset(ctx, c, dataset)
			if err != nil {
				return err
			}
			if pid != s.ProjectID || ds.ID != s.DatasetID {
				return fmt.Errorf("selection project/dataset does not match explicit destination")
			}
			// Recheck source access without replacing the reviewed payload with new IO.
			if len(s.Examples) > 0 {
				ids := []string{}
				for _, ex := range s.Examples {
					ids = append(ids, ex.RunID)
				}
				runs, err := queryRuns(ctx, c, langsmith.RunQueryParams{ID: langsmith.F(ids), Select: langsmith.F([]langsmith.RunQueryParamsSelect{"id", "trace_id"}), Limit: langsmith.F(int64(100))}, pid, len(ids), 0)
				if err != nil {
					return err
				}
				found := map[string]string{}
				for _, r := range runs {
					found[r.ID] = r.TraceID
				}
				for _, ex := range s.Examples {
					if found[ex.RunID] != ex.TraceID {
						return fmt.Errorf("source run %s is missing or its trace does not match", ex.RunID)
					}
				}
			}
			results := []map[string]any{}
			failed := 0
			counts := map[string]int{"created": 0, "skipped": 0, "recovered": 0, "failed": 0}
			for _, ex := range s.Examples {
				id, digest := selectionIdentity(s, ex)
				row := map[string]any{"run_id": ex.RunID, "example_id": id, "status": "failed"}
				existing, getErr := c.SDK.Examples.Get(ctx, id, langsmith.ExampleGetParams{})
				matches := func(got *langsmith.Example) bool {
					if got == nil {
						return false
					}
					io, err := preciseTraceIO(got.JSON.RawJSON())
					return err == nil && got.DatasetID == ds.ID && equalTraceJSON(io.Inputs, ex.Inputs) && equalTraceJSON(io.Outputs, ex.Outputs) && got.Metadata["cli_trace_import_hash"] == digest
				}
				if getErr == nil {
					if matches(existing) {
						row["status"] = "skipped"
					} else {
						row["error"] = "existing deterministic ID has different content; not overwritten"
					}
				} else if !isHTTP404(getErr) {
					row["error"] = "existing example could not be read; no write attempted"
				} else {
					metadata := map[string]any{"cli_trace_import_hash": digest, "source_project_id": pid, "source_run_id": ex.RunID, "source_trace_id": ex.TraceID, "reference_mode": s.ReferenceMode, "inputs_pointer": s.InputsPointer, "outputs_pointer": s.OutputsPointer}
					params := exampleCreateParams(ds.ID, ex.Inputs, ex.Outputs, metadata, "")
					params.ID = langsmith.F(id)
					// IO is frozen explicitly; do not ask the server to copy source outputs.
					params.UseSourceRunIo = langsmith.F(false)
					params.SourceRunID = langsmith.F(ex.RunID)
					params.SourceTraceID = langsmith.F(ex.TraceID)
					params.SourceSessionID = langsmith.F(pid)
					if !ex.StartTime.IsZero() {
						params.SourceRunStartTime = langsmith.F(ex.StartTime)
					}
					_, createErr := c.SDK.Examples.New(ctx, params, option.WithMaxRetries(0))
					got, verifyErr := c.SDK.Examples.Get(ctx, id, langsmith.ExampleGetParams{})
					if verifyErr == nil && matches(got) {
						row["status"] = "created"
						if createErr != nil {
							row["status"] = "recovered"
						}
					} else if createErr != nil {
						row["error"] = "write could not be verified; retry the same selection to reconcile"
					} else {
						row["error"] = "creation not verified; retry this selection to reconcile"
					}
				}
				if row["status"] == "failed" {
					failed++
				}
				results = append(results, row)
				counts[row["status"].(string)]++
			}
			if err := output.OutputJSON(map[string]any{"workspace_id": resultWorkspaceID(), "project_id": pid, "dataset_id": ds.ID, "results": results, "failed": failed, "counts": counts, "total": len(results), "message": "Import processed. Review per-item results and reference answers before evaluation.", "warnings": []string{"Agent outputs are not verified reference answers. Retry uncertain items with the same reviewed selection, not new discovery."}, "next_steps": []string{readNextStep("example", "list", "--dataset", ds.ID)}}, ""); err != nil {
				return err
			}
			if failed > 0 {
				return fmt.Errorf("%d imports failed; retry the same selection after resolving errors", failed)
			}
			return nil
		},
	}
	addProjectFlags(cmd, &project, &projectID)
	cmd.Flags().StringVar(&file, "selection", "", "Reviewed dataset add dry-run payload: inline JSON, file.json, or @file.json")
	_ = cmd.MarkFlagRequired("selection")
	cmd.Flags().StringVar(&dataset, "dataset", "", "Destination dataset name or UUID; must match selection (required)")
	_ = cmd.MarkFlagRequired("dataset")
	return cmd
}
