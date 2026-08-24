package tools

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client/v2"
	"github.com/tphakala/centreon-mcp-go/internal/redact"
)

// RegisterAcknowledgementTools registers all acknowledgement tools.
func RegisterAcknowledgementTools(s *Registrar, client *centreon.Client, logger *slog.Logger) {
	addTool(s, &mcp.Tool{
		Name:        "centreon_acknowledgement_list",
		Description: "Retrieve a paginated list of all acknowledgements across every host and service on the platform, where each acknowledgement marks a problem as seen so it stops sending notifications. Use this for a platform-wide view; scope to one host with centreon_acknowledgement_host_list, to one service with centreon_acknowledgement_service_list, or fetch a single record by ID with centreon_acknowledgement_get. Supports page (default 1), limit (default 30, max 100), and a search filter matched against the name. Read-only.",
		Annotations: readOnlyTool("List acknowledgements"),
	}, acknowledgementListHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_acknowledgement_get",
		Description: "Fetch a single acknowledgement by its numeric id and return its full stored detail. Use this when you already know the id; to discover ids, browse with centreon_acknowledgement_list for the whole platform, centreon_acknowledgement_host_list for one host, or centreon_acknowledgement_service_list for one service. Requires the id field and takes no pagination. Read-only.",
		Annotations: readOnlyTool("Get acknowledgement"),
	}, acknowledgementGetHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_acknowledgement_host_list",
		Description: "List the acknowledgements recorded for one host, identified by the required hostID. Use this to inspect a single host; for a service on that host use centreon_acknowledgement_service_list, and for every acknowledgement on the platform use centreon_acknowledgement_list. Supports page (default 1), limit (default 30, max 100), and a name search filter. Read-only.",
		Annotations: readOnlyTool("List host acknowledgements"),
	}, acknowledgementHostListHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_acknowledgement_service_list",
		Description: "List the acknowledgements recorded for one service, identified by both its hostID and serviceID. Use this to inspect a single service; for the parent host use centreon_acknowledgement_host_list, and for every acknowledgement on the platform use centreon_acknowledgement_list. Supports page (default 1), limit (default 30, max 100), and a name search filter. Read-only.",
		Annotations: readOnlyTool("List service acknowledgements"),
	}, acknowledgementServiceListHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_acknowledgement_host_create",
		Description: "Acknowledge a down or unreachable host, identified by hostID, so Centreon marks the problem as handled and stops resending notifications for it. Use this for a host-level problem; to acknowledge a single failing service use centreon_acknowledgement_service_create, and to clear an existing host acknowledgement use centreon_acknowledgement_host_cancel. Requires hostID and a comment; optional flags set sticky (stays until recovery), notify contacts, persistent comment, and withServices to also acknowledge every service on the host. Writes to Centreon.",
		Annotations: createTool("Acknowledge host problem"),
	}, acknowledgementHostCreateHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_acknowledgement_service_create",
		Description: "Acknowledge a warning, critical, or unknown service on a host, identified by hostID and serviceID, so Centreon marks the problem as handled and stops resending its notifications. Use this for a single service; to acknowledge the whole host use centreon_acknowledgement_host_create, and to clear an existing service acknowledgement use centreon_acknowledgement_service_cancel. Requires hostID, serviceID, and a comment; optional flags set sticky (stays until recovery), notify contacts, and persistent comment. Writes to Centreon.",
		Annotations: createTool("Acknowledge service problem"),
	}, acknowledgementServiceCreateHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_acknowledgement_host_cancel",
		Description: "Remove the current acknowledgement from a host, identified by hostID, which re-enables normal problem notifications while the host is still down or unreachable. Use this to reverse centreon_acknowledgement_host_create; to clear a service acknowledgement instead use centreon_acknowledgement_service_cancel. Requires only hostID and takes no comment. Writes to Centreon.",
		Annotations: deleteTool("Cancel host acknowledgement"),
	}, acknowledgementHostCancelHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_acknowledgement_service_cancel",
		Description: "Remove the current acknowledgement from a service, identified by hostID and serviceID, which re-enables normal problem notifications while the service is still in a non-ok state. Use this to reverse centreon_acknowledgement_service_create; to clear a host acknowledgement instead use centreon_acknowledgement_host_cancel. Requires hostID and serviceID and takes no comment. Writes to Centreon.",
		Annotations: deleteTool("Cancel service acknowledgement"),
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
			reason := redact.Reason(err)
			logger.Error("failed: centreon_acknowledgement_get", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to get acknowledgement %d: %s", in.ID, reason)
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
			reason := redact.Reason(err)
			logger.Error("failed: centreon_acknowledgement_host_list", "error", reason, "hostID", in.HostID)
			res, anyVal := errorResult("failed to list acknowledgements for host %d: %s", in.HostID, reason)
			return res, anyVal, nil
		}
		res, anyVal := listResult(resp)
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
			reason := redact.Reason(err)
			logger.Error("failed: centreon_acknowledgement_service_list", "error", reason, "hostID", in.HostID, "serviceID", in.ServiceID)
			res, anyVal := errorResult("failed to list acknowledgements for service (host=%d, service=%d): %s", in.HostID, in.ServiceID, reason)
			return res, anyVal, nil
		}
		res, anyVal := listResult(resp)
		return res, anyVal, nil
	}
}

func acknowledgementHostCreateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in CreateHostAcknowledgementInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateHostAcknowledgementInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_acknowledgement_host_create")
		logger.Info("centreon_acknowledgement_host_create", "hostID", in.HostID)
		ackReq := &centreon.CreateHostAcknowledgementRequest{
			Comment:             in.Comment,
			IsSticky:            in.IsSticky,
			IsNotifyContacts:    in.IsNotifyContacts,
			IsPersistentComment: in.IsPersistentComment,
			WithServices:        in.WithServices,
		}
		if err := client.Acknowledgements.CreateForHost(ctx, in.HostID, ackReq); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_acknowledgement_host_create", "error", reason, "hostID", in.HostID)
			res, anyVal := errorResult("failed to create acknowledgement for host %d: %s", in.HostID, reason)
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
		ackReq := &centreon.CreateServiceAcknowledgementRequest{
			Comment:             in.Comment,
			IsSticky:            in.IsSticky,
			IsNotifyContacts:    in.IsNotifyContacts,
			IsPersistentComment: in.IsPersistentComment,
		}
		if err := client.Acknowledgements.CreateForService(ctx, in.HostID, in.ServiceID, ackReq); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_acknowledgement_service_create", "error", reason, "hostID", in.HostID, "serviceID", in.ServiceID)
			res, anyVal := errorResult("failed to create acknowledgement for service (host=%d, service=%d): %s", in.HostID, in.ServiceID, reason)
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
			reason := redact.Reason(err)
			logger.Error("failed: centreon_acknowledgement_host_cancel", "error", reason, "hostID", in.HostID)
			res, anyVal := errorResult("failed to cancel acknowledgement for host %d: %s", in.HostID, reason)
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
			reason := redact.Reason(err)
			logger.Error("failed: centreon_acknowledgement_service_cancel", "error", reason, "hostID", in.HostID, "serviceID", in.ServiceID)
			res, anyVal := errorResult("failed to cancel acknowledgement for service (host=%d, service=%d): %s", in.HostID, in.ServiceID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_acknowledgement_service_cancel", "Acknowledgement cancelled for service %d on host %d", in.ServiceID, in.HostID)
		return res, anyVal, nil
	}
}
