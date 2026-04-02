# Centreon MCP Bug Report

**Date**: 2026-04-02  
**Tested against**: centreon-mcp-go (branch `fix/issue-12-client-v1.5.0`) + centreon-go-client v1.5.0  
**Target**: testmon02.spdev.elisa.fi (Centreon test instance)

## Summary

- **14 completely broken tools** (all fail with HTTP 4xx/5xx)
- **4 schema/validation mismatches** (wrong fields exposed or missing)
- **1 search UX bug** (inconsistent wildcard behavior)

---

## CRITICAL: Completely Broken Tools

### Bug 1-5: All `centreon_resource_*` tools have wrong request body structure

**Affected tools**: `centreon_resource_check`, `centreon_resource_acknowledge`, `centreon_resource_downtime`, `centreon_resource_comment`, `centreon_resource_submit`

**Root cause**: The Centreon API expects action parameters wrapped in a named object (e.g., `{"acknowledgement": {...}, "resources": [...]}`), but the client library sends them flat alongside `resources`.

**Code location (client library)**: `centreon-go-client@v1.5.0/operations.go`

| Tool | API error | Expected structure |
|------|-----------|-------------------|
| `resource_acknowledge` | `[acknowledgement] required` | `{"acknowledgement": {"comment": ...}, "resources": [...]}` |
| `resource_downtime` | `[downtime] required` | `{"downtime": {"comment": ...}, "resources": [...]}` |
| `resource_check` | `[check] required` | `{"check": {...}, "resources": [...]}` |
| `resource_comment` | `[resources[0].comment] required, [resources[0].date] required` | Different structure: comment/date inside each resource |
| `resource_submit` | `[resources[0].parent] required` | Parent must always be present (null for hosts) |

**Current payload** (acknowledge example, line 59-65 of `tools/operations_tools.go`):
```json
{
  "resources": [{"type": "host", "id": 1}],
  "comment": "text",
  "is_sticky": true,
  "is_notify_contacts": false,
  "is_persistent_comment": false
}
```

**Expected payload**:
```json
{
  "resources": [{"type": "host", "id": 1, "parent": null}],
  "acknowledgement": {
    "comment": "text",
    "is_sticky": true,
    "is_notify_contacts": false,
    "is_persistent_comment": false
  }
}
```

Additionally, `ResourceRef` (operations.go:9-13) uses `"parent,omitempty"` which omits parent for hosts, but the API requires `"parent": null` for hosts.

**Fix required in**: `centreon-go-client` — restructure all operation request structs with wrapper objects, and always include `parent` (null for hosts).

---

### Bug 6: `centreon_acknowledgement_service_create` sends `with_services` field

**Error**: `HTTP 500: Error on 'with_services': This field was not expected.`

**Code location**: `centreon-go-client@v1.5.0/acknowledgements.go:28-34`

```go
type CreateAcknowledgementRequest struct {
    Comment             string `json:"comment"`
    IsNotifyContacts    bool   `json:"is_notify_contacts"`
    IsPersistentComment bool   `json:"is_persistent_comment"`
    IsSticky            bool   `json:"is_sticky"`
    WithServices        bool   `json:"with_services"`  // <-- always serialized
}
```

The `with_services` field lacks `omitempty`, so even when `false` it's included in JSON. The Centreon API rejects this field on service acknowledgement endpoints.

**Fix**: Either add `omitempty` to `with_services`, or split into two separate request structs for host vs service acknowledgements.

---

### Bug 7-12: All user-related endpoints use wrong API paths

**Affected tools**:

| Tool | URL sent | HTTP Error |
|------|----------|------------|
| `centreon_user_list` | `GET /centreon/api/latest/users` | 404 |
| `centreon_contact_group_list` | `GET /centreon/api/latest/users/contact-groups` | 404 |
| `centreon_contact_template_list` | `GET /centreon/api/latest/users/contact-templates` | 404 |
| `centreon_user_filter_list` | `GET /centreon/api/latest/users/filters` | 404 |
| `centreon_user_filter_create` | `POST /centreon/api/latest/users/filters` | 404 |
| `centreon_user_update` | `PATCH /centreon/api/latest/users/{id}` | 404 |

**Code locations** (centreon-go-client):
- `users.go:34` — `/users`
- `contact_groups.go:25` — `/users/contact-groups`
- `contact_templates.go:24` — `/users/contact-templates`
- `user_filters.go:49` — `/users/filters`

