package cmd

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-cli/internal/apiquery"
	"github.com/anthropics/anthropic-cli/internal/requestflag"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/tidwall/gjson"
	"github.com/urfave/cli/v3"
)

var betaMemoryStoresCreate = cli.Command{
	Name:    "create",
	Usage:   "Create a Memory Store",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:     "name",
			Usage:    "The name of the memory store, 1 to 255 characters long, without control characters. Names don't have to be unique in a workspace.\n\nIn a session, the store's directory under `/mnt/memory/` is based on this name. The name is lowercased and every run of characters other than ASCII letters and digits becomes one hyphen. The session's memory store resource gives the exact path in `mount_path`.\n\nSee the [memory guide](https://platform.claude.com/docs/en/managed-agents/memory#how-the-agent-accesses-memory) to learn more about how agents access memory stores.",
			Required: true,
			BodyPath: "name",
		},
		&requestflag.Flag[string]{
			Name:     "description",
			Usage:    "Free-text description of what the store contains, up to 1,024 characters. Included in the agent's system prompt when the store is attached, so word it to be useful to the agent.",
			BodyPath: "description",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "metadata",
			Usage:    "Key-value pairs for your own use, such as the end user that the store belongs to. The agent doesn't see them.\n\nA store has at most 16 pairs. Keys are 1 to 64 characters long and values at most 512 characters.",
			BodyPath: "metadata",
		},
		&requestflag.Flag[[]string]{
			Name:       "beta",
			Usage:      "This endpoint is in beta: requests must send `agent-memory-2026-07-22` or `managed-agents-2026-04-01` in this header.",
			HeaderPath: "anthropic-beta",
		},
		&requestflag.Flag[string]{
			Name:       "workspace-id",
			Usage:      "Optional header to select the Workspace for this request. The value is a Workspace ID (for example, `wrkspc_011CZkZaBF1tNoB5wlCeusgy`).\n\nOnly needed for credentials that can act on more than one Workspace. A credential that belongs to a specific Workspace may omit it; if sent, it must match that Workspace.",
			HeaderPath: "anthropic-workspace-id",
		},
	},
	Action:          handleBetaMemoryStoresCreate,
	HideHelpCommand: true,
}

var betaMemoryStoresRetrieve = cli.Command{
	Name:    "retrieve",
	Usage:   "Retrieve a Memory Store",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "memory-store-id",
			Usage:     "The ID of the memory store to retrieve (`memstore_...`).",
			Required:  true,
			PathParam: "memory_store_id",
		},
		&requestflag.Flag[[]string]{
			Name:       "beta",
			Usage:      "This endpoint is in beta: requests must send `agent-memory-2026-07-22` or `managed-agents-2026-04-01` in this header.",
			HeaderPath: "anthropic-beta",
		},
		&requestflag.Flag[string]{
			Name:       "workspace-id",
			Usage:      "Optional header to select the Workspace for this request. The value is a Workspace ID (for example, `wrkspc_011CZkZaBF1tNoB5wlCeusgy`).\n\nOnly needed for credentials that can act on more than one Workspace. A credential that belongs to a specific Workspace may omit it; if sent, it must match that Workspace.",
			HeaderPath: "anthropic-workspace-id",
		},
	},
	Action:          handleBetaMemoryStoresRetrieve,
	HideHelpCommand: true,
}

var betaMemoryStoresUpdate = cli.Command{
	Name:    "update",
	Usage:   "Update a Memory Store",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:        "memory-store-id",
			Usage:       "The ID of the memory store to update (`memstore_...`).\n\nThe request fails with HTTP status 400 if the store is archived.",
			Required:    true,
			PathParam:   "memory_store_id",
			DataAliases: []string{"id"},
		},
		&requestflag.Flag[*string]{
			Name:     "description",
			Usage:    "New description for the store, up to 1,024 characters. Pass an empty string to clear it.",
			BodyPath: "description",
		},
		&requestflag.Flag[map[string]any]{
			Name:     "metadata",
			Usage:    "Changes to apply to the memory store's metadata. Keys you leave out keep their values.\n\nSet a key to a string to add or replace it, or to `null` to remove it. After the change, the metadata can hold at most 16 key-value pairs. Keys are 1 to 64 characters long and values at most 512 characters.",
			BodyPath: "metadata",
		},
		&requestflag.Flag[*string]{
			Name:     "name",
			Usage:    "The new name of the memory store, 1 to 255 characters long, without control characters.\n\nSessions created after the change use the new name for the store's directory under `/mnt/memory/`.",
			BodyPath: "name",
		},
		&requestflag.Flag[[]string]{
			Name:       "beta",
			Usage:      "This endpoint is in beta: requests must send `agent-memory-2026-07-22` or `managed-agents-2026-04-01` in this header.",
			HeaderPath: "anthropic-beta",
		},
		&requestflag.Flag[string]{
			Name:       "workspace-id",
			Usage:      "Optional header to select the Workspace for this request. The value is a Workspace ID (for example, `wrkspc_011CZkZaBF1tNoB5wlCeusgy`).\n\nOnly needed for credentials that can act on more than one Workspace. A credential that belongs to a specific Workspace may omit it; if sent, it must match that Workspace.",
			HeaderPath: "anthropic-workspace-id",
		},
	},
	Action:          handleBetaMemoryStoresUpdate,
	HideHelpCommand: true,
}

