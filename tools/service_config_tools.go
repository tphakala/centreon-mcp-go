package tools

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client/v2"
	"github.com/tphakala/centreon-mcp-go/internal/redact"
)

// RegisterServiceConfigTools registers all service configuration tools.
func RegisterServiceConfigTools(s *mcp.Server, client *centreon.Client, logger *slog.Logger) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_list",
		Description: "Retrieve stored service configuration records across all hosts, with optional name search and pagination. Use this to inventory configured services; for live runtime status call centreon_monitoring_service_list instead, and to restrict the listing to one host use centreon_service_list_by_host. Returns page 1 with 30 results by default (maximum 100 per page) and reflects saved configuration only. Read-only.",
		Annotations: readOnlyTool("List services"),
	}, serviceListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_list_by_host",
		Description: "List the stored service configurations attached to one host, identified by its hostID, with optional name search and pagination. Use this instead of centreon_service_list when you already know the parent host, and to fetch one service's full stored configuration by its id use centreon_service_get. Returns page 1 with 30 results by default (maximum 100 per page) and reflects saved configuration, not live monitoring. Read-only.",
		Annotations: readOnlyTool("List services by host"),
	}, serviceListByHostHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_get",
		Description: "Fetch the full stored configuration of one service by its numeric id, including check, notification, and flapping settings plus custom macros (per the Centreon 25.10 API the macros also include those inherited from service templates and commands). Use this after finding the id with centreon_service_list or centreon_service_list_by_host; for live runtime status use centreon_monitoring_resource_service_get instead. The per-service detail endpoint exists on Centreon 25.10 and later only, so on an older Centreon the tool reports it as unsupported on this Centreon version. Read-only.",
		Annotations: readOnlyTool("Get service"),
	}, serviceGetHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_create",
		Description: "Add a new service configuration to a host, requiring the parent hostID and a service name, and optionally inheriting settings from a service template plus a check command, service groups, categories, and custom macros. Use this to define a service; to change an existing one call centreon_service_update, and to remove one call centreon_service_delete. The service is saved to configuration only and does not begin monitoring until centreon_poller_apply pushes the change to the pollers. Writes to Centreon.",
		Annotations: createTool("Create service"),
	}, serviceCreateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_update",
		Description: "Modify selected fields of an existing service configuration identified by its id, such as name, check command, check intervals, active-check mode, or activation, leaving unspecified fields unchanged. Use this to adjust a service already created with centreon_service_create; to add a new one use that tool and to remove one use centreon_service_delete. This is a partial update saved to configuration only, taking effect after centreon_poller_apply pushes it to the pollers. Writes to Centreon.",
		Annotations: updateTool("Update service"),
	}, serviceUpdateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_delete",
		Description: "Permanently remove a service configuration identified by its id. Use this to delete a service defined with centreon_service_create; to change one without removing it use centreon_service_update instead. The deletion is applied to configuration only and takes effect once centreon_poller_apply pushes it to the pollers. Writes to Centreon.",
		Annotations: deleteTool("Delete service"),
	}, serviceDeleteHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_group_list",
		Description: "List stored service group configurations, the named collections that bundle services together, with optional name search and pagination. Use this rather than centreon_service_list, which returns individual services; create a group with centreon_service_group_create and remove one with centreon_service_group_delete. Returns page 1 with 30 results by default (maximum 100 per page) and reflects saved configuration only. Read-only.",
		Annotations: readOnlyTool("List service groups"),
	}, serviceGroupListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_group_create",
		Description: "Define a new service group, a named collection of services, requiring a group name and accepting an optional alias. Use this for grouping rather than centreon_service_create, which defines an individual service; list existing groups with centreon_service_group_list and remove one with centreon_service_group_delete. The group is saved to configuration only and takes effect after centreon_poller_apply pushes it to the pollers. Writes to Centreon.",
		Annotations: createTool("Create service group"),
	}, serviceGroupCreateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_group_delete",
		Description: "Remove a service group configuration identified by its id, deleting the grouping without affecting the member services themselves. Use this to undo a group created with centreon_service_group_create; to review existing groups first call centreon_service_group_list. The deletion is saved to configuration only and takes effect once centreon_poller_apply pushes it to the pollers. Writes to Centreon.",
		Annotations: deleteTool("Delete service group"),
	}, serviceGroupDeleteHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_category_list",
		Description: "List stored service category configurations, the labels used to classify services for filtering and reporting, with optional name search and pagination. Use this rather than centreon_service_group_list, which returns groupings of services; create a category with centreon_service_category_create and remove one with centreon_service_category_delete. Returns page 1 with 30 results by default (maximum 100 per page) and reflects saved configuration only. Read-only.",
		Annotations: readOnlyTool("List service categories"),
	}, serviceCategoryListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_category_create",
		Description: "Define a new service category, a classification label applied to services, requiring a category name and accepting an optional alias. Use this for classification rather than centreon_service_group_create, which builds a collection of services; list existing categories with centreon_service_category_list and remove one with centreon_service_category_delete. The category is saved to configuration only and takes effect after centreon_poller_apply pushes it to the pollers. Writes to Centreon.",
		Annotations: createTool("Create service category"),
	}, serviceCategoryCreateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_category_delete",
		Description: "Remove a service category configuration identified by its id, deleting the classification label without affecting the services that carried it. Use this to undo a category created with centreon_service_category_create; to review existing categories first call centreon_service_category_list. The deletion is saved to configuration only and takes effect once centreon_poller_apply pushes it to the pollers. Writes to Centreon.",
		Annotations: deleteTool("Delete service category"),
	}, serviceCategoryDeleteHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_severity_list",
		Description: "List stored service severity configurations, the prioritization levels that rank how critical a service is, with optional name search and pagination. Use this rather than centreon_service_category_list, which returns classification labels; create a severity with centreon_service_severity_create, change one with centreon_service_severity_update, and remove one with centreon_service_severity_delete. Returns page 1 with 30 results by default (maximum 100 per page) and reflects saved configuration only. Read-only.",
		Annotations: readOnlyTool("List service severities"),
	}, serviceSeverityListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_severity_create",
		Description: "Define a new service severity, a prioritization level for ranking services, requiring a name, a numeric level where a lower number is more severe, and an icon id, with an optional alias. Use this to add a severity; to overwrite an existing one call centreon_service_severity_update, and to remove one call centreon_service_severity_delete. The severity is saved to configuration only and takes effect after centreon_poller_apply pushes it to the pollers. Writes to Centreon.",
		Annotations: createTool("Create service severity"),
	}, serviceSeverityCreateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_severity_update",
		Description: "Overwrite an existing service severity identified by its id, replacing its name, numeric level (lower is more severe), icon id, and optional alias as a full update that sets every field, so supply all intended values. Use this to change a severity created with centreon_service_severity_create; to remove one instead call centreon_service_severity_delete. The change is saved to configuration only and takes effect after centreon_poller_apply pushes it to the pollers. Writes to Centreon.",
		Annotations: updateTool("Update service severity"),
	}, serviceSeverityUpdateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_severity_delete",
		Description: "Remove a service severity configuration identified by its id, deleting the prioritization level from the platform. Use this to undo a severity created with centreon_service_severity_create; to change one instead call centreon_service_severity_update. The deletion is saved to configuration only and takes effect once centreon_poller_apply pushes it to the pollers. Writes to Centreon.",
		Annotations: deleteTool("Delete service severity"),
	}, serviceSeverityDeleteHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_service_template_list",
		Description: "List stored service template configurations, the reusable presets that new services inherit their settings from, with optional name search and pagination. Use this to find a template id to pass to centreon_service_create; for actual services rather than their templates use centreon_service_list. Returns page 1 with 30 results by default (maximum 100 per page) and reflects saved configuration only. Read-only.",
		Annotations: readOnlyTool("List service templates"),
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
	CheckCommandID      *int    `json:"checkCommandID,omitempty"        jsonschema:"Check command ID"`
	MaxCheckAttempts    *int    `json:"maxCheckAttempts,omitempty"      jsonschema:"Maximum check attempts"`
	NormalCheckInterval *int    `json:"normalCheckInterval,omitempty"   jsonschema:"Normal check interval in seconds"`
	RetryCheckInterval  *int    `json:"retryCheckInterval,omitempty"    jsonschema:"Retry check interval in seconds"`
	ActiveCheckEnabled  *int    `json:"activeCheckEnabled,omitempty"    jsonschema:"Active check toggle (0=default, 1=enabled, 2=disabled)"`
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

