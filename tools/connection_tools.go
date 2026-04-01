package tools

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client"
)

// RegisterConnectionTools registers all connection tools.
func RegisterConnectionTools(s *mcp.Server, client *centreon.Client, logger *slog.Logger) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_connection_test",
		Description: "Test connectivity to the Centreon API by performing a lightweight status check.",
	}, connectionTestHandler(client, logger))
}

func connectionTestHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		logger.Debug("centreon_connection_test")
		_, err := client.MonitoringHosts.StatusCounts(ctx)
		if err != nil {
			logger.Error("failed: centreon_connection_test", "error", err)
			res, anyVal := errorResult("connection failed: %v", err)
			return res, anyVal, nil
		}
		res, anyVal := textResult("Connection successful")
		return res, anyVal, nil
	}
}
