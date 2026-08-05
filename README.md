# centreon-mcp-go

An MCP (Model Context Protocol) server for Centreon monitoring, written in Go.

Exposes 88 tools covering real-time monitoring, host and service configuration, downtime and acknowledgement management, infrastructure, users, and notifications. Integrates with any MCP-compatible AI client such as Claude Code or Claude Desktop.

## Features

- **88 tools** across 11 categories: monitoring, operations, downtimes, acknowledgements, host config, service config, infrastructure, users, notifications, platform status, and connection testing
- **Three transport modes**: stdio (default), HTTP (streamable), and HTTP gateway mode
- **Structured JSON logging** via `log/slog` with configurable levels
- **Gateway mode with token cache**: per-request Centreon credentials via HTTP headers, with a 50-minute token cache to avoid repeated logins
- **Self-signed certificate support**: opt-in via `CENTREON_ALLOW_SELF_SIGNED`
- **Cross-host redirect protection**: the Centreon session token (`X-AUTH-TOKEN`) is never forwarded across an HTTP redirect to a different host, preventing credential leakage (CWE-522)

## Requirements

- Go 1.26 or later
- A Centreon instance with REST API v2 access
- API credentials (username/password or API token)

## Installation

### go install

```bash
go install github.com/tphakala/centreon-mcp-go@latest
```

### Build from source

```bash
git clone https://github.com/tphakala/centreon-mcp-go
cd centreon-mcp-go
go build -o centreon-mcp-go .
```

### Container (Podman / Docker)

```bash
# Build the image (or: task image:build)
podman build -t centreon-mcp .

# Run in HTTP mode
podman run -d \
  -e CENTREON_HOST=https://centreon.example.com \
  -e CENTREON_USERNAME=apiuser \
  -e CENTREON_PASSWORD=secret \
  -e MCP_TRANSPORT=http \
  -p 8080:8080 \
  centreon-mcp
```

## Configuration

All configuration is via environment variables.

| Variable                    | Required | Default     | Description                                                  |
|-----------------------------|----------|-------------|--------------------------------------------------------------|
| `CENTREON_HOST`             | Yes      | (none)      | Centreon server base URL (e.g. `https://centreon.example.com`) |
| `CENTREON_USERNAME`         | *        | (none)      | Username for session-based authentication                    |
| `CENTREON_PASSWORD`         | *        | (none)      | Password for session-based authentication                    |
| `CENTREON_TOKEN`            | *        | (none)      | API token (alternative to username + password)               |
| `CENTREON_ALLOW_SELF_SIGNED`| No       | `false`     | Accept self-signed TLS certificates                          |
| `CENTREON_ALLOW_HTTP`       | No       | `false`     | Permit cleartext `http://` Centreon URLs. Off by default; `http` sends credentials unencrypted (CWE-319). |
| `MCP_TRANSPORT`             | No       | `stdio`     | Transport mode: `stdio` or `http`                            |
| `MCP_HTTP_PORT`             | No       | `8080`      | HTTP listen port (HTTP transport only)                       |
| `MCP_HTTP_HOST`             | No       | `0.0.0.0`   | HTTP listen address (HTTP transport only)                    |
| `AUTH_MODE`                 | No       | `env`       | Authentication mode: `env` or `gateway`                      |
| `CENTREON_ALLOWED_HOSTS`    | No       | (none)      | Gateway mode only: comma-separated allowlist of accepted `X-Centreon-Host` values. Unset or empty means any host is accepted. |
| `LOG_LEVEL`                 | No       | `info`      | Log level: `debug`, `info`, `warn`, or `error`               |

\* Either `CENTREON_TOKEN` or both `CENTREON_USERNAME` and `CENTREON_PASSWORD` must be set. In `gateway` auth mode, credentials are supplied per-request via headers instead.

> **Note on redirects:** to protect the session token, the server never follows an HTTP redirect to a different host. Point `CENTREON_HOST` (and, in gateway mode, `X-Centreon-Host`) at the URL that serves the Centreon API directly. A host that redirects to a different hostname (for example an apex-to-`www` or a vanity-to-backend redirect) makes requests fail with `refusing cross-host redirect`; use the final resolved URL instead. Same-host redirects, including an `http` to `https` upgrade, are still followed.

