package tools

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client"
)

// RegisterConnectionTools registers all connection tools. host is the
// credential-redacted Centreon host named in the success result; the caller
// supplies it already redacted (see RegisterAll).
func RegisterConnectionTools(s *mcp.Server, client *centreon.Client, logger *slog.Logger, host string) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_connection_test",
		Description: "Verify that the configured Centreon API credentials authenticate and the API is reachable by fetching host status counts. On success the result names the connected Centreon host, with any embedded credentials redacted. Takes no arguments; call this first to confirm connectivity before using other tools. Read-only.",
		Annotations: readOnlyTool("Test connection"),
	}, connectionTestHandler(client, logger, host))
}

func connectionTestHandler(client *centreon.Client, logger *slog.Logger, host string) func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
	return connectionTestHandlerFn(client.MonitoringHosts.StatusCounts, logger, host)
}

// connectionTestHandlerFn builds the connection-test handler from the status
// fetch as a function value, so it is unit-testable without a live client. host
// is the already-redacted display host named on success; it is never included on
// the failure path, and no credential ever reaches it.
func connectionTestHandlerFn(fetchStatus func(context.Context) (*centreon.HostStatusCount, error), logger *slog.Logger, host string) func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_connection_test")
		logger.Debug("centreon_connection_test")
		_, err := fetchStatus(ctx)
		if err != nil {
			logger.Error("failed: centreon_connection_test", "error", err)
			res, anyVal := errorResult("connection failed: %v", err)
			return res, anyVal, nil
		}
		res, anyVal := textResult("Connection successful (host: %s)", host)
		return res, anyVal, nil
	}
}
