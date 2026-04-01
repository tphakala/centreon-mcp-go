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
}

// CreateTimePeriodInput is the input for the centreon_time_period_create tool.
type CreateTimePeriodInput struct {
	Name  string `json:"name"            jsonschema:"Time period name"`
	Alias string `json:"alias,omitempty" jsonschema:"Time period alias"`
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
		id, err := client.TimePeriods.Create(ctx, centreon.CreateTimePeriodRequest{
			Name:  in.Name,
			Alias: in.Alias,
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
