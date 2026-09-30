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

var annotationQueuesItemsCreate = requestflag.WithInnerFlags(cli.Command{
	Name:    "create",
	Usage:   "Add RUN or THREAD items to a single annotation queue. RUN items require run_id\nunless they are created from a suggested example. THREAD items require thread_id\nand project_id.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "queue-id",
			Required:  true,
			PathParam: "queue_id",
		},
		&requestflag.Flag[bool]{
			Name:      "extend-trace-retention",
			Usage:     "Extend trace retention for added run items",
			QueryPath: "extend_trace_retention",
		},
		&requestflag.Flag[[]map[string]any]{
			Name:     "item",
			BodyPath: "items",
		},
	},
	Action:          handleAnnotationQueuesItemsCreate,
	HideHelpCommand: true,
}, map[string][]requestflag.HasOuterFlag{
	"item": {
		&requestflag.InnerFlag[string]{
			Name:       "item.item-type",
			Usage:      `Allowed values: "RUN", "THREAD".`,
			InnerField: "item_type",
		},
		&requestflag.InnerFlag[string]{
			Name:       "item.project-id",
			InnerField: "project_id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "item.run-id",
			Usage:      "RUN fields",
			InnerField: "run_id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "item.session-id",
			Usage:      "SessionID is an alias for project_id.",
			InnerField: "session_id",
		},
		&requestflag.InnerFlag[string]{
			Name:       "item.source-proposed-example-id",
			Usage:      "SourceProposedExampleID links the queue item to the suggested example\nit was created from, when applicable.",
			InnerField: "source_proposed_example_id",
		},
		&requestflag.InnerFlag[any]{
			Name:       "item.start-time",
			InnerField: "start_time",
		},
		&requestflag.InnerFlag[string]{
			Name:       "item.thread-id",
			InnerField: "thread_id",
		},
	},
})

var annotationQueuesItemsUpdate = cli.Command{
	Name:    "update",
	Usage:   "Partially update mutable timestamps (added_at, last_reviewed_time) for a RUN or\nTHREAD annotation queue item. Omit a field, or pass JSON null, to leave it\nunchanged.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "queue-id",
			Required:  true,
			PathParam: "queue_id",
		},
		&requestflag.Flag[string]{
			Name:      "item-id",
			Required:  true,
			PathParam: "item_id",
		},
		&requestflag.Flag[any]{
			Name:     "added-at",
			BodyPath: "added_at",
		},
		&requestflag.Flag[any]{
			Name:     "last-reviewed-time",
			BodyPath: "last_reviewed_time",
		},
	},
	Action:          handleAnnotationQueuesItemsUpdate,
	HideHelpCommand: true,
}

var annotationQueuesItemsList = cli.Command{
	Name:    "list",
	Usage:   "List RUN and THREAD items in a single annotation queue for one review status\nsection, with opaque cursor pagination. Optional item_type=RUN|THREAD filters\nthe page. Optional min_start_time/max_start_time bound the item's trace start\ntime; items with no start time are excluded when either bound is set.\ndirection=backward returns items before the supplied cursor. The response\ncontains item metadata only, not expanded run or thread payloads.\nstatus=archived returns items whose queue review requirements have been\nsatisfied, not merely items the caller personally marked completed.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "queue-id",
			Required:  true,
			PathParam: "queue_id",
		},
		&requestflag.Flag[string]{
			Name:      "status",
			Usage:     "Review section: needs_my_review, needs_others_review, or archived",
			Required:  true,
			QueryPath: "status",
		},
		&requestflag.Flag[string]{
			Name:      "cursor",
			Usage:     "Opaque pagination cursor",
			QueryPath: "cursor",
		},
		&requestflag.Flag[string]{
			Name:      "direction",
			Usage:     "Pagination direction. backward requires cursor",
			Default:   "forward",
			QueryPath: "direction",
		},
		&requestflag.Flag[string]{
			Name:      "item-type",
			Usage:     "Filter to RUN or THREAD",
			QueryPath: "item_type",
		},
		&requestflag.Flag[any]{
			Name:      "max-start-time",
			Usage:     "Only items whose trace start time is at or before this timestamp. Omit or send the zero time for no bound",
			QueryPath: "max_start_time",
		},
		&requestflag.Flag[any]{
			Name:      "min-start-time",
			Usage:     "Only items whose trace start time is at or after this timestamp. Omit or send the zero time for no bound",
			QueryPath: "min_start_time",
		},
		&requestflag.Flag[int64]{
			Name:      "page-size",
			Usage:     "Page size (max 100)",
			Default:   20,
			QueryPath: "page_size",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleAnnotationQueuesItemsList,
	HideHelpCommand: true,
}