> **Note on transport security:** to keep credentials off the wire, the server rejects a cleartext `http://` `CENTREON_HOST` (except in gateway mode, where it is an unused placeholder), `CENTREON_ALLOWED_HOSTS` entry, or gateway `X-Centreon-Host` value (CWE-319); use `https://`. For a trusted LAN or loopback deployment you can opt back in with `CENTREON_ALLOW_HTTP=true`, but credentials then travel unencrypted. Upgrading an existing `http://` deployment requires setting this flag; every `CENTREON_ALLOWED_HOSTS` entry must include a scheme (`https://`, or `http://` with the flag set).

## Usage with Claude Code

Add the server to your Claude Code settings (typically `~/.claude/settings.json`):

```json
{
  "mcpServers": {
    "centreon": {
      "command": "centreon-mcp-go",
      "env": {
        "CENTREON_HOST": "https://centreon.example.com",
        "CENTREON_USERNAME": "apiuser",
        "CENTREON_PASSWORD": "secret"
      }
    }
  }
}
```

To use an API token instead:

```json
{
  "mcpServers": {
    "centreon": {
      "command": "centreon-mcp-go",
      "env": {
        "CENTREON_HOST": "https://centreon.example.com",
        "CENTREON_TOKEN": "your-api-token"
      }
    }
  }
}
```

## Usage with Claude Desktop

Add the server to `claude_desktop_config.json` (location varies by OS, so check the Claude Desktop documentation):

```json
{
  "mcpServers": {
    "centreon": {
      "command": "/usr/local/bin/centreon-mcp-go",
      "env": {
        "CENTREON_HOST": "https://centreon.example.com",
        "CENTREON_USERNAME": "apiuser",
        "CENTREON_PASSWORD": "secret"
      }
    }
  }
}
```

## HTTP Transport Mode

HTTP mode runs the server as a persistent process with a Streamable HTTP endpoint at `/mcp` and a health endpoint at `/health`.

```bash
export CENTREON_HOST=https://centreon.example.com
export CENTREON_USERNAME=apiuser
export CENTREON_PASSWORD=secret
export MCP_TRANSPORT=http
export MCP_HTTP_PORT=8080
./centreon-mcp-go
```

Health check:

```bash
curl http://localhost:8080/health
# {"authMode":"env","status":"ok","transport":"http","version":"dev"}
```

Configure your MCP client to connect to `http://localhost:8080/mcp`.

## Gateway Mode

Gateway mode is for multi-tenant or shared deployments where each request carries its own Centreon credentials. The server creates a per-request Centreon client authenticated with the supplied credentials.

Enable it with `AUTH_MODE=gateway` alongside `MCP_TRANSPORT=http`. The effective Centreon host and credentials arrive per request via HTTP headers. `CENTREON_HOST` and a startup credential (`CENTREON_TOKEN`, or `CENTREON_USERNAME` + `CENTREON_PASSWORD`) are still required by `LoadConfig` but act only as placeholders. `CENTREON_HOST`'s scheme is not validated in gateway mode; each `X-Centreon-Host` is validated per request instead.

| Header                  | Description                                         |
|-------------------------|-----------------------------------------------------|
| `X-Centreon-Host`       | Required. Centreon server URL                       |
| `X-Centreon-Username`   | Username (use with `X-Centreon-Password`)           |
| `X-Centreon-Password`   | Password (use with `X-Centreon-Username`)           |
| `X-Centreon-Token`      | API token (alternative to username + password)      |

Acquired session tokens are cached for 50 minutes per (host, username) pair to avoid repeated logins.

On graceful shutdown (SIGINT/SIGTERM) the server logs out the Centreon sessions it created so they do not linger on the server until their idle timeout, and it waits for those logout calls to finish before exiting. Shutdown therefore takes a little longer than before, up to roughly 25 seconds if a Centreon host is slow or unreachable, so allow for this in your orchestrator's stop grace period (for example Kubernetes `terminationGracePeriodSeconds`).

### Host allowlist

