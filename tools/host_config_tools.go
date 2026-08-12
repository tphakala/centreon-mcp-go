package tools

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client"
	"github.com/tphakala/centreon-mcp-go/internal/redact"
)

// RegisterHostConfigTools registers all host configuration tools.
func RegisterHostConfigTools(s *mcp.Server, client *centreon.Client, logger *slog.Logger) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_list",
		Description: "Retrieve a paginated list of configured hosts (the machines and devices Centreon monitors) with their IDs, names, and addresses. Use this to discover host IDs or browse the inventory; for one host's full configuration by ID use centreon_host_get, and for live up/down state use centreon_monitoring_host_list. Supports page (default 1), limit (default 30, max 100), and name search, and returns stored configuration only. Read-only.",
		Annotations: readOnlyTool("List hosts"),
	}, hostListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_get",
		Description: "Fetch the full stored configuration of a single host by its numeric ID, including address, templates, groups, categories, and check settings. Use this when you already know the host ID; to search or page through hosts use centreon_host_list, and for current up/down monitoring state use centreon_monitoring_host_get. Requires id and returns configuration only, never live status. Read-only.",
		Annotations: readOnlyTool("Get host"),
	}, hostGetHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_create",
		Description: "Define a new host by attaching it to a monitoring server with a name and an IP address or FQDN, plus optional templates, groups, categories, and macros. Use this to add a machine to monitoring; to change an existing host use centreon_host_update, and to remove one use centreon_host_delete. Requires monitoringServerID, name, and address, and the new host stays configuration-only until pushed live with centreon_poller_apply. Writes to Centreon.",
		Annotations: createTool("Create host"),
	}, hostCreateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_update",
		Description: "Modify selected fields of an existing host such as name, address, check command, check intervals, or activation, leaving unspecified fields untouched. Use this to edit a host previously added with centreon_host_create; to create a host use centreon_host_create and to remove one use centreon_host_delete. Requires id, applies a partial update, and the change stays configuration-only until pushed live with centreon_poller_apply. Writes to Centreon.",
		Annotations: updateTool("Update host"),
	}, hostUpdateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_delete",
		Description: "Permanently remove a host from the configuration by its numeric ID. Use this to decommission a machine; to change it instead of removing it use centreon_host_update, and to inspect it first use centreon_host_get. Requires id, and the deletion stays configuration-only until pushed live with centreon_poller_apply. Writes to Centreon.",
		Annotations: deleteTool("Delete host"),
	}, hostDeleteHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_group_list",
		Description: "Retrieve a paginated list of host groups, the named collections that bundle related hosts together for dashboards and filtering. Use this to discover host group IDs; for one group's details by ID use centreon_host_group_get. Supports page (default 1), limit (default 30, max 100), and name search, and returns stored configuration only. Read-only.",
		Annotations: readOnlyTool("List host groups"),
	}, hostGroupListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_group_get",
		Description: "Fetch the stored configuration of a single host group by its numeric ID, including its name and alias. Use this when you already know the group ID; to search or page through host groups use centreon_host_group_list. Requires id and returns configuration only. Read-only.",
		Annotations: readOnlyTool("Get host group"),
	}, hostGroupGetHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_group_create",
		Description: "Define a new host group, a named container for organizing hosts, with a name and optional alias. Use this to add a grouping; to rename an existing group use centreon_host_group_update, and to remove one use centreon_host_group_delete. Requires name, and the new group stays configuration-only until pushed live with centreon_poller_apply. Writes to Centreon.",
		Annotations: createTool("Create host group"),
	}, hostGroupCreateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_group_update",
		Description: "Overwrite the name and alias of an existing host group identified by its numeric ID. Use this to rename or re-describe a group added with centreon_host_group_create; to create a group use centreon_host_group_create and to remove one use centreon_host_group_delete. Requires id and name, replaces both fields, and the change stays configuration-only until pushed live with centreon_poller_apply. Writes to Centreon.",
		Annotations: updateTool("Update host group"),
	}, hostGroupUpdateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_group_delete",
		Description: "Remove a host group by its numeric ID, dissolving the grouping while leaving the member hosts in place. Use this to drop an unused collection; to rename it instead use centreon_host_group_update, and to inspect it first use centreon_host_group_get. Requires id, and the deletion stays configuration-only until pushed live with centreon_poller_apply. Writes to Centreon.",
		Annotations: deleteTool("Delete host group"),
	}, hostGroupDeleteHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_category_list",
		Description: "Retrieve a paginated list of host categories, the classification tags used to label and organize hosts independently of host groups. Use this to discover category IDs; for one category's details by ID use centreon_host_category_get. Supports page (default 1), limit (default 30, max 100), and name search, and returns stored configuration only. Read-only.",
		Annotations: readOnlyTool("List host categories"),
	}, hostCategoryListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_category_get",
		Description: "Fetch the stored configuration of a single host category by its numeric ID, including its name and alias. Use this when you already know the category ID; to search or page through categories use centreon_host_category_list. Requires id and returns configuration only. Read-only.",
		Annotations: readOnlyTool("Get host category"),
	}, hostCategoryGetHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_category_create",
		Description: "Define a new host category, a label for classifying hosts, with a name and optional alias. Use this to add a classification; to rename an existing category use centreon_host_category_update, and to remove one use centreon_host_category_delete. Requires name, and the new category stays configuration-only until pushed live with centreon_poller_apply. Writes to Centreon.",
		Annotations: createTool("Create host category"),
	}, hostCategoryCreateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_category_update",
		Description: "Overwrite the name and alias of an existing host category identified by its numeric ID. Use this to relabel a category added with centreon_host_category_create; to create a category use centreon_host_category_create and to remove one use centreon_host_category_delete. Requires id and name, replaces both fields, and the change stays configuration-only until pushed live with centreon_poller_apply. Writes to Centreon.",
		Annotations: updateTool("Update host category"),
	}, hostCategoryUpdateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_category_delete",
		Description: "Remove a host category by its numeric ID, unassigning that label from any hosts that carried it. Use this to drop an unused classification; to rename it instead use centreon_host_category_update, and to inspect it first use centreon_host_category_get. Requires id, and the deletion stays configuration-only until pushed live with centreon_poller_apply. Writes to Centreon.",
		Annotations: deleteTool("Delete host category"),
	}, hostCategoryDeleteHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_severity_list",
		Description: "Retrieve a paginated list of host severities, the ranked priority levels (each carrying a level value and an icon) that mark how critical a host is. Use this to discover severity IDs; for one severity's details by ID use centreon_host_severity_get. Supports page (default 1), limit (default 30, max 100), and name search, and returns stored configuration only. Read-only.",
		Annotations: readOnlyTool("List host severities"),
	}, hostSeverityListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_severity_get",
		Description: "Fetch the stored configuration of a single host severity by its numeric ID, including its name, level, and icon. Use this when you already know the severity ID; to search or page through severities use centreon_host_severity_list. Requires id and returns configuration only. Read-only.",
		Annotations: readOnlyTool("Get host severity"),
	}, hostSeverityGetHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_severity_create",
		Description: "Define a new host severity, a priority tier with a name, a numeric level where lower is more severe, and an icon. Use this to add a ranking tier; to change an existing severity use centreon_host_severity_update, and to remove one use centreon_host_severity_delete. Requires name, level, and iconID, and the new severity stays configuration-only until pushed live with centreon_poller_apply. Writes to Centreon.",
		Annotations: createTool("Create host severity"),
	}, hostSeverityCreateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_severity_update",
		Description: "Overwrite the name, alias, level, and icon of an existing host severity identified by its numeric ID. Use this to re-rank a severity added with centreon_host_severity_create; to create a severity use centreon_host_severity_create and to remove one use centreon_host_severity_delete. Requires id, name, level, and iconID, replaces those fields, and the change stays configuration-only until pushed live with centreon_poller_apply. Writes to Centreon.",
		Annotations: updateTool("Update host severity"),
	}, hostSeverityUpdateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_severity_delete",
		Description: "Remove a host severity by its numeric ID, clearing that priority tier from any hosts assigned to it. Use this to drop an unused tier; to re-rank it instead use centreon_host_severity_update, and to inspect it first use centreon_host_severity_get. Requires id, and the deletion stays configuration-only until pushed live with centreon_poller_apply. Writes to Centreon.",
		Annotations: deleteTool("Delete host severity"),
	}, hostSeverityDeleteHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_template_list",
		Description: "Retrieve a paginated list of host templates, the reusable blueprints that hosts inherit checks, macros, and settings from. Use this to find template IDs to pass in the templates field of centreon_host_create; this server exposes host templates as a read-only listing with no get, create, update, or delete counterpart. Supports page (default 1), limit (default 30, max 100), and name search, and returns stored configuration only. Read-only.",
		Annotations: readOnlyTool("List host templates"),
	}, hostTemplateListHandler(client, logger))
}

