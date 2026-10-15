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

var betaOrganizationTelemetryDestinationsWorkspacesList = cli.Command{
	Name:    "list",
	Usage:   "List the workspaces on a destination's workspace list, newest entry first.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "destination-id",
			Usage:     "ID of the destination (`tdst_...`).",
			Required:  true,
			PathParam: "destination_id",
		},
		&requestflag.Flag[int64]{
			Name:      "limit",
			Usage:     "Number of entries to return per page, between 1 and 1,000. Defaults to 20.",
			QueryPath: "limit",
		},
		&requestflag.Flag[string]{
			Name:      "page",
			Usage:     "Opaque cursor from a previous response's `next_page`. Leave it out to get the first page.",
			QueryPath: "page",
		},
		&requestflag.Flag[[]string]{
			Name:       "beta",
			Usage:      "This endpoint is in beta: requests must send `telemetry-destinations-2026-08-11` in this header.",
			HeaderPath: "anthropic-beta",
		},
		&requestflag.Flag[int64]{
			Name:  "max-items",
			Usage: "The maximum number of items to return (use -1 for unlimited).",
		},
	},
	Action:          handleBetaOrganizationTelemetryDestinationsWorkspacesList,
	HideHelpCommand: true,
}

var betaOrganizationTelemetryDestinationsWorkspacesAdd = cli.Command{
	Name:    "add",
	Usage:   "Add one workspace to a destination's workspace list.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "destination-id",
			Usage:     "ID of the destination (`tdst_...`).",
			Required:  true,
			PathParam: "destination_id",
		},
		&requestflag.Flag[string]{
			Name:      "workspace-id",
			Usage:     "ID of the workspace to add (`wrkspc_...`).",
			Required:  true,
			PathParam: "workspace_id",
		},
		&requestflag.Flag[[]string]{
			Name:       "beta",
			Usage:      "This endpoint is in beta: requests must send `telemetry-destinations-2026-08-11` in this header.",
			HeaderPath: "anthropic-beta",
		},
	},
	Action:          handleBetaOrganizationTelemetryDestinationsWorkspacesAdd,
	HideHelpCommand: true,
}

var betaOrganizationTelemetryDestinationsWorkspacesRemove = cli.Command{
	Name:    "remove",
	Usage:   "Remove one workspace from a destination's workspace list.",
	Suggest: true,
	Flags: []cli.Flag{
		&requestflag.Flag[string]{
			Name:      "destination-id",
			Usage:     "ID of the destination (`tdst_...`).",
			Required:  true,
			PathParam: "destination_id",
		},
		&requestflag.Flag[string]{
			Name:      "workspace-id",
			Usage:     "ID of the workspace to remove (`wrkspc_...`).",
			Required:  true,
			PathParam: "workspace_id",
		},
		&requestflag.Flag[[]string]{
			Name:       "beta",
			Usage:      "This endpoint is in beta: requests must send `telemetry-destinations-2026-08-11` in this header.",
			HeaderPath: "anthropic-beta",
		},
	},
	Action:          handleBetaOrganizationTelemetryDestinationsWorkspacesRemove,
	HideHelpCommand: true,
}

func handleBetaOrganizationTelemetryDestinationsWorkspacesList(ctx context.Context, cmd *cli.Command) error {
	client := anthropic.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("destination-id") && len(unusedArgs) > 0 {
		cmd.Set("destination-id", unusedArgs[0])
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

	params := anthropic.BetaOrganizationTelemetryDestinationWorkspaceListParams{}

	format := "explore"
	explicitFormat := cmd.Root().IsSet("format")
	if explicitFormat {
		format = cmd.Root().String("format")
	}
	transform := cmd.Root().String("transform")
	if format == "raw" {
		var res []byte
		options = append(options, option.WithResponseBodyInto(&res))
		_, err = client.Beta.Organization.TelemetryDestinations.Workspaces.List(
			ctx,
			cmd.Value("destination-id").(string),
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
			Title:          "beta:organization:telemetry-destinations:workspaces list",
			Transform:      transform,
		})
	} else {
		iter := client.Beta.Organization.TelemetryDestinations.Workspaces.ListAutoPaging(
			ctx,
			cmd.Value("destination-id").(string),
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
			Title:          "beta:organization:telemetry-destinations:workspaces list",
			Transform:      transform,
		})
	}
}

func handleBetaOrganizationTelemetryDestinationsWorkspacesAdd(ctx context.Context, cmd *cli.Command) error {
	client := anthropic.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("workspace-id") && len(unusedArgs) > 0 {
		cmd.Set("workspace-id", unusedArgs[0])
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

	params := anthropic.BetaOrganizationTelemetryDestinationWorkspaceAddParams{
		DestinationID: cmd.Value("destination-id").(string),
	}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Beta.Organization.TelemetryDestinations.Workspaces.Add(
		ctx,
		cmd.Value("workspace-id").(string),
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
		Title:          "beta:organization:telemetry-destinations:workspaces add",
		Transform:      transform,
	})
}

func handleBetaOrganizationTelemetryDestinationsWorkspacesRemove(ctx context.Context, cmd *cli.Command) error {
	client := anthropic.NewClient(getDefaultRequestOptions(cmd)...)
	unusedArgs := cmd.Args().Slice()
	if !cmd.IsSet("workspace-id") && len(unusedArgs) > 0 {
		cmd.Set("workspace-id", unusedArgs[0])
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

	params := anthropic.BetaOrganizationTelemetryDestinationWorkspaceRemoveParams{
		DestinationID: cmd.Value("destination-id").(string),
	}

	var res []byte
	options = append(options, option.WithResponseBodyInto(&res))
	_, err = client.Beta.Organization.TelemetryDestinations.Workspaces.Remove(
		ctx,
		cmd.Value("workspace-id").(string),
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
		Title:          "beta:organization:telemetry-destinations:workspaces remove",
		Transform:      transform,
	})
}
