package cmd

import (
	"encoding/json"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"github.com/spf13/cobra"
)

type exampleEdit struct {
	ID                 string         `json:"id"`
	ExpectedModifiedAt time.Time      `json:"expected_modified_at"`
	Inputs             map[string]any `json:"inputs,omitempty"`
	Outputs            map[string]any `json:"outputs,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
	Splits             *[]string      `json:"splits,omitempty"`
}

// Preserve explicit empty objects in previews; nil means the field is unchanged.
func (e exampleEdit) MarshalJSON() ([]byte, error) {
	fields := map[string]any{"id": e.ID, "expected_modified_at": e.ExpectedModifiedAt}
	if e.Inputs != nil {
		fields["inputs"] = e.Inputs
	}
	if e.Outputs != nil {
		fields["outputs"] = e.Outputs
	}
	if e.Metadata != nil {
		fields["metadata"] = e.Metadata
	}
	if e.Splits != nil {
		fields["splits"] = e.Splits
	}
	return json.Marshal(fields)
}

func (e exampleEdit) params() langsmith.ExampleUpdateParams {
	p := langsmith.ExampleUpdateParams{}
	if e.Inputs != nil {
		p.Inputs = langsmith.F(e.Inputs)
	}
	if e.Outputs != nil {
		p.Outputs = langsmith.F(e.Outputs)
	}
	if e.Metadata != nil {
		p.Metadata = langsmith.F(e.Metadata)
	}
	if e.Splits != nil {
		p.Split = langsmith.F[langsmith.ExampleUpdateParamsSplitUnion](langsmith.ExampleUpdateParamsSplitArray(*e.Splits))
	}
	return p
}

func newExampleUpdateBulkCmd() *cobra.Command {
	var dataset, file string
	var apply, dryRun bool
	cmd := &cobra.Command{Use: "update-bulk", Short: "Preview or apply a file of up to 100 reviewed example edits", Args: cobra.NoArgs}
	cmd.Long = "Update explicit example IDs in one dataset. Requires --dry-run or --apply. The JSON array must include id and expected_modified_at for each item. Read modified_at from example list. All memberships and timestamps are checked before writes; updates are sequential, not atomic, and cannot prevent concurrent edits after preflight. No automatic retries. On partial failure, inspect every item and refresh timestamps before preparing another file. Omitted fields are unchanged; {} is an explicit empty object, and splits: [] clears memberships."
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if apply == dryRun {
			return datasetInputError("choose exactly one of --dry-run or --apply")
		}
		var edits []exampleEdit
		if err := readDatasetEditFile(file, &edits); err != nil {
			return err
		}
		if len(edits) < 1 || len(edits) > 100 {
			return datasetInputError("edit file must contain 1–100 examples")
		}
		seen := map[string]bool{}
		for _, edit := range edits {
			if err := resourceUUID(edit.ID); err != nil {
				return err
			}
			// UUID spellings must not permit the same resource to be edited twice.
			for id := range seen {
				if sameResourceID(id, edit.ID) {
					return datasetInputError("edit file contains duplicate example IDs")
				}
			}
			seen[edit.ID] = true
			if edit.ExpectedModifiedAt.IsZero() {
				return datasetInputError("every edit requires expected_modified_at from example list")
			}
			if edit.Inputs == nil && edit.Outputs == nil && edit.Metadata == nil && edit.Splits == nil {
				return datasetInputError("each edit must set inputs, outputs, metadata, or splits")
			}
			if edit.Splits != nil {
				if err := validateExampleSplits(*edit.Splits); err != nil {
					return err
				}
			}
		}
		c, err := getClient()
		if err != nil {
			return err
		}
		ds, err := resolveDataset(cmd.Context(), c, dataset)
		if err != nil {
			return err
		}
		for _, edit := range edits {
			ex, err := c.SDK.Examples.Get(cmd.Context(), edit.ID, langsmith.ExampleGetParams{})
			if err != nil {
				return err
			}
			if ex == nil || !sameResourceID(ex.DatasetID, ds.ID) {
				return datasetInputError("an example does not belong to the selected dataset; no updates were sent")
			}
			if !ex.ModifiedAt.Equal(edit.ExpectedModifiedAt) {
				return datasetInputError("an example changed since review; refresh the edit file; no updates were sent")
			}
		}
		if dryRun {
			return output.OutputJSON(map[string]any{"status": "dry_run", "dataset_id": ds.ID, "workspace_id": resultWorkspaceID(), "edits": edits, "total": len(edits), "next_steps": []string{"No examples were changed. Review replacement fields and concurrency timestamps; rerun with the same file and --apply instead of --dry-run. Writes are sequential, not atomic."}}, "")
		}
		results := make([]map[string]any, 0, len(edits))
		failed := 0
		for _, edit := range edits {
			_, err := c.SDK.Examples.Update(cmd.Context(), edit.ID, edit.params(), option.WithMaxRetries(0))
			row := map[string]any{"example_id": edit.ID, "status": "updated"}
			row["verification"] = "acknowledged_not_read_back"
			if err != nil {
				failed++
				row["status"] = "unverified"
				row["verification"] = "unverified"
				row["next_steps"] = "Read this example before retrying; the write may have applied."
			}
			results = append(results, row)
		}
		if err := output.OutputJSON(map[string]any{"dataset_id": ds.ID, "workspace_id": resultWorkspaceID(), "results": results, "total": len(edits), "updated": len(edits) - failed, "unverified": failed, "message": "Example batch processed. Existing experiments were not rerun.", "warnings": []string{"Writes are not atomic. Read unverified examples before retrying; updates may already have applied."}, "next_steps": []string{readNextStep("example", "list", "--dataset", ds.ID)}}, ""); err != nil {
			return err
		}
		if failed > 0 {
			return datasetWriteError()
		}
		return nil
	}
	cmd.Flags().StringVar(&dataset, "dataset", "", "Dataset name or UUID (required)")
	cmd.Flags().StringVar(&file, "file", "", "Reviewed edits array: inline JSON, file.json, or @file.json (required; at most 8 MiB)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate all edits and current dataset membership without writes")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply this exact file after validating memberships and timestamps")
	_ = cmd.MarkFlagRequired("dataset")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}