// MacroInput represents a custom macro.
type MacroInput struct {
	Name        string `json:"name"                    jsonschema:"Macro name"`
	Value       string `json:"value,omitempty"         jsonschema:"Macro value"`
	IsPassword  bool   `json:"isPassword,omitempty"    jsonschema:"Whether the value is a password"`
	Description string `json:"description"             jsonschema:"Macro description"`
}

// CreateHostInput is the input for the centreon_host_create tool.
type CreateHostInput struct {
	MonitoringServerID int          `json:"monitoringServerID"         jsonschema:"Monitoring server ID"`
	Name               string       `json:"name"                       jsonschema:"Host name"`
	Address            string       `json:"address"                    jsonschema:"Host IP address or FQDN"`
	Alias              string       `json:"alias,omitempty"            jsonschema:"Host alias"`
	CheckCommandID     int          `json:"checkCommandID,omitempty"   jsonschema:"Check command ID"`
	Templates          []int        `json:"templates,omitempty"        jsonschema:"Host template IDs to inherit services and config from"`
	Groups             []int        `json:"groups,omitempty"           jsonschema:"Host group IDs"`
	Categories         []int        `json:"categories,omitempty"       jsonschema:"Host category IDs"`
	Macros             []MacroInput `json:"macros,omitempty"     jsonschema:"Custom macros"`
}

