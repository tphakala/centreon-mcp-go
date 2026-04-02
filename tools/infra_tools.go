package tools

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client"
)

// RegisterInfraTools registers all infrastructure tools.
func RegisterInfraTools(s *mcp.Server, client *centreon.Client, logger *slog.Logger) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_server_list",
		Description: "List monitoring servers (pollers). Supports pagination and name filtering.",
	}, serverListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_command_list",
		Description: "List check commands. Supports pagination and name filtering.",
	}, commandListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_time_period_list",
		Description: "List time period configurations. Supports pagination and name filtering.",
	}, timePeriodListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_time_period_get",
		Description: "Get a single time period configuration by ID.",
	}, timePeriodGetHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_time_period_create",
		Description: "Create a new time period configuration.",
	}, timePeriodCreateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_poller_apply",
		Description: "Apply configuration (generate and reload) for a specific monitoring server (poller) by ID.",
	}, pollerApplyHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_poller_apply_all",
		Description: "Apply configuration (generate and reload) for all monitoring servers (pollers).",
	}, pollerApplyAllHandler(client, logger))
}

// TimePeriodDayInput represents a day range in a time period.
type TimePeriodDayInput struct {
	Day       int    `json:"day"        jsonschema:"Day of week (1=Monday through 7=Sunday)"`
	TimeRange string `json:"timeRange"  jsonschema:"Time range (e.g. 00:00-24:00)"`
}

// CreateTimePeriodInput is the input for the centreon_time_period_create tool.
type CreateTimePeriodInput struct {
	Name      string               `json:"name"                jsonschema:"Time period name"`
	Alias     string               `json:"alias,omitempty"     jsonschema:"Time period alias"`
	Days []TimePeriodDayInput `json:"days" jsonschema:"Day definitions (required, use empty array [] if none)"`
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
			logger.Error("failed: centreon_poller_apply", "error", err, "pollerID", in.PollerID)
			res, anyVal := errorResult("failed to apply configuration for poller %d: %v", in.PollerID, err)
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
			logger.Error("failed: centreon_poller_apply_all", "error", err)
			res, anyVal := errorResult("failed to apply configuration for all pollers: %v", err)
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
			logger.Error("failed: centreon_time_period_get", "error", err, "id", in.ID)
			res, anyVal := errorResult("failed to get time period %d: %v", in.ID, err)
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
			logger.Error("failed: centreon_time_period_create", "error", err, "name", in.Name)
			res, anyVal := errorResult("failed to create time period %q: %v", in.Name, err)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_time_period_create", "Created time period with ID %d", id)
		return res, anyVal, nil
	}
}
