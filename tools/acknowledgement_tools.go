package tools

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client"
)

// RegisterAcknowledgementTools registers all acknowledgement tools.
func RegisterAcknowledgementTools(s *mcp.Server, client *centreon.Client, logger *slog.Logger) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_acknowledgement_list",
		Description: "List all acknowledgements. Supports pagination and filtering.",
	}, acknowledgementListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_acknowledgement_get",
		Description: "Get a single acknowledgement by ID.",
	}, acknowledgementGetHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_acknowledgement_host_list",
		Description: "List acknowledgements for a specific host.",
	}, acknowledgementHostListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_acknowledgement_service_list",
		Description: "List acknowledgements for a specific service on a host.",
	}, acknowledgementServiceListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_acknowledgement_host_create",
		Description: "Acknowledge a specific host.",
	}, acknowledgementHostCreateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_acknowledgement_service_create",
		Description: "Acknowledge a specific service on a host.",
	}, acknowledgementServiceCreateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_acknowledgement_host_cancel",
		Description: "Cancel the acknowledgement for a specific host.",
	}, acknowledgementHostCancelHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_acknowledgement_service_cancel",
		Description: "Cancel the acknowledgement for a specific service on a host.",
	}, acknowledgementServiceCancelHandler(client, logger))
}

// CreateHostAcknowledgementInput is the input for the centreon_acknowledgement_host_create tool.
type CreateHostAcknowledgementInput struct {
	HostID              int    `json:"hostID"                        jsonschema:"Host ID"`
	Comment             string `json:"comment"                       jsonschema:"Acknowledgement comment"`
	IsSticky            bool   `json:"isSticky,omitempty"            jsonschema:"Sticky (stays until recovery)"`
	IsNotifyContacts    bool   `json:"isNotifyContacts,omitempty"    jsonschema:"Notify contacts"`
	IsPersistentComment bool   `json:"isPersistentComment,omitempty" jsonschema:"Persistent comment"`
	WithServices        bool   `json:"withServices,omitempty"        jsonschema:"Apply to all services on the host"`
}

// CreateServiceAcknowledgementInput is the input for the centreon_acknowledgement_service_create tool.
type CreateServiceAcknowledgementInput struct {
	HostID              int    `json:"hostID"                        jsonschema:"Host ID"`
	ServiceID           int    `json:"serviceID"                     jsonschema:"Service ID"`
	Comment             string `json:"comment"                       jsonschema:"Acknowledgement comment"`
	IsSticky            bool   `json:"isSticky,omitempty"            jsonschema:"Sticky (stays until recovery)"`
	IsNotifyContacts    bool   `json:"isNotifyContacts,omitempty"    jsonschema:"Notify contacts"`
	IsPersistentComment bool   `json:"isPersistentComment,omitempty" jsonschema:"Persistent comment"`
}

func acknowledgementListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_acknowledgement_list", in, client.Acknowledgements.List)
	}
}

func acknowledgementGetHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_acknowledgement_get")
		logger.Debug("centreon_acknowledgement_get", "id", in.ID)
		ack, err := client.Acknowledgements.Get(ctx, in.ID)
		if err != nil {
			logger.Error("failed: centreon_acknowledgement_get", "error", err, "id", in.ID)
			res, anyVal := errorResult("failed to get acknowledgement %d: %v", in.ID, err)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(ack)
		return res, anyVal, nil
	}
}

func acknowledgementHostListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in HostIDListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in HostIDListInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_acknowledgement_host_list")
		logger.Debug("centreon_acknowledgement_host_list", "hostID", in.HostID, "page", in.Page, "limit", in.Limit, "search", in.Search)
		listIn := ListInput{Page: in.Page, Limit: in.Limit, Search: in.Search}
		opts := buildListOptions(listIn)
		resp, err := client.Acknowledgements.ListForHost(ctx, in.HostID, opts...)
		if err != nil {
			logger.Error("failed: centreon_acknowledgement_host_list", "error", err, "hostID", in.HostID)
			res, anyVal := errorResult("failed to list acknowledgements for host %d: %v", in.HostID, err)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(resp)
		return res, anyVal, nil
	}
}

func acknowledgementServiceListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in HostServiceListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in HostServiceListInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_acknowledgement_service_list")
		logger.Debug("centreon_acknowledgement_service_list", "hostID", in.HostID, "serviceID", in.ServiceID, "page", in.Page, "limit", in.Limit, "search", in.Search)
		listIn := ListInput{Page: in.Page, Limit: in.Limit, Search: in.Search}
		opts := buildListOptions(listIn)
		resp, err := client.Acknowledgements.ListForService(ctx, in.HostID, in.ServiceID, opts...)
		if err != nil {
			logger.Error("failed: centreon_acknowledgement_service_list", "error", err, "hostID", in.HostID, "serviceID", in.ServiceID)
			res, anyVal := errorResult("failed to list acknowledgements for service (host=%d, service=%d): %v", in.HostID, in.ServiceID, err)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(resp)
		return res, anyVal, nil
	}
}

func acknowledgementHostCreateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in CreateHostAcknowledgementInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateHostAcknowledgementInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_acknowledgement_host_create")
		logger.Info("centreon_acknowledgement_host_create", "hostID", in.HostID)
		ackReq := &centreon.CreateAcknowledgementRequest{
			Comment:             in.Comment,
			IsSticky:            in.IsSticky,
			IsNotifyContacts:    in.IsNotifyContacts,
			IsPersistentComment: in.IsPersistentComment,
			WithServices:        in.WithServices,
		}
		if err := client.Acknowledgements.CreateForHost(ctx, in.HostID, ackReq); err != nil {
			logger.Error("failed: centreon_acknowledgement_host_create", "error", err, "hostID", in.HostID)
			res, anyVal := errorResult("failed to create acknowledgement for host %d: %v", in.HostID, err)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_acknowledgement_host_create", "Acknowledgement created for host %d", in.HostID)
		return res, anyVal, nil
	}
}

func acknowledgementServiceCreateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in CreateServiceAcknowledgementInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateServiceAcknowledgementInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_acknowledgement_service_create")
		logger.Info("centreon_acknowledgement_service_create", "hostID", in.HostID, "serviceID", in.ServiceID)
		ackReq := &centreon.CreateAcknowledgementRequest{
			Comment:             in.Comment,
			IsSticky:            in.IsSticky,
			IsNotifyContacts:    in.IsNotifyContacts,
			IsPersistentComment: in.IsPersistentComment,
		}
		if err := client.Acknowledgements.CreateForService(ctx, in.HostID, in.ServiceID, ackReq); err != nil {
			logger.Error("failed: centreon_acknowledgement_service_create", "error", err, "hostID", in.HostID, "serviceID", in.ServiceID)
			res, anyVal := errorResult("failed to create acknowledgement for service (host=%d, service=%d): %v", in.HostID, in.ServiceID, err)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_acknowledgement_service_create", "Acknowledgement created for service %d on host %d", in.ServiceID, in.HostID)
		return res, anyVal, nil
	}
}

func acknowledgementHostCancelHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in HostIDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in HostIDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_acknowledgement_host_cancel")
		logger.Info("centreon_acknowledgement_host_cancel", "hostID", in.HostID)
		if err := client.Acknowledgements.CancelForHost(ctx, in.HostID); err != nil {
			logger.Error("failed: centreon_acknowledgement_host_cancel", "error", err, "hostID", in.HostID)
			res, anyVal := errorResult("failed to cancel acknowledgement for host %d: %v", in.HostID, err)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_acknowledgement_host_cancel", "Acknowledgement cancelled for host %d", in.HostID)
		return res, anyVal, nil
	}
}

func acknowledgementServiceCancelHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in HostServiceInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in HostServiceInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_acknowledgement_service_cancel")
		logger.Info("centreon_acknowledgement_service_cancel", "hostID", in.HostID, "serviceID", in.ServiceID)
		if err := client.Acknowledgements.CancelForService(ctx, in.HostID, in.ServiceID); err != nil {
			logger.Error("failed: centreon_acknowledgement_service_cancel", "error", err, "hostID", in.HostID, "serviceID", in.ServiceID)
			res, anyVal := errorResult("failed to cancel acknowledgement for service (host=%d, service=%d): %v", in.HostID, in.ServiceID, err)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_acknowledgement_service_cancel", "Acknowledgement cancelled for service %d on host %d", in.ServiceID, in.HostID)
		return res, anyVal, nil
	}
}
