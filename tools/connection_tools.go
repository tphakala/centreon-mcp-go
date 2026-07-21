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
		Description: "Verify that the configured Centreon API credentials authenticate and the API is reachable by fetching host status counts. Takes no arguments; call this first to confirm connectivity before using other tools. Read-only.",
		Annotations: readOnlyTool("Test connection"),
	}, connectionTestHandler(client, logger))
}

func connectionTestHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_connection_test")
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
