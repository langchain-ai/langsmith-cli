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

var sandboxesBoxesCreate = requestflag.WithInnerFlags(cli.Command{
	Name:    "create",
	Usage:   "Create a new sandbox from a snapshot. Provide at most one of `snapshot_id` or\n`snapshot_name`; if neither is provided, the server uses the default snapshot.\n`snapshot_name` accepts a Docker-style `name` or `name:tag` reference (a bare\nname resolves to `name:latest`).",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[map[string]any]{
			Name:     "access-delegation",
			Usage:    "AccessDelegation lets code inside the sandbox call the LangSmith API as you, with at most the permissions granted here. Omit for no access.",
			BodyPath: "access_delegation",
		},
		&requestflag.Flag[int64]{
			Name:     "cpu-millicores",
			Usage:    "CPUMillicores optionally requests CPU at millicore granularity (e.g. 500 = 0.5 vCPU); takes precedence over VCPUs. Fractional (sub-vCPU) values are not available for every sandbox.",
			BodyPath: "cpu_millicores",
		},
		&requestflag.Flag[int64]{
			Name:     "delete-after-stop-seconds",
			BodyPath: "delete_after_stop_seconds",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "env-vars",
			BodyPath: "env_vars",
		},
		&requestflag.Flag[int64]{
			Name:     "fs-capacity-bytes",
			BodyPath: "fs_capacity_bytes",
		},
		&requestflag.Flag[int64]{
			Name:     "idle-ttl-seconds",
			BodyPath: "idle_ttl_seconds",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "labels",
			Usage:    "Labels are free-form key/value metadata persisted with the sandbox and returned on reads. Labels from the source snapshot are inherited unless overridden here.",
			BodyPath: "labels",
		},
		&requestflag.Flag[int64]{
			Name:     "mem-bytes",
			Usage:    "Memory for the sandbox, in bytes. Memory is tied to CPU at 4 GiB per vCPU: omit it and it follows that ratio; set it and it must stay within 50% of the ratio for the requested CPU, so a 1 vCPU sandbox accepts 2-6 GiB. Setting memory without CPU derives the CPU from the same ratio. Maximum 64 GiB.",
			BodyPath: "mem_bytes",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "mount-config",
			BodyPath: "mount_config",
		},
		&requestflag.Flag[string]{
			Name:     "name",
			BodyPath: "name",
		},
		&requestflag.Flag[bool]{
			Name:     "preserve-memory-on-stop",
			Usage:    "PreserveMemoryOnStop, when true, suspends the sandbox's memory on a\nvoluntary stop (idle timeout or explicit stop) so the next start resumes\nfrom where it left off. Default false discards memory and keeps only the\nfilesystem, so the next start is a cold boot. Restarts triggered by\ninfrastructure maintenance always preserve memory regardless of this setting.",
			BodyPath: "preserve_memory_on_stop",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "proxy-config",
			BodyPath: "proxy_config",
		},
		&requestflag.Flag[bool]{
			Name:     "restore-memory",
			Usage:    "RestoreMemory selects how the sandbox handles a snapshot's captured memory:\n\n  nil   → if-present: resume from memory when the snapshot has it, else cold-boot (default).\n  true  → always: resume from memory; rejected if the snapshot has none.\n  false → never: always cold-boot.\n\nApplies to this request only.",
			BodyPath: "restore_memory",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "run-config",
			Usage:    "RunConfig overrides the snapshot's run config for this sandbox: user and\nwork_dir replace the snapshot's, env_vars merge over it. The result is\nwhat the sandbox boots with, and what a snapshot captured from it carries.",
			BodyPath: "run_config",
		},
		&requestflag.Flag[string]{
			Name:     "snapshot",
			Usage:    "Snapshot is a Docker-style name or name:tag reference to boot from. A bare name resolves to name:latest.",
			BodyPath: "snapshot",
		},
		&requestflag.Flag[string]{
			Name:     "snapshot-id",
			BodyPath: "snapshot_id",
		},
		&requestflag.Flag[string]{
			Name:     "snapshot-name",
			Usage:    "SnapshotName is a synonym for Snapshot, accepted for compatibility with clients that predate it. Set one or the other.",
			BodyPath: "snapshot_name",
		},
		&requestflag.Flag[[]string]{
			Name:     "tag-value-id",
			BodyPath: "tag_value_ids",
		},
		&requestflag.Flag[int64]{
			Name:     "vcpus",
			BodyPath: "vcpus",
		},
	},
	Action:          handleSandboxesBoxesCreate,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"access-delegation": {
		&requestflag.InnerFlag[string]{
			Name:       "access-delegation.mode",
			Usage:      `Allowed values: "INHERIT", "EXPLICIT".`,
			InnerField: "mode",
		},
		&requestflag.InnerFlag[[]string]{
			Name:       "access-delegation.permissions",
			InnerField: "permissions",
		},
	},
	"mount-config": {
		&requestflag.InnerFlag[map[string]any]{
			Name:       "mount-config.auth",
			InnerField: "auth",
		},
		&requestflag.InnerFlag[[]map[string]any]{
			Name:       "mount-config.mounts",
			InnerField: "mounts",
		},
	},
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
	"run-config": {
		&requestflag.InnerFlag[map[string]any]{
			Name:       "run-config.env-vars",
			InnerField: "env_vars",
		},
		&requestflag.InnerFlag[string]{
			Name:       "run-config.user",
			InnerField: "user",
		},
		&requestflag.InnerFlag[string]{
			Name:       "run-config.work-dir",
			InnerField: "work_dir",
		},
	},
})

