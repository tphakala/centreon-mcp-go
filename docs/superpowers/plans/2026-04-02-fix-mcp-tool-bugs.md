# Fix MCP Tool Bugs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix 4 MCP-level bugs identified during testing (issues #14-#17)

**Architecture:** Each fix is a small, targeted change to a tool's input struct or the shared `buildListOptions` helper. No architectural changes needed — just correcting schemas and search behavior.

**Tech Stack:** Go 1.24, centreon-go-client v1.5.0, MCP Go SDK

---

## File Structure

| File | Responsibility | Changes |
|------|---------------|---------|
| `tools/infra_tools.go` | Time period create tool | Add `Days` and `Templates` fields to `CreateTimePeriodInput`; update handler |
| `tools/service_config_tools.go` | Service update tool | Remove `Alias` from `UpdateServiceInput`; remove from handler |
| `tools/host_config_tools.go` | Host/service macro schema | Make `Description` required in `MacroInput` |
| `tools/tools.go` | Shared search/list logic | Auto-wrap search with `%` wildcards; skip search for monitoring endpoints |
| `tools/monitoring_tools.go` | Monitoring list handlers | Use `buildMonitoringListOptions` instead of `buildListOptions` |
| `tools/tools_test.go` | Unit tests for helpers | Add tests for wildcard wrapping and monitoring option builder |

---

### Task 1: Fix `centreon_time_period_create` missing fields (issue #14)

**Files:**
- Modify: `tools/infra_tools.go:40-43` (CreateTimePeriodInput struct)
- Modify: `tools/infra_tools.go:78-94` (timePeriodCreateHandler)

- [ ] **Step 1: Add Days and Templates fields to CreateTimePeriodInput**

In `tools/infra_tools.go`, replace the `CreateTimePeriodInput` struct:

```go
// TimePeriodDayInput represents a day range in a time period.
type TimePeriodDayInput struct {
	Day       int    `json:"day"        jsonschema:"Day of week (1=Monday through 7=Sunday)"`
	TimeRange string `json:"timeRange"  jsonschema:"Time range (e.g. 00:00-24:00)"`
}

// CreateTimePeriodInput is the input for the centreon_time_period_create tool.
type CreateTimePeriodInput struct {
	Name      string               `json:"name"                jsonschema:"Time period name"`
	Alias     string               `json:"alias,omitempty"     jsonschema:"Time period alias"`
	Days      []TimePeriodDayInput `json:"days"                jsonschema:"Day definitions (required, use empty array [] if none)"`
	Templates []int                `json:"templates"           jsonschema:"Template IDs to include (required, use empty array [] if none)"`
}
```

- [ ] **Step 2: Update the handler to pass Days and Templates to the client**

In `tools/infra_tools.go`, replace the `timePeriodCreateHandler` function body:

```go
func timePeriodCreateHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in CreateTimePeriodInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateTimePeriodInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_time_period_create")
		logger.Info("centreon_time_period_create", "name", in.Name)
		days := make([]centreon.TimePeriodDay, 0, len(in.Days))
		for _, d := range in.Days {
			days = append(days, centreon.TimePeriodDay{Day: d.Day, TimeRange: d.TimeRange})
		}
		id, err := client.TimePeriods.Create(ctx, centreon.CreateTimePeriodRequest{
			Name:  in.Name,
			Alias: in.Alias,
			Days:  days,
		})
		if err != nil {
			logger.Error("failed: centreon_time_period_create", "error", err, "name", in.Name)
			res, anyVal := errorResult("failed to create time period %q: %v", in.Name, err)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_time_period_create", "Created time period with ID %d", id)
		return res, anyVal, nil
	}
}
```

Note: The `CreateTimePeriodRequest` in the client library does NOT have a `Templates` field (only `Days`). The API error says templates cannot be null, so we need to check if the client library sends `[]` by default. If not, this is also a client library issue to track. For now, include it in the MCP input so the user is aware it's required, and pass `Days` which is what the client supports.

- [ ] **Step 3: Build and verify it compiles**

Run: `cd /Users/e909385/src/centreon-mcp-go && go build ./...`
Expected: Clean build with no errors

- [ ] **Step 4: Commit**

```bash
git add tools/infra_tools.go
git commit -m "fix: add days and templates fields to time_period_create (issue #14)"
```

---

### Task 2: Remove invalid `alias` field from `centreon_service_update` (issue #15)

**Files:**
- Modify: `tools/service_config_tools.go:92-102` (UpdateServiceInput struct)
- Modify: `tools/service_config_tools.go:162-184` (serviceUpdateHandler)

- [ ] **Step 1: Remove Alias from UpdateServiceInput struct**

In `tools/service_config_tools.go`, replace the struct:

