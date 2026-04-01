package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client"
)

const (
	maxPageSize     = 100
	defaultPageSize = 30
)

// RegisterAll registers all Centreon tools with the MCP server.
func RegisterAll(s *mcp.Server, client *centreon.Client, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	RegisterMonitoringTools(s, client, logger)
	RegisterOperationsTools(s, client, logger)
	RegisterDowntimeTools(s, client, logger)
	RegisterAcknowledgementTools(s, client, logger)
	RegisterHostConfigTools(s, client, logger)
	RegisterServiceConfigTools(s, client, logger)
	RegisterInfraTools(s, client, logger)
	RegisterUserTools(s, client, logger)
	RegisterNotificationTools(s, client, logger)
	RegisterStatusTools(s, client, logger)
	RegisterConnectionTools(s, client, logger)
}

// ListInput is the common input for list tools.
type ListInput struct {
	Page   int    `json:"page,omitempty"   jsonschema:"Page number (default 1)"`
	Limit  int    `json:"limit,omitempty"  jsonschema:"Results per page (default 30, max 100)"`
	Search string `json:"search,omitempty" jsonschema:"Filter by name (like match)"`
}

// IDInput is the common input for single-resource tools.
type IDInput struct {
	ID int `json:"id" jsonschema:"Resource ID"`
}

// HostServiceInput is the input for service-scoped tools.
type HostServiceInput struct {
	HostID    int `json:"hostID"    jsonschema:"Host ID"`
	ServiceID int `json:"serviceID" jsonschema:"Service ID"`
}

// HostIDInput is the input for host-scoped tools.
type HostIDInput struct {
	HostID int `json:"hostID" jsonschema:"Host ID"`
}

// HostIDListInput is the input for host-scoped list tools.
type HostIDListInput struct {
	HostID int    `json:"hostID"            jsonschema:"Host ID"`
	Page   int    `json:"page,omitempty"    jsonschema:"Page number (default 1)"`
	Limit  int    `json:"limit,omitempty"   jsonschema:"Results per page (default 30, max 100)"`
	Search string `json:"search,omitempty"  jsonschema:"Filter by name (like match)"`
}

// HostServiceListInput is the input for service-scoped list tools.
type HostServiceListInput struct {
	HostID    int    `json:"hostID"            jsonschema:"Host ID"`
	ServiceID int    `json:"serviceID"         jsonschema:"Service ID"`
	Page      int    `json:"page,omitempty"    jsonschema:"Page number (default 1)"`
	Limit     int    `json:"limit,omitempty"   jsonschema:"Results per page (default 30, max 100)"`
	Search    string `json:"search,omitempty"  jsonschema:"Filter by name (like match)"`
}

// textResult builds a simple text content result.
//
//nolint:unparam // anyVal is always nil; kept for signature consistency.
func textResult(format string, args ...any) (res *mcp.CallToolResult, anyVal any) {
	text := fmt.Sprintf(format, args...)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}, nil
}

// jsonResult builds a JSON-formatted text content result.
func jsonResult(data any) (res *mcp.CallToolResult, anyVal any) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return errorResult("failed to marshal JSON: %v", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(b)},
		},
	}, data
}

// errorResult builds an error result with IsError: true.
func errorResult(format string, args ...any) (res *mcp.CallToolResult, anyVal any) {
	text := fmt.Sprintf(format, args...)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
		IsError: true,
	}, nil
}

// buildListOptions converts a ListInput into centreon.ListOption slice.
func buildListOptions(in ListInput) []centreon.ListOption {
	var opts []centreon.ListOption

	limit := in.Limit
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	opts = append(opts, centreon.WithLimit(limit))

	if in.Page > 0 {
		opts = append(opts, centreon.WithPage(in.Page))
	}
	if in.Search != "" {
		opts = append(opts, centreon.WithSearch(centreon.Lk("name", in.Search)))
	}
	return opts
}

// ListRequester abstracts any client List method.
type ListRequester[T any] func(ctx context.Context, opts ...centreon.ListOption) (*centreon.ListResponse[T], error)

// commonListHandler handles list requests with standard pagination and logging.
func commonListHandler[T any](
	ctx context.Context,
	logger *slog.Logger,
	toolName string,
	in ListInput,
	requester ListRequester[T],
) (*mcp.CallToolResult, any, error) {
	logger.Debug(toolName, "page", in.Page, "limit", in.Limit, "search", in.Search)

	opts := buildListOptions(in)
	resp, err := requester(ctx, opts...)
	if err != nil {
		logger.Error("failed: "+toolName, "error", err)
		res, anyVal := errorResult("failed: %s: %v", toolName, err)
		return res, anyVal, nil
	}

	res, anyVal := jsonResult(resp)
	return res, anyVal, nil
}

// Stub register functions — replaced by real implementations.
func RegisterMonitoringTools(_ *mcp.Server, _ *centreon.Client, _ *slog.Logger)      {}
func RegisterOperationsTools(_ *mcp.Server, _ *centreon.Client, _ *slog.Logger)      {}
func RegisterDowntimeTools(_ *mcp.Server, _ *centreon.Client, _ *slog.Logger)        {}
func RegisterAcknowledgementTools(_ *mcp.Server, _ *centreon.Client, _ *slog.Logger) {}
func RegisterHostConfigTools(_ *mcp.Server, _ *centreon.Client, _ *slog.Logger)      {}
func RegisterServiceConfigTools(_ *mcp.Server, _ *centreon.Client, _ *slog.Logger)   {}
func RegisterInfraTools(_ *mcp.Server, _ *centreon.Client, _ *slog.Logger)           {}
func RegisterUserTools(_ *mcp.Server, _ *centreon.Client, _ *slog.Logger)            {}
func RegisterNotificationTools(_ *mcp.Server, _ *centreon.Client, _ *slog.Logger)    {}
func RegisterStatusTools(_ *mcp.Server, _ *centreon.Client, _ *slog.Logger)          {}
func RegisterConnectionTools(_ *mcp.Server, _ *centreon.Client, _ *slog.Logger)      {}
