# CRUD Gaps: Time Periods, Host Categories, Host Severities, Service Severities

**Date:** 2026-04-02  
**Status:** Approved

## Background

The Go client (`centreon-go-client` v1.7.0) supports full CRUD for four configuration resources that the MCP server only partially exposes. This spec covers adding the 11 missing MCP tools to close these gaps.

Resources where Contact Groups and Contact Templates are intentionally list-only — the Centreon API itself does not support write operations on them.

## Gaps

| Resource | Client supports | MCP has | Missing in MCP |
|---|---|---|---|
| Time Periods | List, Get, Create, Update (PUT), Delete | List, Get, Create | `update`, `delete` |
| Host Categories | List, Get, Create, Update (PUT), Delete | List | `get`, `create`, `update`, `delete` |
| Host Severities | List, Get, Create, Update (PUT), Delete | List | `get`, `create`, `update`, `delete` |
| Service Severities | List, Create, Update (PUT), Delete | List | `create`, `update`, `delete` |

Note: `ServiceSeverityService` in the Go client does not have a `Get(id)` method. Tracked in tphakala/centreon-go-client#33. A `centreon_service_severity_get` MCP tool will be added once the client supports it.

## New Tools (11)

### Time Periods (`infra_tools.go`)

**`centreon_time_period_update`**
- Full replace (PUT) of a time period by ID.
- Input: `id int`, `name string`, `alias string` (optional), `days []TimePeriodDayInput`, `templates []int` (optional)
- Uses `client.TimePeriods.Update(ctx, id, &UpdateTimePeriodRequest{...})`

**`centreon_time_period_delete`**
- Delete a time period by ID.
- Input: `id int`
- Uses `client.TimePeriods.Delete(ctx, id)`

### Host Categories (`host_config_tools.go`)

**`centreon_host_category_get`**
- Get a single host category by ID.
- Input: `id int`
- Uses `client.HostCategories.Get(ctx, id)`

**`centreon_host_category_create`**
- Create a new host category.
- Input: `name string`, `alias string` (optional)
- Uses `client.HostCategories.Create(ctx, CreateHostCategoryRequest{...})`

**`centreon_host_category_update`**
- Full replace (PUT) of a host category by ID.
- Input: `id int`, `name string`, `alias string` (optional)
- Uses `client.HostCategories.Update(ctx, id, UpdateHostCategoryRequest{...})`

**`centreon_host_category_delete`**
- Delete a host category by ID.
- Input: `id int`
- Uses `client.HostCategories.Delete(ctx, id)`

### Host Severities (`host_config_tools.go`)

**`centreon_host_severity_get`**
- Get a single host severity by ID.
- Input: `id int`
- Uses `client.HostSeverities.Get(ctx, id)`

**`centreon_host_severity_create`**
- Create a new host severity.
- Input: `name string`, `alias string` (optional), `level int`, `iconID int`
- Uses `client.HostSeverities.Create(ctx, CreateHostSeverityRequest{...})`

**`centreon_host_severity_update`**
- Full replace (PUT) of a host severity by ID.
- Input: `id int`, `name string`, `alias string` (optional), `level int`, `iconID int`
- Uses `client.HostSeverities.Update(ctx, id, UpdateHostSeverityRequest{...})`

**`centreon_host_severity_delete`**
- Delete a host severity by ID.
- Input: `id int`
- Uses `client.HostSeverities.Delete(ctx, id)`

### Service Severities (`service_config_tools.go`)

**`centreon_service_severity_create`**
- Create a new service severity.
- Input: `name string`, `alias string` (optional), `level int`, `iconID int`
- Uses `client.ServiceSeverities.Create(ctx, CreateServiceSeverityRequest{...})`

**`centreon_service_severity_update`**
- Full replace (PUT) of a service severity by ID.
- Input: `id int`, `name string`, `alias string` (optional), `level int`, `iconID int`
- Uses `client.ServiceSeverities.Update(ctx, id, UpdateServiceSeverityRequest{...})`

**`centreon_service_severity_delete`**
- Delete a service severity by ID.
- Input: `id int`
- Uses `client.ServiceSeverities.Delete(ctx, id)`

## Update Semantics

All `_update` tools use full PUT (not partial PATCH), consistent with existing `centreon_host_group_update` and the `UpdateTimePeriodRequest` in the client. Callers must supply all required fields; omitting them sends zero-values.

## Implementation Approach

**Option A: In-place additions** — add new handlers alongside existing ones in the three existing files. No new files, no refactoring. Registration calls added to `RegisterInfraTools`, `RegisterHostConfigTools`, and `RegisterServiceConfigTools`.

## Input Struct Naming

Follow the existing pattern:
- `CreateHostCategoryInput`, `UpdateHostCategoryInput`
- `CreateHostSeverityInput`, `UpdateHostSeverityInput`
- `CreateServiceSeverityInput`, `UpdateServiceSeverityInput`
- `UpdateTimePeriodInput` (update only — create already exists as `CreateTimePeriodInput`)

## Error Handling

All handlers follow the established pattern: on error, return `errorResult(...)` with `nil` error; on success, return `successResult(...)` or `jsonResult(...)` for get operations.

## Testing

Add test coverage in `tools/infra_tools_test.go` and `tools/tools_test.go` following the existing mock-based patterns.
