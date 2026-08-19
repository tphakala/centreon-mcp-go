package tools

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client/v2"
	"github.com/tphakala/centreon-mcp-go/internal/redact"
)

// RegisterNotificationTools registers all notification policy tools.
func RegisterNotificationTools(s *mcp.Server, client *centreon.Client, logger *slog.Logger) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_notification_policy_host_get",
		Description: "Fetch the configured notification policy for one host identified by its numeric hostID, reporting whether notifications are enabled and which users and contact groups receive them. Use this for a host; for a specific service on that host use centreon_notification_policy_service_get, and to resolve the referenced recipients use centreon_user_list or centreon_contact_group_list. Reads stored configuration, not live monitoring state. Read-only.",
		Annotations: readOnlyTool("Get host notification policy"),
	}, notificationPolicyHostGetHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_notification_policy_service_get",
		Description: "Fetch the configured notification policy for one service, identified by both its numeric hostID and serviceID, reporting whether notifications are enabled and which users and contact groups receive them. Use this for a service; for the host-level policy use centreon_notification_policy_host_get, and to resolve the referenced recipients use centreon_user_list or centreon_contact_group_list. Reads stored configuration, not live monitoring state. Read-only.",
		Annotations: readOnlyTool("Get service notification policy"),
	}, notificationPolicyServiceGetHandler(client, logger))
}

func notificationPolicyHostGetHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in HostIDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in HostIDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_notification_policy_host_get")
		logger.Debug("centreon_notification_policy_host_get", "hostID", in.HostID)
		np, err := client.NotificationPolicies.GetForHost(ctx, in.HostID)
		if err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_notification_policy_host_get", "error", reason, "hostID", in.HostID)
			res, anyVal := errorResult("failed to get notification policy for host %d: %s", in.HostID, reason)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(np)
		return res, anyVal, nil
	}
}

func notificationPolicyServiceGetHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in HostServiceInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in HostServiceInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_notification_policy_service_get")
		logger.Debug("centreon_notification_policy_service_get", "hostID", in.HostID, "serviceID", in.ServiceID)
		np, err := client.NotificationPolicies.GetForService(ctx, in.HostID, in.ServiceID)
		if err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_notification_policy_service_get", "error", reason, "hostID", in.HostID, "serviceID", in.ServiceID)
			res, anyVal := errorResult("failed to get notification policy for service (host=%d, service=%d): %s", in.HostID, in.ServiceID, reason)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(np)
		return res, anyVal, nil
	}
}