// CreateServiceSeverityInput is the input for the centreon_service_severity_create tool.
type CreateServiceSeverityInput struct {
	Name   string `json:"name"            jsonschema:"Service severity name"`
	Alias  string `json:"alias,omitempty" jsonschema:"Service severity alias"`
	Level  int    `json:"level"           jsonschema:"Severity level (lower = more severe)"`
	IconID int    `json:"iconID"          jsonschema:"Icon ID"`
}

// UpdateServiceSeverityInput is the input for the centreon_service_severity_update tool.
type UpdateServiceSeverityInput struct {
	ID     int    `json:"id"              jsonschema:"Service severity ID"`
	Name   string `json:"name"            jsonschema:"Service severity name"`
	Alias  string `json:"alias,omitempty" jsonschema:"Service severity alias"`
	Level  int    `json:"level"           jsonschema:"Severity level (lower = more severe)"`
	IconID int    `json:"iconID"          jsonschema:"Icon ID"`
}

func serviceListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_service_list", in, client.Services.List)
	}
}

func serviceListByHostHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in HostIDListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in HostIDListInput) (*mcp.CallToolResult, any, error) {
		listIn := ListInput{Page: in.Page, Limit: in.Limit, Search: in.Search}
		return commonListHandler(ctx, logger, "centreon_service_list_by_host", listIn, func(ctx context.Context, opts ...centreon.ListOption) (*centreon.ListResponse[centreon.Service], error) {
			return client.Services.ListByHost(ctx, in.HostID, opts...)
		})
	}
}