By default gateway mode connects to whatever `X-Centreon-Host` a caller supplies, so an exposed endpoint can be used as an SSRF or open-proxy primitive. Set `CENTREON_ALLOWED_HOSTS` to a comma-separated list of permitted host URLs to restrict this; requests whose `X-Centreon-Host` does not match an entry exactly are rejected. When the variable is unset or empty, behavior is unchanged (any host is accepted) and the server logs a warning at startup. Matching is exact and case-sensitive, so list each host URL as callers send it.

```bash
export CENTREON_ALLOWED_HOSTS="https://centreon.example.com,https://centreon2.example.com"
```

**Note:** Deploy behind a reverse proxy (nginx, Caddy, Traefik, etc.) in production. Do not expose gateway mode directly to untrusted clients, as it accepts arbitrary Centreon credentials. Set `CENTREON_ALLOWED_HOSTS` to limit which Centreon servers the gateway will connect to.

```bash
export MCP_TRANSPORT=http
export AUTH_MODE=gateway
export CENTREON_ALLOWED_HOSTS="https://centreon.example.com"
./centreon-mcp-go
```

## Tool Reference

| File                        | Category                | Count | Example tools                                                                     |
|-----------------------------|-------------------------|-------|-----------------------------------------------------------------------------------|
| `monitoring_tools.go`       | Real-time monitoring    | 10    | `centreon_monitoring_host_list`, `centreon_monitoring_service_list`, `centreon_monitoring_resource_list` |
| `operations_tools.go`       | Bulk resource operations| 5     | `centreon_resource_acknowledge`, `centreon_resource_downtime`, `centreon_resource_check` |
| `downtime_tools.go`         | Downtime management     | 9     | `centreon_downtime_list`, `centreon_downtime_host_create`, `centreon_downtime_service_cancel` |
| `acknowledgement_tools.go`  | Acknowledgements        | 8     | `centreon_acknowledgement_list`, `centreon_acknowledgement_host_create`, `centreon_acknowledgement_service_cancel` |
| `host_config_tools.go`      | Host configuration      | 21    | `centreon_host_create`, `centreon_host_group_list`, `centreon_host_template_list` |
| `service_config_tools.go`   | Service configuration   | 16    | `centreon_service_create`, `centreon_service_group_list`, `centreon_service_template_list` |
| `infra_tools.go`            | Infrastructure          | 9     | `centreon_server_list`, `centreon_command_list`, `centreon_poller_apply`, `centreon_poller_apply_all` |
| `user_tools.go`             | Users and contacts      | 6     | `centreon_user_list`, `centreon_contact_group_list`, `centreon_user_filter_create` |
| `notification_tools.go`     | Notification policies   | 2     | `centreon_notification_policy_host_get`, `centreon_notification_policy_service_get` |
| `status_tools.go`           | Platform status         | 1     | `centreon_platform_status`                                                        |
| `connection_tools.go`       | Connection testing      | 1     | `centreon_connection_test`                                                        |

**Total: 88 tools**

## Centreon API Permissions

The Centreon user account used by this server requires access to the Centreon REST API v2. The required permissions depend on which tools you use:

- **Read-only monitoring** (list/get tools): standard user access with monitoring visibility
- **Acknowledgements and downtimes**: permission to create and cancel acknowledgements and downtimes on monitored resources
- **Configuration tools** (host_create, service_create, etc.): configuration administrator access
- **User/contact management**: user administration access
- **Platform status**: administrator or operator access

For a minimal read-only deployment, a standard monitoring user with API access is sufficient. For full tool coverage, an administrator account is recommended.

## Development

This project uses [Task](https://taskfile.dev) for development commands.

| Command          | Description                                   |
|------------------|-----------------------------------------------|
| `task check`     | Run all local checks: fmt, tidy, vet, lint, test |
| `task ci`        | Full CI pipeline: check then build            |
| `task go:build`  | Build the binary                              |
| `task go:run`    | Run the server in stdio mode                  |
| `task go:test`   | Run tests with race detector                  |
| `task go:lint`   | Run golangci-lint                             |
| `task go:fmt`    | Format code with goimports                    |
| `task go:tidy`   | Tidy go modules                               |
| `task go:clean`  | Remove build artifacts                        |
| `task image:build` | Build container image (default: podman)     |

To use Docker instead of Podman for image builds:

```bash
task image:build CONTAINER_TOOL=docker
```

## License

Apache-2.0. See [LICENSE](LICENSE) for details.
