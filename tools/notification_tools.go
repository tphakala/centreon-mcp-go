package tools

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client"
)

// RegisterNotificationTools registers all notification policy tools.
func RegisterNotificationTools(s *mcp.Server, client *centreon.Client, logger *slog.Logger) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_notification_policy_host_get",
		Description: "Get the notification policy for a specific host.",
	}, notificationPolicyHostGetHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_notification_policy_service_get",
		Description: "Get the notification policy for a specific service on a host.",
	}, notificationPolicyServiceGetHandler(client, logger))
}

func notificationPolicyHostGetHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in HostIDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in HostIDInput) (*mcp.CallToolResult, any, error) {
		logger.Debug("centreon_notification_policy_host_get", "hostID", in.HostID)
		np, err := client.NotificationPolicies.GetForHost(ctx, in.HostID)
		if err != nil {
			logger.Error("failed: centreon_notification_policy_host_get", "error", err, "hostID", in.HostID)
			res, anyVal := errorResult("failed to get notification policy for host %d: %v", in.HostID, err)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(np)
		return res, anyVal, nil
	}
}

func notificationPolicyServiceGetHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in HostServiceInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in HostServiceInput) (*mcp.CallToolResult, any, error) {
		logger.Debug("centreon_notification_policy_service_get", "hostID", in.HostID, "serviceID", in.ServiceID)
		np, err := client.NotificationPolicies.GetForService(ctx, in.HostID, in.ServiceID)
		if err != nil {
			logger.Error("failed: centreon_notification_policy_service_get", "error", err, "hostID", in.HostID, "serviceID", in.ServiceID)
			res, anyVal := errorResult("failed to get notification policy for service (host=%d, service=%d): %v", in.HostID, in.ServiceID, err)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(np)
		return res, anyVal, nil
	}
}
