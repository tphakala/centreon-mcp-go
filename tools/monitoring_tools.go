package tools

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client/v2"
	"github.com/tphakala/centreon-mcp-go/internal/redact"
)

// RegisterMonitoringTools registers all monitoring tools.
func RegisterMonitoringTools(s *Registrar, client *centreon.Client, logger *slog.Logger) {
	addTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_host_list",
		Description: "Retrieve the real-time monitoring state of all hosts (current status, plugin output, acknowledgement, and downtime flags) as reported by the pollers. Use this for live health across many hosts; for one host by ID use centreon_monitoring_host_get, and for stored host configuration instead of live state use centreon_host_list. Reflects current engine state, not saved config; paginates with page (default 1) and limit (default 30, max 100). Read-only.",
		Annotations: readOnlyTool("List monitoring hosts"),
	}, monitoringHostListHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_host_get",
		Description: "Fetch the real-time monitoring state of one host by its numeric monitoring ID, including current status, last check, plugin output, and active problem flags. Use this when you already know the host ID; to browse or page through many hosts use centreon_monitoring_host_list, and for the alternative unified resource view use centreon_monitoring_resource_host_get. Requires id; returns live engine state rather than stored configuration. Read-only.",
		Annotations: readOnlyTool("Get monitoring host"),
	}, monitoringHostGetHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_host_services",
		Description: "List the real-time monitoring state of every service attached to one host, given its hostID. Use this to see all checks on a single host; to list services across all hosts use centreon_monitoring_service_list, and to fetch one specific service use centreon_monitoring_resource_service_get. Requires hostID; paginates with page (default 1) and limit (default 30, max 100) and reflects current engine state, not stored config. Read-only.",
		Annotations: readOnlyTool("List host services"),
	}, monitoringHostServicesHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_host_timeline",
		Description: "Retrieve the chronological event history for one host (state changes, notifications, acknowledgements, downtime, and comments) given its hostID. Use this to investigate what happened over time; for the current point-in-time snapshot instead use centreon_monitoring_host_get. Requires hostID; paginates with page (default 1) and limit (default 30, max 100) over live monitoring events. Read-only.",
		Annotations: readOnlyTool("Get host timeline"),
	}, monitoringHostTimelineHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_host_status_counts",
		Description: "Summarize how many hosts are currently in each monitoring state (up, down, unreachable, pending) as a single aggregate, taking no arguments. Use this for a quick host health total; for the per-host detail behind the numbers use centreon_monitoring_host_list, and for hosts plus services plus servers in one call use centreon_platform_status. Reflects current engine state, not stored configuration. Read-only.",
		Annotations: readOnlyTool("Host status counts"),
	}, monitoringHostStatusCountsHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_service_list",
		Description: "Retrieve the real-time monitoring state of all services across every host (current status, plugin output, acknowledgement, and downtime flags). Use this for live health across many services; to limit to one host's services use centreon_monitoring_host_services, and for stored service configuration instead of live state use centreon_service_list. Paginates with page (default 1) and limit (default 30, max 100) and reflects current engine state, not saved config. Read-only.",
		Annotations: readOnlyTool("List monitoring services"),
	}, monitoringServiceListHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_service_status_counts",
		Description: "Summarize how many services are currently in each monitoring state (ok, warning, critical, unknown, pending) as a single aggregate, taking no arguments. Use this for a quick service health total; for the host-side equivalent use centreon_monitoring_host_status_counts, and for the per-service detail behind the numbers use centreon_monitoring_service_list. Reflects current engine state, not stored configuration. Read-only.",
		Annotations: readOnlyTool("Service status counts"),
	}, monitoringServiceStatusCountsHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_service_metrics",
		Description: "Retrieve the current performance metric values for one service, given its hostID and serviceID: each metric's name, unit, current value, and warning/critical thresholds. Use this to answer 'what is the current CPU load / disk usage / latency', which the status tools do not report; for the service's event history use centreon_monitoring_service_timeline. Requires hostID and serviceID. A service with no performance data returns an empty list, not an error; a nonexistent host or service id also returns an empty list, since Centreon does not distinguish the two here. Reflects current engine state, not stored configuration. Read-only.",
		Annotations: readOnlyTool("Get service metrics"),
	}, monitoringServiceMetricsHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_service_timeline",
		Description: "Retrieve the chronological event history for one service (state changes, notifications, acknowledgements, downtime, and comments), given its hostID and serviceID. This is the service-scoped mirror of centreon_monitoring_host_timeline; use it to investigate a single flapping or failing service without filtering the host-wide timeline. Requires hostID and serviceID; paginates with page (default 1) and limit (default 30, max 100) over live monitoring events. Read-only.",
		Annotations: readOnlyTool("Get service timeline"),
	}, monitoringServiceTimelineHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_resource_list",
		Description: "List hosts and services together in Centreon's unified resource view, each with its real-time monitoring status. Use this when you want hosts and services in one combined stream; for only hosts use centreon_monitoring_host_list and for only services use centreon_monitoring_service_list. Optional filters: search (resource name, like match), hostName (a host and its services, like match), monitoringServer (exact poller name); resourceTypes (host, service, metaservice), statuses (OK, WARNING, CRITICAL, DOWN, ...), statusTypes (hard, soft), states (unhandled_problems, acknowledged, in_downtime, ...); hostGroups, serviceGroups, hostCategories, serviceCategories (exact name, any of); plus sortBy (name, status, last_status_change) with sortOrder (ASC or DESC). Multiple values in one filter match any of them, and the filters then apply together. Paginates with page (default 1) and limit (default 30, max 100) and reflects current engine state, not stored configuration. Read-only.",
		Annotations: readOnlyTool("List monitoring resources"),
	}, monitoringResourceListHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_monitoring_resource_host_get",
		Description: "Fetch one host's real-time monitoring status through Centreon's unified resource API, given its numeric id. Use this for the resource-view representation of a host; for the host-endpoint representation of the same live state use centreon_monitoring_host_get. Requires id; returns live engine state rather than stored configuration. Read-only.",
		Annotations: readOnlyTool("Get resource host"),
	}, monitoringResourceHostGetHandler(client, logger))

	addTool(s, &mcp.Tool{
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

func monitoringServiceMetricsHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in HostServiceInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in HostServiceInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_monitoring_service_metrics")
		logger.Debug("centreon_monitoring_service_metrics", "hostID", in.HostID, "serviceID", in.ServiceID)
		metrics, err := client.MonitoringServices.Metrics(ctx, in.HostID, in.ServiceID)
		if err != nil {
			reason := versionSensitiveReason(err)
			logger.Error("failed: centreon_monitoring_service_metrics", "error", reason, "hostID", in.HostID, "serviceID", in.ServiceID)
			res, anyVal := errorResult("failed to get metrics for service (host=%d, service=%d): %s", in.HostID, in.ServiceID, reason)
			return res, anyVal, nil
		}
		// The client returns a nil slice for a service with no performance data;
		// emit an empty JSON array rather than null so the model sees a list.
		if metrics == nil {
			metrics = []centreon.Metric{}
		}
		res, anyVal := jsonResult(metrics)
		return res, anyVal, nil
	}
}

func monitoringServiceTimelineHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in MonitoringHostServiceListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in MonitoringHostServiceListInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_monitoring_service_timeline")
		logger.Debug("centreon_monitoring_service_timeline", "hostID", in.HostID, "serviceID", in.ServiceID, "page", in.Page, "limit", in.Limit)
		opts := buildMonitoringListOptions(MonitoringListInput{Page: in.Page, Limit: in.Limit})
		resp, err := client.MonitoringServices.Timeline(ctx, in.HostID, in.ServiceID, opts...)
		if err != nil {
			reason := versionSensitiveReason(err)
			logger.Error("failed: centreon_monitoring_service_timeline", "error", reason, "hostID", in.HostID, "serviceID", in.ServiceID)
			res, anyVal := errorResult("failed to get timeline for service (host=%d, service=%d): %s", in.HostID, in.ServiceID, reason)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(resp)
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
// name, host and poller filters plus sorting, unlike the other monitoring list
// tools. The array filters (types, statuses, groups, categories) map to the
// endpoint's structured query parameters; the Centreon resources endpoint
// combines multiple values within one filter as OR and different filters as AND
// (per its documented behavior; not verified against a live instance here).
type MonitoringResourceListInput struct {
	Page             int    `json:"page,omitempty"             jsonschema:"Page number (default 1)"`
	Limit            int    `json:"limit,omitempty"            jsonschema:"Results per page (default 30, max 100)"`
	Search           string `json:"search,omitempty"           jsonschema:"Filter by resource name (like match on host and service names)"`
	HostName         string `json:"hostName,omitempty"         jsonschema:"Filter by host name (like match); matches the host and every service on it"`
	MonitoringServer string `json:"monitoringServer,omitempty" jsonschema:"Filter by monitoring server (poller) name (exact match)"`
	SortBy           string `json:"sortBy,omitempty"           jsonschema:"Sort field: name, status, or last_status_change (status sorts by severity)"`
	SortOrder        string `json:"sortOrder,omitempty"        jsonschema:"Sort direction: ASC (default) or DESC; requires sortBy"`

	ResourceTypes     []string `json:"resourceTypes,omitempty"     jsonschema:"Filter by resource type. Allowed: host, service, metaservice"`
	Statuses          []string `json:"statuses,omitempty"          jsonschema:"Filter by status (uppercase). Allowed: OK, UP, WARNING, DOWN, CRITICAL, UNREACHABLE, UNKNOWN, PENDING"`
	StatusTypes       []string `json:"statusTypes,omitempty"       jsonschema:"Filter by status type. Allowed: hard, soft"`
	States            []string `json:"states,omitempty"            jsonschema:"Filter by state. Allowed: unhandled_problems, resources_problems, in_downtime, acknowledged, in_flapping, all"`
	HostGroups        []string `json:"hostGroups,omitempty"        jsonschema:"Filter by host group name (exact match, any of)"`
	ServiceGroups     []string `json:"serviceGroups,omitempty"     jsonschema:"Filter by service group name (exact match, any of)"`
	HostCategories    []string `json:"hostCategories,omitempty"    jsonschema:"Filter by host category name (exact match, any of)"`
	ServiceCategories []string `json:"serviceCategories,omitempty" jsonschema:"Filter by service category name (exact match, any of)"`
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
	opts = appendResourceArrayFilters(opts, in)
	return appendResourceSort(opts, in.SortBy, in.SortOrder)
}

// appendResourceArrayFilters appends the structured array-filter options (types,
// statuses, groups, categories) for every filter that has at least one non-blank
// value. Blank entries are dropped, and a filter with no surviving value emits no
// query parameter. Values are passed through verbatim; Centreon validates them
// server-side, so an invalid value surfaces as an API error rather than being
// silently rewritten here.
func appendResourceArrayFilters(opts []centreon.ListOption, in *MonitoringResourceListInput) []centreon.ListOption {
	if v := nonEmptyTrimmed(in.ResourceTypes); v != nil {
		opts = append(opts, centreon.WithResourceTypes(v...))
	}
	if v := nonEmptyTrimmed(in.Statuses); v != nil {
		opts = append(opts, centreon.WithStatuses(v...))
	}
	if v := nonEmptyTrimmed(in.StatusTypes); v != nil {
		opts = append(opts, centreon.WithStatusTypes(v...))
	}
	if v := nonEmptyTrimmed(in.States); v != nil {
		opts = append(opts, centreon.WithStates(v...))
	}
	if v := nonEmptyTrimmed(in.HostGroups); v != nil {
		opts = append(opts, centreon.WithHostGroupNames(v...))
	}
	if v := nonEmptyTrimmed(in.ServiceGroups); v != nil {
		opts = append(opts, centreon.WithServiceGroupNames(v...))
	}
	if v := nonEmptyTrimmed(in.HostCategories); v != nil {
		opts = append(opts, centreon.WithHostCategoryNames(v...))
	}
	if v := nonEmptyTrimmed(in.ServiceCategories); v != nil {
		opts = append(opts, centreon.WithServiceCategoryNames(v...))
	}
	return opts
}

// nonEmptyTrimmed returns the space-trimmed, non-blank entries of vals in order,
// or nil when nothing survives, so a filter of only blank values emits no param.
func nonEmptyTrimmed(vals []string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if t := strings.TrimSpace(v); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// versionSensitiveReason is redact.Reason, except a Centreon 404 is rendered as a
// hint that the resource may be missing or the endpoint may not exist on the
// connected Centreon version. A routing 404 the client has classified as an
// absent route (IsRouteNotFound) gets the precise "API route not present"
// wording; any other 404 keeps the ambiguous wording, because a resource-404 and
// an older-Centreon-404 are indistinguishable without that route signal. It suits
// tools whose endpoint may be absent on an older Centreon; like redact.Reason it
// emits only fixed strings and the status classification (IsRouteNotFound returns
// only a bool after the client vets the response body), never a credential-bearing
// message from the error.
func versionSensitiveReason(err error) string {
	if centreon.IsRouteNotFound(err) {
		return "unsupported on this Centreon version (API route not present)"
	}
	if isNotFoundStatus(err) {
		return "not found, or unsupported on this Centreon version"
	}
	return redact.Reason(err)
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
