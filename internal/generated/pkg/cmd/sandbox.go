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

var sandboxesListUsageCosts = cli.Command{
	Name:    "list-usage-costs",
	Usage:   "Returns priced usage per sandbox or snapshot and UTC hour in the half-open\nrequested interval. LCU uses the recorded compute amount for sandboxes;\nsnapshots have zero LCU. LSU allocates the recorded workspace storage amount\nproportionally to attributed bytes, including checkpoints on their sandbox and\nsnapshots as separate resources. Resource filters preserve each resource's\nshare. Rate changes do not reprice recorded amounts. An access-filtered page can\nhave no items and a non-null next_cursor; continue until next_cursor is null.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[any]{
			Name:      "end-time",
			Usage:     "Exclusive RFC3339 end time; the range must not exceed 31 days",
			Required:  true,
			QueryPath: "end_time",
		},
		&requestflag.Flag[any]{
			Name:      "start-time",
			Usage:     "Inclusive RFC3339 start time",
			Required:  true,
			QueryPath: "start_time",
		},
		&requestflag.Flag[string]{
			Name:      "cursor",
			Usage:     "Opaque pagination cursor",
			QueryPath: "cursor",
		},
		&requestflag.Flag[string]{
			Name:      "granularity",
			Usage:     "HOUR returns hourly buckets. RESOURCE sums each resource over the requested interval and sets period_start to start_time.",
			Default:   "HOUR",
			QueryPath: "granularity",
		},
		&requestflag.Flag[int64]{
			Name:      "page-size",
			Usage:     "Maximum rows to return",
			Default:   20,
			QueryPath: "page_size",
		},
		&requestflag.Flag[[]string]{
			Name:      "resource-id",
			Usage:     "Resource UUID filter; repeat this parameter up to 100 times",
			QueryPath: "resource_ids",
		},
		&requestflag.Flag[string]{
			Name:      "resource-type",
			Usage:     "Resource type filter",
			QueryPath: "resource_type",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleSandboxesListUsageCosts,
	HideHelpCommand: true,
}

func handleSandboxesListUsageCosts(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.SandboxListUsageCostsParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Sandboxes.ListUsageCosts(ctx, params, options...)
		if err != nil {
			return err
		}
		obj := gjson.ParseBytes(res)
		return ShowJSON(obj, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "sandboxes list-usage-costs",
			Transform:      transform,
		})
	} else {
		iter := client.Sandboxes.ListUsageCostsAutoPaging(ctx, params, options...)
		maxItems := int64(-1)
		if cmd.IsSet("max-items") {
			maxItems = cmd.Value("max-items").(int64)
		}
		return ShowJSONIterator(iter, maxItems, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "sandboxes list-usage-costs",
			Transform:      transform,
		})
	}
}
