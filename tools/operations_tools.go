package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client"
)

// RegisterOperationsTools registers all bulk operation tools.
func RegisterOperationsTools(s *mcp.Server, client *centreon.Client, logger *slog.Logger) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_resource_acknowledge",
		Description: "Acknowledge a host or service resource to suppress notifications.",
	}, bulkAcknowledgeHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_resource_downtime",
		Description: "Schedule downtime for a host or service resource.",
	}, bulkDowntimeHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_resource_check",
		Description: "Force an immediate check for a host or service resource.",
	}, bulkCheckHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_resource_submit",
		Description: "Submit a passive check result for a host or service resource.",
	}, bulkSubmitHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_resource_comment",
		Description: "Add a comment to a host or service resource.",
	}, bulkCommentHandler(client, logger))
}

// BulkAcknowledgeInput is the input for the centreon_resource_acknowledge tool.
type BulkAcknowledgeInput struct {
	Type                string `json:"type"                          jsonschema:"Resource type: host or service,enum=host,enum=service"`
	ID                  int    `json:"id"                            jsonschema:"Resource ID"`
	ParentID            int    `json:"parentID,omitempty"            jsonschema:"Parent host ID (required for services)"`
	Comment             string `json:"comment"                       jsonschema:"Acknowledgement comment"`
	IsSticky            bool   `json:"isSticky,omitempty"            jsonschema:"Sticky (stays until recovery)"`
	IsNotifyContacts    bool   `json:"isNotifyContacts,omitempty"    jsonschema:"Notify contacts"`
	IsPersistentComment bool   `json:"isPersistentComment,omitempty" jsonschema:"Persistent comment"`
}

func bulkAcknowledgeHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in BulkAcknowledgeInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in BulkAcknowledgeInput) (*mcp.CallToolResult, any, error) {
		logger.Info("centreon_resource_acknowledge", "type", in.Type, "id", in.ID, "comment", in.Comment)
		ref := centreon.ResourceRef{Type: in.Type, ID: in.ID}
		if in.ParentID != 0 {
			ref.Parent = &centreon.ResourceRef{Type: "host", ID: in.ParentID}
		}
		req := &centreon.AcknowledgeRequest{
			Resources:           []centreon.ResourceRef{ref},
			Comment:             in.Comment,
			IsSticky:            in.IsSticky,
			IsNotifyContacts:    in.IsNotifyContacts,
			IsPersistentComment: in.IsPersistentComment,
		}
		if err := client.Operations.Acknowledge(ctx, req); err != nil {
			logger.Error("failed: centreon_resource_acknowledge", "error", err)
			res, anyVal := errorResult("failed to acknowledge: %v", err)
			return res, anyVal, nil
		}
		res, anyVal := textResult("Acknowledged %s %d", in.Type, in.ID)
		return res, anyVal, nil
	}
}

// BulkDowntimeInput is the input for the centreon_resource_downtime tool.
type BulkDowntimeInput struct {
	Type      string `json:"type"                jsonschema:"Resource type: host or service,enum=host,enum=service"`
	ID        int    `json:"id"                  jsonschema:"Resource ID"`
	ParentID  int    `json:"parentID,omitempty"  jsonschema:"Parent host ID (required for services)"`
	Comment   string `json:"comment"             jsonschema:"Downtime comment"`
	StartTime string `json:"startTime"           jsonschema:"Downtime start time (RFC3339 format, e.g. 2006-01-02T15:04:05Z)"`
	EndTime   string `json:"endTime"             jsonschema:"Downtime end time (RFC3339 format, e.g. 2006-01-02T15:04:05Z)"`
	IsFixed   bool   `json:"isFixed,omitempty"   jsonschema:"Fixed downtime (true) or flexible (false)"`
	Duration  int    `json:"duration,omitempty"  jsonschema:"Duration in seconds (for flexible downtime)"`
}

func bulkDowntimeHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in BulkDowntimeInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in BulkDowntimeInput) (*mcp.CallToolResult, any, error) {
		logger.Info("centreon_resource_downtime", "type", in.Type, "id", in.ID, "start", in.StartTime, "end", in.EndTime)
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
		ref := centreon.ResourceRef{Type: in.Type, ID: in.ID}
		if in.ParentID != 0 {
			ref.Parent = &centreon.ResourceRef{Type: "host", ID: in.ParentID}
		}
		downtimeReq := &centreon.DowntimeRequest{
			Resources: []centreon.ResourceRef{ref},
			Comment:   in.Comment,
			StartTime: startTime,
			EndTime:   endTime,
			Fixed:     in.IsFixed,
			Duration:  in.Duration,
		}
		if err := client.Operations.Downtime(ctx, downtimeReq); err != nil {
			logger.Error("failed: centreon_resource_downtime", "error", err)
			res, anyVal := errorResult("failed to schedule downtime: %v", err)
			return res, anyVal, nil
		}
		res, anyVal := textResult("Scheduled downtime for %s %d", in.Type, in.ID)
		return res, anyVal, nil
	}
}