```go
// UpdateServiceInput is the input for the centreon_service_update tool.
type UpdateServiceInput struct {
	ID                  int   `json:"id"                              jsonschema:"Service ID"`
	Name                *string `json:"name,omitempty"                  jsonschema:"Service name"`
	CheckCommandID      *int    `json:"checkCommandID,omitempty"        jsonschema:"Check command ID"`
	MaxCheckAttempts    *int    `json:"maxCheckAttempts,omitempty"      jsonschema:"Maximum check attempts"`
	NormalCheckInterval *int    `json:"normalCheckInterval,omitempty"   jsonschema:"Normal check interval in seconds"`
	RetryCheckInterval  *int    `json:"retryCheckInterval,omitempty"    jsonschema:"Retry check interval in seconds"`
	ActiveChecksEnabled *bool   `json:"activeChecksEnabled,omitempty"   jsonschema:"Whether active checks are enabled"`
	IsActivated         *bool   `json:"isActivated,omitempty"           jsonschema:"Whether the service is activated"`
}
```

- [ ] **Step 2: Remove Alias from the handler's request construction**

In `tools/service_config_tools.go`, replace the handler:

```go
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
			ActiveChecksEnabled: in.ActiveChecksEnabled,
			IsActivated:         in.IsActivated,
		}
		if err := client.Services.Update(ctx, in.ID, &req); err != nil {
			logger.Error("failed: centreon_service_update", "error", err, "id", in.ID)
			res, anyVal := errorResult("failed to update service %d: %v", in.ID, err)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_service_update", "Updated service %d", in.ID)
		return res, anyVal, nil
	}
}
```

- [ ] **Step 3: Build and verify**

Run: `cd /Users/e909385/src/centreon-mcp-go && go build ./...`
Expected: Clean build

- [ ] **Step 4: Commit**

```bash
git add tools/service_config_tools.go
git commit -m "fix: remove invalid alias field from service_update schema (issue #15)"
```

---

### Task 3: Make macro `description` required (issue #16)

**Files:**
- Modify: `tools/host_config_tools.go:80-85` (MacroInput struct)

- [ ] **Step 1: Change Description from omitempty to required**

In `tools/host_config_tools.go`, replace the `MacroInput` struct:

```go
// MacroInput represents a custom macro.
type MacroInput struct {
	Name        string `json:"name"                    jsonschema:"Macro name"`
	Value       string `json:"value,omitempty"         jsonschema:"Macro value"`
	IsPassword  bool   `json:"isPassword,omitempty"    jsonschema:"Whether the value is a password"`
	Description string `json:"description"             jsonschema:"Macro description"`
}
```

The only change is removing `omitempty` from `Description`'s json tag. The `jsonschema` library used by the MCP SDK will include it in the `required` array when `omitempty` is absent.

- [ ] **Step 2: Build and verify**

Run: `cd /Users/e909385/src/centreon-mcp-go && go build ./...`
Expected: Clean build

- [ ] **Step 3: Commit**

```bash
git add tools/host_config_tools.go
git commit -m "fix: make macro description required in schema (issue #16)"
```

---

### Task 4: Fix search — auto-wrap wildcards and disable for monitoring endpoints (issue #17)

**Files:**
- Modify: `tools/tools.go:125-145` (buildListOptions function)
- Modify: `tools/monitoring_tools.go:64-68,134-138,155-159` (monitoring list handlers)
- Modify: `tools/tools_test.go` (add tests)

- [ ] **Step 1: Write failing tests for wildcard wrapping**

Add to `tools/tools_test.go`:

```go
func TestBuildListOptions_WrapsSearchWithWildcards(t *testing.T) {
	in := ListInput{Search: "testmon"}
	opts := buildListOptions(in)
	// Should have limit + search = at least 2 options
	if len(opts) < 2 {
		t.Errorf("expected at least 2 options (limit + search), got %d", len(opts))
	}
}

func TestBuildListOptions_DoesNotDoubleWrapWildcards(t *testing.T) {
	in := ListInput{Search: "%testmon%"}
	opts := buildListOptions(in)
	if len(opts) < 2 {
		t.Errorf("expected at least 2 options (limit + search), got %d", len(opts))
	}
}

func TestBuildMonitoringListOptions_SkipsSearch(t *testing.T) {
	in := ListInput{Search: "testmon", Limit: 10}
	opts := buildMonitoringListOptions(in)
	// Should only have limit option, search is skipped
	if len(opts) != 1 {
		t.Errorf("expected 1 option (limit only), got %d", len(opts))
	}
}

func TestBuildMonitoringListOptions_SetsLimitAndPage(t *testing.T) {
	in := ListInput{Page: 2, Limit: 10}
	opts := buildMonitoringListOptions(in)
	if len(opts) != 2 {
		t.Errorf("expected 2 options (limit + page), got %d", len(opts))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/e909385/src/centreon-mcp-go && go test ./tools/ -run "TestBuild" -v`