var annotationQueuesItemsCreateStatus = cli.Command{
	Name:    "create-status",
	Usage:   "Log the caller's reviewer status for a RUN or THREAD annotation queue item. A\nnull status re-shows the item for this reviewer.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "queue-item-id",
			Required:  true,
			PathParam: "queue_item_id",
		},
		&requestflag.Flag[string]{
			Name:     "override-added-at",
			BodyPath: "override_added_at",
		},
		&requestflag.Flag[string]{
			Name:     "status",
			Usage:    `Allowed values: "viewed", "completed".`,
			BodyPath: "status",
		},
	},
	Action:          handleAnnotationQueuesItemsCreateStatus,
	HideHelpCommand: true,
}

var annotationQueuesItemsDeleteAll = cli.Command{
	Name:    "delete-all",
	Usage:   "Remove RUN or THREAD items from a single annotation queue by item ID. Both\nactive and completed items can be removed.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "queue-id",
			Required:  true,
			PathParam: "queue_id",
		},
		&requestflag.Flag[[]string]{
			Name:     "item-id",
			BodyPath: "item_ids",
		},
	},
	Action:          handleAnnotationQueuesItemsDeleteAll,
	HideHelpCommand: true,
}

var annotationQueuesItemsRetrieveCount = cli.Command{
	Name:    "retrieve-count",
	Usage:   "Returns the number of annotation queue items in one status bucket. The two time\nwindows are independent: start_time/end_time bound when an item was archived,\nmin_start_time/max_start_time bound when its trace ran. Items with no trace\nstart time are excluded when either of the latter is set.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "queue-id",
			Required:  true,
			PathParam: "queue_id",
		},
		&requestflag.Flag[string]{
			Name:      "status",
			Usage:     "Count bucket: all, needs_my_review, needs_others_review, or archived.",
			Required:  true,
			QueryPath: "status",
		},
		&requestflag.Flag[string]{
			Name:      "end-time",
			Usage:     "Archived strictly before this time. Only used when status=archived",
			QueryPath: "end_time",
		},
		&requestflag.Flag[any]{
			Name:      "max-start-time",
			Usage:     "Trace started at or before this time",
			QueryPath: "max_start_time",
		},
		&requestflag.Flag[any]{
			Name:      "min-start-time",
			Usage:     "Trace started at or after this time",
			QueryPath: "min_start_time",
		},
		&requestflag.Flag[string]{
			Name:      "start-time",
			Usage:     "Archived strictly after this time. Only used when status=archived",
			QueryPath: "start_time",
		},
	},
	Action:          handleAnnotationQueuesItemsRetrieveCount,
	HideHelpCommand: true,
}

var annotationQueuesItemsRetrievePlacement = cli.Command{
	Name:    "retrieve-placement",
	Usage:   "Resolve a RUN or THREAD item to its current review section and zero-based\nposition for deep linking. The returned cursor counts RUN and THREAD items\ntogether, so it is only valid for a list request with no item_type or start-time\nfilter.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "queue-id",
			Required:  true,
			PathParam: "queue_id",
		},
		&requestflag.Flag[string]{
			Name:      "item-id",
			Required:  true,
			PathParam: "item_id",
		},
	},
	Action:          handleAnnotationQueuesItemsRetrievePlacement,
	HideHelpCommand: true,
}