// UpdateHostInput is the input for the centreon_host_update tool.
type UpdateHostInput struct {
	ID                  int     `json:"id"                              jsonschema:"Host ID"`
	Name                *string `json:"name,omitempty"                  jsonschema:"Host name"`
	Alias               *string `json:"alias,omitempty"                 jsonschema:"Host alias"`
	Address             *string `json:"address,omitempty"               jsonschema:"Host IP address or FQDN"`
	CheckCommandID      *int    `json:"checkCommandID,omitempty"        jsonschema:"Check command ID"`
	MaxCheckAttempts    *int    `json:"maxCheckAttempts,omitempty"      jsonschema:"Maximum check attempts"`
	NormalCheckInterval *int    `json:"normalCheckInterval,omitempty"   jsonschema:"Normal check interval in seconds"`
	RetryCheckInterval  *int    `json:"retryCheckInterval,omitempty"    jsonschema:"Retry check interval in seconds"`
	ActiveCheckEnabled  *int    `json:"activeCheckEnabled,omitempty"    jsonschema:"Active check toggle (0=default, 1=enabled, 2=disabled)"`
	IsActivated         *bool   `json:"isActivated,omitempty"           jsonschema:"Whether the host is activated"`
}

// CreateHostGroupInput is the input for the centreon_host_group_create tool.
type CreateHostGroupInput struct {
	Name  string `json:"name"            jsonschema:"Host group name"`
	Alias string `json:"alias,omitempty" jsonschema:"Host group alias"`
}

// UpdateHostGroupInput is the input for the centreon_host_group_update tool.
type UpdateHostGroupInput struct {
	ID    int    `json:"id"              jsonschema:"Host group ID"`
	Name  string `json:"name"            jsonschema:"Host group name"`
	Alias string `json:"alias,omitempty" jsonschema:"Host group alias"`
}

