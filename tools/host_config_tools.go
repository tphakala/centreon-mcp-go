package tools

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client"
)

// RegisterHostConfigTools registers all host configuration tools.
func RegisterHostConfigTools(s *mcp.Server, client *centreon.Client, logger *slog.Logger) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_list",
		Description: "List host configurations. Supports pagination and name filtering.",
	}, hostListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_get",
		Description: "Get a single host configuration by ID.",
	}, hostGetHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_create",
		Description: "Create a new host configuration.",
	}, hostCreateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_update",
		Description: "Update an existing host configuration (partial update).",
	}, hostUpdateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_delete",
		Description: "Delete a host configuration by ID.",
	}, hostDeleteHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_group_list",
		Description: "List host group configurations. Supports pagination and name filtering.",
	}, hostGroupListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_group_get",
		Description: "Get a single host group configuration by ID.",
	}, hostGroupGetHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_group_create",
		Description: "Create a new host group configuration.",
	}, hostGroupCreateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_group_update",
		Description: "Replace an existing host group configuration (full update).",
	}, hostGroupUpdateHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_group_delete",
		Description: "Delete a host group configuration by ID.",
	}, hostGroupDeleteHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_category_list",
		Description: "List host category configurations. Supports pagination and name filtering.",
	}, hostCategoryListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_severity_list",
		Description: "List host severity configurations. Supports pagination and name filtering.",
	}, hostSeverityListHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_host_template_list",
		Description: "List host template configurations. Supports pagination and name filtering.",
	}, hostTemplateListHandler(client, logger))
}

// CreateHostInput is the input for the centreon_host_create tool.
type CreateHostInput struct {
	MonitoringServerID int    `json:"monitoringServerID"         jsonschema:"Monitoring server ID"`
	Name               string `json:"name"                       jsonschema:"Host name"`
	Address            string `json:"address"                    jsonschema:"Host IP address or FQDN"`
	Alias              string `json:"alias,omitempty"            jsonschema:"Host alias"`
	CheckCommandID     int    `json:"checkCommandID,omitempty"   jsonschema:"Check command ID"`
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
	ActiveChecksEnabled *bool   `json:"activeChecksEnabled,omitempty"   jsonschema:"Whether active checks are enabled"`
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

func hostListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_host_list", in, client.Hosts.List)
	}
}

func hostGetHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		logger.Debug("centreon_host_get", "id", in.ID)
		host, err := client.Hosts.GetByID(ctx, in.ID)
		if err != nil {
			logger.Error("failed: centreon_host_get", "error", err, "id", in.ID)
			res, anyVal := errorResult("failed to get host %d: %v", in.ID, err)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(host)
		return res, anyVal, nil
	}
}

func hostCreateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in CreateHostInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateHostInput) (*mcp.CallToolResult, any, error) {
		logger.Info("centreon_host_create", "name", in.Name, "address", in.Address)
		id, err := client.Hosts.Create(ctx, centreon.CreateHostRequest{
			MonitoringServerID: in.MonitoringServerID,
			Name:               in.Name,
			Address:            in.Address,
			Alias:              in.Alias,
			CheckCommandID:     in.CheckCommandID,
		})
		if err != nil {
			logger.Error("failed: centreon_host_create", "error", err, "name", in.Name)
			res, anyVal := errorResult("failed to create host %q: %v", in.Name, err)
			return res, anyVal, nil
		}
		res, anyVal := textResult("Created host with ID %d", id)
		return res, anyVal, nil
	}
}

func hostUpdateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in UpdateHostInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in UpdateHostInput) (*mcp.CallToolResult, any, error) {
		logger.Info("centreon_host_update", "id", in.ID)
		req := centreon.UpdateHostRequest{
			Name:                in.Name,
			Alias:               in.Alias,
			Address:             in.Address,
			CheckCommandID:      in.CheckCommandID,
			MaxCheckAttempts:    in.MaxCheckAttempts,
			NormalCheckInterval: in.NormalCheckInterval,
			RetryCheckInterval:  in.RetryCheckInterval,
			ActiveChecksEnabled: in.ActiveChecksEnabled,
			IsActivated:         in.IsActivated,
		}
		if err := client.Hosts.Update(ctx, in.ID, req); err != nil {
			logger.Error("failed: centreon_host_update", "error", err, "id", in.ID)
			res, anyVal := errorResult("failed to update host %d: %v", in.ID, err)
			return res, anyVal, nil
		}
		res, anyVal := textResult("Updated host %d", in.ID)
		return res, anyVal, nil
	}
}

func hostDeleteHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		logger.Info("centreon_host_delete", "id", in.ID)
		if err := client.Hosts.Delete(ctx, in.ID); err != nil {
			logger.Error("failed: centreon_host_delete", "error", err, "id", in.ID)
			res, anyVal := errorResult("failed to delete host %d: %v", in.ID, err)
			return res, anyVal, nil
		}
		res, anyVal := textResult("Deleted host %d", in.ID)
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
		logger.Debug("centreon_host_group_get", "id", in.ID)
		hg, err := client.HostGroups.Get(ctx, in.ID)
		if err != nil {
			logger.Error("failed: centreon_host_group_get", "error", err, "id", in.ID)
			res, anyVal := errorResult("failed to get host group %d: %v", in.ID, err)
			return res, anyVal, nil
		}
		res, anyVal := jsonResult(hg)
		return res, anyVal, nil
	}
}

func hostGroupCreateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in CreateHostGroupInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateHostGroupInput) (*mcp.CallToolResult, any, error) {
		logger.Info("centreon_host_group_create", "name", in.Name)
		id, err := client.HostGroups.Create(ctx, centreon.CreateHostGroupRequest{
			Name:  in.Name,
			Alias: in.Alias,
		})
		if err != nil {
			logger.Error("failed: centreon_host_group_create", "error", err, "name", in.Name)
			res, anyVal := errorResult("failed to create host group %q: %v", in.Name, err)
			return res, anyVal, nil
		}
		res, anyVal := textResult("Created host group with ID %d", id)
		return res, anyVal, nil
	}
}

func hostGroupUpdateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in UpdateHostGroupInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in UpdateHostGroupInput) (*mcp.CallToolResult, any, error) {
		logger.Info("centreon_host_group_update", "id", in.ID)
		if err := client.HostGroups.Update(ctx, in.ID, centreon.UpdateHostGroupRequest{
			Name:  in.Name,
			Alias: in.Alias,
		}); err != nil {
			logger.Error("failed: centreon_host_group_update", "error", err, "id", in.ID)
			res, anyVal := errorResult("failed to update host group %d: %v", in.ID, err)
			return res, anyVal, nil
		}
		res, anyVal := textResult("Updated host group %d", in.ID)
		return res, anyVal, nil
	}
}

func hostGroupDeleteHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
		logger.Info("centreon_host_group_delete", "id", in.ID)
		if err := client.HostGroups.Delete(ctx, in.ID); err != nil {
			logger.Error("failed: centreon_host_group_delete", "error", err, "id", in.ID)
			res, anyVal := errorResult("failed to delete host group %d: %v", in.ID, err)
			return res, anyVal, nil
		}
		res, anyVal := textResult("Deleted host group %d", in.ID)
		return res, anyVal, nil
	}
}

func hostCategoryListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
		return commonListHandler(ctx, logger, "centreon_host_category_list", in, client.HostCategories.List)
	}
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
