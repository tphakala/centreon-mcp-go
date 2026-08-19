package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client/v2"
	"github.com/tphakala/centreon-mcp-go/internal/redact"
)

// RegisterDowntimeTools registers all downtime tools.
func RegisterDowntimeTools(s *Registrar, client *centreon.Client, logger *slog.Logger) {
	addTool(s, &mcp.Tool{
		Name:        "centreon_downtime_list",
		Description: "Retrieve scheduled maintenance windows across the entire platform, spanning every host and service, that suppress notifications during planned outages. Use this for a platform-wide view; narrow to one host with centreon_downtime_host_list or one service with centreon_downtime_service_list, and fetch a single window by ID with centreon_downtime_get. Paginated (page 1, limit 30, max 100 per page) with optional name search. Read-only.",
		Annotations: readOnlyTool("List downtimes"),
	}, downtimeListHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_downtime_get",
		Description: "Fetch the complete detail of one scheduled maintenance window identified by its numeric downtime ID, including its comment, start and end times, and fixed or flexible type. Use this when you already hold the ID; discover IDs first with centreon_downtime_list, centreon_downtime_host_list, or centreon_downtime_service_list. Returns a single record. Read-only.",
		Annotations: readOnlyTool("Get downtime"),
	}, downtimeGetHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_downtime_cancel",
		Description: "Cancel one scheduled maintenance window by its numeric downtime ID, ending the notification suppression it applied. Use this to remove a single window; cancel every window on a host with centreon_downtime_host_cancel or every window on a service with centreon_downtime_service_cancel, and schedule a new window with centreon_downtime_host_create or centreon_downtime_service_create. Removes the identified downtime. Writes to Centreon.",
		Annotations: deleteTool("Cancel downtime"),
	}, downtimeCancelHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_downtime_host_list",
		Description: "List the scheduled maintenance windows attached to one host identified by its host ID. Use this to scope results to a single host; list every window platform-wide with centreon_downtime_list, or restrict to one service on the host with centreon_downtime_service_list. Paginated (page 1, limit 30, max 100 per page) with optional name search. Read-only.",
		Annotations: readOnlyTool("List host downtimes"),
	}, downtimeHostListHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_downtime_service_list",
		Description: "List the scheduled maintenance windows attached to one service, identified by its host ID and service ID together. Use this to scope results to a single service; widen to every window on the host with centreon_downtime_host_list or the whole platform with centreon_downtime_list. Paginated (page 1, limit 30, max 100 per page) with optional name search. Read-only.",
		Annotations: readOnlyTool("List service downtimes"),
	}, downtimeServiceListHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_downtime_host_create",
		Description: "Schedule a maintenance window on one host so its notifications are suppressed between a start and end time, optionally covering every service on the host. Use this for host-level downtime; schedule a single service instead with centreon_downtime_service_create, and remove a window with centreon_downtime_host_cancel or centreon_downtime_cancel. Requires hostID, a comment, and RFC3339 startTime and endTime with endTime after startTime; set isFixed for a fixed window or supply duration in seconds for a flexible one, and set withServices to include all of the host's services. Writes to Centreon.",
		Annotations: createTool("Schedule host downtime"),
	}, downtimeHostCreateHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_downtime_service_create",
		Description: "Schedule a maintenance window on one service, identified by its host ID and service ID, so only that service's notifications are suppressed between a start and end time. Use this for a single service; cover an entire host with centreon_downtime_host_create, and remove a window with centreon_downtime_service_cancel or centreon_downtime_cancel. Requires hostID, serviceID, a comment, and RFC3339 startTime and endTime with endTime after startTime; set isFixed for a fixed window or supply duration in seconds for a flexible one. Writes to Centreon.",
		Annotations: createTool("Schedule service downtime"),
	}, downtimeServiceCreateHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_downtime_host_cancel",
		Description: "Cancel every scheduled maintenance window currently attached to one host, identified by its host ID, restoring that host's notifications. Use this to clear a host in bulk; remove a single window by ID with centreon_downtime_cancel, or clear one service with centreon_downtime_service_cancel. Removes all of the host's downtimes in one call. Writes to Centreon.",
		Annotations: deleteTool("Cancel host downtimes"),
	}, downtimeHostCancelHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_downtime_service_cancel",
		Description: "Cancel every scheduled maintenance window currently attached to one service, identified by its host ID and service ID, restoring that service's notifications. Use this to clear a service in bulk; clear the whole host with centreon_downtime_host_cancel, or remove a single window by ID with centreon_downtime_cancel. Removes all of the service's downtimes in one call. Writes to Centreon.",
		Annotations: deleteTool("Cancel service downtimes"),
	}, downtimeServiceCancelHandler(client, logger))
}

