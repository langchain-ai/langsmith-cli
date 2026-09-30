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

var reposCreate = cli.Command{
	Name:    "create",
	Usage:   "Create a repo.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[bool]{
			Name:     "is-public",
			Required: true,
			BodyPath: "is_public",
		},
		&requestflag.Flag[string]{
			Name:     "repo-handle",
			Required: true,
			BodyPath: "repo_handle",
		},
		&requestflag.Flag[*string]{
			Name:     "description",
			BodyPath: "description",
		},
		&requestflag.Flag[*string]{
			Name:     "readme",
			BodyPath: "readme",
		},
		&requestflag.Flag[string]{
			Name:     "repo-type",
			Usage:    `Allowed values: "prompt", "file", "agent", "skill".`,
			Default:  "prompt",
			BodyPath: "repo_type",
		},
		&requestflag.Flag[*bool]{
			Name:     "restricted-mode",
			BodyPath: "restricted_mode",
		},
		&requestflag.Flag[*string]{
			Name:     "source",
			Usage:    `Allowed values: "internal", "external".`,
			BodyPath: "source",
		},
		&requestflag.Flag[any]{
			Name:     "tag-value-id",
			BodyPath: "tag_value_ids",
		},
		&requestflag.Flag[any]{
			Name:     "tag",
			BodyPath: "tags",
		},
	},
	Action:          handleReposCreate,
	HideHelpCommand: true,
}

var reposRetrieve = cli.Command{
	Name:    "retrieve",
	Usage:   "Get a repo.",
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
	},
	Action:          handleReposRetrieve,
	HideHelpCommand: true,
}

var reposUpdate = cli.Command{
	Name:    "update",
	Usage:   "Update a repo.",
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
		&requestflag.Flag[*string]{
			Name:     "description",
			BodyPath: "description",
		},
		&requestflag.Flag[*bool]{
			Name:     "is-archived",
			BodyPath: "is_archived",
		},
		&requestflag.Flag[*bool]{
			Name:     "is-public",
			BodyPath: "is_public",
		},
		&requestflag.Flag[*string]{
			Name:     "readme",
			BodyPath: "readme",
		},
		&requestflag.Flag[*bool]{
			Name:     "restricted-mode",
			BodyPath: "restricted_mode",
		},
		&requestflag.Flag[any]{
			Name:     "tag",
			BodyPath: "tags",
		},
	},
	Action:          handleReposUpdate,
	HideHelpCommand: true,
}

var reposList = cli.Command{
	Name:    "list",
	Usage:   "Get all repos.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[*bool]{
			Name:      "has-commits",
			QueryPath: "has_commits",
		},
		&requestflag.Flag[bool]{
			Name:      "include-owners",
			Default:   false,
			QueryPath: "include_owners",
		},
		&requestflag.Flag[*string]{
			Name:      "is-archived",
			Usage:     `Allowed values: "true", "allow", "false".`,
			QueryPath: "is_archived",
		},
		&requestflag.Flag[*string]{
			Name:      "is-public",
			Usage:     `Allowed values: "true", "false".`,
			QueryPath: "is_public",
		},
		&requestflag.Flag[int64]{
			Name:      "limit",
			Default:   20,
			QueryPath: "limit",
		},
		&requestflag.Flag[int64]{
			Name:      "offset",
			Default:   0,
			QueryPath: "offset",
		},
		&requestflag.Flag[*string]{
			Name:      "query",
			QueryPath: "query",
		},
		&requestflag.Flag[*string]{
			Name:      "single-repo-type",
			Usage:     `Allowed values: "prompt", "file", "agent", "skill".`,
			QueryPath: "repo_type",
		},
		&requestflag.Flag[any]{
			Name:      "repo-type",
			QueryPath: "repo_types",
		},
		&requestflag.Flag[*string]{
			Name:      "sort-direction",
			Usage:     `Allowed values: "asc", "desc".`,
			QueryPath: "sort_direction",
		},
		&requestflag.Flag[*string]{
			Name:      "sort-field",
			Usage:     `Allowed values: "num_likes", "num_downloads", "num_views", "updated_at", "relevance".`,
			QueryPath: "sort_field",
		},
		&requestflag.Flag[*string]{
			Name:      "source",
			Usage:     `Allowed values: "internal", "external".`,
			QueryPath: "source",
		},
		&requestflag.Flag[any]{
			Name:      "tag-value-id",
			QueryPath: "tag_value_id",
		},
		&requestflag.Flag[any]{
			Name:      "tag",
			QueryPath: "tags",
		},
		&requestflag.Flag[*string]{
			Name:      "tenant-handle",
			QueryPath: "tenant_handle",
		},
		&requestflag.Flag[*string]{
			Name:      "tenant-id",
			QueryPath: "tenant_id",
		},
		&requestflag.Flag[*string]{
			Name:      "upstream-repo-handle",
			QueryPath: "upstream_repo_handle",
		},
		&requestflag.Flag[*string]{
			Name:      "upstream-repo-owner",
			QueryPath: "upstream_repo_owner",
		},
		&requestflag.Flag[bool]{
			Name:      "with-latest-manifest",
			Default:   false,
			QueryPath: "with_latest_manifest",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleReposList,
	HideHelpCommand: true,
}

var reposDelete = cli.Command{
	Name:    "delete",
	Usage:   "Delete a repo.",
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
	},
	Action:          handleReposDelete,
	HideHelpCommand: true,
}

func handleReposCreate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.RepoNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Repos.New(ctx, params, options...)
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
		Title:          "repos create",
		Transform:      transform,
	})
}

func handleReposRetrieve(ctx context.Context, cmd *cli.Command) error {
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

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Repos.Get(
		ctx,
		cmd.Value("owner").(string),
		cmd.Value("repo").(string),
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
		Title:          "repos retrieve",
		Transform:      transform,
	})
}

func handleReposUpdate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.RepoUpdateParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Repos.Update(
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
		Title:          "repos update",
		Transform:      transform,
	})
}

func handleReposList(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.RepoListParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Repos.List(ctx, params, options...)
		if err != nil {
			return err
		}
		obj := gjson.ParseBytes(res)
		return ShowJSON(obj, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "repos list",
			Transform:      transform,
		})
	} else {
		iter := client.Repos.ListAutoPaging(ctx, params, options...)
		maxItems := int64(-1)
		if cmd.IsSet("max-items") {
			maxItems = cmd.Value("max-items").(int64)
		}
		return ShowJSONIterator(iter, maxItems, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "repos list",
			Transform:      transform,
		})
	}
}

func handleReposDelete(ctx context.Context, cmd *cli.Command) error {
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

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Repos.Delete(
		ctx,
		cmd.Value("owner").(string),
		cmd.Value("repo").(string),
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
		Title:          "repos delete",
		Transform:      transform,
	})
}
