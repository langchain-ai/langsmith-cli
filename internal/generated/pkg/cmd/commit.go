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

var commitsCreate = cli.Command{
	Name:    "create",
	Usage:   "Creates a new commit in a repository. Requires authentication and write access\nto the repository.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "owner",
			Required:  true,
			PathParam: "owner",
		},
		&requestflag.Flag[string]{
			Name:      "repo",
			Required:  true,
			PathParam: "repo",
		},
		&requestflag.Flag[string]{
			Name:     "description",
			BodyPath: "description",
		},
		&requestflag.Flag[any]{
			Name:     "manifest",
			BodyPath: "manifest",
		},
		&requestflag.Flag[string]{
			Name:     "parent-commit",
			BodyPath: "parent_commit",
		},
		&requestflag.Flag[any]{
			Name:     "skip-webhooks",
			Usage:    "SkipWebhooks allows skipping webhook notifications. Can be true (boolean) to skip all, or an array of webhook UUIDs to skip specific ones.",
			BodyPath: "skip_webhooks",
		},
	},
	Action:          handleCommitsCreate,
	HideHelpCommand: true,
}

var commitsRetrieve = cli.Command{
	Name:    "retrieve",
	Usage:   "Retrieves a specific commit by hash, tag, or \"latest\" for a repository. This\nendpoint supports both authenticated and unauthenticated access. Authenticated\nusers can access private repos, while unauthenticated users can only access\npublic repos. Commit resolution logic:",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "owner",
			Required:  true,
			PathParam: "owner",
		},
		&requestflag.Flag[string]{
			Name:      "repo",
			Required:  true,
			PathParam: "repo",
		},
		&requestflag.Flag[string]{
			Name:      "commit",
			Required:  true,
			PathParam: "commit",
		},
		&requestflag.Flag[bool]{
			Name:      "get-examples",
			Default:   false,
			QueryPath: "get_examples",
		},
		&requestflag.Flag[string]{
			Name:      "include",
			Usage:     `Comma-separated list of optional fields: "model", "is_draft"`,
			QueryPath: "include",
		},
		&requestflag.Flag[bool]{
			Name:      "include-model",
			Usage:     "Deprecated: use Include instead",
			Default:   false,
			QueryPath: "include_model",
		},
		&requestflag.Flag[bool]{
			Name:      "is-view",
			Default:   false,
			QueryPath: "is_view",
		},
	},
	Action:          handleCommitsRetrieve,
	HideHelpCommand: true,
}

var commitsList = cli.Command{
	Name:    "list",
	Usage:   "List commits for a repository, with pagination support. This endpoint supports\nboth authenticated and unauthenticated access. Authenticated users can access\nprivate repositories; unauthenticated users can only access public repositories.\nThe include_stats parameter controls whether download and view statistics are\ncomputed (defaults to true).",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "owner",
			Required:  true,
			PathParam: "owner",
		},
		&requestflag.Flag[string]{
			Name:      "repo",
			Required:  true,
			PathParam: "repo",
		},
		&requestflag.Flag[bool]{
			Name:      "include-stats",
			Usage:     "IncludeStats determines whether to compute num_downloads and num_views",
			Default:   true,
			QueryPath: "include_stats",
		},
		&requestflag.Flag[int64]{
			Name:      "limit",
			Usage:     "Limit is the pagination limit",
			Default:   20,
			QueryPath: "limit",
		},
		&requestflag.Flag[int64]{
			Name:      "offset",
			Usage:     "Offset is the pagination offset",
			Default:   0,
			QueryPath: "offset",
		},
		&requestflag.Flag[string]{
			Name:      "tag",
			Usage:     `Tag filters commits to only those with a specific tag (e.g. "production", "staging")`,
			QueryPath: "tag",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleCommitsList,
	HideHelpCommand: true,
}

func handleCommitsCreate(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("owner") && len(unusedArgs) > 0 {
		cmd.Set("owner", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if !cmd.IsSet("repo") && len(unusedArgs) > 0 {
		cmd.Set("repo", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
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

	params := langsmith.CommitNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Commits.New(
		ctx,
		cmd.Value("owner").(string),
		cmd.Value("repo").(string),
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
		Title:          "commits create",
		Transform:      transform,
	})
}

func handleCommitsRetrieve(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("owner") && len(unusedArgs) > 0 {
		cmd.Set("owner", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if !cmd.IsSet("repo") && len(unusedArgs) > 0 {
		cmd.Set("repo", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if !cmd.IsSet("commit") && len(unusedArgs) > 0 {
		cmd.Set("commit", unusedArgs[0])
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

	params := langsmith.CommitGetParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Commits.Get(
		ctx,
		cmd.Value("owner").(string),
		cmd.Value("repo").(string),
		cmd.Value("commit").(string),
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
		Title:          "commits retrieve",
		Transform:      transform,
	})
}

func handleCommitsList(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("owner") && len(unusedArgs) > 0 {
		cmd.Set("owner", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if !cmd.IsSet("repo") && len(unusedArgs) > 0 {
		cmd.Set("repo", unusedArgs[0])
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

	params := langsmith.CommitListParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Commits.List(
			ctx,
			cmd.Value("owner").(string),
			cmd.Value("repo").(string),
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
			Title:          "commits list",
			Transform:      transform,
		})
	} else {
		iter := client.Commits.ListAutoPaging(
			ctx,
			cmd.Value("owner").(string),
			cmd.Value("repo").(string),
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
			Title:          "commits list",
			Transform:      transform,
		})
	}
}