var sandboxesBoxesRetrieve = cli.Command{
	Name:    "retrieve",
	Usage:   "Retrieve a sandbox by name. Stale provisioning sandboxes are auto-failed.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
	},
	Action:          handleSandboxesBoxesRetrieve,
	HideHelpCommand: true,
}

var sandboxesBoxesUpdate = requestflag.WithInnerFlags(cli.Command{
	Name:    "update",
	Usage:   "Update a sandbox's display name, retention, resources, tags, or proxy\nconfiguration. The name must be unique within the tenant. Proxy configuration\nsent to a sandbox that is not running is stored and applied when it next starts.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
		&requestflag.Flag[int64]{
			Name:     "cpu-millicores",
			BodyPath: "cpu_millicores",
		},
		&requestflag.Flag[int64]{
			Name:     "delete-after-stop-seconds",
			BodyPath: "delete_after_stop_seconds",
		},
		&requestflag.Flag[int64]{
			Name:     "fs-capacity-bytes",
			BodyPath: "fs_capacity_bytes",
		},
		&requestflag.Flag[int64]{
			Name:     "idle-ttl-seconds",
			BodyPath: "idle_ttl_seconds",
		},
		&requestflag.Flag[int64]{
			Name:     "mem-bytes",
			Usage:    "New memory for the sandbox, in bytes. The 4 GiB per vCPU ratio applies when the sandbox is created; a resize enforces only the maximum of 64 GiB.",
			BodyPath: "mem_bytes",
		},
		&requestflag.Flag[string]{
			Name:     "name",
			BodyPath: "name",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "proxy-config",
			BodyPath: "proxy_config",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "run-config",
			Usage:    "RunConfig changes what subsequent commands run with: user and work_dir\nreplace the current values, env_vars merge over them. Commands already\nrunning are unaffected.",
			BodyPath: "run_config",
		},
		&requestflag.Flag[[]string]{
			Name:     "tag-value-id",
			BodyPath: "tag_value_ids",
		},
		&requestflag.Flag[int64]{
			Name:     "vcpus",
			BodyPath: "vcpus",
		},
	},
	Action:          handleSandboxesBoxesUpdate,
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
	"run-config": {
		&requestflag.InnerFlag[map[string]any]{
			Name:       "run-config.env-vars",
			InnerField: "env_vars",
		},
		&requestflag.InnerFlag[string]{
			Name:       "run-config.user",
			InnerField: "user",
		},
		&requestflag.InnerFlag[string]{
			Name:       "run-config.work-dir",
			InnerField: "work_dir",
		},
	},
})

