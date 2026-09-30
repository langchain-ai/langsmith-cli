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

var issuesRetrieve = cli.Command{
	Name:    "retrieve",
	Usage:   "**Beta:** This endpoint is in active development and may change without notice.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "id",
			Required:  true,
			PathParam: "id",
		},
		&requestflag.Flag[bool]{
			Name:      "include-linear-context",
			Usage:     "Include current Linear workflow state and validated linked GitHub pull request URLs",
			QueryPath: "include_linear_context",
		},
	},
	Action:          handleIssuesRetrieve,
	HideHelpCommand: true,
}

var issuesList = cli.Command{
	Name:    "list",
	Usage:   "**Beta:** This endpoint is in active development and may change without notice.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[[]string]{
			Name:      "activity",
			Usage:     "Filter by Engine activity (repeatable; OR semantics)",
			QueryPath: "activity",
		},
		&requestflag.Flag[int64]{
			Name:      "limit",
			Usage:     "Page size (positive integer; defaults to 50, capped at 500)",
			QueryPath: "limit",
		},
		&requestflag.Flag[int64]{
			Name:      "offset",
			Usage:     "Page offset (non-negative integer; at most 100000)",
			QueryPath: "offset",
		},
		&requestflag.Flag[string]{
			Name:      "session-id",
			Usage:     "Filter by session ID (UUID)",
			QueryPath: "session_id",
		},
		&requestflag.Flag[string]{
			Name:      "session-name",
			Usage:     "Filter by session name (exact match)",
			QueryPath: "session_name",
		},
		&requestflag.Flag[int64]{
			Name:      "severity",
			Usage:     "Filter by severity",
			QueryPath: "severity",
		},
		&requestflag.Flag[[]int64]{
			Name:      "severity-exact",
			Usage:     "Filter by exact severity (repeatable; OR semantics)",
			QueryPath: "severity_exact",
		},
		&requestflag.Flag[string]{
			Name:      "sort-by",
			Usage:     "Sort field",
			QueryPath: "sort_by",
		},
		&requestflag.Flag[string]{
			Name:      "status",
			Usage:     "Filter by status",
			QueryPath: "status",
		},
		&requestflag.Flag[bool]{
			Name:      "status-first",
			Usage:     "Group results by issue lifecycle status before applying sort_by",
			QueryPath: "status_first",
		},
		&requestflag.Flag[string]{
			Name:      "tag",
			Usage:     "Filter by tag (exact match)",
			QueryPath: "tag",
		},
		&requestflag.Flag[string]{
			Name:      "trace-id",
			Usage:     "Return only issues with a linked run in this trace",
			QueryPath: "trace_id",
		},
		&requestflag.Flag[string]{
			Name:      "updated-at",
			Usage:     "Return only issues updated at or after this RFC3339 timestamp",
			QueryPath: "updated_at",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleIssuesList,
	HideHelpCommand: true,
}

func handleIssuesRetrieve(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.IssueGetParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Issues.Get(
		ctx,
		cmd.Value("id").(string),
		params,
		options...,
	)
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
		Title:          "issues retrieve",
		Transform:      transform,
	})
}

func handleIssuesList(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.IssueListParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Issues.List(ctx, params, options...)
		if err != nil {
			return err
		}
		obj := gjson.ParseBytes(res)
		return ShowJSON(obj, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "issues list",
			Transform:      transform,
		})
	} else {
		iter := client.Issues.ListAutoPaging(ctx, params, options...)
		maxItems := int64(-1)
		if cmd.IsSet("max-items") {
			maxItems = cmd.Value("max-items").(int64)
		}
		return ShowJSONIterator(iter, maxItems, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "issues list",
			Transform:      transform,
		})
	}
}