// CreateHostDowntimeInput is the input for the centreon_downtime_host_create tool.
type CreateHostDowntimeInput struct {
	HostID       int    `json:"hostID"                 jsonschema:"Host ID"`
	Comment      string `json:"comment"                jsonschema:"Downtime comment"`
	StartTime    string `json:"startTime"              jsonschema:"Start time in RFC3339 format"`
	EndTime      string `json:"endTime"                jsonschema:"End time in RFC3339 format"`
	IsFixed      bool   `json:"isFixed,omitempty"      jsonschema:"Fixed downtime (default false)"`
	Duration     int    `json:"duration,omitempty"     jsonschema:"Duration in seconds (for flexible downtimes)"`
	WithServices bool   `json:"withServices,omitempty" jsonschema:"Apply to all services on the host"`
}

// CreateServiceDowntimeInput is the input for the centreon_downtime_service_create tool.
type CreateServiceDowntimeInput struct {
	HostID    int    `json:"hostID"             jsonschema:"Host ID"`
	ServiceID int    `json:"serviceID"          jsonschema:"Service ID"`
	Comment   string `json:"comment"            jsonschema:"Downtime comment"`
	StartTime string `json:"startTime"          jsonschema:"Start time in RFC3339 format"`
	EndTime   string `json:"endTime"            jsonschema:"End time in RFC3339 format"`
	IsFixed   bool   `json:"isFixed,omitempty"  jsonschema:"Fixed downtime (default false)"`
	Duration  int    `json:"duration,omitempty" jsonschema:"Duration in seconds (for flexible downtimes)"`
}

func downtimeListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_downtime_list", in, client.Downtimes.List)
	}
}

func downtimeGetHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_downtime_get")
		logger.Debug("centreon_downtime_get", "id", in.ID)
		downtime, err := client.Downtimes.Get(ctx, in.ID)
		if err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_downtime_get", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to get downtime %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(downtime)
		return res, anyVal, nil
	}
}

func downtimeCancelHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_downtime_cancel")
		logger.Info("centreon_downtime_cancel", "id", in.ID)
		if err := client.Downtimes.Cancel(ctx, in.ID); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_downtime_cancel", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to cancel downtime %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_downtime_cancel", "Downtime %d cancelled", in.ID)
		return res, anyVal, nil
	}
}

func downtimeHostListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in HostIDListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in HostIDListInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_downtime_host_list")
		logger.Debug("centreon_downtime_host_list", "hostID", in.HostID, "page", in.Page, "limit", in.Limit, "search", in.Search)
		listIn := ListInput{Page: in.Page, Limit: in.Limit, Search: in.Search}
		opts := buildListOptions(listIn)
		resp, err := client.Downtimes.ListForHost(ctx, in.HostID, opts...)
		if err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_downtime_host_list", "error", reason, "hostID", in.HostID)
			res, anyVal := errorResult("failed to list downtimes for host %d: %s", in.HostID, reason)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(resp)
		return res, anyVal, nil
	}
}

func downtimeServiceListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in HostServiceListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in HostServiceListInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_downtime_service_list")
		logger.Debug("centreon_downtime_service_list", "hostID", in.HostID, "serviceID", in.ServiceID, "page", in.Page, "limit", in.Limit, "search", in.Search)
		listIn := ListInput{Page: in.Page, Limit: in.Limit, Search: in.Search}
		opts := buildListOptions(listIn)
		resp, err := client.Downtimes.ListForService(ctx, in.HostID, in.ServiceID, opts...)
		if err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_downtime_service_list", "error", reason, "hostID", in.HostID, "serviceID", in.ServiceID)
			res, anyVal := errorResult("failed to list downtimes for service (host=%d, service=%d): %s", in.HostID, in.ServiceID, reason)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(resp)
		return res, anyVal, nil
	}
}

func downtimeHostCreateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in CreateHostDowntimeInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateHostDowntimeInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_downtime_host_create")
		logger.Info("centreon_downtime_host_create", "hostID", in.HostID, "start", in.StartTime, "end", in.EndTime)
		startTime, err := time.Parse(time.RFC3339, in.StartTime)
		if err != nil {
			res, anyVal := errorResult("invalid startTime %q: must be RFC3339 format: %v", in.StartTime, err)
			return res, anyVal, nil
		}
		endTime, err := time.Parse(time.RFC3339, in.EndTime)
		if err != nil {
			res, anyVal := errorResult("invalid endTime %q: must be RFC3339 format: %v", in.EndTime, err)
			return res, anyVal, nil
		}
		if !endTime.After(startTime) {
			res, anyVal := errorResult("endTime must be after startTime")
			return res, anyVal, nil
		}
		downtimeReq := &centreon.CreateHostDowntimeRequest{
			Comment:      in.Comment,
			StartTime:    startTime,
			EndTime:      endTime,
			IsFixed:      in.IsFixed,
			Duration:     in.Duration,
			WithServices: in.WithServices,
		}
		if err := client.Downtimes.CreateForHost(ctx, in.HostID, downtimeReq); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_downtime_host_create", "error", reason, "hostID", in.HostID)
			res, anyVal := errorResult("failed to create downtime for host %d: %s", in.HostID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_downtime_host_create", "Downtime scheduled for host %d", in.HostID)
		return res, anyVal, nil
	}
}

func downtimeServiceCreateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in CreateServiceDowntimeInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateServiceDowntimeInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_downtime_service_create")
		logger.Info("centreon_downtime_service_create", "hostID", in.HostID, "serviceID", in.ServiceID, "start", in.StartTime, "end", in.EndTime)
		startTime, err := time.Parse(time.RFC3339, in.StartTime)
		if err != nil {
			logger.Error("failed: centreon_downtime_service_create", "error", err, "hostID", in.HostID, "serviceID", in.ServiceID, "field", "startTime")
			res, anyVal := errorResult("invalid startTime %q: must be RFC3339 format: %v", in.StartTime, err)
			return res, anyVal, nil
		}
		endTime, err := time.Parse(time.RFC3339, in.EndTime)
		if err != nil {
			logger.Error("failed: centreon_downtime_service_create", "error", err, "hostID", in.HostID, "serviceID", in.ServiceID, "field", "endTime")
			res, anyVal := errorResult("invalid endTime %q: must be RFC3339 format: %v", in.EndTime, err)
			return res, anyVal, nil
		}
		if !endTime.After(startTime) {
			res, anyVal := errorResult("endTime must be after startTime")
			return res, anyVal, nil
		}
		downtimeReq := &centreon.CreateServiceDowntimeRequest{
			Comment:   in.Comment,
			StartTime: startTime,
			EndTime:   endTime,
			IsFixed:   in.IsFixed,
			Duration:  in.Duration,
		}
		if err := client.Downtimes.CreateForService(ctx, in.HostID, in.ServiceID, downtimeReq); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_downtime_service_create", "error", reason, "hostID", in.HostID, "serviceID", in.ServiceID)
			res, anyVal := errorResult("failed to create downtime for service (host=%d, service=%d): %s", in.HostID, in.ServiceID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_downtime_service_create", "Downtime scheduled for service %d on host %d", in.ServiceID, in.HostID)
		return res, anyVal, nil
	}
}

func downtimeHostCancelHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in HostIDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in HostIDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_downtime_host_cancel")
		logger.Info("centreon_downtime_host_cancel", "hostID", in.HostID)
		if err := client.Downtimes.CancelForHost(ctx, in.HostID); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_downtime_host_cancel", "error", reason, "hostID", in.HostID)
			res, anyVal := errorResult("failed to cancel downtimes for host %d: %s", in.HostID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_downtime_host_cancel", "Downtime cancelled for host %d", in.HostID)
		return res, anyVal, nil
	}
}

func downtimeServiceCancelHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in HostServiceInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in HostServiceInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_downtime_service_cancel")
		logger.Info("centreon_downtime_service_cancel", "hostID", in.HostID, "serviceID", in.ServiceID)
		if err := client.Downtimes.CancelForService(ctx, in.HostID, in.ServiceID); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_downtime_service_cancel", "error", reason, "hostID", in.HostID, "serviceID", in.ServiceID)
			res, anyVal := errorResult("failed to cancel downtimes for service (host=%d, service=%d): %s", in.HostID, in.ServiceID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_downtime_service_cancel", "Downtime cancelled for service %d on host %d", in.ServiceID, in.HostID)
		return res, anyVal, nil
	}
}
