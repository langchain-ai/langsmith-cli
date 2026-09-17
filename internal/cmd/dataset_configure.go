package cmd

import (
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"github.com/spf13/cobra"
)

type datasetConfiguration struct {
	Inputs          map[string]any           `json:"inputs_schema_definition,omitempty"`
	Outputs         map[string]any           `json:"outputs_schema_definition,omitempty"`
	Transformations *[]datasetTransformation `json:"transformations,omitempty"`
}

type datasetTransformation struct {
	Path []string                                          `json:"path"`
	Type langsmith.DatasetTransformationTransformationType `json:"transformation_type"`
}

func newDatasetConfigureCmd() *cobra.Command {
	var dataset, file string
	var dryRun, apply bool
	cmd := &cobra.Command{Use: "configure", Short: "Preview or update dataset schemas and transformations from JSON", Args: cobra.NoArgs}
	cmd.Long = "Read current configuration with dataset get. Supply a JSON object containing inputs_schema_definition, outputs_schema_definition, and/or transformations. Omitted fields are unchanged; use {} for an unrestricted schema or [] to clear transformations. Requires --dry-run or --apply. Dry-run validates the file and reads the dataset; server-side schema validation and authorization are only checked on apply."
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if apply == dryRun {
			return datasetInputError("choose exactly one of --dry-run or --apply")
		}
		var config datasetConfiguration
		if err := readDatasetEditFile(file, &config); err != nil {
			return err
		}
		if config.Inputs == nil && config.Outputs == nil && config.Transformations == nil {
			return datasetInputError("provide a schema or transformations in the configuration file")
		}
		p := langsmith.DatasetUpdateParams{}
		if config.Inputs != nil {
			p.InputsSchemaDefinition = langsmith.F[langsmith.DatasetUpdateParamsInputsSchemaDefinitionUnion](langsmith.DatasetUpdateParamsInputsSchemaDefinitionMap(config.Inputs))
		}
		if config.Outputs != nil {
			p.OutputsSchemaDefinition = langsmith.F[langsmith.DatasetUpdateParamsOutputsSchemaDefinitionUnion](langsmith.DatasetUpdateParamsOutputsSchemaDefinitionMap(config.Outputs))
		}
		if config.Transformations != nil {
			items := langsmith.DatasetUpdateParamsTransformationsArray{}
			for _, item := range *config.Transformations {
				if item.Path == nil || !item.Type.IsKnown() {
					return datasetInputError("each transformation requires a path array and a supported transformation_type")
				}
				items = append(items, langsmith.DatasetTransformationParam{Path: langsmith.F(item.Path), TransformationType: langsmith.F(item.Type)})
			}
			p.Transformations = langsmith.F[langsmith.DatasetUpdateParamsTransformationsUnion](items)
		}
		c, err := getClient()
		if err != nil {
			return err
		}
		ds, err := resolveDataset(cmd.Context(), c, dataset)
		if err != nil {
			return err
		}
		if dryRun {
			return output.OutputJSON(map[string]any{"status": "dry_run", "dataset_id": ds.ID, "workspace_id": resultWorkspaceID(), "request": p, "next_steps": []string{"No configuration was changed. Review the request, then rerun with the same file and --apply instead of --dry-run. Server acceptance is not validated by this preview."}}, "")
		}
		result, err := c.SDK.Datasets.Update(cmd.Context(), ds.ID, p, option.WithMaxRetries(0))
		if err != nil {
			return datasetWriteError()
		}
		return output.OutputJSON(map[string]any{"status": "updated", "dataset_id": ds.ID, "workspace_id": resultWorkspaceID(), "result": result, "verification": "acknowledged_not_read_back", "next_steps": []string{readNextStep("dataset", "get", ds.ID)}}, "")
	}
	cmd.Flags().StringVar(&dataset, "dataset", "", "Dataset name or UUID (required)")
	cmd.Flags().StringVar(&file, "file", "", "Configuration: inline JSON, file.json, or @file.json (required; at most 8 MiB)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate and preview without updating configuration")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the supplied configuration")
	_ = cmd.MarkFlagRequired("dataset")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}