var sandboxesBoxesList = cli.Command{
	Name:    "list",
	Usage:   "List sandboxes for the authenticated tenant, with optional filtering, sorting,\nand pagination. Page with page_size and cursor: replay the response's\nnext_cursor until it comes back null, which is the only signal that no pages\nremain. Cursors are opaque and only valid on this endpoint; do not parse or\nconstruct one.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "created-by",
			Usage:     "Filter by creator identity. Only 'me' is supported.",
			QueryPath: "created_by",
		},
		&requestflag.Flag[string]{
			Name:      "cursor",
			Usage:     "Opaque pagination cursor from a prior response's next_cursor",
			QueryPath: "cursor",
		},
		&requestflag.Flag[[]string]{
			Name:      "label",
			Usage:     "Filter by label. Repeatable; all must match. Use 'key' to match on key presence or 'key=value' for equality.",
			QueryPath: "label",
		},
		&requestflag.Flag[int64]{
			Name:      "limit",
			Usage:     "Deprecated: use page_size. Maximum number of results",
			Default:   50,
			QueryPath: "limit",
		},
		&requestflag.Flag[string]{
			Name:      "name-contains",
			Usage:     "Filter by name substring",
			QueryPath: "name_contains",
		},
		&requestflag.Flag[int64]{
			Name:      "offset",
			Usage:     "Deprecated: use cursor. Pagination offset",
			Default:   0,
			QueryPath: "offset",
		},
		&requestflag.Flag[int64]{
			Name:      "page-size",
			Usage:     "Number of results per page",
			Default:   20,
			QueryPath: "page_size",
		},
		&requestflag.Flag[string]{
			Name:      "sort-by",
			Usage:     "Sort column (name, status, created_at, stopped_at, idle_ttl_seconds, delete_after_stop_seconds)",
			Default:   "created_at",
			QueryPath: "sort_by",
		},
		&requestflag.Flag[string]{
			Name:      "sort-direction",
			Usage:     "Deprecated: use sort_order. Sort direction (asc, desc)",
			Default:   "desc",
			QueryPath: "sort_direction",
		},
		&requestflag.Flag[string]{
			Name:      "sort-order",
			Usage:     "Sort direction (asc, desc)",
			Default:   "desc",
			QueryPath: "sort_order",
		},
		&requestflag.Flag[string]{
			Name:      "status",
			Usage:     "Filter by status (provisioning, ready, failed, stopped, deleting)",
			QueryPath: "status",
		},
		&requestflag.Flag[[]string]{
			Name:      "tag-value-id",
			Usage:     "Filter by workspace resource tag value IDs; all must match",
			QueryPath: "tag_value_id",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleSandboxesBoxesList,
	HideHelpCommand: true,
}

var sandboxesBoxesDelete = cli.Command{
	Name:    "delete",
	Usage:   "Delete a sandbox by name or UUID. Tears down the sandbox runtime and removes the\nDB record.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
	},
	Action:          handleSandboxesBoxesDelete,
	HideHelpCommand: true,
}

