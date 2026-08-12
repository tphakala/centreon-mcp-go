package tools

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client"
	"github.com/tphakala/centreon-mcp-go/internal/redact"
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
		Description: "List hosts and services together in Centreon's unified resource view, each with its real-time monitoring status. Use this when you want hosts and services in one combined stream; for only hosts use centreon_monitoring_host_list and for only services use centreon_monitoring_service_list. Optional filters: search (resource name, like match), hostName (a host and its services, like match), monitoringServer (exact poller name), plus sortBy (name, status, last_status_change) with sortOrder (ASC or DESC). Paginates with page (default 1) and limit (default 30, max 100) and reflects current engine state, not stored configuration. Read-only.",
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
			reason := redact.Reason(err)
			logger.Error("failed: centreon_monitoring_host_list", "error", reason)
			res, anyVal := errorResult("failed: centreon_monitoring_host_list: %s", reason)
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
			reason := redact.Reason(err)
			logger.Error("failed: centreon_monitoring_host_get", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to get monitoring host %d: %s", in.ID, reason)
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
			reason := redact.Reason(err)
			logger.Error("failed: centreon_monitoring_host_services", "error", reason, "hostID", in.HostID)
			res, anyVal := errorResult("failed to list services for host %d: %s", in.HostID, reason)
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
			reason := redact.Reason(err)
			logger.Error("failed: centreon_monitoring_host_timeline", "error", reason, "hostID", in.HostID)
			res, anyVal := errorResult("failed to get timeline for host %d: %s", in.HostID, reason)
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
			reason := redact.Reason(err)
			logger.Error("failed: centreon_monitoring_host_status_counts", "error", reason)
			res, anyVal := errorResult("failed to get host status counts: %s", reason)
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
			reason := redact.Reason(err)
			logger.Error("failed: centreon_monitoring_service_list", "error", reason)
			res, anyVal := errorResult("failed: centreon_monitoring_service_list: %s", reason)
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
			reason := redact.Reason(err)
			logger.Error("failed: centreon_monitoring_service_status_counts", "error", reason)
			res, anyVal := errorResult("failed to get service status counts: %s", reason)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(counts)
		return res, anyVal, nil
	}
}

// searchFieldName is the Centreon "name" search field, shared by the config-endpoint
// search (buildListOptions) and the monitoring/resources search; sortDirAsc and
// sortDirDesc are the accepted sort directions.
const (
	searchFieldName = "name"
	sortDirAsc      = "ASC"
	sortDirDesc     = "DESC"
)

// resourceSortFields maps user-facing sort keys to the Centreon monitoring/resources
// sort fields. "status" sorts by severity. Field names verified against centreon-web
// DbReadResourceRepository.php; NOT measured against a live instance.
var resourceSortFields = map[string]string{
	searchFieldName:      searchFieldName,
	"status":             "status_severity_code",
	"last_status_change": "last_status_change",
}

// MonitoringResourceListInput is the input for centreon_monitoring_resource_list.
// The unified resources endpoint honors search and sort_by, so this tool exposes
// name, host and poller filters plus sorting, unlike the other monitoring list tools.
type MonitoringResourceListInput struct {
	Page             int    `json:"page,omitempty"             jsonschema:"Page number (default 1)"`
	Limit            int    `json:"limit,omitempty"            jsonschema:"Results per page (default 30, max 100)"`
	Search           string `json:"search,omitempty"           jsonschema:"Filter by resource name (like match on host and service names)"`
	HostName         string `json:"hostName,omitempty"         jsonschema:"Filter by host name (like match); matches the host and every service on it"`
	MonitoringServer string `json:"monitoringServer,omitempty" jsonschema:"Filter by monitoring server (poller) name (exact match)"`
	SortBy           string `json:"sortBy,omitempty"           jsonschema:"Sort field: name, status, or last_status_change (status sorts by severity)"`
	SortOrder        string `json:"sortOrder,omitempty"        jsonschema:"Sort direction: ASC (default) or DESC; requires sortBy"`
}

