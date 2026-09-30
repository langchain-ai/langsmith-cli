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

var promptWebhooksCreate = cli.Command{
	Name:    "create",
	Usage:   "Create a new prompt webhook.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "url",
			Usage:    "URL that receives a POST request with the prompt event payload.",
			Required: true,
			BodyPath: "url",
		},
		&requestflag.Flag[*string]{
			Name:     "id",
			Usage:    "ID to assign to the webhook. Generated when omitted.",
			BodyPath: "id",
		},
		&requestflag.Flag[any]{
			Name:     "exclude-prompt",
			Usage:    "IDs of prompts whose events never fire the webhook.",
			BodyPath: "exclude_prompts",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "headers",
			Usage:    "HTTP headers sent with each request, for example an Authorization header. Values are stored encrypted and are masked in responses to callers who cannot update prompts.",
			BodyPath: "headers",
		},
		&requestflag.Flag[any]{
			Name:     "include-prompt",
			Usage:    "IDs of the prompts whose events fire the webhook. When empty, events from every prompt in the workspace fire it.",
			BodyPath: "include_prompts",
		},
		&requestflag.Flag[[]string]{
			Name:     "trigger",
			Usage:    "Events that fire the webhook: commit (a new prompt commit), tag:create, and tag:update. A webhook with no triggers never fires.",
			BodyPath: "triggers",
		},
	},
	Action:          handlePromptWebhooksCreate,
	HideHelpCommand: true,
}

var promptWebhooksRetrieve = cli.Command{
	Name:    "retrieve",
	Usage:   "Get a specific prompt webhook.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "webhook-id",
			Required:  true,
			PathParam: "webhook_id",
		},
	},
	Action:          handlePromptWebhooksRetrieve,
	HideHelpCommand: true,
}

var promptWebhooksUpdate = cli.Command{
	Name:    "update",
	Usage:   "Update a specific prompt webhook.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "webhook-id",
			Required:  true,
			PathParam: "webhook_id",
		},
		&requestflag.Flag[any]{
			Name:     "exclude-prompt",
			Usage:    "IDs of prompts whose events never fire the webhook, replacing the current list. Omit to keep it.",
			BodyPath: "exclude_prompts",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "headers",
			Usage:    "HTTP headers sent with each request, replacing all current headers. Omit to keep them. Masked values (********) from a read are rejected.",
			BodyPath: "headers",
		},
		&requestflag.Flag[any]{
			Name:     "include-prompt",
			Usage:    "IDs of the prompts whose events fire the webhook, replacing the current list. Omit to keep it.",
			BodyPath: "include_prompts",
		},
		&requestflag.Flag[any]{
			Name:     "trigger",
			Usage:    "Events that fire the webhook (commit, tag:create, tag:update), replacing the current list. Omit to keep it.",
			BodyPath: "triggers",
		},
		&requestflag.Flag[*string]{
			Name:     "url",
			Usage:    "URL that receives the POST requests. Omit to keep the current URL.",
			BodyPath: "url",
		},
	},
	Action:          handlePromptWebhooksUpdate,
	HideHelpCommand: true,
}

var promptWebhooksList = cli.Command{
	Name:            "list",
	Usage:           "List all prompt webhooks for the current tenant.",
	Suggest:         true,
	Flags:           []cli.Flag{},
	Action:          handlePromptWebhooksList,
	HideHelpCommand: true,
}

var promptWebhooksDelete = cli.Command{
	Name:    "delete",
	Usage:   "Delete a specific prompt webhook.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "webhook-id",
			Required:  true,
			PathParam: "webhook_id",
		},
	},
	Action:          handlePromptWebhooksDelete,
	HideHelpCommand: true,
}

