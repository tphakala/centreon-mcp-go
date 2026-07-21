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
		Description: "Retrieve Centreon users (contacts), returning each user's id, name, alias, email, admin flag, and activation state, with an optional name search. To change a user's name, alias, or email use centreon_user_update; for the related grouping and template objects use centreon_contact_group_list or centreon_contact_template_list. Defaults to page 1 with 30 results per page (max 100) and matches the search term as a name substring. Read-only.",
		Annotations: readOnlyTool("List users"),
	}, userListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_user_update",
		Description: "Apply a partial update to one existing user (contact) identified by its numeric id, changing only the name, alias, or email fields you supply and leaving the rest untouched. Use this after locating the target with centreon_user_list; to save user-scoped resource filters instead use centreon_user_filter_create. Sends a PATCH, so omitted fields are preserved, and reports the updated user id. Writes to Centreon.",
		Annotations: updateTool("Update user"),
	}, userUpdateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_contact_group_list",
		Description: "List Centreon contact groups, the named collections of users used as notification recipients, with an optional name search. Use this to discover contact group names and ids that appear in notification policies from centreon_notification_policy_host_get; for the individual members use centreon_user_list. Defaults to page 1 with 30 results per page (max 100) and matches the search term as a name substring. Read-only.",
		Annotations: readOnlyTool("List contact groups"),
	}, contactGroupListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_contact_template_list",
		Description: "List Centreon contact templates, the reusable configuration presets that users (contacts) inherit their settings from, with an optional name search. Use this to find template names and ids that describe contact defaults; for concrete users use centreon_user_list and for user groupings use centreon_contact_group_list. Defaults to page 1 with 30 results per page (max 100) and matches the search term as a name substring. Read-only.",
		Annotations: readOnlyTool("List contact templates"),
	}, contactTemplateListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_user_filter_list",
		Description: "List saved user filters, the named sets of resource-status search criteria stored per user, returning each filter's id, name, and criteria, with an optional name search. Use this to review or find an existing filter before saving a new one with centreon_user_filter_create. Defaults to page 1 with 30 results per page (max 100) and matches the search term as a name substring. Read-only.",
		Annotations: readOnlyTool("List user filters"),
	}, userFilterListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_user_filter_create",
		Description: "Save a new user filter from a required name and an optional list of resource-status criteria, returning the new filter's numeric id. Use this to persist a reusable filter; to review existing filters first call centreon_user_filter_list, and to edit users themselves use centreon_user_update. Adds a new record without altering existing filters. Writes to Centreon.",
		Annotations: createTool("Create user filter"),
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
		ctx = centreon.WithToolName(ctx, "centreon_user_update")
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
		res, anyVal := successResult(logger, "centreon_user_update", "Updated user %d", in.ID)
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
		ctx = centreon.WithToolName(ctx, "centreon_user_filter_create")
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
		res, anyVal := successResult(logger, "centreon_user_filter_create", "Created user filter with ID %d", id)
		return res, anyVal, nil
	}
}
