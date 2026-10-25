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

var sandboxesBrowsersAttachmentsLinksCreate = cli.Command{
	Name:    "create",
	Usage:   "Beta: mint a view link and an act link for one window, as every attachment\nresponse does, with a chosen lifetime and the origins allowed to embed the\nwindow's pages in an iframe. Without embed_origins no page may frame them.\nview_embed_url and act_embed_url carry the same tokens in the URL fragment, for\nan iframe, so no server sees them. An embedded page sends its parent window\nmessages when control changes and when it connects or disconnects (see the\nbrowser guide). Either link opens the window to anyone who holds it until it\nexpires; detaching the window ends it early. A stopped browser starts when links\nare minted and when a link is opened, and resumes with its windows. Minting\ntakes being the browser's creator or a role with sandboxes:exec.",
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
		&requestflag.Flag[[]string]{
			Name:     "embed-origin",
			Usage:    "EmbedOrigins are the origins whose pages may show the window's pages in\nan iframe, such as https://app.example.com: https, or http on\nlocalhost, with no path. Without them no page may frame the window.",
			BodyPath: "embed_origins",
		},
		&requestflag.Flag[int64]{
			Name:     "ttl-seconds",
			Usage:    "TTLSeconds is how long the links work, from 60 seconds to 24 hours.\nDefaults to 24 hours.",
			BodyPath: "ttl_seconds",
		},
	},
	Action:          handleSandboxesBrowsersAttachmentsLinksCreate,
	HideHelpCommand: true,
}

func handleSandboxesBrowsersAttachmentsLinksCreate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.SandboxBrowserAttachmentLinkNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sandboxes.Browsers.Attachments.Links.New(
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
		Title:          "sandboxes:browsers:attachments:links create",
		Transform:      transform,
	})
}
