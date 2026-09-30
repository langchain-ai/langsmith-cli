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

var reposDirectoriesList = cli.Command{
	Name:    "list",
	Usage:   "Resolves the flattened file tree for an agent or skill repository at a specific\ncommit, tag, or latest.",
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
			Usage:     "Commit hash/tag to resolve (defaults to latest)",
			QueryPath: "commit",
		},
	},
	Action:          handleReposDirectoriesList,
	HideHelpCommand: true,
}

var reposDirectoriesDelete = cli.Command{
	Name:    "delete",
	Usage:   "Deletes an agent or skill repository and its owned child file repositories.",
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
			Name:      "repo-type",
			Usage:     "Repository type to delete; a different type is treated as not found",
			QueryPath: "repo_type",
		},
	},
	Action:          handleReposDirectoriesDelete,
	HideHelpCommand: true,
}

var reposDirectoriesCommit = cli.Command{
	Name:    "commit",
	Usage:   "Creates a new directory commit for an agent or skill repository by applying\nfile/link create, update, and delete operations. Linked directories default to\nthe LATEST selector; use COMMIT to pin one commit. The legacy commit_id write\nfield is deprecated and resolves as LATEST.",
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
		&requestflag.Flag[map[string]any]{
			Name:     "files",
			Usage:    "Paths to create, update, link, delete, or unlink. Use null to delete or unlink an existing path.",
			BodyPath: "files",
		},
		&requestflag.Flag[string]{
			Name:     "parent-commit",
			BodyPath: "parent_commit",
		},
		&requestflag.Flag[bool]{
			Name:     "skip-webhooks",
			Usage:    "SkipWebhooks suppresses Context Hub commit webhooks for this commit.",
			BodyPath: "skip_webhooks",
		},
	},
	Action:          handleReposDirectoriesCommit,
	HideHelpCommand: true,
}

func handleReposDirectoriesList(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.RepoDirectoryListParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Repos.Directories.List(
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
		Title:          "repos:directories list",
		Transform:      transform,
	})
}

func handleReposDirectoriesDelete(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.RepoDirectoryDeleteParams{}

	return client.Repos.Directories.Delete(
		ctx,
		cmd.Value("owner").(string),
		cmd.Value("repo").(string),
		params,
		options...,
	)
}

func handleReposDirectoriesCommit(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.RepoDirectoryCommitParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Repos.Directories.Commit(
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
		Title:          "repos:directories commit",
		Transform:      transform,
	})
}
