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

var agentsCreate = cli.Command{
	Name:    "create",
	Usage:   "Creates an Agent with a caller-assigned ID. Returns 409 if the ID already\nexists. Creates all four environments unless production_only is set for a FLEET\nagent. agent_type defaults to CUSTOM. The authenticated user becomes the owner\nwhen available. Requires permission to create projects. The response does not\ninclude runtime bindings.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "id",
			Usage:    "Unique agent identifier: 1-63 lowercase letters, digits, or hyphens,\nstarting with a letter and ending with a letter or digit.",
			Required: true,
			BodyPath: "id",
		},
		&requestflag.Flag[string]{
			Name:     "source-type",
			Usage:    "Agent runtime kind. Use EXTERNAL for an application hosted outside LangSmith.",
			Required: true,
			BodyPath: "source_type",
		},
		&requestflag.Flag[string]{
			Name:     "agent-type",
			Usage:    "AgentType defaults to CUSTOM when omitted.",
			BodyPath: "agent_type",
		},
		&requestflag.Flag[string]{
			Name:     "name",
			Usage:    "Display name. Defaults to the agent ID.",
			BodyPath: "name",
		},
		&requestflag.Flag[bool]{
			Name:     "production-only",
			Usage:    "ProductionOnly creates only the PRODUCTION environment. It is accepted\nonly when SourceType is FLEET; other sources always get all four\nenvironments.",
			BodyPath: "production_only",
		},
	},
	Action:          handleAgentsCreate,
	HideHelpCommand: true,
}

var agentsRetrieve = cli.Command{
	Name:    "retrieve",
	Usage:   "Returns an Agent with the environments visible to the caller. The path value is\nthe user-assigned Agent ID. Pass include=runtime_bindings to embed each\nenvironment's runtime bindings.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "agent-id",
			Required:  true,
			PathParam: "agent_id",
		},
		&requestflag.Flag[string]{
			Name:      "include",
			Usage:     "Relation to embed. runtime_bindings adds each environment's runtime bindings; an empty array means no runtime is linked.",
			QueryPath: "include",
		},
	},
	Action:          handleAgentsRetrieve,
	HideHelpCommand: true,
}

var agentsUpdate = cli.Command{
	Name:    "update",
	Usage:   "Updates an Agent's name, source_type, agent_type, or owner_id. Omitted fields\nkeep their values. Changing or clearing owner_id additionally requires\nworkspaces:manage; assigning an owner also requires workspaces:read and active\nworkspace membership. Fleet ownership cannot be changed through this endpoint.\nChanging agent_type requires workspaces:manage in addition to project update\naccess; repeating the current type is a no-op. Source type can change only for\nan EXTERNAL Agent without runtime bindings. The id is immutable and is rejected\nwhen present. Requires update permission on every environment project.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "agent-id",
			Required:  true,
			PathParam: "agent_id",
		},
		&requestflag.Flag[string]{
			Name:     "agent-type",
			Usage:    "New classification. Omit to keep the current classification.",
			BodyPath: "agent_type",
		},
		&requestflag.Flag[string]{
			Name:     "name",
			Usage:    "New display name. Omit to keep the current name.",
			BodyPath: "name",
		},
		&requestflag.Flag[*string]{
			Name:     "owner-id",
			Usage:    "Omit to leave the owner unchanged; null clears it.",
			BodyPath: "owner_id",
		},
		&requestflag.Flag[string]{
			Name:     "source-type",
			Usage:    "New runtime kind. Only EXTERNAL agents without runtime bindings can change kind.",
			BodyPath: "source_type",
		},
	},
	Action:          handleAgentsUpdate,
	HideHelpCommand: true,
}

var agentsList = cli.Command{
	Name:    "list",
	Usage:   "Returns Agents visible to the caller. Each item is the same Agent shape as GET\n/api/v1/agents/{agent_id}, including the environments the caller may read. The\nlist never embeds runtime bindings. An unknown query parameter, including limit,\nreturns 400; use page_size to set the page size.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[[]string]{
			Name:      "id",
			Usage:     "Exact Agent ids (repeat the parameter; maximum 100)",
			QueryPath: "id",
		},
		&requestflag.Flag[string]{
			Name:      "cursor",
			Usage:     "Opaque pagination cursor",
			QueryPath: "cursor",
		},
		&requestflag.Flag[string]{
			Name:      "name",
			Usage:     "Case-insensitive partial Agent name",
			QueryPath: "name",
		},
		&requestflag.Flag[int64]{
			Name:      "page-size",
			Usage:     "Maximum number of Agents to return",
			Default:   20,
			QueryPath: "page_size",
		},
		&requestflag.Flag[string]{
			Name:      "sort-by",
			Usage:     "Field to sort by. When name is set, exact name matches still come first.",
			Default:   "last_run_start_time",
			QueryPath: "sort_by",
		},
		&requestflag.Flag[string]{
			Name:      "sort-order",
			Usage:     "Sort direction",
			Default:   "desc",
			QueryPath: "sort_order",
		},
		&requestflag.Flag[string]{
			Name:      "source-type",
			Usage:     "Agent source type",
			QueryPath: "source_type",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleAgentsList,
	HideHelpCommand: true,
}

var agentsDelete = cli.Command{
	Name:    "delete",
	Usage:   "Permanently deletes an Agent, along with its environments, runtime bindings,\nruntimes, and tracing projects. This cannot be undone. Deleting an Agent that\ndoes not exist returns 204.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "agent-id",
			Required:  true,
			PathParam: "agent_id",
		},
	},
	Action:          handleAgentsDelete,
	HideHelpCommand: true,
}

func handleAgentsCreate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.AgentNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Agents.New(ctx, params, options...)
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
		Title:          "agents create",
		Transform:      transform,
	})
}

func handleAgentsRetrieve(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("agent-id") && len(unusedArgs) > 0 {
		cmd.Set("agent-id", unusedArgs[0])
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

	params := langsmith.AgentGetParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Agents.Get(
		ctx,
		cmd.Value("agent-id").(string),
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
		Title:          "agents retrieve",
		Transform:      transform,
	})
}

func handleAgentsUpdate(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("agent-id") && len(unusedArgs) > 0 {
		cmd.Set("agent-id", unusedArgs[0])
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

	params := langsmith.AgentUpdateParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Agents.Update(
		ctx,
		cmd.Value("agent-id").(string),
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
		Title:          "agents update",
		Transform:      transform,
	})
}

func handleAgentsList(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.AgentListParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Agents.List(ctx, params, options...)
		if err != nil {
			return err
		}
		obj := gjson.ParseBytes(res)
		return ShowJSON(obj, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "agents list",
			Transform:      transform,
		})
	} else {
		iter := client.Agents.ListAutoPaging(ctx, params, options...)
		maxItems := int64(-1)
		if cmd.IsSet("max-items") {
			maxItems = cmd.Value("max-items").(int64)
		}
		return ShowJSONIterator(iter, maxItems, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "agents list",
			Transform:      transform,
		})
	}
}

func handleAgentsDelete(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("agent-id") && len(unusedArgs) > 0 {
		cmd.Set("agent-id", unusedArgs[0])
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

	return client.Agents.Delete(ctx, cmd.Value("agent-id").(string), options...)
}