var sandboxesBoxesCreateSnapshot = requestflag.WithInnerFlags(cli.Command{
	Name:    "create-snapshot",
	Usage:   "Create a snapshot by capturing the current state of a sandbox or promoting an\nexisting checkpoint.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
		&requestflag.Flag[string]{
			Name:     "name",
			Required: true,
			BodyPath: "name",
		},
		&requestflag.Flag[string]{
			Name:     "checkpoint",
			Usage:    "if omitted, creates a fresh checkpoint from the running VM",
			BodyPath: "checkpoint",
		},
		&requestflag.Flag[string]{
			Name:     "description",
			Usage:    "Description says what this snapshot's image can do, so a caller can hand it to an agent as a capability summary. At most 1024 characters.",
			BodyPath: "description",
		},
		&requestflag.Flag[string]{
			Name:     "docker-image",
			Usage:    "sandbox-local Docker image to export",
			BodyPath: "docker_image",
		},
		&requestflag.Flag[int64]{
			Name:     "fs-capacity-bytes",
			Usage:    "required for Docker image export unless the sandbox has a capacity",
			BodyPath: "fs_capacity_bytes",
		},
		&requestflag.Flag[bool]{
			Name:     "include-memory",
			Usage:    "IncludeMemory, when true, captures a full VM memory snapshot\nalongside the filesystem clone. Only honored when the sandbox is running\nAND Checkpoint is omitted (i.e. a fresh in-VM checkpoint is requested).\nDefaults to false to keep snapshots small unless memory restore is\nexplicitly desired.",
			BodyPath: "include_memory",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "labels",
			Usage:    "Labels seed the captured snapshot's labels.",
			BodyPath: "labels",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "run-config",
			Usage:    "RunConfig overrides the runtime configuration the snapshot carries: for a\ndocker_image export, the image's USER, WORKDIR and ENV; for a capture of\nthe running VM, the sandbox's own. user and work_dir replace, env_vars\nmerge.",
			BodyPath: "run_config",
		},
		&requestflag.Flag[string]{
			Name:     "tag",
			Usage:    `mutable Docker-style tag; defaults to "latest"`,
			BodyPath: "tag",
		},
	},
	Action:          handleSandboxesBoxesCreateSnapshot,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"run-config": {
		&requestflag.InnerFlag[map[string]any]{
			Name:       "run-config.env-vars",
			InnerField: "env_vars",
		},
		&requestflag.InnerFlag[string]{
			Name:       "run-config.user",
			InnerField: "user",
		},
		&requestflag.InnerFlag[string]{
			Name:       "run-config.work-dir",
			InnerField: "work_dir",
		},
	},
})

var sandboxesBoxesDeleteServiceURL = cli.Command{
	Name:    "delete-service-url",
	Usage:   "Removes the sharing grant for one port, or for every port when port is omitted.\nA LangSmith login URL stops working immediately. A previously minted service\ntoken is not revoked and stays valid until it expires, but no new one can be\nissued from the removed grant.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
		&requestflag.Flag[int64]{
			Name:      "port",
			Usage:     "Port to stop sharing. Omit to stop sharing every port.",
			QueryPath: "port",
		},
	},
	Action:          handleSandboxesBoxesDeleteServiceURL,
	HideHelpCommand: true,
}

var sandboxesBoxesGenerateDownloadURL = cli.Command{
	Name:    "generate-download-url",
	Usage:   "Generate a tokenized link that downloads a single file from a sandbox with no\nfurther authentication. This mints a token rather than creating an addressable\nresource, so it returns 200 with no Location header. The token pins the sandbox,\nthe file path, the response content type and disposition, and the sandbox flags,\nso a link cannot be repointed at another file or served under a weaker policy.\nThe file is always served with a Content-Security-Policy: a sandbox directive,\nplus a default-src holding every fetch to the file's own download host and a set\nof pre-approved third-party origins. csp_sandbox_flags may loosen the sandbox\nwith allow-downloads, allow-forms, allow-modals, allow-orientation-lock,\nallow-pointer-lock, allow-popups, allow-presentation, allow-same-origin,\nallow-scripts, or allow-top-navigation-by-user-activation. Every file is served\nfrom its own host, derived from the sandbox and the path, so allow-same-origin\ngives a page localStorage and IndexedDB that no other file can read, and\nre-minting a link for the same file keeps them. csp_sandbox set to false drops\nthe sandbox directive altogether, and csp_sandbox_flags must then be omitted.\ncsp_source_bundles selects the third-party origins: cdnjs, google-fonts,\njsdelivr, and unpkg are all allowed when the field is omitted, 'none' holds the\nfile to its own host, and 'any' sends no default-src at all. Links never expire\nunless expires_in_seconds is set. The link is served from the sandbox service\ndomain, not the API host.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
		&requestflag.Flag[string]{
			Name:     "path",
			Required: true,
			BodyPath: "path",
		},
		&requestflag.Flag[string]{
			Name:     "content-disposition",
			BodyPath: "content_disposition",
		},
		&requestflag.Flag[string]{
			Name:     "content-type",
			BodyPath: "content_type",
		},
		&requestflag.Flag[bool]{
			Name:     "csp-sandbox",
			Usage:    "CSPSandbox false serves the file with no CSP sandbox directive; omit to keep it.",
			BodyPath: "csp_sandbox",
		},
		&requestflag.Flag[[]string]{
			Name:     "csp-sandbox-flag",
			Usage:    "CSPSandboxFlags loosen the CSP sandbox the file is served under; omit for the most restrictive policy.",
			BodyPath: "csp_sandbox_flags",
		},
		&requestflag.Flag[[]string]{
			Name:     "csp-source-bundle",
			Usage:    "CSPSourceBundles allow the served file to fetch from named third-party origins; omit to send no fetch directive.",
			BodyPath: "csp_source_bundles",
		},
		&requestflag.Flag[int64]{
			Name:     "expires-in-seconds",
			Usage:    "ExpiresInSeconds is optional; a link with no expiry never expires.",
			BodyPath: "expires_in_seconds",
		},
	},
	Action:          handleSandboxesBoxesGenerateDownloadURL,
	HideHelpCommand: true,
}

