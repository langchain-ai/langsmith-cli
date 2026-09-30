// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package cmd

import (
	"context"
	"fmt"

	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/apiquery"
	"github.com/langchain-ai/langsmith-cli/internal/generated/internal/requestflag"
	"github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"github.com/tidwall/gjson"
	"github.com/urfave/cli/v3"
)

var sandboxesSnapshotsCreate = requestflag.WithInnerFlags(cli.Command{
	Name:    "create",
	Usage:   "Create a snapshot from a Docker image (async build). Names use lowercase\nregistry-style components separated by slashes, up to 255 characters. The\nsystem/ namespace is read-only.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "docker-image",
			Required: true,
			BodyPath: "docker_image",
		},
		&requestflag.Flag[int64]{
			Name:     "fs-capacity-bytes",
			Required: true,
			BodyPath: "fs_capacity_bytes",
		},
		&requestflag.Flag[string]{
			Name:     "name",
			Required: true,
			BodyPath: "name",
		},
		&requestflag.Flag[string]{
			Name:     "description",
			Usage:    "Description says what this snapshot's image can do, so a caller can hand it to an agent as a capability summary. At most 1024 characters.",
			BodyPath: "description",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "labels",
			Usage:    "Labels seed the snapshot's labels, overriding any label of the same key derived from the Docker image.",
			BodyPath: "labels",
		},
		&requestflag.Flag[string]{
			Name:     "registry-id",
			BodyPath: "registry_id",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "run-config",
			Usage:    "RunConfig overrides the runtime configuration taken from the Docker image.\nEvery sandbox created from the snapshot runs as the image's USER, in its\nWORKDIR, with its ENV beneath the sandbox's own env_vars; user and\nwork_dir given here replace the image's, and env_vars merge over it.",
			BodyPath: "run_config",
		},
		&requestflag.Flag[string]{
			Name:     "tag",
			Usage:    `mutable Docker-style tag; defaults to "latest"`,
			BodyPath: "tag",
		},
	},
	Action:          handleSandboxesSnapshotsCreate,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"run-config": {
		&requestflag.InnerFlag[map[string]any]{
			Name:       "run-config.env-vars",
			InnerField: "env_vars",
		},
		&requestflag.InnerFlag[string]{
			Name:       "run-config.user",
			InnerField: "user",
		},
		&requestflag.InnerFlag[string]{
			Name:       "run-config.work-dir",
			InnerField: "work_dir",
		},
	},
})

var sandboxesSnapshotsRetrieve = cli.Command{
	Name:    "retrieve",
	Usage:   "Get a sandbox snapshot by ID or a registry-style reference, including\nsystem/default:latest. URL-encode references containing slashes. A bare name\nmeans name:latest, falling back to the newest ready untagged snapshot of that\nname. To list the tags under a name, use\n/api/v2/sandboxes/snapshots-by-name/{name}.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "snapshot-id",
			Required:  true,
			PathParam: "snapshot_id",
		},
	},
	Action:          handleSandboxesSnapshotsRetrieve,
	HideHelpCommand: true,
}

var sandboxesSnapshotsList = cli.Command{
	Name:    "list",
	Usage:   "List workspace and published system snapshots, with optional filtering, sorting,\nand pagination. Page with page_size and cursor: replay the response's\nnext_cursor until it comes back null, which is the only signal that no pages\nremain. Cursors are opaque and only valid on this endpoint; do not parse or\nconstruct one.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "created-by",
			Usage:     "Filter by creator identity. Only 'me' is supported.",
			QueryPath: "created_by",
		},
		&requestflag.Flag[string]{
			Name:      "cursor",
			Usage:     "Opaque pagination cursor from a prior response's next_cursor",
			QueryPath: "cursor",
		},
		&requestflag.Flag[[]string]{
			Name:      "label",
			Usage:     "Filter by label. Repeatable; all must match. Use 'key' to match on key presence or 'key=value' for equality.",
			QueryPath: "label",
		},
		&requestflag.Flag[int64]{
			Name:      "limit",
			Usage:     "Deprecated: use page_size. Maximum number of results",
			Default:   50,
			QueryPath: "limit",
		},
		&requestflag.Flag[string]{
			Name:      "name-contains",
			Usage:     "Filter by name substring",
			QueryPath: "name_contains",
		},
		&requestflag.Flag[int64]{
			Name:      "offset",
			Usage:     "Deprecated: use cursor. Pagination offset",
			Default:   0,
			QueryPath: "offset",
		},
		&requestflag.Flag[int64]{
			Name:      "page-size",
			Usage:     "Number of results per page",
			Default:   20,
			QueryPath: "page_size",
		},
		&requestflag.Flag[string]{
			Name:      "sort-by",
			Usage:     "Sort column (name, status, created_at)",
			Default:   "created_at",
			QueryPath: "sort_by",
		},
		&requestflag.Flag[string]{
			Name:      "sort-direction",
			Usage:     "Deprecated: use sort_order. Sort direction (asc, desc)",
			Default:   "desc",
			QueryPath: "sort_direction",
		},
		&requestflag.Flag[string]{
			Name:      "sort-order",
			Usage:     "Sort direction (asc, desc)",
			Default:   "desc",
			QueryPath: "sort_order",
		},
		&requestflag.Flag[string]{
			Name:      "status",
			Usage:     "Filter by status (building, ready, failed, deleting)",
			QueryPath: "status",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleSandboxesSnapshotsList,
	HideHelpCommand: true,
}

