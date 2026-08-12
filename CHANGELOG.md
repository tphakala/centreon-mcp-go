# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
