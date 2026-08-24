package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client/v2"
	"github.com/tphakala/centreon-mcp-go/internal/redact"
)

// RegisterOperationsTools registers all bulk operation tools.
func RegisterOperationsTools(s *Registrar, client *centreon.Client, logger *slog.Logger) {
	addTool(s, &mcp.Tool{
		Name:        "centreon_resource_acknowledge",
		Description: "Acknowledge an active problem on a monitored host or service so Centreon stops repeat notifications while someone investigates, identifying the resource by type (host or service) and id, plus the parent host id for a service, and attaching a required comment. Use this for an ongoing incident; use centreon_resource_downtime to silence a resource during planned maintenance, or centreon_resource_comment to annotate without suppressing notifications. Optional flags keep the acknowledgement sticky until recovery, notify contacts, and persist the comment; clear it later with centreon_acknowledgement_host_cancel or centreon_acknowledgement_service_cancel. Writes to Centreon.",
		Annotations: createTool("Acknowledge resource"),
	}, bulkAcknowledgeHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_resource_downtime",
		Description: "Schedule a maintenance downtime window on a monitored host or service to suppress its alerts and notifications between an RFC3339 start and end time, identifying the resource by type (host or service) and id, plus the parent host id for a service. Use this for planned maintenance; use centreon_resource_acknowledge to silence an unplanned problem that is already active. Fixed downtime covers the whole window while flexible downtime starts on the first problem and runs for the given duration; cancel it with centreon_downtime_host_cancel or centreon_downtime_service_cancel. Writes to Centreon.",
		Annotations: createTool("Schedule resource downtime"),
	}, bulkDowntimeHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_resource_check",
		Description: "Force the monitoring engine to run an active check of a host or service right now instead of waiting for its next scheduled check, identifying the resource by type (host or service) and id, plus the parent host id for a service. Use this to refresh state on demand after a suspected recovery; use centreon_resource_submit instead to push an externally computed result rather than triggering the engine's own check. The check runs asynchronously and updates the resource's live status and output once it completes. Writes to Centreon.",
		Annotations: forceCheckTool("Force resource check"),
	}, bulkCheckHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_resource_submit",
		Description: "Submit a passive check result for a host or service, setting its status (0=OK/UP, 1=WARNING/DOWN, 2=CRITICAL/UNREACHABLE, 3=UNKNOWN) with an output message and optional performance data, identifying the resource by type (host or service) and id, plus the parent host id for a service. Use this to report a result computed outside Centreon; use centreon_resource_check instead to make the engine run its own active check. The submitted status and output immediately replace the resource's live state without the engine re-checking it. Writes to Centreon.",
		Annotations: updateTool("Submit check result"),
	}, bulkSubmitHandler(client, logger))

	addTool(s, &mcp.Tool{
		Name:        "centreon_resource_comment",
		Description: "Attach a free-text comment to a monitored host or service to record context for operators, identifying the resource by type (host or service) and id, plus the parent host id for a service. Use this to annotate a resource without changing its state; use centreon_resource_acknowledge instead when you also want to suppress notifications for an active problem. The comment appears in the resource timeline and does not affect checks or alerting. Writes to Centreon.",
		Annotations: createTool("Comment on resource"),
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
		ctx = centreon.WithToolName(ctx, "centreon_resource_acknowledge")
		logger.Info("centreon_resource_acknowledge", "type", in.Type, "id", in.ID, "comment", in.Comment)
		ref := centreon.ResourceRef{Type: in.Type, ID: in.ID}
		if in.ParentID != 0 {
			ref.Parent = &centreon.ParentRef{ID: in.ParentID}
		}
		req := &centreon.AcknowledgeRequest{
			Resources:           []centreon.ResourceRef{ref},
			Comment:             in.Comment,
			IsSticky:            in.IsSticky,
			IsNotifyContacts:    in.IsNotifyContacts,
			IsPersistentComment: in.IsPersistentComment,
		}
		if err := client.Operations.Acknowledge(ctx, req); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_resource_acknowledge", "error", reason)
			res, anyVal := errorResult("failed to acknowledge: %s", reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_resource_acknowledge", "Acknowledged %s %d", in.Type, in.ID)
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
		ctx = centreon.WithToolName(ctx, "centreon_resource_downtime")
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
			ref.Parent = &centreon.ParentRef{ID: in.ParentID}
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
			reason := redact.Reason(err)
			logger.Error("failed: centreon_resource_downtime", "error", reason)
			res, anyVal := errorResult("failed to schedule downtime: %s", reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_resource_downtime", "Scheduled downtime for %s %d", in.Type, in.ID)
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
		ctx = centreon.WithToolName(ctx, "centreon_resource_check")
		logger.Info("centreon_resource_check", "type", in.Type, "id", in.ID)
		ref := centreon.ResourceRef{Type: in.Type, ID: in.ID}
		if in.ParentID != 0 {
			ref.Parent = &centreon.ParentRef{ID: in.ParentID}
		}
		req := &centreon.CheckRequest{
			Resources: []centreon.ResourceRef{ref},
		}
		if err := client.Operations.Check(ctx, req); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_resource_check", "error", reason)
			res, anyVal := errorResult("failed to force check: %s", reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_resource_check", "Forced check for %s %d", in.Type, in.ID)
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
		ctx = centreon.WithToolName(ctx, "centreon_resource_submit")
		logger.Info("centreon_resource_submit", "type", in.Type, "id", in.ID, "status", in.Status)
		var parent *centreon.ParentRef
		if in.ParentID != 0 {
			parent = &centreon.ParentRef{ID: in.ParentID}
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
			reason := redact.Reason(err)
			logger.Error("failed: centreon_resource_submit", "error", reason)
			res, anyVal := errorResult("failed to submit check result: %s", reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_resource_submit", "Submitted check result for %s %d (status=%d)", in.Type, in.ID, in.Status)
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
		ctx = centreon.WithToolName(ctx, "centreon_resource_comment")
		logger.Info("centreon_resource_comment", "type", in.Type, "id", in.ID, "comment", in.Comment)
		ref := centreon.ResourceRef{Type: in.Type, ID: in.ID}
		if in.ParentID != 0 {
			ref.Parent = &centreon.ParentRef{ID: in.ParentID}
		}
		req := &centreon.CommentRequest{
			Resources: []centreon.ResourceRef{ref},
			Comment:   in.Comment,
		}
		if err := client.Operations.Comment(ctx, req); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_resource_comment", "error", reason)
			res, anyVal := errorResult("failed to add comment: %s", reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_resource_comment", "Added comment to %s %d", in.Type, in.ID)
		return res, anyVal, nil
	}
}