var sandboxesBoxesGenerateServiceURL = cli.Command{
	Name:    "generate-service-url",
	Usage:   "Create a short-lived JWT for accessing an HTTP service running on a specific\nport inside a sandbox. Returns a browser_url (sets auth cookie via redirect), a\nservice_url (for use with the X-Langsmith-Sandbox-Service-Token header), the raw\ntoken, and its expiry. Set access=restricted|workspace to instead enable durable\nLangSmith login (no token; users authenticate with their normal LangSmith\nsession), or access=off to disable it. LangSmith login and token access are\nmutually exclusive per service URL.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
		&requestflag.Flag[string]{
			Name:     "access",
			Usage:    "Access selects the login mode, mutually exclusive with the minted token.\nOmit the field for token mode: mint a short-lived service token (default).\n  \"restricted\" — LangSmith login: the sandbox's creator, or any user with SandboxesExec on it (admins by default).\n  \"workspace\"  — LangSmith login: any member of the owning workspace.\n  \"off\"        — remove an existing LangSmith login grant and mint a token.\nA LangSmith login grant is durable; token mode is refused (409) while one exists.",
			BodyPath: "access",
		},
		&requestflag.Flag[int64]{
			Name:     "expires-in-seconds",
			BodyPath: "expires_in_seconds",
		},
		&requestflag.Flag[int64]{
			Name:     "port",
			BodyPath: "port",
		},
	},
	Action:          handleSandboxesBoxesGenerateServiceURL,
	HideHelpCommand: true,
}

var sandboxesBoxesGetStatus = cli.Command{
	Name:    "get-status",
	Usage:   "Retrieve the lightweight status of a sandbox for polling.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
	},
	Action:          handleSandboxesBoxesGetStatus,
	HideHelpCommand: true,
}

var sandboxesBoxesListServiceURLs = cli.Command{
	Name:    "list-service-urls",
	Usage:   "Returns one entry per port the sandbox is currently reachable on, so a caller\ncan see what is shared before turning it off. Expired token grants are omitted.\nCursors are opaque and only valid on this endpoint; do not parse or construct\none.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
		&requestflag.Flag[string]{
			Name:      "cursor",
			Usage:     "Opaque pagination cursor from a prior response's next_cursor",
			QueryPath: "cursor",
		},
		&requestflag.Flag[int64]{
			Name:      "page-size",
			Usage:     "Number of results per page",
			Default:   20,
			QueryPath: "page_size",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleSandboxesBoxesListServiceURLs,
	HideHelpCommand: true,
}

var sandboxesBoxesStart = cli.Command{
	Name:    "start",
	Usage:   "Start a stopped or failed sandbox. This endpoint is not idempotent.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
	},
	Action:          handleSandboxesBoxesStart,
	HideHelpCommand: true,
}

var sandboxesBoxesStop = cli.Command{
	Name:    "stop",
	Usage:   "Stop a ready sandbox. This endpoint is not idempotent; the filesystem is\npreserved for later restart.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "name",
			Required:  true,
			PathParam: "name",
		},
	},
	Action:          handleSandboxesBoxesStop,
	HideHelpCommand: true,
}