var promptWebhooksTest = requestflag.WithInnerFlags(cli.Command{
	Name:    "test",
	Usage:   "Test a specific prompt webhook.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[map[string]any]{
			Name:     "payload",
			Required: true,
			BodyPath: "payload",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "webhook",
			Usage:    "Base schema for prompt webhooks.",
			Required: true,
			BodyPath: "webhook",
		},
	},
	Action:          handlePromptWebhooksTest,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"payload": {
		&requestflag.InnerFlag[string]{
			Name:       "payload.commit-hash",
			InnerField: "commit_hash",
		},
		&requestflag.InnerFlag[string]{
			Name:       "payload.created-at",
			InnerField: "created_at",
		},
		&requestflag.InnerFlag[string]{
			Name:       "payload.created-by",
			InnerField: "created_by",
		},
		&requestflag.InnerFlag[string]{
			Name:       "payload.event",
			Usage:      "Valid trigger types for prompt webhooks.",
			InnerField: "event",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "payload.manifest",
			InnerField: "manifest",
		},
		&requestflag.InnerFlag[string]{
			Name:       "payload.prompt-id",
			InnerField: "prompt_id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "payload.prompt-name",
			InnerField: "prompt_name",
		},
		&requestflag.InnerFlag[*string]{
			Name:       "payload.tag-name",
			InnerField: "tag_name",
		},
	},
	"webhook": {
		&requestflag.InnerFlag[string]{
			Name:       "webhook.url",
			Usage:      "URL that receives a POST request with the prompt event payload.",
			InnerField: "url",
		},
		&requestflag.InnerFlag[any]{
			Name:       "webhook.exclude-prompts",
			Usage:      "IDs of prompts whose events never fire the webhook.",
			InnerField: "exclude_prompts",
		},
		&requestflag.InnerFlag[map[string]any]{
			Name:       "webhook.headers",
			Usage:      "HTTP headers sent with each request, for example an Authorization header. Values are stored encrypted and are masked in responses to callers who cannot update prompts.",
			InnerField: "headers",
		},
		&requestflag.InnerFlag[any]{
			Name:       "webhook.include-prompts",
			Usage:      "IDs of the prompts whose events fire the webhook. When empty, events from every prompt in the workspace fire it.",
			InnerField: "include_prompts",
		},
		&requestflag.InnerFlag[[]string]{
			Name:       "webhook.triggers",
			Usage:      "Events that fire the webhook: commit (a new prompt commit), tag:create, and tag:update. A webhook with no triggers never fires.",
			InnerField: "triggers",
		},
	},
})

func handlePromptWebhooksCreate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.PromptWebhookNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.PromptWebhooks.New(ctx, params, options...)
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
		Title:          "prompt-webhooks create",
		Transform:      transform,
	})
}

func handlePromptWebhooksRetrieve(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("webhook-id") && len(unusedArgs) > 0 {
		cmd.Set("webhook-id", unusedArgs[0])
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
	_, err = client.PromptWebhooks.Get(ctx, cmd.Value("webhook-id").(string), options...)
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
		Title:          "prompt-webhooks retrieve",
		Transform:      transform,
	})
}

func handlePromptWebhooksUpdate(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("webhook-id") && len(unusedArgs) > 0 {
		cmd.Set("webhook-id", unusedArgs[0])
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

	params := langsmith.PromptWebhookUpdateParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.PromptWebhooks.Update(
		ctx,
		cmd.Value("webhook-id").(string),
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
		Title:          "prompt-webhooks update",
		Transform:      transform,
	})
}

func handlePromptWebhooksList(ctx context.Context, cmd *cli.Command) error {
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

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.PromptWebhooks.List(ctx, options...)
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
		Title:          "prompt-webhooks list",
		Transform:      transform,
	})
}

func handlePromptWebhooksDelete(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("webhook-id") && len(unusedArgs) > 0 {
		cmd.Set("webhook-id", unusedArgs[0])
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
	_, err = client.PromptWebhooks.Delete(ctx, cmd.Value("webhook-id").(string), options...)
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
		Title:          "prompt-webhooks delete",
		Transform:      transform,
	})
}

func handlePromptWebhooksTest(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.PromptWebhookTestParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.PromptWebhooks.Test(ctx, params, options...)
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
		Title:          "prompt-webhooks test",
		Transform:      transform,
	})
}
