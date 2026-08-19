package tools

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client/v2"
	"github.com/tphakala/centreon-mcp-go/internal/redact"
)

// RegisterInfraTools registers all infrastructure tools.
func RegisterInfraTools(s *mcp.Server, client *centreon.Client, logger *slog.Logger) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_server_list",
		Description: "Retrieve the monitoring servers (pollers) that run checks and collect results across the Centreon platform, each with its identifier and configuration state. Use this to discover poller IDs before pushing stored configuration to one with centreon_poller_apply; it does not return check commands (centreon_command_list) or time periods (centreon_time_period_list). Supports name search and pagination (page 1, limit 30, max 100). Read-only.",
		Annotations: readOnlyTool("List monitoring servers"),
	}, serverListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_command_list",
		Description: "Retrieve the command definitions (checks, notifications, and other plugin invocations) that hosts and services reference to run their monitoring plugins. Use this to find command names and IDs when configuring monitoring; unlike centreon_server_list (pollers) or centreon_time_period_list (schedules), these are the executable command templates. Supports name search and pagination (page 1, limit 30, max 100). Read-only.",
		Annotations: readOnlyTool("List commands"),
	}, commandListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_time_period_list",
		Description: "Retrieve the time period definitions that control when checks run and notifications are sent, such as 24x7 or workhours schedules. Use this to find time period IDs, then centreon_time_period_get for the full weekday range detail of one; these are schedules, distinct from pollers (centreon_server_list) and commands (centreon_command_list). Supports name search and pagination (page 1, limit 30, max 100). Read-only.",
		Annotations: readOnlyTool("List time periods"),
	}, timePeriodListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_time_period_get",
		Description: "Fetch one time period by its numeric id, returning its name, alias, and full set of weekday time ranges. Use this once you have an id from centreon_time_period_list and need the complete schedule detail rather than a summary row. Requires the id parameter. Read-only.",
		Annotations: readOnlyTool("Get time period"),
	}, timePeriodGetHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_time_period_create",
		Description: "Define a new time period from a name and a list of weekday time ranges (alias optional), returning the new numeric id. Use this to add a schedule; to change one that already exists use centreon_time_period_update, and to review current schedules first use centreon_time_period_list. Run centreon_poller_apply afterward to push the configuration live. Writes to Centreon.",
		Annotations: createTool("Create time period"),
	}, timePeriodCreateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_time_period_update",
		Description: "Replace the time period identified by id with the supplied name, alias, weekday ranges, and inherited templates as a full overwrite (omitted optional fields are cleared). Use this to change a schedule that already exists; to add a brand-new one use centreon_time_period_create. Run centreon_poller_apply afterward to push the configuration live. Writes to Centreon.",
		Annotations: updateTool("Update time period"),
	}, timePeriodUpdateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_time_period_delete",
		Description: "Permanently remove the time period with the given id from the stored configuration. Use this to retire a schedule; to change it in place instead use centreon_time_period_update. Requires the id parameter; run centreon_poller_apply afterward to push the change live. Writes to Centreon.",
		Annotations: deleteTool("Delete time period"),
	}, timePeriodDeleteHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_poller_apply",
		Description: "Generate and reload the monitoring configuration for a single poller identified by pollerID, pushing stored config changes into the running engine and reloading that poller. Use this after create, update, or delete operations to make them take effect on one poller; to apply to every poller at once use centreon_poller_apply_all. Requires pollerID and may briefly reload the poller. Writes to Centreon.",
		Annotations: updateTool("Apply poller config"),
	}, pollerApplyHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_poller_apply_all",
		Description: "Generate and reload the monitoring configuration for every poller on the platform, pushing all pending stored config changes into the running engines. Use this to activate configuration changes fleet-wide after edits; to target just one poller by id use centreon_poller_apply. Takes no arguments and may briefly reload each poller. Writes to Centreon.",
		Annotations: updateTool("Apply all poller configs"),
	}, pollerApplyAllHandler(client, logger))
}

// TimePeriodDayInput represents a day range in a time period.
type TimePeriodDayInput struct {
	Day       int    `json:"day"        jsonschema:"Day of week (1=Monday through 7=Sunday)"`
	TimeRange string `json:"timeRange"  jsonschema:"Time range (e.g. 00:00-24:00)"`
}

// CreateTimePeriodInput is the input for the centreon_time_period_create tool.
type CreateTimePeriodInput struct {
	Name  string               `json:"name"                jsonschema:"Time period name"`
	Alias string               `json:"alias,omitempty"     jsonschema:"Time period alias"`
	Days  []TimePeriodDayInput `json:"days" jsonschema:"Day definitions (required, use empty array [] if none)"`
}