func handleSandboxesBoxesCreate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.SandboxBoxNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sandboxes.Boxes.New(ctx, params, options...)
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
		Title:          "sandboxes:boxes create",
		Transform:      transform,
	})
}

func handleSandboxesBoxesRetrieve(ctx context.Context, cmd *cli.Command) error {
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
	_, err = client.Sandboxes.Boxes.Get(ctx, cmd.Value("name").(string), options...)
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
		Title:          "sandboxes:boxes retrieve",
		Transform:      transform,
	})
}

func handleSandboxesBoxesUpdate(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.SandboxBoxUpdateParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sandboxes.Boxes.Update(
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
		Title:          "sandboxes:boxes update",
		Transform:      transform,
	})
}

func handleSandboxesBoxesList(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.SandboxBoxListParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Sandboxes.Boxes.List(ctx, params, options...)
		if err != nil {
			return err
		}
		obj := gjson.ParseBytes(res)
		return ShowJSON(obj, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "sandboxes:boxes list",
			Transform:      transform,
		})
	} else {
		iter := client.Sandboxes.Boxes.ListAutoPaging(ctx, params, options...)
		maxItems := int64(-1)
		if cmd.IsSet("max-items") {
			maxItems = cmd.Value("max-items").(int64)
		}
		return ShowJSONIterator(iter, maxItems, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "sandboxes:boxes list",
			Transform:      transform,
		})
	}
}

func handleSandboxesBoxesDelete(ctx context.Context, cmd *cli.Command) error {
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

	return client.Sandboxes.Boxes.Delete(ctx, cmd.Value("name").(string), options...)
}

func handleSandboxesBoxesCreateSnapshot(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.SandboxBoxNewSnapshotParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sandboxes.Boxes.NewSnapshot(
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
		Title:          "sandboxes:boxes create-snapshot",
		Transform:      transform,
	})
}

func handleSandboxesBoxesDeleteServiceURL(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.SandboxBoxDeleteServiceURLParams{}

	return client.Sandboxes.Boxes.DeleteServiceURL(
		ctx,
		cmd.Value("name").(string),
		params,
		options...,
	)
}

func handleSandboxesBoxesGenerateDownloadURL(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.SandboxBoxGenerateDownloadURLParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sandboxes.Boxes.GenerateDownloadURL(
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
		Title:          "sandboxes:boxes generate-download-url",
		Transform:      transform,
	})
}

func handleSandboxesBoxesGenerateServiceURL(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.SandboxBoxGenerateServiceURLParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Sandboxes.Boxes.GenerateServiceURL(
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
		Title:          "sandboxes:boxes generate-service-url",
		Transform:      transform,
	})
}

func handleSandboxesBoxesGetStatus(ctx context.Context, cmd *cli.Command) error {
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
	_, err = client.Sandboxes.Boxes.GetStatus(ctx, cmd.Value("name").(string), options...)
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
		Title:          "sandboxes:boxes get-status",
		Transform:      transform,
	})
}

func handleSandboxesBoxesListServiceURLs(ctx context.Context, cmd *cli.Command) error {
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

	params := langsmith.SandboxBoxListServiceURLsParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Sandboxes.Boxes.ListServiceURLs(
			ctx,
			cmd.Value("name").(string),
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
			Title:          "sandboxes:boxes list-service-urls",
			Transform:      transform,
		})
	} else {
		iter := client.Sandboxes.Boxes.ListServiceURLsAutoPaging(
			ctx,
			cmd.Value("name").(string),
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
			Title:          "sandboxes:boxes list-service-urls",
			Transform:      transform,
		})
	}
}

func handleSandboxesBoxesStart(ctx context.Context, cmd *cli.Command) error {
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
	_, err = client.Sandboxes.Boxes.Start(ctx, cmd.Value("name").(string), options...)
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
		Title:          "sandboxes:boxes start",
		Transform:      transform,
	})
}

func handleSandboxesBoxesStop(ctx context.Context, cmd *cli.Command) error {
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

	return client.Sandboxes.Boxes.Stop(ctx, cmd.Value("name").(string), options...)
}