var betaMemoryStoresList = cli.Command{
	Name:    "list",
	Usage:   "List Memory Stores",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[any]{
			Name:      "created-at-gte",
			Usage:     "Return only memory stores created at or after this time (inclusive).",
			QueryPath: "created_at[gte]",
		},
		&requestflag.Flag[any]{
			Name:      "created-at-lte",
			Usage:     "Return only memory stores created at or before this time (inclusive).",
			QueryPath: "created_at[lte]",
		},
		&requestflag.Flag[bool]{
			Name:      "include-archived",
			Usage:     "When `true`, archived stores are included in the results. Defaults to `false` (archived stores are excluded).",
			QueryPath: "include_archived",
		},
		&requestflag.Flag[int64]{
			Name:      "limit",
			Usage:     "Maximum number of stores to return per page. Must be between 1 and 100. Defaults to 20 when omitted.",
			QueryPath: "limit",
		},
		&requestflag.Flag[string]{
			Name:      "page",
			Usage:     "The `next_page` value from a previous response, to get the next page. Omit it to get the first page.",
			QueryPath: "page",
		},
		&requestflag.Flag[[]string]{
			Name:       "beta",
			Usage:      "This endpoint is in beta: requests must send `agent-memory-2026-07-22` or `managed-agents-2026-04-01` in this header.",
			HeaderPath: "anthropic-beta",
		},
		&requestflag.Flag[string]{
			Name:       "workspace-id",
			Usage:      "Optional header to select the Workspace for this request. The value is a Workspace ID (for example, `wrkspc_011CZkZaBF1tNoB5wlCeusgy`).\n\nOnly needed for credentials that can act on more than one Workspace. A credential that belongs to a specific Workspace may omit it; if sent, it must match that Workspace.",
			HeaderPath: "anthropic-workspace-id",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleBetaMemoryStoresList,
	HideHelpCommand: true,
}

var betaMemoryStoresDelete = cli.Command{
	Name:    "delete",
	Usage:   "Delete a Memory Store",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "memory-store-id",
			Usage:     "The ID of the memory store to delete (`memstore_...`).\n\nDeleting a store also deletes all its memories and memory versions. It can't be undone.",
			Required:  true,
			PathParam: "memory_store_id",
		},
		&requestflag.Flag[[]string]{
			Name:       "beta",
			Usage:      "This endpoint is in beta: requests must send `agent-memory-2026-07-22` or `managed-agents-2026-04-01` in this header.",
			HeaderPath: "anthropic-beta",
		},
		&requestflag.Flag[string]{
			Name:       "workspace-id",
			Usage:      "Optional header to select the Workspace for this request. The value is a Workspace ID (for example, `wrkspc_011CZkZaBF1tNoB5wlCeusgy`).\n\nOnly needed for credentials that can act on more than one Workspace. A credential that belongs to a specific Workspace may omit it; if sent, it must match that Workspace.",
			HeaderPath: "anthropic-workspace-id",
		},
	},
	Action:          handleBetaMemoryStoresDelete,
	HideHelpCommand: true,
}

var betaMemoryStoresArchive = cli.Command{
	Name:    "archive",
	Usage:   "Archive a Memory Store",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "memory-store-id",
			Usage:     "The ID of the memory store to archive (`memstore_...`).\n\nArchiving can't be undone. Archiving a store that is already archived returns it unchanged.",
			Required:  true,
			PathParam: "memory_store_id",
		},
		&requestflag.Flag[[]string]{
			Name:       "beta",
			Usage:      "This endpoint is in beta: requests must send `agent-memory-2026-07-22` or `managed-agents-2026-04-01` in this header.",
			HeaderPath: "anthropic-beta",
		},
		&requestflag.Flag[string]{
			Name:       "workspace-id",
			Usage:      "Optional header to select the Workspace for this request. The value is a Workspace ID (for example, `wrkspc_011CZkZaBF1tNoB5wlCeusgy`).\n\nOnly needed for credentials that can act on more than one Workspace. A credential that belongs to a specific Workspace may omit it; if sent, it must match that Workspace.",
			HeaderPath: "anthropic-workspace-id",
		},
	},
	Action:          handleBetaMemoryStoresArchive,
	HideHelpCommand: true,
}

