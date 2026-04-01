package tools

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client"
)

// PlatformStatus combines host status counts, service status counts, and monitoring servers.
type PlatformStatus struct {
	Hosts    *centreon.HostStatusCount                              `json:"hosts"`
	Services *centreon.ServiceStatusCount                           `json:"services"`
	Servers  *centreon.ListResponse[centreon.MonitoringServer]      `json:"servers"`
}

// RegisterStatusTools registers all platform status tools.
func RegisterStatusTools(s *mcp.Server, client *centreon.Client, logger *slog.Logger) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_platform_status",
		Description: "Get a combined platform overview: host status counts, service status counts, and monitoring servers.",
	}, platformStatusHandler(client, logger))
}

func platformStatusHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_platform_status")
		logger.Debug("centreon_platform_status")

		hosts, err := client.MonitoringHosts.StatusCounts(ctx)
		if err != nil {
			logger.Error("failed: centreon_platform_status (hosts)", "error", err)
			res, anyVal := errorResult("failed to get host status counts: %v", err)
			return res, anyVal, nil
		}

		services, err := client.MonitoringServices.StatusCounts(ctx)
		if err != nil {
			logger.Error("failed: centreon_platform_status (services)", "error", err)
			res, anyVal := errorResult("failed to get service status counts: %v", err)
			return res, anyVal, nil
		}

		servers, err := client.MonitoringServers.List(ctx)
		if err != nil {
			logger.Error("failed: centreon_platform_status (servers)", "error", err)
			res, anyVal := errorResult("failed to get monitoring servers: %v", err)
			return res, anyVal, nil
		}

		status := PlatformStatus{
			Hosts:    hosts,
			Services: services,
			Servers:  servers,
		}
		res, anyVal := jsonResult(status)
		return res, anyVal, nil
	}
}
