package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client/v2"
	"github.com/tphakala/centreon-mcp-go/internal/redact"
)

const (
	maxPageSize     = 100
	defaultPageSize = 30
)

// RegisterAll registers all Centreon tools with the MCP server. host is the
// display-only Centreon host surfaced by the status and connection tools; it
// must already be stripped of credentials by the caller (displayHost in package
// main removes all userinfo), as the tools package prints it verbatim.
func RegisterAll(s *mcp.Server, client *centreon.Client, logger *slog.Logger, host string) {
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
	RegisterStatusTools(s, client, logger, host)
	RegisterConnectionTools(s, client, logger, host)
}

// readOnlyTool annotates a tool that only reads from the Centreon API (open world).
// DestructiveHint is set false explicitly (redundant under ReadOnlyHint, but unambiguous for scanners).
func readOnlyTool(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true, DestructiveHint: new(false), OpenWorldHint: new(true)}
}

// createTool annotates a tool that additively creates a record or submits an additive action.
func createTool(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{Title: title, DestructiveHint: new(false), IdempotentHint: false, OpenWorldHint: new(true)}
}

// updateTool annotates a tool that overwrites an existing record or applies configuration changes.
func updateTool(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{Title: title, DestructiveHint: new(true), IdempotentHint: true, OpenWorldHint: new(true)}
}

// deleteTool annotates a tool that deletes a record or cancels a scheduled action.
func deleteTool(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{Title: title, DestructiveHint: new(true), IdempotentHint: true, OpenWorldHint: new(true)}
}

// ListInput is the common input for list tools.
type ListInput struct {
	Page   int    `json:"page,omitempty"   jsonschema:"Page number (default 1)"`
	Limit  int    `json:"limit,omitempty"  jsonschema:"Results per page (default 30, max 100)"`
	Search string `json:"search,omitempty" jsonschema:"Filter by name (like match)"`
}

// MonitoringListInput is the input for the host and service monitoring list tools,
// which support pagination only.
type MonitoringListInput struct {
	Page  int `json:"page,omitempty"  jsonschema:"Page number (default 1)"`
	Limit int `json:"limit,omitempty" jsonschema:"Results per page (default 30, max 100)"`
}

// MonitoringHostIDListInput is the input for host-scoped monitoring list tools
// (host services and host timeline), which support pagination only.
type MonitoringHostIDListInput struct {
	HostID int `json:"hostID"           jsonschema:"Host ID"`
	Page   int `json:"page,omitempty"   jsonschema:"Page number (default 1)"`
	Limit  int `json:"limit,omitempty"  jsonschema:"Results per page (default 30, max 100)"`
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

// successResult logs a mutation success at INFO and builds a text result.
//
//nolint:unparam // anyVal is always nil; kept for signature consistency with jsonResult and errorResult.
func successResult(logger *slog.Logger, toolName, format string, args ...any) (res *mcp.CallToolResult, anyVal any) {
	text := fmt.Sprintf(format, args...)
	logger.Info(toolName+" succeeded", "result", text)
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

// wrapLikePattern wraps s with SQL % wildcards for LIKE matching, unless a
// wildcard is already present at that boundary. An empty string is unchanged.
func wrapLikePattern(s string) string {
	if s == "" {
		return s
	}
	if s[0] != '%' {
		s = "%" + s
	}
	if s[len(s)-1] != '%' {
		s += "%"
	}
	return s
}

// pagingOptions returns the clamped limit and optional page options shared by the
// list builders. Limit defaults to defaultPageSize and is capped at maxPageSize.
func pagingOptions(page, limit int) []centreon.ListOption {
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	opts := []centreon.ListOption{centreon.WithLimit(limit)}
	if page > 0 {
		opts = append(opts, centreon.WithPage(page))
	}
	return opts
}

// buildListOptions converts a ListInput into a centreon.ListOption slice. Search
// terms are wrapped with SQL wildcards (%) for consistent LIKE matching across
// configuration endpoints.
func buildListOptions(in ListInput) []centreon.ListOption {
	opts := pagingOptions(in.Page, in.Limit)
	if in.Search != "" {
		opts = append(opts, centreon.WithSearch(centreon.Lk(searchFieldName, wrapLikePattern(in.Search))))
	}
	return opts
}

// buildMonitoringListOptions converts a MonitoringListInput into a centreon.ListOption
// slice for monitoring endpoints that support pagination only.
func buildMonitoringListOptions(in MonitoringListInput) []centreon.ListOption {
	return pagingOptions(in.Page, in.Limit)
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
	ctx = centreon.WithToolName(ctx, toolName)
	logger.Debug(toolName, "page", in.Page, "limit", in.Limit, "search", in.Search)

	opts := buildListOptions(in)
	resp, err := requester(ctx, opts...)
	if err != nil {
		// Classify the upstream error before it reaches the log or the client:
		// it can be a *url.Error embedding the base-URL credential (CWE-532;
		// #63, #71). This is the single sink shared by every list tool that
		// routes through commonListHandler.
		reason := redact.Reason(err)
		logger.Error("failed: "+toolName, "error", reason)
		res, anyVal := errorResult("failed: %s: %s", toolName, reason)
		return res, anyVal, nil
	}

	logger.Debug(toolName+" completed", "results", len(resp.Result), "total", resp.Meta.Total, "page", resp.Meta.Page)

	res, anyVal := jsonResult(resp)
	return res, anyVal, nil
}