func handleAnnotationQueuesItemsCreate(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("queue-id") && len(unusedArgs) > 0 {
		cmd.Set("queue-id", unusedArgs[0])
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

	params := langsmith.AnnotationQueueItemNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.AnnotationQueues.Items.New(
		ctx,
		cmd.Value("queue-id").(string),
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
		Title:          "annotation-queues:items create",
		Transform:      transform,
	})
}

func handleAnnotationQueuesItemsUpdate(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("queue-id") && len(unusedArgs) > 0 {
		cmd.Set("queue-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if !cmd.IsSet("item-id") && len(unusedArgs) > 0 {
		cmd.Set("item-id", unusedArgs[0])
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

	params := langsmith.AnnotationQueueItemUpdateParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.AnnotationQueues.Items.Update(
		ctx,
		cmd.Value("queue-id").(string),
		cmd.Value("item-id").(string),
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
		Title:          "annotation-queues:items update",
		Transform:      transform,
	})
}

func handleAnnotationQueuesItemsList(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("queue-id") && len(unusedArgs) > 0 {
		cmd.Set("queue-id", unusedArgs[0])
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

	params := langsmith.AnnotationQueueItemListParams{}

	format := cmd.Root().String("format")
	explicitFormat := cmd.Root().IsSet("format")
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.AnnotationQueues.Items.List(
			ctx,
			cmd.Value("queue-id").(string),
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
			Title:          "annotation-queues:items list",
			Transform:      transform,
		})
	} else {
		iter := client.AnnotationQueues.Items.ListAutoPaging(
			ctx,
			cmd.Value("queue-id").(string),
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
			Title:          "annotation-queues:items list",
			Transform:      transform,
		})
	}
}

func handleAnnotationQueuesItemsCreateStatus(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("queue-item-id") && len(unusedArgs) > 0 {
		cmd.Set("queue-item-id", unusedArgs[0])
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

	params := langsmith.AnnotationQueueItemNewStatusParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.AnnotationQueues.Items.NewStatus(
		ctx,
		cmd.Value("queue-item-id").(string),
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
		Title:          "annotation-queues:items create-status",
		Transform:      transform,
	})
}

func handleAnnotationQueuesItemsDeleteAll(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("queue-id") && len(unusedArgs) > 0 {
		cmd.Set("queue-id", unusedArgs[0])
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

	params := langsmith.AnnotationQueueItemDeleteAllParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.AnnotationQueues.Items.DeleteAll(
		ctx,
		cmd.Value("queue-id").(string),
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
		Title:          "annotation-queues:items delete-all",
		Transform:      transform,
	})
}

func handleAnnotationQueuesItemsRetrieveCount(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("queue-id") && len(unusedArgs) > 0 {
		cmd.Set("queue-id", unusedArgs[0])
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

	params := langsmith.AnnotationQueueItemGetCountParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.AnnotationQueues.Items.GetCount(
		ctx,
		cmd.Value("queue-id").(string),
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
		Title:          "annotation-queues:items retrieve-count",
		Transform:      transform,
	})
}

func handleAnnotationQueuesItemsRetrievePlacement(ctx context.Context, cmd *cli.Command) error {
	client := langsmith.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("queue-id") && len(unusedArgs) > 0 {
		cmd.Set("queue-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if !cmd.IsSet("item-id") && len(unusedArgs) > 0 {
		cmd.Set("item-id", unusedArgs[0])
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
	_, err = client.AnnotationQueues.Items.GetPlacement(
		ctx,
		cmd.Value("queue-id").(string),
		cmd.Value("item-id").(string),
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
		Title:          "annotation-queues:items retrieve-placement",
		Transform:      transform,
	})
}
