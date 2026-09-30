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

var sandboxesRegistriesCreate = cli.Command{
	Name:    "create",
	Usage:   "Create a sandbox registry for pulling private images.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "name",
			Required: true,
			BodyPath: "name",
		},
		&requestflag.Flag[string]{
			Name:     "url",
			Required: true,
			BodyPath: "url",
		},
		&requestflag.Flag[string]{
			Name:     "auth-type",
			Usage:    `Allowed values: "DOCKER_CONFIG", "AWS_ROLE".`,
			BodyPath: "auth_type",
		},
		&requestflag.Flag[string]{
			Name:     "aws-role-arn",
			BodyPath: "aws_role_arn",
		},
		&requestflag.Flag[string]{
			Name:     "password",
			BodyPath: "password",
		},
		&requestflag.Flag[string]{
			Name:     "username",
			BodyPath: "username",
		},
	},
	Action:          handleSandboxesRegistriesCreate,
	HideHelpCommand: true,
}

var sandboxesRegistriesRetrieve = cli.Command{
	Name:    "retrieve",
	Usage:   "Get a sandbox registry by name.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
	},
	Action:          handleSandboxesRegistriesRetrieve,
	HideHelpCommand: true,
}

var sandboxesRegistriesUpdate = cli.Command{
	Name:    "update",
	Usage:   "Update a sandbox registry's name and/or credentials.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
		&requestflag.Flag[string]{
			Name:     "auth-type",
			Usage:    `Allowed values: "DOCKER_CONFIG", "AWS_ROLE".`,
			BodyPath: "auth_type",
		},
		&requestflag.Flag[string]{
			Name:     "aws-role-arn",
			BodyPath: "aws_role_arn",
		},
		&requestflag.Flag[string]{
			Name:     "name",
			BodyPath: "name",
		},
		&requestflag.Flag[string]{
			Name:     "password",
			BodyPath: "password",
		},
		&requestflag.Flag[string]{
			Name:     "url",
			BodyPath: "url",
		},
		&requestflag.Flag[string]{
			Name:     "username",
			BodyPath: "username",
		},
	},
	Action:          handleSandboxesRegistriesUpdate,
	HideHelpCommand: true,
}

var sandboxesRegistriesList = cli.Command{
	Name:    "list",
	Usage:   "List sandbox registries for pulling private images.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[int64]{
			Name:      "limit",
			Usage:     "Maximum number of registries to return",
			QueryPath: "limit",
		},
		&requestflag.Flag[string]{
			Name:      "name-contains",
			Usage:     "Filter to registries whose name contains this substring",
			QueryPath: "name_contains",
		},
		&requestflag.Flag[int64]{
			Name:      "offset",
			Usage:     "Number of registries to skip",
			QueryPath: "offset",
		},
	},
	Action:          handleSandboxesRegistriesList,
	HideHelpCommand: true,
}

var sandboxesRegistriesDelete = cli.Command{
	Name:    "delete",
	Usage:   "Delete a sandbox registry by name.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
	},
	Action:          handleSandboxesRegistriesDelete,
	HideHelpCommand: true,
}

func handleSandboxesRegistriesCreate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.SandboxRegistryNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sandboxes.Registries.New(ctx, params, options...)
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
		Title:          "sandboxes:registries create",
		Transform:      transform,
	})
}

func handleSandboxesRegistriesRetrieve(ctx context.Context, cmd *cli.Command) error {
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
	_, err = client.Sandboxes.Registries.Get(ctx, cmd.Value("name").(string), options...)
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
		Title:          "sandboxes:registries retrieve",
		Transform:      transform,
	})
}

func handleSandboxesRegistriesUpdate(ctx context.Context, cmd *cli.Command) error {
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
		ApplicationJSON,
		false,
	)
	if err != nil {
		return err
	}

	params := langsmith.SandboxRegistryUpdateParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sandboxes.Registries.Update(
		ctx,
		cmd.Value("name").(string),
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
		Title:          "sandboxes:registries update",
		Transform:      transform,
	})
}

func handleSandboxesRegistriesList(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.SandboxRegistryListParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sandboxes.Registries.List(ctx, params, options...)
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
		Title:          "sandboxes:registries list",
		Transform:      transform,
	})
}

func handleSandboxesRegistriesDelete(ctx context.Context, cmd *cli.Command) error {
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

	return client.Sandboxes.Registries.Delete(ctx, cmd.Value("name").(string), options...)
}
