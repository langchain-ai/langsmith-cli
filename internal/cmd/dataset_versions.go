package cmd

import (
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"github.com/langchain-ai/langsmith-go/shared"
	"github.com/spf13/cobra"
)

func newDatasetVersionCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "version", Short: "Inspect dataset history, compare versions, and tag snapshots"}
	for _, action := range []string{"list", "get", "diff", "tag"} {
		cmd.AddCommand(newDatasetVersionAction(action))
	}
	return cmd
}

func newDatasetVersionAction(action string) *cobra.Command {
	var dataset, asOf, from, to, tag string
	var limit, offset int64
	var dryRun bool
	descriptions := map[string]string{
		"list": "List a bounded page of dataset versions",
		"get":  "Resolve a timestamp or tag to a dataset version",
		"diff": "List added, modified, and removed example IDs between two versions",
		"tag":  "Create or move a version tag to an explicit timestamp or existing tag",
	}
	cmd := &cobra.Command{Use: action, Short: descriptions[action], Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if action == "list" && (limit < 1 || limit > 100 || offset < 0 || offset > (1<<63-1)-limit) {
			return datasetInputError("--limit must be 1–100; --offset must be nonnegative and leave room for the next page")
		}
		if action == "diff" && (strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "") {
			return datasetInputError("provide --from and --to timestamps or tags")
		}
		if action == "tag" && (strings.TrimSpace(tag) == "" || strings.TrimSpace(asOf) == "") {
			return datasetInputError("provide a nonblank --tag and explicit --as-of timestamp or tag")
		}
		c, err := getClient()
		if err != nil {
			return err
		}
		ds, err := resolveDataset(cmd.Context(), c, dataset)
		if err != nil {
			return err
		}
		result := map[string]any{"dataset_id": ds.ID, "workspace_id": resultWorkspaceID()}
		switch action {
		case "list":
			page, err := c.SDK.Datasets.Versions.List(cmd.Context(), ds.ID, langsmith.DatasetVersionListParams{Limit: langsmith.F(limit), Offset: langsmith.F(offset)})
			if err != nil {
				return err
			}
			result["items"], result["pagination"] = page.Items, describeOffsetPage(len(page.Items), limit, offset)
			result["limit"], result["offset"] = limit, offset
			emptyResultGuidance(result, len(page.Items), "No dataset versions returned on this page.", "Confirm the dataset and retry with --offset 0. Use dataset version get --dataset with the same dataset to resolve latest.")
		case "get", "tag":
			p := langsmith.DatasetGetVersionParams{}
			var opts []option.RequestOption
			if asOf == "" {
				asOf = "latest"
			}
			if asOf != "" {
				if timestamp, err := time.Parse(time.RFC3339Nano, asOf); err == nil {
					// The generated time query serializer drops subsecond precision.
					opts = append(opts, option.WithQuery("as_of", timestamp.Format(time.RFC3339Nano)))
				} else {
					p.Tag = langsmith.F(asOf)
				}
			}
			version, err := c.SDK.Datasets.GetVersion(cmd.Context(), ds.ID, p, opts...)
			if err != nil {
				return err
			}
			if version == nil || version.AsOf.IsZero() {
				return datasetInputError("service did not return a resolved dataset version")
			}
			if action == "tag" {
				result["tag"], result["as_of"] = tag, version.AsOf
				if dryRun {
					result["status"] = "dry_run"
					result["next_steps"] = []string{"No tag was changed. Review the resolved timestamp; remove --dry-run only after approving creation or movement of this tag."}
				} else {
					resolvedAt := version.AsOf
					_, err = c.SDK.Datasets.UpdateTags(cmd.Context(), ds.ID, langsmith.DatasetUpdateTagsParams{Tag: langsmith.F(tag), AsOf: langsmith.F[langsmith.DatasetUpdateTagsParamsAsOfUnion](shared.UnionString(resolvedAt.Format(time.RFC3339Nano)))}, option.WithMaxRetries(0))
					if err != nil {
						return datasetWriteError()
					}
					version, err = c.SDK.Datasets.GetVersion(cmd.Context(), ds.ID, langsmith.DatasetGetVersionParams{Tag: langsmith.F(tag)})
					if err != nil || version == nil || !version.AsOf.Equal(resolvedAt) {
						return datasetWriteError()
					}
					result["status"] = "updated"
				}
			}
			result["version"] = version
		case "diff":
			diff, err := c.SDK.Datasets.Versions.GetDiff(cmd.Context(), ds.ID, langsmith.DatasetVersionGetDiffParams{
				FromVersion: langsmith.F[langsmith.DatasetVersionGetDiffParamsFromVersionUnion](shared.UnionString(from)),
				ToVersion:   langsmith.F[langsmith.DatasetVersionGetDiffParamsToVersionUnion](shared.UnionString(to)),
			})
			if err != nil {
				return err
			}
			result["from"], result["to"], result["diff"] = from, to, diff
		}
		return output.OutputJSON(result, "")
	}
	cmd.Flags().StringVar(&dataset, "dataset", "", "Dataset name or UUID (required)")
	_ = cmd.MarkFlagRequired("dataset")
	switch action {
	case "list":
		cmd.Flags().Int64Var(&limit, "limit", 20, "Page size (1–100)")
		cmd.Flags().Int64Var(&offset, "offset", 0, "Pagination offset")
	case "get", "tag":
		cmd.Flags().StringVar(&asOf, "as-of", "", "Version timestamp or tag (get defaults to latest)")
		if action == "tag" {
			cmd.Flags().StringVar(&tag, "tag", "", "Tag to create or move")
			cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Resolve the version and preview without moving the tag")
		}
	case "diff":
		cmd.Flags().StringVar(&from, "from", "", "Starting timestamp or tag")
		cmd.Flags().StringVar(&to, "to", "", "Ending timestamp or tag")
	}
	return cmd
}

func datasetWriteError() error {
	return commandDiagnostic{"dataset_write_unverified", "dataset write did not complete with a verified response", "Read the affected dataset, version tag, or example before retrying. No automatic write retry was performed."}
}

func newDatasetSplitCmd() *cobra.Command {
	var dataset, asOf string
	cmd := &cobra.Command{Use: "split", Short: "Inspect named dataset subsets"}
	list := &cobra.Command{Use: "list", Short: "List split names at a timestamp or tag", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getClient()
		if err != nil {
			return err
		}
		ds, err := resolveDataset(cmd.Context(), c, dataset)
		if err != nil {
			return err
		}
		p := langsmith.DatasetSplitGetParams{}
		if asOf != "" {
			p.AsOf = langsmith.F[langsmith.DatasetSplitGetParamsAsOfUnion](shared.UnionString(asOf))
		}
		splits, err := c.SDK.Datasets.Splits.Get(cmd.Context(), ds.ID, p)
		if err != nil {
			return err
		}
		version := asOf
		if version == "" {
			version = "latest"
		}
		result := map[string]any{"dataset_id": ds.ID, "workspace_id": resultWorkspaceID(), "splits": splits, "as_of": version}
		if splits != nil {
			emptyResultGuidance(result, len(*splits), "No named splits returned at this dataset version.", "Check --as-of and the dataset. Assign split memberships with example update --split or example update-bulk; existing memberships are replaced, so include any you want to preserve.")
		}
		return output.OutputJSON(result, "")
	}}
	list.Flags().StringVar(&dataset, "dataset", "", "Dataset name or UUID (required)")
	list.Flags().StringVar(&asOf, "as-of", "", "Version timestamp or tag (default: latest)")
	_ = list.MarkFlagRequired("dataset")
	cmd.AddCommand(list)
	return cmd
}
