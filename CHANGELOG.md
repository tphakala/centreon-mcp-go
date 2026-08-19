# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `centreon_monitoring_service_metrics`: retrieve a service's current performance
  metric values (name, unit, current value, and warning/critical thresholds) by
  `hostID` and `serviceID`. A service with no performance data returns an empty
  list, not an error. (#50)
- `centreon_monitoring_service_timeline`: the service-scoped mirror of
  `centreon_monitoring_host_timeline`, returning one service's chronological event
  history (state changes, notifications, acknowledgements, downtime, comments) by
  `hostID` and `serviceID`, with pagination. (#51)
- `centreon_monitoring_resource_list` gains structured server-side filters:
  `resourceTypes`, `statuses`, `statusTypes`, `states`, and the `hostGroups`,
  `serviceGroups`, `hostCategories`, and `serviceCategories` name filters, on top
  of the existing search, host, poller, and sort options. Multiple values within a
  filter match as OR and the filters combine as AND. (#52)
- `centreon_host_get` now returns the custom macros defined directly on a host,
  along with the fuller per-host detail (SNMP, check-command, notification,
  flapping, event-handler, and note/icon fields). It reads from the per-host detail
  endpoint added in Centreon 25.10, whose shape differs from the list shape: the
  monitoring-server, severity, and time-period relationships are reported as their
  numeric IDs (`monitoring_server_id`, `severity_id`, `check_timeperiod_id`, and so
  on) rather than as `{id, name}` objects. Against an older Centreon the tool
  transparently falls back to the previous list lookup, which keeps the named
  objects but has no macros. `centreon_host_list` is unchanged (macros belong on
  the per-host get). (#49)
- `centreon_service_get`: fetch one service's full stored configuration by its
  numeric id, including custom macros (which, per the Centreon 25.10 API, also
  cover macros inherited from service templates and commands). It mirrors
  `centreon_host_get`, but because the services configuration API has no
  macro-free per-id fallback, a Centreon older than 25.10 (where the per-service
  detail GET route is not registered) reports it as unsupported on this Centreon
  version rather than degrading to a macro-free shape. (#88)

### Changed

- The `github.com/tphakala/centreon-go-client` dependency is updated to v2.1.0.
  v2.0.0 adopted upstream fixes for tools this server already ships: downtime and
  token timestamps are truncated to whole seconds (Centreon 25.10 rejects
  fractional RFC3339), bulk resource operations normalize a nil resource list to
  `[]`, and per-id monitoring detail decodes use the correct keys; it also demotes
  client-side logging of a caller-cancelled request from error to debug. v2.1.0
  types time-period exceptions into `{id, day_range, time_range}` (any other
  fields the server sends are dropped, and the key order becomes fixed), changing
  the JSON shape that `centreon_time_period_get` and `centreon_time_period_list`
  return, and adds the routing-404 classifier used by the version hints below.
  On Centreon 25.10 the server returns exactly those fields, so the data is
  unchanged there. (#33)
- `centreon_user_update` now recognizes when the connected Centreon registers no
  user-write route (a routing 404, as on Centreon 25.10 where users and contacts
  are read-only through the v2 REST API) and reports that plainly instead of a
  bare HTTP 404. Its own description and the two descriptions that point at it
  note the 25.10 limitation. (#87)
- `centreon_monitoring_service_metrics`, `centreon_monitoring_service_timeline`,
  and `centreon_service_get` render a routing 404 (an endpoint absent on an older
  Centreon) with a precise "API route not present" hint via the client's
  `IsRouteNotFound` classifier; an ambiguous 404 keeps the previous "not found, or
  unsupported on this Centreon version" wording. (#50, #51, #88)

### Security

- Centreon client-call errors are now classified to a fixed, credential-safe
  description before they reach a server log or an MCP tool response, so a
  credential embedded in the base URL (`CENTREON_HOST`, or a caller-supplied
  `X-Centreon-Host` header in gateway mode) can no longer leak through an error
  value, including the mis-parsed-authority case where Go's own masking sees no
  userinfo to hide (CWE-532). (#63, #71)
- Host-URL redaction no longer echoes a password when the userinfo boundary is
  percent-encoded, in server logs or in the configuration errors that quote a host.
  Two shapes leaked: a `@` delimiter whose own hex digits are encoded
  (`%25%34%30`), and a `:` separator written `%3A`, which Go reads as part of a
  bare username so its own masking hides nothing. The encoded `:` case is the more
  reachable of the two, since such a host passes startup validation and runs
  normally. It leaked through four paths, including a bracketed-IPv6 carve-out and
  a tail search that both looked for a raw `:` only (CWE-532). (#68, #75)

### Changed

- Breaking: a cleartext `http://` Centreon host is rejected by default. Existing
  `http://` deployments must set `CENTREON_ALLOW_HTTP=true` to opt back in. (#28)
- Breaking: a host URL that names no hostname is rejected. The port-only forms
  `http://:8080` (with `CENTREON_ALLOW_HTTP=true`) and `https://:8443` (with
  `CENTREON_ALLOW_SELF_SIGNED=true`) previously worked because Go treats an empty
  hostname as localhost. Write the hostname out, for example `localhost:8080`.
  This also changes the outgoing `Host` header from `:8080` to `localhost:8080`,
  so a name-based reverse proxy in front of Centreon may route differently
  afterwards. (#57, #58)
- When a Centreon host URL carries a legitimate `@` after the authority, the
  read-only status and connection tools report the literal `(redacted host)`
  instead of a host name, because the authority cannot be identified without
  risking exposure of a credential. (#66)
- A host URL whose credential boundary is only visible after percent-decoding is
  now logged as `https://user:xxxxx`, or `https://xxxxx` when even the username
  boundary is unreadable, instead of being logged verbatim. The host name is lost
  in those cases because its position cannot be trusted. A URL carrying only one
  half of a credential shape is still logged unchanged, so an encoded `@` in the
  path of a portless host does not trigger it; note a port supplies the missing
  `:`, so the same path behind `host:8443` is masked to `https://host:xxxxx`.
  Configuration errors that quote a rejected host now name the parse failure
  ("invalid port after host") instead of quoting the offending span, which is what
  reprinted the credential; the masked host is still quoted alongside. (#68, #75)

## [1.0.0]

- Initial tagged release.

[Unreleased]: https://github.com/tphakala/centreon-mcp-go/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/tphakala/centreon-mcp-go/releases/tag/v1.0.0