// buildMonitoringResourceListOptions converts a MonitoringResourceListInput into
// centreon.ListOption values for the unified monitoring/resources endpoint. It
// returns an error for an invalid sort field or direction so the handler can
// reject the request before calling the API.
func buildMonitoringResourceListOptions(in *MonitoringResourceListInput) ([]centreon.ListOption, error) {
	opts := pagingOptions(in.Page, in.Limit)
	if f := buildResourceSearchFilter(in); f != nil {
		opts = append(opts, centreon.WithSearch(f))
	}
	return appendResourceSort(opts, in.SortBy, in.SortOrder)
}

// buildResourceSearchFilter combines the optional name, host and poller filters in
// a fixed order for deterministic output. It returns nil when no filter is set.
func buildResourceSearchFilter(in *MonitoringResourceListInput) centreon.Filter {
	var filters []centreon.Filter
	if s := strings.TrimSpace(in.Search); s != "" {
		filters = append(filters, centreon.Lk(searchFieldName, wrapLikePattern(s)))
	}
	if s := strings.TrimSpace(in.HostName); s != "" {
		filters = append(filters, centreon.Lk("h.name", wrapLikePattern(s)))
	}
	if s := strings.TrimSpace(in.MonitoringServer); s != "" {
		filters = append(filters, centreon.Eq("monitoring_server_name", s))
	}
	switch len(filters) {
	case 0:
		return nil
	case 1:
		return filters[0]
	default:
		return centreon.And(filters...)
	}
}

// appendResourceSort appends a WithSort option for the given sort field and
// direction. It leaves opts unchanged when sortBy is empty, and errors on an
// unknown field, an invalid direction, or a direction supplied without a field.
func appendResourceSort(opts []centreon.ListOption, sortBy, sortOrder string) ([]centreon.ListOption, error) {
	sortBy = strings.TrimSpace(sortBy)
	sortOrder = strings.TrimSpace(sortOrder)
	if sortBy == "" {
		if sortOrder != "" {
			return nil, fmt.Errorf("sortOrder %q requires sortBy", sortOrder)
		}
		return opts, nil
	}
	field, ok := resourceSortFields[strings.ToLower(sortBy)]
	if !ok {
		return nil, fmt.Errorf("unknown sortBy %q (want name, status, or last_status_change)", sortBy)
	}
	dir := sortDirAsc
	if sortOrder != "" {
		dir = strings.ToUpper(sortOrder)
		if dir != sortDirAsc && dir != sortDirDesc {
			return nil, fmt.Errorf("invalid sortOrder %q (want ASC or DESC)", sortOrder)
		}
	}
	return append(opts, centreon.WithSort(map[string]string{field: dir})), nil
}

func monitoringResourceListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in MonitoringResourceListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in MonitoringResourceListInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_monitoring_resource_list")
		logger.Debug("centreon_monitoring_resource_list", "page", in.Page, "limit", in.Limit, "search", in.Search, "hostName", in.HostName, "monitoringServer", in.MonitoringServer, "sortBy", in.SortBy, "sortOrder", in.SortOrder)
		opts, err := buildMonitoringResourceListOptions(&in)
		if err != nil {
			logger.Error("failed: centreon_monitoring_resource_list", "error", err)
			res, anyVal := errorResult("centreon_monitoring_resource_list: invalid input: %v", err)
			return res, anyVal, nil
		}
		resp, err := client.Monitoring.List(ctx, opts...)
		if err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_monitoring_resource_list", "error", reason)
			res, anyVal := errorResult("failed: centreon_monitoring_resource_list: %s", reason)
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
			reason := redact.Reason(err)
			logger.Error("failed: centreon_monitoring_resource_host_get", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to get monitoring resource host %d: %s", in.ID, reason)
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
			reason := redact.Reason(err)
			logger.Error("failed: centreon_monitoring_resource_service_get", "error", reason, "hostID", in.HostID, "serviceID", in.ServiceID)
			res, anyVal := errorResult("failed to get monitoring resource service (host=%d, service=%d): %s", in.HostID, in.ServiceID, reason)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(svc)
		return res, anyVal, nil
	}
}
