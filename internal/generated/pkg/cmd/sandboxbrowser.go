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

var sandboxesBrowsersCreate = requestflag.WithInnerFlags(cli.Command{
	Name:    "create",
	Usage:   "Beta: create a browser sandbox, which runs one persistent Chrome that other\nsandboxes attach to. Logins made in it persist across restarts. It keeps its\nmemory when it stops. A stopped browser is deleted after the sandbox default,\nalong with its logins, unless delete_after_stop_seconds sets another delay or 0\nto keep it.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[int64]{
			Name:     "delete-after-stop-seconds",
			Usage:    "DeleteAfterStopSeconds deletes the browser this long after it stops.\nDefaults to the sandbox default; 0 keeps a stopped browser until it is\ndeleted.",
			BodyPath: "delete_after_stop_seconds",
		},
		&requestflag.Flag[bool]{
			Name:     "hidpi",
			Usage:    "HiDPI renders pages at twice the resolution, so the live view is sharp on\nHiDPI screens, at four times the pixels for the browser to render and\nstream. Off by default, and fixed once the browser is created.",
			BodyPath: "hidpi",
		},
		&requestflag.Flag[int64]{
			Name:     "idle-ttl-seconds",
			Usage:    "IdleTTLSeconds stops the browser after this long without use. A stopped\nbrowser resumes with its memory on the next connection. Defaults to the\nsandbox default; 0 disables the idle stop.",
			BodyPath: "idle_ttl_seconds",
		},
		&requestflag.Flag[int64]{
			Name:     "mem-bytes",
			BodyPath: "mem_bytes",
		},
		&requestflag.Flag[string]{
			Name:     "name",
			Usage:    "Name of the browser sandbox. Generated when omitted, as \"browser-\"\nfollowed by the browser's ID.",
			BodyPath: "name",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "proxy-config",
			BodyPath: "proxy_config",
		},
		&requestflag.Flag[int64]{
			Name:     "vcpus",
			Usage:    "VCPUs and MemBytes size the browser sandbox, as for any sandbox. With\nneither set, a browser gets 2 vCPUs and 8 GiB.",
			BodyPath: "vcpus",
		},
	},
	Action:          handleSandboxesBrowsersCreate,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"proxy-config": {
		&requestflag.InnerFlag[map[string]any]{
			Name:       "proxy-config.access-control",
			InnerField: "access_control",
		},
		&requestflag.InnerFlag[[]map[string]any]{
			Name:       "proxy-config.callbacks",
			InnerField: "callbacks",
		},
		&requestflag.InnerFlag[string]{
			Name:       "proxy-config.description",
			Usage:      "Description says what this configuration as a whole lets the sandbox reach, complementing the per-rule descriptions. At most 1024 characters.",
			InnerField: "description",
		},
		&requestflag.InnerFlag[[]string]{
			Name:       "proxy-config.no-proxy",
			InnerField: "no_proxy",
		},
		&requestflag.InnerFlag[[]map[string]any]{
			Name:       "proxy-config.rules",
			InnerField: "rules",
		},
	},
})

var sandboxesBrowsersRetrieve = cli.Command{
	Name:    "retrieve",
	Usage:   "Beta: get a browser and the windows attached to it.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
	},
	Action:          handleSandboxesBrowsersRetrieve,
	HideHelpCommand: true,
}

var sandboxesBrowsersLoadExtensions = cli.Command{
	Name:    "load-extensions",
	Usage:   "Beta: bring the browser's Chrome in line with the unpacked extensions in its\n/var/lib/browserd/extensions (one per directory). Of several with the same\nmanifest name, only the highest manifest version is loaded, so a newer version\ninstalled beside an older one replaces it. Extensions placed there that Chrome\ndoes not have yet are loaded while it runs, so windows, tabs and connections are\nleft as they are. When an extension Chrome has was removed or replaced, Chrome\ncannot unload it while running, so it is relaunched instead with the directory's\nextensions: logins stay, open tabs close and connections drop, and the call\nreturns once every window is back.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
	},
	Action:          handleSandboxesBrowsersLoadExtensions,
	HideHelpCommand: true,
}

func handleSandboxesBrowsersCreate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.SandboxBrowserNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sandboxes.Browsers.New(ctx, params, options...)
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
		Title:          "sandboxes:browsers create",
		Transform:      transform,
	})
}

func handleSandboxesBrowsersRetrieve(ctx context.Context, cmd *cli.Command) error {
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
	_, err = client.Sandboxes.Browsers.Get(ctx, cmd.Value("name").(string), options...)
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
		Title:          "sandboxes:browsers retrieve",
		Transform:      transform,
	})
}

func handleSandboxesBrowsersLoadExtensions(ctx context.Context, cmd *cli.Command) error {
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
	_, err = client.Sandboxes.Browsers.LoadExtensions(ctx, cmd.Value("name").(string), options...)
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
		Title:          "sandboxes:browsers load-extensions",
		Transform:      transform,
	})
}
