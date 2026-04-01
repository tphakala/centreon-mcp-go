package tools

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client"
)

// RegisterServiceConfigTools registers all service configuration tools.
func RegisterServiceConfigTools(s *mcp.Server, client *centreon.Client, logger *slog.Logger) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_list",
		Description: "List service configurations. Supports pagination and name filtering.",
	}, serviceListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_get",
		Description: "Get a single service configuration by ID.",
	}, serviceGetHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_create",
		Description: "Create a new service configuration.",
	}, serviceCreateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_update",
		Description: "Update an existing service configuration (partial update).",
	}, serviceUpdateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_delete",
		Description: "Delete a service configuration by ID.",
	}, serviceDeleteHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_group_list",
		Description: "List service group configurations. Supports pagination and name filtering.",
	}, serviceGroupListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_group_create",
		Description: "Create a new service group configuration.",
	}, serviceGroupCreateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_group_delete",
		Description: "Delete a service group configuration by ID.",
	}, serviceGroupDeleteHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_category_list",
		Description: "List service category configurations. Supports pagination and name filtering.",
	}, serviceCategoryListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_category_create",
		Description: "Create a new service category configuration.",
	}, serviceCategoryCreateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_category_delete",
		Description: "Delete a service category configuration by ID.",
	}, serviceCategoryDeleteHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_severity_list",
		Description: "List service severity configurations. Supports pagination and name filtering.",
	}, serviceSeverityListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_template_list",
		Description: "List service template configurations. Supports pagination and name filtering.",
	}, serviceTemplateListHandler(client, logger))
}

// CreateServiceInput is the input for the centreon_service_create tool.
type CreateServiceInput struct {
	HostID            int          `json:"hostID"                       jsonschema:"Host ID"`
	Name              string       `json:"name"                         jsonschema:"Service name"`
	Alias             string       `json:"alias,omitempty"              jsonschema:"Service alias"`
	CheckCommandID    int          `json:"checkCommandID,omitempty"     jsonschema:"Check command ID"`
	ServiceTemplateID int          `json:"serviceTemplateID,omitempty"  jsonschema:"Service template ID to inherit config from"`
	ServiceGroups     []int        `json:"serviceGroups,omitempty"      jsonschema:"Service group IDs"`
	ServiceCategories []int        `json:"serviceCategories,omitempty"  jsonschema:"Service category IDs"`
	Macros            []MacroInput `json:"macros,omitempty"             jsonschema:"Custom macros"`
}

// UpdateServiceInput is the input for the centreon_service_update tool.
type UpdateServiceInput struct {
	ID                  int     `json:"id"                              jsonschema:"Service ID"`
	Name                *string `json:"name,omitempty"                  jsonschema:"Service name"`
	Alias               *string `json:"alias,omitempty"                 jsonschema:"Service alias"`
	CheckCommandID      *int    `json:"checkCommandID,omitempty"        jsonschema:"Check command ID"`
	MaxCheckAttempts    *int    `json:"maxCheckAttempts,omitempty"      jsonschema:"Maximum check attempts"`
	NormalCheckInterval *int    `json:"normalCheckInterval,omitempty"   jsonschema:"Normal check interval in seconds"`
	RetryCheckInterval  *int    `json:"retryCheckInterval,omitempty"    jsonschema:"Retry check interval in seconds"`
	ActiveChecksEnabled *bool   `json:"activeChecksEnabled,omitempty"   jsonschema:"Whether active checks are enabled"`
	IsActivated         *bool   `json:"isActivated,omitempty"           jsonschema:"Whether the service is activated"`
}

// CreateServiceGroupInput is the input for the centreon_service_group_create tool.
type CreateServiceGroupInput struct {
	Name  string `json:"name"            jsonschema:"Service group name"`
	Alias string `json:"alias,omitempty" jsonschema:"Service group alias"`
}

// CreateServiceCategoryInput is the input for the centreon_service_category_create tool.
type CreateServiceCategoryInput struct {
	Name  string `json:"name"            jsonschema:"Service category name"`
	Alias string `json:"alias,omitempty" jsonschema:"Service category alias"`
}

func serviceListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_service_list", in, client.Services.List)
	}
}

func serviceGetHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		logger.Debug("centreon_service_get", "id", in.ID)
		svc, err := client.Services.GetByID(ctx, in.ID)
		if err != nil {
			logger.Error("failed: centreon_service_get", "error", err, "id", in.ID)
			res, anyVal := errorResult("failed to get service %d: %v", in.ID, err)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(svc)
		return res, anyVal, nil
	}
}

func serviceCreateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in CreateServiceInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateServiceInput) (*mcp.CallToolResult, any, error) {
		logger.Info("centreon_service_create", "name", in.Name, "hostID", in.HostID)
		macros := make([]centreon.Macro, 0, len(in.Macros))
		for _, m := range in.Macros {
			macros = append(macros, centreon.Macro{
				Name: m.Name, Value: m.Value, IsPassword: m.IsPassword, Description: m.Description,
			})
		}
		id, err := client.Services.Create(ctx, &centreon.CreateServiceRequest{
			HostID:            in.HostID,
			Name:              in.Name,
			Alias:             in.Alias,
			CheckCommandID:    in.CheckCommandID,
			ServiceTemplateID: in.ServiceTemplateID,
			ServiceGroups:     in.ServiceGroups,
			ServiceCategories: in.ServiceCategories,
			Macros:            macros,
		})
		if err != nil {
			logger.Error("failed: centreon_service_create", "error", err, "name", in.Name)
			res, anyVal := errorResult("failed to create service %q: %v", in.Name, err)
			return res, anyVal, nil
		}
		res, anyVal := textResult("Created service with ID %d", id)
		return res, anyVal, nil
	}
}

func serviceUpdateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in UpdateServiceInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in UpdateServiceInput) (*mcp.CallToolResult, any, error) {
		logger.Info("centreon_service_update", "id", in.ID)
		req := centreon.UpdateServiceRequest{
			Name:                in.Name,
			Alias:               in.Alias,
			CheckCommandID:      in.CheckCommandID,
			MaxCheckAttempts:    in.MaxCheckAttempts,
			NormalCheckInterval: in.NormalCheckInterval,
			RetryCheckInterval:  in.RetryCheckInterval,
			ActiveChecksEnabled: in.ActiveChecksEnabled,
			IsActivated:         in.IsActivated,
		}
		if err := client.Services.Update(ctx, in.ID, &req); err != nil {
			logger.Error("failed: centreon_service_update", "error", err, "id", in.ID)
			res, anyVal := errorResult("failed to update service %d: %v", in.ID, err)
			return res, anyVal, nil
		}
		res, anyVal := textResult("Updated service %d", in.ID)
		return res, anyVal, nil
	}
}

func serviceDeleteHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		logger.Info("centreon_service_delete", "id", in.ID)
		if err := client.Services.Delete(ctx, in.ID); err != nil {
			logger.Error("failed: centreon_service_delete", "error", err, "id", in.ID)
			res, anyVal := errorResult("failed to delete service %d: %v", in.ID, err)
			return res, anyVal, nil
		}
		res, anyVal := textResult("Deleted service %d", in.ID)
		return res, anyVal, nil
	}
}

func serviceGroupListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_service_group_list", in, client.ServiceGroups.List)
	}
}

func serviceGroupCreateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in CreateServiceGroupInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateServiceGroupInput) (*mcp.CallToolResult, any, error) {
		logger.Info("centreon_service_group_create", "name", in.Name)
		id, err := client.ServiceGroups.Create(ctx, centreon.CreateServiceGroupRequest{
			Name:  in.Name,
			Alias: in.Alias,
		})
		if err != nil {
			logger.Error("failed: centreon_service_group_create", "error", err, "name", in.Name)
			res, anyVal := errorResult("failed to create service group %q: %v", in.Name, err)
			return res, anyVal, nil
		}
		res, anyVal := textResult("Created service group with ID %d", id)
		return res, anyVal, nil
	}
}

func serviceGroupDeleteHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		logger.Info("centreon_service_group_delete", "id", in.ID)
		if err := client.ServiceGroups.Delete(ctx, in.ID); err != nil {
			logger.Error("failed: centreon_service_group_delete", "error", err, "id", in.ID)
			res, anyVal := errorResult("failed to delete service group %d: %v", in.ID, err)
			return res, anyVal, nil
		}
		res, anyVal := textResult("Deleted service group %d", in.ID)
		return res, anyVal, nil
	}
}

func serviceCategoryListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_service_category_list", in, client.ServiceCategories.List)
	}
}

func serviceCategoryCreateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in CreateServiceCategoryInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateServiceCategoryInput) (*mcp.CallToolResult, any, error) {
		logger.Info("centreon_service_category_create", "name", in.Name)
		id, err := client.ServiceCategories.Create(ctx, centreon.CreateServiceCategoryRequest{
			Name:  in.Name,
			Alias: in.Alias,
		})
		if err != nil {
			logger.Error("failed: centreon_service_category_create", "error", err, "name", in.Name)
			res, anyVal := errorResult("failed to create service category %q: %v", in.Name, err)
			return res, anyVal, nil
		}
		res, anyVal := textResult("Created service category with ID %d", id)
		return res, anyVal, nil
	}
}

func serviceCategoryDeleteHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		logger.Info("centreon_service_category_delete", "id", in.ID)
		if err := client.ServiceCategories.Delete(ctx, in.ID); err != nil {
			logger.Error("failed: centreon_service_category_delete", "error", err, "id", in.ID)
			res, anyVal := errorResult("failed to delete service category %d: %v", in.ID, err)
			return res, anyVal, nil
		}
		res, anyVal := textResult("Deleted service category %d", in.ID)
		return res, anyVal, nil
	}
}

func serviceSeverityListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_service_severity_list", in, client.ServiceSeverities.List)
	}
}

func serviceTemplateListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_service_template_list", in, client.ServiceTemplates.List)
	}
}
