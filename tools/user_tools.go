package tools

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client"
)

// RegisterUserTools registers all user and contact tools.
func RegisterUserTools(s *mcp.Server, client *centreon.Client, logger *slog.Logger) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_user_list",
		Description: "List users (contacts). Supports pagination and name filtering.",
	}, userListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_user_update",
		Description: "Update an existing user (partial update via PATCH).",
	}, userUpdateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_contact_group_list",
		Description: "List contact groups. Supports pagination and name filtering.",
	}, contactGroupListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_contact_template_list",
		Description: "List contact templates. Supports pagination and name filtering.",
	}, contactTemplateListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_user_filter_list",
		Description: "List user filters. Supports pagination and name filtering.",
	}, userFilterListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_user_filter_create",
		Description: "Create a new user filter.",
	}, userFilterCreateHandler(client, logger))
}

// UpdateUserInput is the input for the centreon_user_update tool.
type UpdateUserInput struct {
	ID    int     `json:"id"             jsonschema:"User ID"`
	Name  *string `json:"name,omitempty" jsonschema:"User name"`
	Alias *string `json:"alias,omitempty" jsonschema:"User alias"`
	Email *string `json:"email,omitempty" jsonschema:"User email address"`
}

// CreateUserFilterInput is the input for the centreon_user_filter_create tool.
type CreateUserFilterInput struct {
	Name     string                    `json:"name"               jsonschema:"Filter name"`
	Criteria []centreon.FilterCriteria `json:"criteria,omitempty" jsonschema:"Filter criteria"`
}

func userListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_user_list", in, client.Users.List)
	}
}

func userUpdateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in UpdateUserInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in UpdateUserInput) (*mcp.CallToolResult, any, error) {
		logger.Info("centreon_user_update", "id", in.ID)
		req := centreon.UpdateUserRequest{
			Name:  in.Name,
			Alias: in.Alias,
			Email: in.Email,
		}
		if err := client.Users.Update(ctx, in.ID, req); err != nil {
			logger.Error("failed: centreon_user_update", "error", err, "id", in.ID)
			res, anyVal := errorResult("failed to update user %d: %v", in.ID, err)
			return res, anyVal, nil
		}
		res, anyVal := textResult("Updated user %d", in.ID)
		return res, anyVal, nil
	}
}

func contactGroupListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_contact_group_list", in, client.ContactGroups.List)
	}
}

func contactTemplateListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_contact_template_list", in, client.ContactTemplates.List)
	}
}

func userFilterListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_user_filter_list", in, client.UserFilters.List)
	}
}

func userFilterCreateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in CreateUserFilterInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateUserFilterInput) (*mcp.CallToolResult, any, error) {
		logger.Info("centreon_user_filter_create", "name", in.Name)
		id, err := client.UserFilters.Create(ctx, centreon.CreateUserFilterRequest{
			Name:     in.Name,
			Criteria: in.Criteria,
		})
		if err != nil {
			logger.Error("failed: centreon_user_filter_create", "error", err, "name", in.Name)
			res, anyVal := errorResult("failed to create user filter %q: %v", in.Name, err)
			return res, anyVal, nil
		}
		res, anyVal := textResult("Created user filter with ID %d", id)
		return res, anyVal, nil
	}
}