// BulkCheckInput is the input for the centreon_resource_check tool.
type BulkCheckInput struct {
	Type     string `json:"type"               jsonschema:"Resource type: host or service,enum=host,enum=service"`
	ID       int    `json:"id"                 jsonschema:"Resource ID"`
	ParentID int    `json:"parentID,omitempty" jsonschema:"Parent host ID (required for services)"`
}

func bulkCheckHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in BulkCheckInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in BulkCheckInput) (*mcp.CallToolResult, any, error) {
		logger.Info("centreon_resource_check", "type", in.Type, "id", in.ID)
		ref := centreon.ResourceRef{Type: in.Type, ID: in.ID}
		if in.ParentID != 0 {
			ref.Parent = &centreon.ResourceRef{Type: "host", ID: in.ParentID}
		}
		req := &centreon.CheckRequest{
			Resources: []centreon.ResourceRef{ref},
		}
		if err := client.Operations.Check(ctx, req); err != nil {
			logger.Error("failed: centreon_resource_check", "error", err)
			res, anyVal := errorResult("failed to force check: %v", err)
			return res, anyVal, nil
		}
		res, anyVal := textResult("Forced check for %s %d", in.Type, in.ID)
		return res, anyVal, nil
	}
}

// BulkSubmitInput is the input for the centreon_resource_submit tool.
type BulkSubmitInput struct {
	Type     string `json:"type"               jsonschema:"Resource type: host or service,enum=host,enum=service"`
	ID       int    `json:"id"                 jsonschema:"Resource ID"`
	ParentID int    `json:"parentID,omitempty" jsonschema:"Parent host ID (required for services)"`
	Status   int    `json:"status"             jsonschema:"Check result status (0=OK/UP, 1=WARNING/DOWN, 2=CRITICAL/UNREACHABLE, 3=UNKNOWN)"`
	Output   string `json:"output"             jsonschema:"Check output message"`
	PerfData string `json:"perfData,omitempty" jsonschema:"Performance data string"`
}

func bulkSubmitHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in BulkSubmitInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in BulkSubmitInput) (*mcp.CallToolResult, any, error) {
		logger.Info("centreon_resource_submit", "type", in.Type, "id", in.ID, "status", in.Status)
		var parent *centreon.ResourceRef
		if in.ParentID != 0 {
			parent = &centreon.ResourceRef{Type: "host", ID: in.ParentID}
		}
		req := &centreon.SubmitResultRequest{
			Resources: []centreon.SubmitResource{
				{
					Type:     in.Type,
					ID:       in.ID,
					Parent:   parent,
					Status:   in.Status,
					Output:   in.Output,
					PerfData: in.PerfData,
				},
			},
		}
		if err := client.Operations.Submit(ctx, req); err != nil {
			logger.Error("failed: centreon_resource_submit", "error", err)
			res, anyVal := errorResult("failed to submit check result: %v", err)
			return res, anyVal, nil
		}
		res, anyVal := textResult("Submitted check result for %s %d (status=%d)", in.Type, in.ID, in.Status)
		return res, anyVal, nil
	}
}

// BulkCommentInput is the input for the centreon_resource_comment tool.
type BulkCommentInput struct {
	Type     string `json:"type"               jsonschema:"Resource type: host or service,enum=host,enum=service"`
	ID       int    `json:"id"                 jsonschema:"Resource ID"`
	ParentID int    `json:"parentID,omitempty" jsonschema:"Parent host ID (required for services)"`
	Comment  string `json:"comment"            jsonschema:"Comment text"`
}

func bulkCommentHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in BulkCommentInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in BulkCommentInput) (*mcp.CallToolResult, any, error) {
		logger.Info("centreon_resource_comment", "type", in.Type, "id", in.ID, "comment", in.Comment)
		ref := centreon.ResourceRef{Type: in.Type, ID: in.ID}
		if in.ParentID != 0 {
			ref.Parent = &centreon.ResourceRef{Type: "host", ID: in.ParentID}
		}
		req := &centreon.CommentRequest{
			Resources: []centreon.ResourceRef{ref},
			Comment:   in.Comment,
		}
		if err := client.Operations.Comment(ctx, req); err != nil {
			logger.Error("failed: centreon_resource_comment", "error", err)
			res, anyVal := errorResult("failed to add comment: %v", err)
			return res, anyVal, nil
		}
		res, anyVal := textResult("Added comment to %s %d", in.Type, in.ID)
		return res, anyVal, nil
	}
}