// serviceGetHandlerFn backs centreon_service_get. The per-service detail GET
// carries custom macros but exists only on Centreon 25.10 and later; unlike
// hostGetHandlerFn there is no macro-free list fallback (ServiceService has no
// GetByID). On an older Centreon the per-id GET route is unregistered, so the
// call returns a routing 404 that versionSensitiveReason renders with the precise
// "API route not present" hint; a resource 404 (the service is genuinely missing)
// or a proxy that strips the route body falls back to the ambiguous wording.
// getDetail is injected for testing.
func serviceGetHandlerFn(
	getDetail func(context.Context, int) (*centreon.ServiceDetail, error),
	logger *slog.Logger,
) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_service_get")
		logger.Debug("centreon_service_get", "id", in.ID)
		detail, err := getDetail(ctx, in.ID)
		if err != nil {
			reason := versionSensitiveReason(err)
			logger.Error("failed: centreon_service_get", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to get service %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(detail)
		return res, anyVal, nil
	}
}

func serviceGetHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return serviceGetHandlerFn(client.Services.Get, logger)
}

func serviceCreateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in CreateServiceInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateServiceInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_service_create")
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
			reason := redact.Reason(err)
			logger.Error("failed: centreon_service_create", "error", reason, "name", in.Name)
			res, anyVal := errorResult("failed to create service %q: %s", in.Name, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_service_create", "Created service with ID %d", id)
		return res, anyVal, nil
	}
}

func serviceUpdateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in UpdateServiceInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in UpdateServiceInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_service_update")
		logger.Info("centreon_service_update", "id", in.ID)
		req := centreon.UpdateServiceRequest{
			Name:                in.Name,
			CheckCommandID:      in.CheckCommandID,
			MaxCheckAttempts:    in.MaxCheckAttempts,
			NormalCheckInterval: in.NormalCheckInterval,
			RetryCheckInterval:  in.RetryCheckInterval,
			ActiveCheckEnabled:  in.ActiveCheckEnabled,
			IsActivated:         in.IsActivated,
		}
		if err := client.Services.Update(ctx, in.ID, &req); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_service_update", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to update service %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_service_update", "Updated service %d", in.ID)
		return res, anyVal, nil
	}
}

func serviceDeleteHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_service_delete")
		logger.Info("centreon_service_delete", "id", in.ID)
		if err := client.Services.Delete(ctx, in.ID); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_service_delete", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to delete service %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_service_delete", "Deleted service %d", in.ID)
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
		ctx = centreon.WithToolName(ctx, "centreon_service_group_create")
		logger.Info("centreon_service_group_create", "name", in.Name)
		id, err := client.ServiceGroups.Create(ctx, centreon.CreateServiceGroupRequest{
			Name:  in.Name,
			Alias: in.Alias,
		})
		if err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_service_group_create", "error", reason, "name", in.Name)
			res, anyVal := errorResult("failed to create service group %q: %s", in.Name, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_service_group_create", "Created service group with ID %d", id)
		return res, anyVal, nil
	}
}

func serviceGroupDeleteHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_service_group_delete")
		logger.Info("centreon_service_group_delete", "id", in.ID)
		if err := client.ServiceGroups.Delete(ctx, in.ID); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_service_group_delete", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to delete service group %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_service_group_delete", "Deleted service group %d", in.ID)
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
		ctx = centreon.WithToolName(ctx, "centreon_service_category_create")
		logger.Info("centreon_service_category_create", "name", in.Name)
		id, err := client.ServiceCategories.Create(ctx, centreon.CreateServiceCategoryRequest{
			Name:  in.Name,
			Alias: in.Alias,
		})
		if err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_service_category_create", "error", reason, "name", in.Name)
			res, anyVal := errorResult("failed to create service category %q: %s", in.Name, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_service_category_create", "Created service category with ID %d", id)
		return res, anyVal, nil
	}
}

func serviceCategoryDeleteHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_service_category_delete")
		logger.Info("centreon_service_category_delete", "id", in.ID)
		if err := client.ServiceCategories.Delete(ctx, in.ID); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_service_category_delete", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to delete service category %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_service_category_delete", "Deleted service category %d", in.ID)
		return res, anyVal, nil
	}
}

func serviceSeverityListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_service_severity_list", in, client.ServiceSeverities.List)
	}
}

func serviceSeverityCreateHandlerFn(
	fn func(context.Context, centreon.CreateServiceSeverityRequest) (int, error),
	logger *slog.Logger,
) func(ctx context.Context, req *mcp.CallToolRequest, in CreateServiceSeverityInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateServiceSeverityInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_service_severity_create")
		logger.Info("centreon_service_severity_create", "name", in.Name)
		id, err := fn(ctx, centreon.CreateServiceSeverityRequest{
			Name:   in.Name,
			Alias:  in.Alias,
			Level:  in.Level,
			IconID: in.IconID,
		})
		if err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_service_severity_create", "error", reason, "name", in.Name)
			res, anyVal := errorResult("failed to create service severity %q: %s", in.Name, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_service_severity_create", "Created service severity with ID %d", id)
		return res, anyVal, nil
	}
}

func serviceSeverityCreateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in CreateServiceSeverityInput) (*mcp.CallToolResult, any, error) {
	return serviceSeverityCreateHandlerFn(client.ServiceSeverities.Create, logger)
}

func serviceSeverityUpdateHandlerFn(
	fn func(context.Context, int, centreon.UpdateServiceSeverityRequest) error,
	logger *slog.Logger,
) func(ctx context.Context, req *mcp.CallToolRequest, in UpdateServiceSeverityInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in UpdateServiceSeverityInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_service_severity_update")
		logger.Info("centreon_service_severity_update", "id", in.ID)
		if err := fn(ctx, in.ID, centreon.UpdateServiceSeverityRequest{
			Name:   in.Name,
			Alias:  in.Alias,
			Level:  in.Level,
			IconID: in.IconID,
		}); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_service_severity_update", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to update service severity %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_service_severity_update", "Updated service severity %d", in.ID)
		return res, anyVal, nil
	}
}

func serviceSeverityUpdateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in UpdateServiceSeverityInput) (*mcp.CallToolResult, any, error) {
	return serviceSeverityUpdateHandlerFn(client.ServiceSeverities.Update, logger)
}

func serviceSeverityDeleteHandlerFn(
	fn func(context.Context, int) error,
	logger *slog.Logger,
) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_service_severity_delete")
		logger.Info("centreon_service_severity_delete", "id", in.ID)
		if err := fn(ctx, in.ID); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_service_severity_delete", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to delete service severity %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_service_severity_delete", "Deleted service severity %d", in.ID)
		return res, anyVal, nil
	}
}

func serviceSeverityDeleteHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return serviceSeverityDeleteHandlerFn(client.ServiceSeverities.Delete, logger)
}

func serviceTemplateListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_service_template_list", in, client.ServiceTemplates.List)
	}
}