var sandboxesSnapshotsDelete = cli.Command{
	Name:    "delete",
	Usage:   "Delete a snapshot by ID or by a Docker-style name[:tag] reference. The\nunderlying storage is reclaimed asynchronously.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "snapshot-id",
			Required:  true,
			PathParam: "snapshot_id",
		},
	},
	Action:          handleSandboxesSnapshotsDelete,
	HideHelpCommand: true,
}

var sandboxesSnapshotsRetrieveByName = cli.Command{
	Name:    "retrieve-by-name",
	Usage:   "Get a snapshot name and every tag under it, with the snapshot each tag resolves\nto. To fetch one snapshot, use /api/v2/sandboxes/snapshots/{snapshot_id}.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
	},
	Action:          handleSandboxesSnapshotsRetrieveByName,
	HideHelpCommand: true,
}

func handleSandboxesSnapshotsCreate(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()

	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatRepeat,
		ApplicationJSON,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.SandboxSnapshotNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sandboxes.Snapshots.New(ctx, params, options...)
	if err != nil {
		return err
	}

	obj := gjson.ParseBytes(res)
	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	return ShowJSON(obj, ShowJSONOpts{
		ExplicitFormat: explicitFormat,
		Format:         format,
		RawOutput:      cmd.Root().Bool("raw-output"),
		Title:          "sandboxes:snapshots create",
		Transform:      transform,
	})
}

func handleSandboxesSnapshotsRetrieve(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("snapshot-id") && len(unusedArgs) > 0 {
		cmd.Set("snapshot-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatRepeat,
		EmptyBody,
		false,
	)
	if err != nil {
		return err
	}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sandboxes.Snapshots.Get(ctx, cmd.Value("snapshot-id").(string), options...)
	if err != nil {
		return err
	}

	obj := gjson.ParseBytes(res)
	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	return ShowJSON(obj, ShowJSONOpts{
		ExplicitFormat: explicitFormat,
		Format:         format,
		RawOutput:      cmd.Root().Bool("raw-output"),
		Title:          "sandboxes:snapshots retrieve",
		Transform:      transform,
	})
}

func handleSandboxesSnapshotsList(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()

	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatRepeat,
		EmptyBody,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.SandboxSnapshotListParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Sandboxes.Snapshots.List(ctx, params, options...)
		if err != nil {
			return err
		}
		obj := gjson.ParseBytes(res)
		return ShowJSON(obj, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "sandboxes:snapshots list",
			Transform:      transform,
		})
	} else {
		iter := client.Sandboxes.Snapshots.ListAutoPaging(ctx, params, options...)
		maxItems := int64(-1)
		if cmd.IsSet("max-items") {
			maxItems = cmd.Value("max-items").(int64)
		}
		return ShowJSONIterator(iter, maxItems, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "sandboxes:snapshots list",
			Transform:      transform,
		})
	}
}

func handleSandboxesSnapshotsDelete(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("snapshot-id") && len(unusedArgs) > 0 {
		cmd.Set("snapshot-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatRepeat,
		EmptyBody,
		false,
	)
	if err != nil {
		return err
	}

	return client.Sandboxes.Snapshots.Delete(ctx, cmd.Value("snapshot-id").(string), options...)
}

func handleSandboxesSnapshotsRetrieveByName(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("name") && len(unusedArgs) > 0 {
		cmd.Set("name", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatRepeat,
		EmptyBody,
		false,
	)
	if err != nil {
		return err
	}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sandboxes.Snapshots.GetByName(ctx, cmd.Value("name").(string), options...)
	if err != nil {
		return err
	}

	obj := gjson.ParseBytes(res)
	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	return ShowJSON(obj, ShowJSONOpts{
		ExplicitFormat: explicitFormat,
		Format:         format,
		RawOutput:      cmd.Root().Bool("raw-output"),
		Title:          "sandboxes:snapshots retrieve-by-name",
		Transform:      transform,
	})
}
