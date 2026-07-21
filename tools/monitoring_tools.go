package tools

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client"
)

// RegisterMonitoringTools registers all monitoring tools.
func RegisterMonitoringTools(s *mcp.Server, client *centreon.Client, logger *slog.Logger) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_host_list",
		Description: "Retrieve the real-time monitoring state of all hosts (current status, plugin output, acknowledgement, and downtime flags) as reported by the pollers. Use this for live health across many hosts; for one host by ID use centreon_monitoring_host_get, and for stored host configuration instead of live state use centreon_host_list. Reflects current engine state, not saved config; paginates with page (default 1) and limit (default 30, max 100). Read-only.",
		Annotations: readOnlyTool("List monitoring hosts"),
	}, monitoringHostListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_host_get",
		Description: "Fetch the real-time monitoring state of one host by its numeric monitoring ID, including current status, last check, plugin output, and active problem flags. Use this when you already know the host ID; to browse or page through many hosts use centreon_monitoring_host_list, and for the alternative unified resource view use centreon_monitoring_resource_host_get. Requires id; returns live engine state rather than stored configuration. Read-only.",
		Annotations: readOnlyTool("Get monitoring host"),
	}, monitoringHostGetHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_host_services",
		Description: "List the real-time monitoring state of every service attached to one host, given its hostID. Use this to see all checks on a single host; to list services across all hosts use centreon_monitoring_service_list, and to fetch one specific service use centreon_monitoring_resource_service_get. Requires hostID; paginates with page (default 1) and limit (default 30, max 100) and reflects current engine state, not stored config. Read-only.",
		Annotations: readOnlyTool("List host services"),
	}, monitoringHostServicesHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_host_timeline",
		Description: "Retrieve the chronological event history for one host (state changes, notifications, acknowledgements, downtime, and comments) given its hostID. Use this to investigate what happened over time; for the current point-in-time snapshot instead use centreon_monitoring_host_get. Requires hostID; paginates with page (default 1) and limit (default 30, max 100) over live monitoring events. Read-only.",
		Annotations: readOnlyTool("Get host timeline"),
	}, monitoringHostTimelineHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_host_status_counts",
		Description: "Summarize how many hosts are currently in each monitoring state (up, down, unreachable, pending) as a single aggregate, taking no arguments. Use this for a quick host health total; for the per-host detail behind the numbers use centreon_monitoring_host_list, and for hosts plus services plus servers in one call use centreon_platform_status. Reflects current engine state, not stored configuration. Read-only.",
		Annotations: readOnlyTool("Host status counts"),
	}, monitoringHostStatusCountsHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_service_list",
		Description: "Retrieve the real-time monitoring state of all services across every host (current status, plugin output, acknowledgement, and downtime flags). Use this for live health across many services; to limit to one host's services use centreon_monitoring_host_services, and for stored service configuration instead of live state use centreon_service_list. Paginates with page (default 1) and limit (default 30, max 100) and reflects current engine state, not saved config. Read-only.",
		Annotations: readOnlyTool("List monitoring services"),
	}, monitoringServiceListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_service_status_counts",
		Description: "Summarize how many services are currently in each monitoring state (ok, warning, critical, unknown, pending) as a single aggregate, taking no arguments. Use this for a quick service health total; for the host-side equivalent use centreon_monitoring_host_status_counts, and for the per-service detail behind the numbers use centreon_monitoring_service_list. Reflects current engine state, not stored configuration. Read-only.",
		Annotations: readOnlyTool("Service status counts"),
	}, monitoringServiceStatusCountsHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_resource_list",
		Description: "List hosts and services together in Centreon's unified resource view, each with its real-time monitoring status. Use this when you want hosts and services in one combined stream; for only hosts use centreon_monitoring_host_list and for only services use centreon_monitoring_service_list. Paginates with page (default 1) and limit (default 30, max 100) and reflects current engine state, not stored configuration. Read-only.",
		Annotations: readOnlyTool("List monitoring resources"),
	}, monitoringResourceListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_resource_host_get",
		Description: "Fetch one host's real-time monitoring status through Centreon's unified resource API, given its numeric id. Use this for the resource-view representation of a host; for the host-endpoint representation of the same live state use centreon_monitoring_host_get. Requires id; returns live engine state rather than stored configuration. Read-only.",
		Annotations: readOnlyTool("Get resource host"),
	}, monitoringResourceHostGetHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_resource_service_get",
		Description: "Fetch one service's real-time monitoring status through Centreon's unified resource API, given both its hostID and serviceID. This is the only tool that returns a single monitoring service by ID; to list services instead use centreon_monitoring_service_list or, scoped to one host, centreon_monitoring_host_services. Requires hostID and serviceID; returns live engine state rather than stored configuration. Read-only.",
		Annotations: readOnlyTool("Get resource service"),
	}, monitoringResourceServiceGetHandler(client, logger))
}

func monitoringHostListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in MonitoringListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in MonitoringListInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_monitoring_host_list")
		logger.Debug("centreon_monitoring_host_list", "page", in.Page, "limit", in.Limit)
		opts := buildMonitoringListOptions(in)
		resp, err := client.MonitoringHosts.List(ctx, opts...)
		if err != nil {
			logger.Error("failed: centreon_monitoring_host_list", "error", err)
			res, anyVal := errorResult("failed: centreon_monitoring_host_list: %v", err)
			return res, anyVal, nil
		}
		logger.Debug("centreon_monitoring_host_list completed", "results", len(resp.Result), "total", resp.Meta.Total)
		res, anyVal := jsonResult(resp)
		return res, anyVal, nil
	}
}

func monitoringHostGetHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_monitoring_host_get")
		logger.Debug("centreon_monitoring_host_get", "id", in.ID)
		host, err := client.MonitoringHosts.Get(ctx, in.ID)
		if err != nil {
			logger.Error("failed: centreon_monitoring_host_get", "error", err, "id", in.ID)
			res, anyVal := errorResult("failed to get monitoring host %d: %v", in.ID, err)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(host)
		return res, anyVal, nil
	}
}

func monitoringHostServicesHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in MonitoringHostIDListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in MonitoringHostIDListInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_monitoring_host_services")
		logger.Debug("centreon_monitoring_host_services", "hostID", in.HostID, "page", in.Page, "limit", in.Limit)
		listIn := MonitoringListInput{Page: in.Page, Limit: in.Limit}
		opts := buildMonitoringListOptions(listIn)
		resp, err := client.MonitoringHosts.Services(ctx, in.HostID, opts...)
		if err != nil {
			logger.Error("failed: centreon_monitoring_host_services", "error", err, "hostID", in.HostID)
			res, anyVal := errorResult("failed to list services for host %d: %v", in.HostID, err)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(resp)
		return res, anyVal, nil
	}
}

func monitoringHostTimelineHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in MonitoringHostIDListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in MonitoringHostIDListInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_monitoring_host_timeline")
		logger.Debug("centreon_monitoring_host_timeline", "hostID", in.HostID, "page", in.Page, "limit", in.Limit)
		listIn := MonitoringListInput{Page: in.Page, Limit: in.Limit}
		opts := buildMonitoringListOptions(listIn)
		resp, err := client.MonitoringHosts.Timeline(ctx, in.HostID, opts...)
		if err != nil {
			logger.Error("failed: centreon_monitoring_host_timeline", "error", err, "hostID", in.HostID)
			res, anyVal := errorResult("failed to get timeline for host %d: %v", in.HostID, err)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(resp)
		return res, anyVal, nil
	}
}

func monitoringHostStatusCountsHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_monitoring_host_status_counts")
		logger.Debug("centreon_monitoring_host_status_counts")
		counts, err := client.MonitoringHosts.StatusCounts(ctx)
		if err != nil {
			logger.Error("failed: centreon_monitoring_host_status_counts", "error", err)
			res, anyVal := errorResult("failed to get host status counts: %v", err)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(counts)
		return res, anyVal, nil
	}
}

func monitoringServiceListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in MonitoringListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in MonitoringListInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_monitoring_service_list")
		logger.Debug("centreon_monitoring_service_list", "page", in.Page, "limit", in.Limit)
		opts := buildMonitoringListOptions(in)
		resp, err := client.MonitoringServices.List(ctx, opts...)
		if err != nil {
			logger.Error("failed: centreon_monitoring_service_list", "error", err)
			res, anyVal := errorResult("failed: centreon_monitoring_service_list: %v", err)
			return res, anyVal, nil
		}
		logger.Debug("centreon_monitoring_service_list completed", "results", len(resp.Result), "total", resp.Meta.Total)
		res, anyVal := jsonResult(resp)
		return res, anyVal, nil
	}
}

func monitoringServiceStatusCountsHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_monitoring_service_status_counts")
		logger.Debug("centreon_monitoring_service_status_counts")
		counts, err := client.MonitoringServices.StatusCounts(ctx)
		if err != nil {
			logger.Error("failed: centreon_monitoring_service_status_counts", "error", err)
			res, anyVal := errorResult("failed to get service status counts: %v", err)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(counts)
		return res, anyVal, nil
	}
}

func monitoringResourceListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in MonitoringListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in MonitoringListInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_monitoring_resource_list")
		logger.Debug("centreon_monitoring_resource_list", "page", in.Page, "limit", in.Limit)
		opts := buildMonitoringListOptions(in)
		resp, err := client.Monitoring.List(ctx, opts...)
		if err != nil {
			logger.Error("failed: centreon_monitoring_resource_list", "error", err)
			res, anyVal := errorResult("failed: centreon_monitoring_resource_list: %v", err)
			return res, anyVal, nil
		}
		logger.Debug("centreon_monitoring_resource_list completed", "results", len(resp.Result), "total", resp.Meta.Total)
		res, anyVal := jsonResult(resp)
		return res, anyVal, nil
	}
}

func monitoringResourceHostGetHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_monitoring_resource_host_get")
		logger.Debug("centreon_monitoring_resource_host_get", "id", in.ID)
		host, err := client.Monitoring.GetHost(ctx, in.ID)
		if err != nil {
			logger.Error("failed: centreon_monitoring_resource_host_get", "error", err, "id", in.ID)
			res, anyVal := errorResult("failed to get monitoring resource host %d: %v", in.ID, err)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(host)
		return res, anyVal, nil
	}
}

func monitoringResourceServiceGetHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in HostServiceInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in HostServiceInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_monitoring_resource_service_get")
		logger.Debug("centreon_monitoring_resource_service_get", "hostID", in.HostID, "serviceID", in.ServiceID)
		svc, err := client.Monitoring.GetService(ctx, in.HostID, in.ServiceID)
		if err != nil {
			logger.Error("failed: centreon_monitoring_resource_service_get", "error", err, "hostID", in.HostID, "serviceID", in.ServiceID)
			res, anyVal := errorResult("failed to get monitoring resource service (host=%d, service=%d): %v", in.HostID, in.ServiceID, err)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(svc)
		return res, anyVal, nil
	}
}
