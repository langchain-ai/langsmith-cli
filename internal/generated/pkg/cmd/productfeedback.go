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

var productFeedbackCreate = requestflag.WithInnerFlags(cli.Command{
	Name:    "create",
	Usage:   "**Alpha:** This endpoint is in active development and may change without notice.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "category",
			Usage:    `Allowed values: "BUG", "FEATURE_REQUEST", "USABILITY", "DOCUMENTATION", "OTHER".`,
			Required: true,
			BodyPath: "category",
		},
		&requestflag.Flag[string]{
			Name:     "message",
			Required: true,
			BodyPath: "message",
		},
		&requestflag.Flag[string]{
			Name:     "source",
			Usage:    `Allowed values: "LANGSMITH_CLI".`,
			Required: true,
			BodyPath: "source",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "client",
			BodyPath: "client",
		},
		&requestflag.Flag[string]{
			Name:       "idempotency-key",
			HeaderPath: "Idempotency-Key",
		},
	},
	Action:          handleProductFeedbackCreate,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"client": {
		&requestflag.InnerFlag[string]{
			Name:       "client.architecture",
			InnerField: "architecture",
		},
		&requestflag.InnerFlag[string]{
			Name:       "client.os",
			InnerField: "os",
		},
		&requestflag.InnerFlag[string]{
			Name:       "client.version",
			InnerField: "version",
		},
	},
})

var productFeedbackRetrieve = cli.Command{
	Name:    "retrieve",
	Usage:   "**Alpha:** This endpoint is in active development and may change without notice.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "id",
			Required:  true,
			PathParam: "id",
		},
	},
	Action:          handleProductFeedbackRetrieve,
	HideHelpCommand: true,
}

func handleProductFeedbackCreate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.ProductFeedbackNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.ProductFeedback.New(ctx, params, options...)
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
		Title:          "product-feedback create",
		Transform:      transform,
	})
}

func handleProductFeedbackRetrieve(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("id") && len(unusedArgs) > 0 {
		cmd.Set("id", unusedArgs[0])
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
	_, err = client.ProductFeedback.Get(ctx, cmd.Value("id").(string), options...)
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
		Title:          "product-feedback retrieve",
		Transform:      transform,
	})
}
