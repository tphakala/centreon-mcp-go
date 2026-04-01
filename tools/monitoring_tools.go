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
		Description: "List monitored hosts with real-time status. Supports pagination and name filtering.",
	}, monitoringHostListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_host_get",
		Description: "Get a single monitored host by ID with real-time status details.",
	}, monitoringHostGetHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_host_services",
		Description: "List all services for a specific monitored host.",
	}, monitoringHostServicesHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_host_timeline",
		Description: "Get timeline events for a specific monitored host.",
	}, monitoringHostTimelineHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_host_status_counts",
		Description: "Get host counts by state (up, down, unreachable, pending).",
	}, monitoringHostStatusCountsHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_service_list",
		Description: "List monitored services with real-time status. Supports pagination and name filtering.",
	}, monitoringServiceListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_service_status_counts",
		Description: "Get service counts by state (ok, warning, critical, unknown, pending).",
	}, monitoringServiceStatusCountsHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_resource_list",
		Description: "List all monitored resources (hosts and services) in a unified view.",
	}, monitoringResourceListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_resource_host_get",
		Description: "Get a single host via the unified monitoring resource API.",
	}, monitoringResourceHostGetHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_resource_service_get",
		Description: "Get a single service via the unified monitoring resource API. This is the only way to get a single monitoring service by ID.",
	}, monitoringResourceServiceGetHandler(client, logger))
}

func monitoringHostListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_monitoring_host_list", in, client.MonitoringHosts.List)
	}
}

func monitoringHostGetHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
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

func monitoringHostServicesHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in HostIDListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in HostIDListInput) (*mcp.CallToolResult, any, error) {
		logger.Debug("centreon_monitoring_host_services", "hostID", in.HostID, "page", in.Page, "limit", in.Limit)
		listIn := ListInput{Page: in.Page, Limit: in.Limit, Search: in.Search}
		opts := buildListOptions(listIn)
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

func monitoringHostTimelineHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in HostIDListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in HostIDListInput) (*mcp.CallToolResult, any, error) {
		logger.Debug("centreon_monitoring_host_timeline", "hostID", in.HostID, "page", in.Page, "limit", in.Limit)
		listIn := ListInput{Page: in.Page, Limit: in.Limit, Search: in.Search}
		opts := buildListOptions(listIn)
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

func monitoringServiceListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_monitoring_service_list", in, client.MonitoringServices.List)
	}
}

func monitoringServiceStatusCountsHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
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

func monitoringResourceListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_monitoring_resource_list", in, client.Monitoring.List)
	}
}

func monitoringResourceHostGetHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
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