**Fix required in**: `centreon-go-client` — correct the API endpoint paths. These may need to be under `/configuration/users` or similar depending on the Centreon API version.

---

### Bug 13-14: Downtime cancel by host/service uses wrong HTTP method

**Affected tools**: `centreon_downtime_host_cancel`, `centreon_downtime_service_cancel`

**Error**: `HTTP 405: Method Not Allowed (Allow: POST, GET)`

**Code location**: `centreon-go-client@v1.5.0/downtimes.go:97-104`
```go
func (s *DowntimeService) CancelForHost(ctx context.Context, hostID int) error {
    return s.client.delete(ctx, fmt.Sprintf("/monitoring/hosts/%d/downtimes", hostID))
}
```

The client sends `DELETE /monitoring/hosts/{id}/downtimes` but this endpoint only allows POST and GET. Downtime cancellation requires a different approach.

**Note**: `centreon_downtime_cancel` (by downtime ID) works correctly using `DELETE /monitoring/downtimes/{id}`.

---

## HIGH: Schema/Validation Bugs

### Bug 15: `centreon_time_period_create` missing required fields

**Error**: `HTTP 422: days: should not be null. templates: should not be null.`

**Code location**: Tool schema in MCP only exposes `name` and `alias` parameters.

The Centreon API requires `days` (array of day/time_range objects) and `templates` (array, can be empty `[]`).

---

### Bug 16: `centreon_service_update` exposes invalid `alias` field

**Error**: `HTTP 400: The property alias is not defined and the definition does not allow additional properties`

**Code location**: `tools/service_tools.go` — `ServiceUpdateInput` struct includes `Alias` field, but the Centreon service update API does not accept `alias`.

---

### Bug 17: `centreon_host_create` macro `description` should be required

**Error**: `HTTP 400: [macros[0].description] The property description is required`

**Code location**: Macro struct in `tools/host_tools.go` has `required: ["name"]` but should have `required: ["name", "description"]`.

---

### Bug 18: `centreon_monitoring_host_list` search parameter broken

**Error**: `HTTP 500: The parameter name is not allowed`

The search parameter uses `Lk("name", ...)` which generates `{"name": {"$lk": "..."}}`. The monitoring host list endpoint does not accept this search format.

**Code location**: `tools/tools.go:141-143` — `buildListOptions()` applies the same search strategy to all endpoints, but monitoring endpoints have different search parameter support.

---

## MEDIUM: Search UX Bug

### Bug 19: Config list search requires SQL wildcards on some endpoints

**Symptom**: `centreon_host_list` search for `SantaCare` returns 0 results. Search for `%SantaCare%` returns matches. But `centreon_service_list` search for `Ping` (without wildcards) returns 84 matches.

**Root cause**: The `$lk` operator on configuration host endpoints requires SQL-style `%` wildcards, but on service endpoints it doesn't.

**Recommendation**: The MCP should auto-wrap search terms with `%` wildcards when using `$lk` operator to provide consistent UX across all list tools.

---

## LOW: Server-Side Issues (Not MCP Bugs)

### `centreon_server_list` / `centreon_platform_status`

`centreon_server_list` returns HTTP 500 from the Centreon API when querying `/configuration/monitoring-servers`. This cascades to `centreon_platform_status` which depends on it. Likely a Centreon server-side issue.

---

## Working Correctly

The following tools all passed testing successfully:

**Configuration CRUD**: host list/get/create/update/delete, service list/list_by_host/create/update/delete, host_group list/get/create/update/delete, service_group list/create/delete, service_category list/create/delete, host_category list, host_severity list, service_severity list, host_template list, service_template list, command list, time_period list/get

**Monitoring read**: monitoring_host list/get, monitoring_service list, monitoring_resource list, monitoring_resource_host get, monitoring_resource_service get, monitoring_host_services, monitoring_host_timeline, host_status_counts, service_status_counts

**Downtime/Ack read**: downtime list/get, downtime_host list, downtime_service list, acknowledgement list/get, acknowledgement_host list, acknowledgement_service list

**Downtime/Ack write**: downtime_host create, downtime_service create, acknowledgement_host create, downtime cancel (by ID), acknowledgement_host_cancel, acknowledgement_service_cancel

**Other**: connection_test, notification_policy_host get, notification_policy_service get

**Error handling**: All tested tools return clear error messages for invalid IDs (404), SQL injection and XSS inputs are handled safely.