// UpdateTimePeriodInput is the input for the centreon_time_period_update tool.
type UpdateTimePeriodInput struct {
	ID        int                  `json:"id"                  jsonschema:"Time period ID"`
	Name      string               `json:"name"                jsonschema:"Time period name"`
	Alias     string               `json:"alias,omitempty"     jsonschema:"Time period alias"`
	Days      []TimePeriodDayInput `json:"days"                jsonschema:"Day definitions (required, use empty array [] if none)"`
	Templates []int                `json:"templates,omitempty" jsonschema:"Template IDs to inherit from"`
}

// PollerApplyInput is the input for the centreon_poller_apply tool.
type PollerApplyInput struct {
	PollerID int `json:"pollerID" jsonschema:"Poller (monitoring server) ID"`
}

func pollerApplyHandlerFn(
	fn func(context.Context, int) error,
	logger *slog.Logger,
) func(ctx context.Context, req *mcp.CallToolRequest, in PollerApplyInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in PollerApplyInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_poller_apply")
		logger.Info("centreon_poller_apply", "pollerID", in.PollerID)
		if err := fn(ctx, in.PollerID); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_poller_apply", "error", reason, "pollerID", in.PollerID)
			res, anyVal := errorResult("failed to apply configuration for poller %d: %s", in.PollerID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_poller_apply", "Applied configuration for poller %d", in.PollerID)
		return res, anyVal, nil
	}
}

func pollerApplyAllHandlerFn(
	fn func(context.Context) error,
	logger *slog.Logger,
) func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_poller_apply_all")
		logger.Info("centreon_poller_apply_all")
		if err := fn(ctx); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_poller_apply_all", "error", reason)
			res, anyVal := errorResult("failed to apply configuration for all pollers: %s", reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_poller_apply_all", "Applied configuration for all pollers")
		return res, anyVal, nil
	}
}

func pollerApplyHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in PollerApplyInput) (*mcp.CallToolResult, any, error) {
	return pollerApplyHandlerFn(client.MonitoringServers.GenerateAndReload, logger)
}

func pollerApplyAllHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
	return pollerApplyAllHandlerFn(client.MonitoringServers.GenerateAndReloadAll, logger)
}

func serverListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_server_list", in, client.MonitoringServers.List)
	}
}

func commandListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_command_list", in, client.Commands.List)
	}
}

func timePeriodListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_time_period_list", in, client.TimePeriods.List)
	}
}

func timePeriodGetHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_time_period_get")
		logger.Debug("centreon_time_period_get", "id", in.ID)
		tp, err := client.TimePeriods.Get(ctx, in.ID)
		if err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_time_period_get", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to get time period %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(tp)
		return res, anyVal, nil
	}
}

func timePeriodCreateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in CreateTimePeriodInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateTimePeriodInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_time_period_create")
		logger.Info("centreon_time_period_create", "name", in.Name)
		days := make([]centreon.TimePeriodDay, 0, len(in.Days))
		for _, d := range in.Days {
			days = append(days, centreon.TimePeriodDay{Day: d.Day, TimeRange: d.TimeRange})
		}
		id, err := client.TimePeriods.Create(ctx, &centreon.CreateTimePeriodRequest{
			Name:  in.Name,
			Alias: in.Alias,
			Days:  days,
		})
		if err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_time_period_create", "error", reason, "name", in.Name)
			res, anyVal := errorResult("failed to create time period %q: %s", in.Name, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_time_period_create", "Created time period with ID %d", id)
		return res, anyVal, nil
	}
}

func timePeriodUpdateHandlerFn(
	fn func(context.Context, int, *centreon.UpdateTimePeriodRequest) error,
	logger *slog.Logger,
) func(ctx context.Context, req *mcp.CallToolRequest, in UpdateTimePeriodInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in UpdateTimePeriodInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_time_period_update")
		logger.Info("centreon_time_period_update", "id", in.ID)
		days := make([]centreon.TimePeriodDay, 0, len(in.Days))
		for _, d := range in.Days {
			days = append(days, centreon.TimePeriodDay{Day: d.Day, TimeRange: d.TimeRange})
		}
		if err := fn(ctx, in.ID, &centreon.UpdateTimePeriodRequest{
			Name:      in.Name,
			Alias:     in.Alias,
			Days:      days,
			Templates: in.Templates,
		}); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_time_period_update", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to update time period %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_time_period_update", "Updated time period %d", in.ID)
		return res, anyVal, nil
	}
}

func timePeriodUpdateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in UpdateTimePeriodInput) (*mcp.CallToolResult, any, error) {
	return timePeriodUpdateHandlerFn(client.TimePeriods.Update, logger)
}

func timePeriodDeleteHandlerFn(
	fn func(context.Context, int) error,
	logger *slog.Logger,
) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_time_period_delete")
		logger.Info("centreon_time_period_delete", "id", in.ID)
		if err := fn(ctx, in.ID); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_time_period_delete", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to delete time period %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_time_period_delete", "Deleted time period %d", in.ID)
		return res, anyVal, nil
	}
}

func timePeriodDeleteHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return timePeriodDeleteHandlerFn(client.TimePeriods.Delete, logger)
}
