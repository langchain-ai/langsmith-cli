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

var chartsPreview = requestflag.WithInnerFlags(cli.Command{
	Name:    "preview",
	Usage:   "Get a preview for a chart without actually creating it.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[map[string]any]{
			Name:     "bucket-info",
			Required: true,
			BodyPath: "bucket_info",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "chart",
			Required: true,
			BodyPath: "chart",
		},
	},
	Action:          handleChartsPreview,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"bucket-info": {
		&requestflag.InnerFlag[any]{
			Name:       "bucket-info.end-time",
			InnerField: "end_time",
		},
		&requestflag.InnerFlag[bool]{
			Name:       "bucket-info.omit-data",
			InnerField: "omit_data",
		},
		&requestflag.InnerFlag[any]{
			Name:       "bucket-info.start-time",
			InnerField: "start_time",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "bucket-info.stride",
			Usage:      "Timedelta input.",
			InnerField: "stride",
		},
		&requestflag.InnerFlag[string]{
			Name:       "bucket-info.timezone",
			InnerField: "timezone",
		},
	},
	"chart": {
		&requestflag.InnerFlag[[]map[string]any]{
			Name:       "chart.series",
			InnerField: "series",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "chart.common-filters",
			InnerField: "common_filters",
		},
	},
})

func handleChartsPreview(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.ChartPreviewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Charts.Preview(ctx, params, options...)
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
		Title:          "charts preview",
		Transform:      transform,
	})
}