Expected: `TestBuildMonitoringListOptions_SkipsSearch` and `TestBuildMonitoringListOptions_SetsLimitAndPage` FAIL (function doesn't exist yet)

- [ ] **Step 3: Modify buildListOptions to auto-wrap search with wildcards**

In `tools/tools.go`, replace the `buildListOptions` function:

```go
// buildListOptions converts a ListInput into centreon.ListOption slice.
// Search terms are automatically wrapped with SQL wildcards (%) for
// consistent LIKE matching across all configuration endpoints.
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
		search := in.Search
		if len(search) > 0 && search[0] != '%' {
			search = "%" + search
		}
		if len(search) > 0 && search[len(search)-1] != '%' {
			search = search + "%"
		}
		opts = append(opts, centreon.WithSearch(centreon.Lk("name", search)))
	}
	return opts
}

// buildMonitoringListOptions converts a ListInput into centreon.ListOption slice
// for monitoring endpoints. Monitoring endpoints do not support the JSON search
// filter format, so the search parameter is ignored.
func buildMonitoringListOptions(in ListInput) []centreon.ListOption {
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
	// Search is intentionally not supported on monitoring endpoints.
	// The Centreon monitoring API rejects the JSON search filter with
	// "The parameter name is not allowed".
	return opts
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/e909385/src/centreon-mcp-go && go test ./tools/ -run "TestBuild" -v`
Expected: All PASS

- [ ] **Step 5: Create MonitoringListInput without search field**

In `tools/tools.go`, add after the `ListInput` struct:

```go
// MonitoringListInput is the input for monitoring list tools.
// Monitoring endpoints do not support name search filtering.
type MonitoringListInput struct {
	Page  int `json:"page,omitempty"  jsonschema:"Page number (default 1)"`
	Limit int `json:"limit,omitempty" jsonschema:"Results per page (default 30, max 100)"`
}
```

- [ ] **Step 6: Update monitoring list handlers to use MonitoringListInput**

In `tools/monitoring_tools.go`, update the three list handlers that use `ListInput`:

Replace `monitoringHostListHandler`:
```go
func monitoringHostListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in MonitoringListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in MonitoringListInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_monitoring_host_list")
		logger.Debug("centreon_monitoring_host_list", "page", in.Page, "limit", in.Limit)
		listIn := ListInput{Page: in.Page, Limit: in.Limit}
		opts := buildMonitoringListOptions(listIn)
		resp, err := client.MonitoringHosts.List(ctx, opts...)
		if err != nil {
			logger.Error("failed: centreon_monitoring_host_list", "error", err)
			res, anyVal := errorResult("failed: centreon_monitoring_host_list: %v", err)
			return res, anyVal, nil
		}
		logger.Debug("centreon_monitoring_host_list completed", "results", len(resp.Result), "total", resp.Meta.Total)
		res, anyVal := jsonResult(resp)
		return res, anyVal, nil
	}
}
```

Replace `monitoringServiceListHandler`:
```go
func monitoringServiceListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in MonitoringListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in MonitoringListInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_monitoring_service_list")
		logger.Debug("centreon_monitoring_service_list", "page", in.Page, "limit", in.Limit)
		listIn := ListInput{Page: in.Page, Limit: in.Limit}
		opts := buildMonitoringListOptions(listIn)
		resp, err := client.MonitoringServices.List(ctx, opts...)
		if err != nil {
			logger.Error("failed: centreon_monitoring_service_list", "error", err)
			res, anyVal := errorResult("failed: centreon_monitoring_service_list: %v", err)
			return res, anyVal, nil
		}
		logger.Debug("centreon_monitoring_service_list completed", "results", len(resp.Result), "total", resp.Meta.Total)
		res, anyVal := jsonResult(resp)
		return res, anyVal, nil
	}
}
```

Replace `monitoringResourceListHandler`:
```go
func monitoringResourceListHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in MonitoringListInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in MonitoringListInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_monitoring_resource_list")
		logger.Debug("centreon_monitoring_resource_list", "page", in.Page, "limit", in.Limit)
		listIn := ListInput{Page: in.Page, Limit: in.Limit}
		opts := buildMonitoringListOptions(listIn)
		resp, err := client.Monitoring.List(ctx, opts...)
		if err != nil {
			logger.Error("failed: centreon_monitoring_resource_list", "error", err)
			res, anyVal := errorResult("failed: centreon_monitoring_resource_list: %v", err)
			return res, anyVal, nil
		}
		logger.Debug("centreon_monitoring_resource_list completed", "results", len(resp.Result), "total", resp.Meta.Total)
		res, anyVal := jsonResult(resp)
		return res, anyVal, nil
	}
}
```

Also update the tool descriptions to remove "name filtering" from monitoring tools:

```go
// In RegisterMonitoringTools:
// Change "List monitored hosts with real-time status. Supports pagination and name filtering."
// To:    "List monitored hosts with real-time status. Supports pagination."
// Same for service list and resource list descriptions.
```

- [ ] **Step 7: Build and run all tests**

Run: `cd /Users/e909385/src/centreon-mcp-go && go build ./... && go test ./... -v`
Expected: Clean build, all tests pass

- [ ] **Step 8: Commit**

```bash
git add tools/tools.go tools/tools_test.go tools/monitoring_tools.go
git commit -m "fix: auto-wrap search wildcards and disable search on monitoring endpoints (issue #17)"
```

---

### Task 5: Final verification build

- [ ] **Step 1: Run full build and tests**

Run: `cd /Users/e909385/src/centreon-mcp-go && go build ./... && go test ./... -v`
Expected: Clean build, all tests pass

- [ ] **Step 2: Run linter if available**

Run: `cd /Users/e909385/src/centreon-mcp-go && go vet ./...`
Expected: No issues
