package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client"
)

// RegisterDowntimeTools registers all downtime tools.
func RegisterDowntimeTools(s *mcp.Server, client *centreon.Client, logger *slog.Logger) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_downtime_list",
		Description: "List all scheduled downtimes. Supports pagination and filtering.",
	}, downtimeListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_downtime_get",
		Description: "Get a single scheduled downtime by ID.",
	}, downtimeGetHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_downtime_cancel",
		Description: "Cancel a scheduled downtime by ID.",
	}, downtimeCancelHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_downtime_host_list",
		Description: "List scheduled downtimes for a specific host.",
	}, downtimeHostListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_downtime_service_list",
		Description: "List scheduled downtimes for a specific service on a host.",
	}, downtimeServiceListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_downtime_host_create",
		Description: "Schedule a downtime for a specific host.",
	}, downtimeHostCreateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_downtime_service_create",
		Description: "Schedule a downtime for a specific service on a host.",
	}, downtimeServiceCreateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_downtime_host_cancel",
		Description: "Cancel all scheduled downtimes for a specific host.",
	}, downtimeHostCancelHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_downtime_service_cancel",
		Description: "Cancel all scheduled downtimes for a specific service on a host.",
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
			logger.Error("failed: centreon_downtime_get", "error", err, "id", in.ID)
			res, anyVal := errorResult("failed to get downtime %d: %v", in.ID, err)
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
			logger.Error("failed: centreon_downtime_cancel", "error", err, "id", in.ID)
			res, anyVal := errorResult("failed to cancel downtime %d: %v", in.ID, err)
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
			logger.Error("failed: centreon_downtime_host_list", "error", err, "hostID", in.HostID)
			res, anyVal := errorResult("failed to list downtimes for host %d: %v", in.HostID, err)
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
			logger.Error("failed: centreon_downtime_service_list", "error", err, "hostID", in.HostID, "serviceID", in.ServiceID)
			res, anyVal := errorResult("failed to list downtimes for service (host=%d, service=%d): %v", in.HostID, in.ServiceID, err)
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
		downtimeReq := &centreon.CreateDowntimeRequest{
			Comment:      in.Comment,
			StartTime:    startTime,
			EndTime:      endTime,
			IsFixed:      in.IsFixed,
			Duration:     in.Duration,
			WithServices: in.WithServices,
		}
		if err := client.Downtimes.CreateForHost(ctx, in.HostID, downtimeReq); err != nil {
			logger.Error("failed: centreon_downtime_host_create", "error", err, "hostID", in.HostID)
			res, anyVal := errorResult("failed to create downtime for host %d: %v", in.HostID, err)
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
		downtimeReq := &centreon.CreateDowntimeRequest{
			Comment:   in.Comment,
			StartTime: startTime,
			EndTime:   endTime,
			IsFixed:   in.IsFixed,
			Duration:  in.Duration,
		}
		if err := client.Downtimes.CreateForService(ctx, in.HostID, in.ServiceID, downtimeReq); err != nil {
			logger.Error("failed: centreon_downtime_service_create", "error", err, "hostID", in.HostID, "serviceID", in.ServiceID)
			res, anyVal := errorResult("failed to create downtime for service (host=%d, service=%d): %v", in.HostID, in.ServiceID, err)
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
			logger.Error("failed: centreon_downtime_host_cancel", "error", err, "hostID", in.HostID)
			res, anyVal := errorResult("failed to cancel downtimes for host %d: %v", in.HostID, err)
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
			logger.Error("failed: centreon_downtime_service_cancel", "error", err, "hostID", in.HostID, "serviceID", in.ServiceID)
			res, anyVal := errorResult("failed to cancel downtimes for service (host=%d, service=%d): %v", in.HostID, in.ServiceID, err)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_downtime_service_cancel", "Downtime cancelled for service %d on host %d", in.ServiceID, in.HostID)
		return res, anyVal, nil
	}
}