func handleBetaMemoryStoresCreate(ctx context.Context, cmd *cli.Command) error {
	client := anthropic.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()

	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatBrackets,
		ApplicationJSON,
		false,
	)
	if err != nil {
		return err
	}

	params := anthropic.BetaMemoryStoreNewParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Beta.MemoryStores.New(ctx, params, options...)
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
		Title:          "beta:memory-stores create",
		Transform:      transform,
	})
}

func handleBetaMemoryStoresRetrieve(ctx context.Context, cmd *cli.Command) error {
	client := anthropic.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("memory-store-id") && len(unusedArgs) > 0 {
		cmd.Set("memory-store-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatBrackets,
		EmptyBody,
		false,
	)
	if err != nil {
		return err
	}

	params := anthropic.BetaMemoryStoreGetParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Beta.MemoryStores.Get(
		ctx,
		cmd.Value("memory-store-id").(string),
		params,
		options...,
	)
	if err != nil {
		return err
	}

	obj := gjson.ParseBytes(res)
	format := "explore"
	explicitFormat := cmd.Root().IsSet("format")
	if explicitFormat {
		format = cmd.Root().String("format")
	}
	transform := cmd.Root().String("transform")
	return ShowJSON(obj, ShowJSONOpts{
		ExplicitFormat: explicitFormat,
		Format:         format,
		RawOutput:      cmd.Root().Bool("raw-output"),
		Title:          "beta:memory-stores retrieve",
		Transform:      transform,
	})
}

func handleBetaMemoryStoresUpdate(ctx context.Context, cmd *cli.Command) error {
	client := anthropic.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("memory-store-id") && len(unusedArgs) > 0 {
		cmd.Set("memory-store-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatBrackets,
		ApplicationJSON,
		false,
	)
	if err != nil {
		return err
	}

	params := anthropic.BetaMemoryStoreUpdateParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Beta.MemoryStores.Update(
		ctx,
		cmd.Value("memory-store-id").(string),
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
		Title:          "beta:memory-stores update",
		Transform:      transform,
	})
}

func handleBetaMemoryStoresList(ctx context.Context, cmd *cli.Command) error {
	client := anthropic.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()

	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatBrackets,
		EmptyBody,
		false,
	)
	if err != nil {
		return err
	}

	params := anthropic.BetaMemoryStoreListParams{}

	format := "explore"
	explicitFormat := cmd.Root().IsSet("format")
	if explicitFormat {
		format = cmd.Root().String("format")
	}
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Beta.MemoryStores.List(ctx, params, options...)
		if err != nil {
			return err
		}
		obj := gjson.ParseBytes(res)
		return ShowJSON(obj, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "beta:memory-stores list",
			Transform:      transform,
		})
	} else {
		iter := client.Beta.MemoryStores.ListAutoPaging(ctx, params, options...)
		maxItems := int64(-1)
		if cmd.IsSet("max-items") {
			maxItems = cmd.Value("max-items").(int64)
		}
		return ShowJSONIterator(iter, maxItems, ShowJSONOpts{
			ExplicitFormat: explicitFormat,
			Format:         format,
			RawOutput:      cmd.Root().Bool("raw-output"),
			Title:          "beta:memory-stores list",
			Transform:      transform,
		})
	}
}

func handleBetaMemoryStoresDelete(ctx context.Context, cmd *cli.Command) error {
	client := anthropic.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("memory-store-id") && len(unusedArgs) > 0 {
		cmd.Set("memory-store-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatBrackets,
		EmptyBody,
		false,
	)
	if err != nil {
		return err
	}

	params := anthropic.BetaMemoryStoreDeleteParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Beta.MemoryStores.Delete(
		ctx,
		cmd.Value("memory-store-id").(string),
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
		Title:          "beta:memory-stores delete",
		Transform:      transform,
	})
}

func handleBetaMemoryStoresArchive(ctx context.Context, cmd *cli.Command) error {
	client := anthropic.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("memory-store-id") && len(unusedArgs) > 0 {
		cmd.Set("memory-store-id", unusedArgs[0])
		unusedArgs = unusedArgs[1:]
	}
	if len(unusedArgs) > 0 {
		return fmt.Errorf("Unexpected extra arguments: %v", unusedArgs)
	}

	options, err := flagOptions(
		cmd,
		apiquery.NestedQueryFormatBrackets,
		apiquery.ArrayQueryFormatBrackets,
		EmptyBody,
		false,
	)
	if err != nil {
		return err
	}

	params := anthropic.BetaMemoryStoreArchiveParams{}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Beta.MemoryStores.Archive(
		ctx,
		cmd.Value("memory-store-id").(string),
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
		Title:          "beta:memory-stores archive",
		Transform:      transform,
	})
}
