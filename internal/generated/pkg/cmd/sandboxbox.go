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

var sandboxesBoxesDeleteServiceURL = cli.Command{
	Name:    "delete-service-url",
	Usage:   "Removes the sharing grant for one port, or for every port when port is omitted.\nA LangSmith login URL stops working immediately. A previously minted service\ntoken is not revoked and stays valid until it expires, but no new one can be\nissued from the removed grant.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
		&requestflag.Flag[int64]{
			Name:      "port",
			Usage:     "Port to stop sharing. Omit to stop sharing every port.",
			QueryPath: "port",
		},
	},
	Action:          handleSandboxesBoxesDeleteServiceURL,
	HideHelpCommand: true,
}

var sandboxesBoxesListServiceURLs = cli.Command{
	Name:    "list-service-urls",
	Usage:   "Returns one entry per port the sandbox is currently reachable on, so a caller\ncan see what is shared before turning it off. Expired token grants are omitted.\nCursors are opaque and only valid on this endpoint; do not parse or construct\none.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
		&requestflag.Flag[string]{
			Name:      "cursor",
			Usage:     "Opaque pagination cursor from a prior response's next_cursor",
			QueryPath: "cursor",
		},
		&requestflag.Flag[int64]{
			Name:      "page-size",
			Usage:     "Number of results per page",
			Default:   20,
			QueryPath: "page_size",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleSandboxesBoxesListServiceURLs,
	HideHelpCommand: true,
}

func handleSandboxesBoxesDeleteServiceURL(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.SandboxBoxDeleteServiceURLParams{}

	return client.Sandboxes.Boxes.DeleteServiceURL(
		ctx,
		cmd.Value("name").(string),
		params,
		options...,
	)
}

func handleSandboxesBoxesListServiceURLs(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.SandboxBoxListServiceURLsParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Sandboxes.Boxes.ListServiceURLs(
			ctx,
			cmd.Value("name").(string),
			params,
			options...,
		)
		if err != nil {
			return err
		}
		obj := gjson.ParseBytes(res)
		return ShowJSON(obj, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "sandboxes:boxes list-service-urls",
			Transform:      transform,
		})
	} else {
		iter := client.Sandboxes.Boxes.ListServiceURLsAutoPaging(
			ctx,
			cmd.Value("name").(string),
			params,
			options...,
		)
		maxItems := int64(-1)
		if cmd.IsSet("max-items") {
			maxItems = cmd.Value("max-items").(int64)
		}
		return ShowJSONIterator(iter, maxItems, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "sandboxes:boxes list-service-urls",
			Transform:      transform,
		})
	}
}