// CreateHostCategoryInput is the input for the centreon_host_category_create tool.
type CreateHostCategoryInput struct {
	Name  string `json:"name"            jsonschema:"Host category name"`
	Alias string `json:"alias,omitempty" jsonschema:"Host category alias"`
}

// UpdateHostCategoryInput is the input for the centreon_host_category_update tool.
type UpdateHostCategoryInput struct {
	ID    int    `json:"id"              jsonschema:"Host category ID"`
	Name  string `json:"name"            jsonschema:"Host category name"`
	Alias string `json:"alias,omitempty" jsonschema:"Host category alias"`
}

// CreateHostSeverityInput is the input for the centreon_host_severity_create tool.
type CreateHostSeverityInput struct {
	Name   string `json:"name"            jsonschema:"Host severity name"`
	Alias  string `json:"alias,omitempty" jsonschema:"Host severity alias"`
	Level  int    `json:"level"           jsonschema:"Severity level (lower = more severe)"`
	IconID int    `json:"iconID"          jsonschema:"Icon ID"`
}

// UpdateHostSeverityInput is the input for the centreon_host_severity_update tool.
type UpdateHostSeverityInput struct {
	ID     int    `json:"id"              jsonschema:"Host severity ID"`
	Name   string `json:"name"            jsonschema:"Host severity name"`
	Alias  string `json:"alias,omitempty" jsonschema:"Host severity alias"`
	Level  int    `json:"level"           jsonschema:"Severity level (lower = more severe)"`
	IconID int    `json:"iconID"          jsonschema:"Icon ID"`
}

func hostListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_host_list", in, client.Hosts.List)
	}
}

func hostGetHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_host_get")
		logger.Debug("centreon_host_get", "id", in.ID)
		host, err := client.Hosts.GetByID(ctx, in.ID)
		if err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_host_get", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to get host %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(host)
		return res, anyVal, nil
	}
}

func hostCreateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in CreateHostInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateHostInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_host_create")
		logger.Info("centreon_host_create", "name", in.Name, "address", in.Address)
		macros := make([]centreon.Macro, 0, len(in.Macros))
		for _, m := range in.Macros {
			macros = append(macros, centreon.Macro{
				Name: m.Name, Value: m.Value, IsPassword: m.IsPassword, Description: m.Description,
			})
		}
		id, err := client.Hosts.Create(ctx, &centreon.CreateHostRequest{
			MonitoringServerID: in.MonitoringServerID,
			Name:               in.Name,
			Address:            in.Address,
			Alias:              in.Alias,
			CheckCommandID:     in.CheckCommandID,
			Templates:          in.Templates,
			Groups:             in.Groups,
			Categories:         in.Categories,
			Macros:             macros,
		})
		if err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_host_create", "error", reason, "name", in.Name)
			res, anyVal := errorResult("failed to create host %q: %s", in.Name, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_host_create", "Created host with ID %d", id)
		return res, anyVal, nil
	}
}

func hostUpdateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in UpdateHostInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in UpdateHostInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_host_update")
		logger.Info("centreon_host_update", "id", in.ID)
		req := centreon.UpdateHostRequest{
			Name:                in.Name,
			Alias:               in.Alias,
			Address:             in.Address,
			CheckCommandID:      in.CheckCommandID,
			MaxCheckAttempts:    in.MaxCheckAttempts,
			NormalCheckInterval: in.NormalCheckInterval,
			RetryCheckInterval:  in.RetryCheckInterval,
			ActiveCheckEnabled:  in.ActiveCheckEnabled,
			IsActivated:         in.IsActivated,
		}
		if err := client.Hosts.Update(ctx, in.ID, &req); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_host_update", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to update host %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_host_update", "Updated host %d", in.ID)
		return res, anyVal, nil
	}
}

func hostDeleteHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_host_delete")
		logger.Info("centreon_host_delete", "id", in.ID)
		if err := client.Hosts.Delete(ctx, in.ID); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_host_delete", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to delete host %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_host_delete", "Deleted host %d", in.ID)
		return res, anyVal, nil
	}
}

func hostGroupListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_host_group_list", in, client.HostGroups.List)
	}
}

func hostGroupGetHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_host_group_get")
		logger.Debug("centreon_host_group_get", "id", in.ID)
		hg, err := client.HostGroups.Get(ctx, in.ID)
		if err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_host_group_get", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to get host group %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(hg)
		return res, anyVal, nil
	}
}

func hostGroupCreateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in CreateHostGroupInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateHostGroupInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_host_group_create")
		logger.Info("centreon_host_group_create", "name", in.Name)
		id, err := client.HostGroups.Create(ctx, centreon.CreateHostGroupRequest{
			Name:  in.Name,
			Alias: in.Alias,
		})
		if err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_host_group_create", "error", reason, "name", in.Name)
			res, anyVal := errorResult("failed to create host group %q: %s", in.Name, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_host_group_create", "Created host group with ID %d", id)
		return res, anyVal, nil
	}
}

func hostGroupUpdateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in UpdateHostGroupInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in UpdateHostGroupInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_host_group_update")
		logger.Info("centreon_host_group_update", "id", in.ID)
		if err := client.HostGroups.Update(ctx, in.ID, centreon.UpdateHostGroupRequest{
			Name:  in.Name,
			Alias: in.Alias,
		}); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_host_group_update", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to update host group %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_host_group_update", "Updated host group %d", in.ID)
		return res, anyVal, nil
	}
}

func hostGroupDeleteHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_host_group_delete")
		logger.Info("centreon_host_group_delete", "id", in.ID)
		if err := client.HostGroups.Delete(ctx, in.ID); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_host_group_delete", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to delete host group %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_host_group_delete", "Deleted host group %d", in.ID)
		return res, anyVal, nil
	}
}

func hostCategoryListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_host_category_list", in, client.HostCategories.List)
	}
}

func hostCategoryGetHandlerFn(
	fn func(context.Context, int) (*centreon.HostCategory, error),
	logger *slog.Logger,
) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_host_category_get")
		logger.Debug("centreon_host_category_get", "id", in.ID)
		cat, err := fn(ctx, in.ID)
		if err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_host_category_get", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to get host category %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(cat)
		return res, anyVal, nil
	}
}

func hostCategoryGetHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return hostCategoryGetHandlerFn(client.HostCategories.Get, logger)
}

func hostCategoryCreateHandlerFn(
	fn func(context.Context, centreon.CreateHostCategoryRequest) (int, error),
	logger *slog.Logger,
) func(ctx context.Context, req *mcp.CallToolRequest, in CreateHostCategoryInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateHostCategoryInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_host_category_create")
		logger.Info("centreon_host_category_create", "name", in.Name)
		id, err := fn(ctx, centreon.CreateHostCategoryRequest{
			Name:  in.Name,
			Alias: in.Alias,
		})
		if err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_host_category_create", "error", reason, "name", in.Name)
			res, anyVal := errorResult("failed to create host category %q: %s", in.Name, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_host_category_create", "Created host category with ID %d", id)
		return res, anyVal, nil
	}
}

func hostCategoryCreateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in CreateHostCategoryInput) (*mcp.CallToolResult, any, error) {
	return hostCategoryCreateHandlerFn(client.HostCategories.Create, logger)
}

func hostCategoryUpdateHandlerFn(
	fn func(context.Context, int, centreon.UpdateHostCategoryRequest) error,
	logger *slog.Logger,
) func(ctx context.Context, req *mcp.CallToolRequest, in UpdateHostCategoryInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in UpdateHostCategoryInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_host_category_update")
		logger.Info("centreon_host_category_update", "id", in.ID)
		if err := fn(ctx, in.ID, centreon.UpdateHostCategoryRequest{
			Name:  in.Name,
			Alias: in.Alias,
		}); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_host_category_update", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to update host category %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_host_category_update", "Updated host category %d", in.ID)
		return res, anyVal, nil
	}
}

func hostCategoryUpdateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in UpdateHostCategoryInput) (*mcp.CallToolResult, any, error) {
	return hostCategoryUpdateHandlerFn(client.HostCategories.Update, logger)
}

func hostCategoryDeleteHandlerFn(
	fn func(context.Context, int) error,
	logger *slog.Logger,
) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_host_category_delete")
		logger.Info("centreon_host_category_delete", "id", in.ID)
		if err := fn(ctx, in.ID); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_host_category_delete", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to delete host category %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_host_category_delete", "Deleted host category %d", in.ID)
		return res, anyVal, nil
	}
}

func hostCategoryDeleteHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return hostCategoryDeleteHandlerFn(client.HostCategories.Delete, logger)
}

func hostSeverityGetHandlerFn(
	fn func(context.Context, int) (*centreon.HostSeverity, error),
	logger *slog.Logger,
) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_host_severity_get")
		logger.Debug("centreon_host_severity_get", "id", in.ID)
		sev, err := fn(ctx, in.ID)
		if err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_host_severity_get", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to get host severity %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(sev)
		return res, anyVal, nil
	}
}

func hostSeverityGetHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return hostSeverityGetHandlerFn(client.HostSeverities.Get, logger)
}

func hostSeverityCreateHandlerFn(
	fn func(context.Context, centreon.CreateHostSeverityRequest) (int, error),
	logger *slog.Logger,
) func(ctx context.Context, req *mcp.CallToolRequest, in CreateHostSeverityInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateHostSeverityInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_host_severity_create")
		logger.Info("centreon_host_severity_create", "name", in.Name)
		id, err := fn(ctx, centreon.CreateHostSeverityRequest{
			Name:   in.Name,
			Alias:  in.Alias,
			Level:  in.Level,
			IconID: in.IconID,
		})
		if err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_host_severity_create", "error", reason, "name", in.Name)
			res, anyVal := errorResult("failed to create host severity %q: %s", in.Name, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_host_severity_create", "Created host severity with ID %d", id)
		return res, anyVal, nil
	}
}

func hostSeverityCreateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in CreateHostSeverityInput) (*mcp.CallToolResult, any, error) {
	return hostSeverityCreateHandlerFn(client.HostSeverities.Create, logger)
}

func hostSeverityUpdateHandlerFn(
	fn func(context.Context, int, centreon.UpdateHostSeverityRequest) error,
	logger *slog.Logger,
) func(ctx context.Context, req *mcp.CallToolRequest, in UpdateHostSeverityInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in UpdateHostSeverityInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_host_severity_update")
		logger.Info("centreon_host_severity_update", "id", in.ID)
		if err := fn(ctx, in.ID, centreon.UpdateHostSeverityRequest{
			Name:   in.Name,
			Alias:  in.Alias,
			Level:  in.Level,
			IconID: in.IconID,
		}); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_host_severity_update", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to update host severity %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_host_severity_update", "Updated host severity %d", in.ID)
		return res, anyVal, nil
	}
}

func hostSeverityUpdateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in UpdateHostSeverityInput) (*mcp.CallToolResult, any, error) {
	return hostSeverityUpdateHandlerFn(client.HostSeverities.Update, logger)
}

func hostSeverityDeleteHandlerFn(
	fn func(context.Context, int) error,
	logger *slog.Logger,
) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_host_severity_delete")
		logger.Info("centreon_host_severity_delete", "id", in.ID)
		if err := fn(ctx, in.ID); err != nil {
			reason := redact.Reason(err)
			logger.Error("failed: centreon_host_severity_delete", "error", reason, "id", in.ID)
			res, anyVal := errorResult("failed to delete host severity %d: %s", in.ID, reason)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_host_severity_delete", "Deleted host severity %d", in.ID)
		return res, anyVal, nil
	}
}

func hostSeverityDeleteHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return hostSeverityDeleteHandlerFn(client.HostSeverities.Delete, logger)
}

func hostSeverityListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_host_severity_list", in, client.HostSeverities.List)
	}
}

func hostTemplateListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_host_template_list", in, client.HostTemplates.List)
	}
}
