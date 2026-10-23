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

var sandboxesBrowsersAttachmentsCreate = cli.Command{
	Name:    "create",
	Usage:   "Beta: give a sandbox its own window of the browser, or, with no sandbox, open a\nwindow for the person calling (or, with controller \"agent\", for a client to\ndrive over api_cdp_url). The response's cdp_url is the Chrome DevTools endpoint\nthe sandbox connects to, and api_cdp_url the same endpoint through the API; a\nclient can open more tabs, which stay in its window. Windows share the browser's\ncookies and logins. Keeping each sandbox to its own window is best effort, not a\nsecurity boundary: attach only sandboxes you would trust with every login in the\nbrowser. With share_localhost, the browser can also reach the servers the\nsandbox runs, at the response's localhost_alias. Attaching takes, for the\nbrowser and for the attached sandbox each, being its creator or a role with\nsandboxes:exec. With unrestricted, nothing is filtered: the window's CDP clients\n(its sandbox, or anyone using its api_cdp_url) get the whole browser, including\npeople's windows, every site's cookies, and commands that close it. It needs\nnothing beyond what any attach needs.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
		&requestflag.Flag[string]{
			Name:     "controller",
			Usage:    "Controller is who drives a window opened with no sandbox: \"user\"\n(default) for a person in the live view, or \"agent\" for a client that\ndrives it over api_cdp_url. A sandbox's window always starts with\n\"agent\".",
			BodyPath: "controller",
		},
		&requestflag.Flag[int64]{
			Name:     "height",
			BodyPath: "height",
		},
		&requestflag.Flag[string]{
			Name:     "window-name",
			Usage:    "Name labels the window in the live view; defaults to the sandbox name.",
			BodyPath: "name",
		},
		&requestflag.Flag[string]{
			Name:     "sandbox",
			Usage:    "Sandbox is the name or ID of the sandbox to attach. Leave it out to open a\nwindow for the person calling, to use from the live view.",
			BodyPath: "sandbox",
		},
		&requestflag.Flag[bool]{
			Name:     "share-localhost",
			Usage:    "ShareLocalhost shares the sandbox's localhost with the browser, so pages\ncan open the servers it runs, such as dev servers, at the response's\nlocalhost_alias. It reaches every port of the sandbox, including servers\nbound only to its localhost. Off by default. Not available for a window\nopened with no sandbox, or when the sandbox is itself a browser.",
			BodyPath: "share_localhost",
		},
		&requestflag.Flag[bool]{
			Name:     "unrestricted",
			Usage:    "Unrestricted gives the window's CDP clients (its sandbox, or anyone using\nits api_cdp_url) the whole browser, with nothing filtered: every window\nand tab including people's, every site's cookies, and commands that close\nor crash the browser. Taking over its window does not pause them.",
			BodyPath: "unrestricted",
		},
		&requestflag.Flag[int64]{
			Name:     "width",
			Usage:    "Width and Height size the window's page in CSS pixels; set both or\nneither. Without them the window opens at 1280x800.",
			BodyPath: "width",
		},
	},
	Action:          handleSandboxesBrowsersAttachmentsCreate,
	HideHelpCommand: true,
}

var sandboxesBrowsersAttachmentsUpdate = cli.Command{
	Name:    "update",
	Usage:   "Beta: size a window so its page is width by height CSS pixels. The window keeps\nthat size until resized again, through this call or over CDP with\nBrowser.setWindowBounds; the live view scales to fit it rather than resizing it.\nA browser that is not running cannot be resized.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
		&requestflag.Flag[string]{
			Name:      "attachment-id",
			Required:  true,
			PathParam: "attachment_id",
		},
		&requestflag.Flag[int64]{
			Name:     "height",
			BodyPath: "height",
		},
		&requestflag.Flag[int64]{
			Name:     "width",
			Usage:    "Width and Height are the window's new page size in CSS pixels.",
			BodyPath: "width",
		},
	},
	Action:          handleSandboxesBrowsersAttachmentsUpdate,
	HideHelpCommand: true,
}

var sandboxesBrowsersAttachmentsDelete = cli.Command{
	Name:    "delete",
	Usage:   "Beta: close the window and stop the sandbox from making new connections to the\nbrowser.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
		&requestflag.Flag[string]{
			Name:      "attachment-id",
			Required:  true,
			PathParam: "attachment_id",
		},
	},
	Action:          handleSandboxesBrowsersAttachmentsDelete,
	HideHelpCommand: true,
}

var sandboxesBrowsersAttachmentsSetController = cli.Command{
	Name:    "set-controller",
	Usage:   "Beta: give the window to a person (\"user\") or back to the attached sandbox\n(\"agent\"). It takes effect at once and does not wait for anyone. While a person\nhas control, the attached sandbox's commands that read or act on the page fail\nwith a \"human has control\" error, and it receives no page content. The window's\ncurrent controller is in the browser's attachments.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
		&requestflag.Flag[string]{
			Name:      "attachment-id",
			Required:  true,
			PathParam: "attachment_id",
		},
		&requestflag.Flag[string]{
			Name:     "controller",
			Usage:    "Controller is \"user\" to give the window to a person, or \"agent\" to give\nit back to the attached sandbox.",
			BodyPath: "controller",
		},
	},
	Action:          handleSandboxesBrowsersAttachmentsSetController,
	HideHelpCommand: true,
}

func handleSandboxesBrowsersAttachmentsCreate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.SandboxBrowserAttachmentNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sandboxes.Browsers.Attachments.New(
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
		Title:          "sandboxes:browsers:attachments create",
		Transform:      transform,
	})
}

func handleSandboxesBrowsersAttachmentsUpdate(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("name") && len(unusedArgs) > 0 {
		cmd.Set("name", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if !cmd.IsSet("attachment-id") && len(unusedArgs) > 0 {
		cmd.Set("attachment-id", unusedArgs[0])
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

	params := langsmith.SandboxBrowserAttachmentUpdateParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sandboxes.Browsers.Attachments.Update(
		ctx,
		cmd.Value("name").(string),
		cmd.Value("attachment-id").(string),
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
		Title:          "sandboxes:browsers:attachments update",
		Transform:      transform,
	})
}

func handleSandboxesBrowsersAttachmentsDelete(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("name") && len(unusedArgs) > 0 {
		cmd.Set("name", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if !cmd.IsSet("attachment-id") && len(unusedArgs) > 0 {
		cmd.Set("attachment-id", unusedArgs[0])
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

	return client.Sandboxes.Browsers.Attachments.Delete(
		ctx,
		cmd.Value("name").(string),
		cmd.Value("attachment-id").(string),
		options...,
	)
}

func handleSandboxesBrowsersAttachmentsSetController(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("name") && len(unusedArgs) > 0 {
		cmd.Set("name", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if !cmd.IsSet("attachment-id") && len(unusedArgs) > 0 {
		cmd.Set("attachment-id", unusedArgs[0])
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

	params := langsmith.SandboxBrowserAttachmentSetControllerParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sandboxes.Browsers.Attachments.SetController(
		ctx,
		cmd.Value("name").(string),
		cmd.Value("attachment-id").(string),
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
		Title:          "sandboxes:browsers:attachments set-controller",
		Transform:      transform,
	})
}
