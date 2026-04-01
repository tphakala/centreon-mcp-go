# centreon-mcp-go

An MCP (Model Context Protocol) server for Centreon monitoring, written in Go.

Exposes 73 tools covering real-time monitoring, host and service configuration, downtime and acknowledgement management, infrastructure, users, and notifications. Integrates with any MCP-compatible AI client such as Claude Code or Claude Desktop.

## Features

- **73 tools** across 11 categories — monitoring, operations, downtimes, acknowledgements, host config, service config, infrastructure, users, notifications, platform status, and connection testing
- **Three transport modes** — stdio (default), HTTP (streamable), and HTTP gateway mode
- **Structured JSON logging** via `log/slog` with configurable levels
- **Gateway mode with token cache** — per-request Centreon credentials via HTTP headers, with a 50-minute token cache to avoid repeated logins
- **Self-signed certificate support** — opt-in via `CENTREON_ALLOW_SELF_SIGNED`

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
# Build the image
podman build -t centreon-mcp-go .

# Run in HTTP mode
podman run -d \
  -e CENTREON_HOST=https://centreon.example.com \
  -e CENTREON_USERNAME=apiuser \
  -e CENTREON_PASSWORD=secret \
  -e MCP_TRANSPORT=http \
  -p 8080:8080 \
  centreon-mcp-go
```

## Configuration

All configuration is via environment variables.

| Variable                    | Required | Default     | Description                                                  |
|-----------------------------|----------|-------------|--------------------------------------------------------------|
| `CENTREON_HOST`             | Yes      | —           | Centreon server base URL (e.g. `https://centreon.example.com`) |
| `CENTREON_USERNAME`         | *        | —           | Username for session-based authentication                    |
| `CENTREON_PASSWORD`         | *        | —           | Password for session-based authentication                    |
| `CENTREON_TOKEN`            | *        | —           | API token (alternative to username + password)               |
| `CENTREON_ALLOW_SELF_SIGNED`| No       | `false`     | Accept self-signed TLS certificates                          |
| `MCP_TRANSPORT`             | No       | `stdio`     | Transport mode: `stdio` or `http`                            |
| `MCP_HTTP_PORT`             | No       | `8080`      | HTTP listen port (HTTP transport only)                       |
| `MCP_HTTP_HOST`             | No       | `0.0.0.0`   | HTTP listen address (HTTP transport only)                    |
| `AUTH_MODE`                 | No       | `env`       | Authentication mode: `env` or `gateway`                      |
| `LOG_LEVEL`                 | No       | `info`      | Log level: `debug`, `info`, `warn`, or `error`               |

\* Either `CENTREON_TOKEN` or both `CENTREON_USERNAME` and `CENTREON_PASSWORD` must be set. In `gateway` auth mode, credentials are supplied per-request via headers instead.

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

Add the server to `claude_desktop_config.json` (location varies by OS — check the Claude Desktop documentation):

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

Enable it with `AUTH_MODE=gateway` alongside `MCP_TRANSPORT=http`. In this mode, `CENTREON_HOST`, `CENTREON_USERNAME`, `CENTREON_PASSWORD`, and `CENTREON_TOKEN` are not required at startup — they are provided per request via HTTP headers.

| Header                  | Description                                         |
|-------------------------|-----------------------------------------------------|
| `X-Centreon-Host`       | Required. Centreon server URL                       |
| `X-Centreon-Username`   | Username (use with `X-Centreon-Password`)           |
| `X-Centreon-Password`   | Password (use with `X-Centreon-Username`)           |
| `X-Centreon-Token`      | API token (alternative to username + password)      |

Acquired session tokens are cached for 50 minutes per (host, username) pair to avoid repeated logins.

**Note:** Deploy behind a reverse proxy (nginx, Caddy, Traefik, etc.) in production. Do not expose gateway mode directly to untrusted clients, as it accepts arbitrary Centreon credentials.

```bash
export MCP_TRANSPORT=http
export AUTH_MODE=gateway
./centreon-mcp-go
```

## Tool Reference

| File                        | Category                | Count | Example tools                                                                     |
|-----------------------------|-------------------------|-------|-----------------------------------------------------------------------------------|
| `monitoring_tools.go`       | Real-time monitoring    | 10    | `centreon_monitoring_host_list`, `centreon_monitoring_service_list`, `centreon_monitoring_resource_list` |
| `operations_tools.go`       | Bulk resource operations| 5     | `centreon_resource_acknowledge`, `centreon_resource_downtime`, `centreon_resource_check` |
| `downtime_tools.go`         | Downtime management     | 9     | `centreon_downtime_list`, `centreon_downtime_host_create`, `centreon_downtime_service_cancel` |
| `acknowledgement_tools.go`  | Acknowledgements        | 8     | `centreon_acknowledgement_list`, `centreon_acknowledgement_host_create`, `centreon_acknowledgement_service_cancel` |
| `host_config_tools.go`      | Host configuration      | 13    | `centreon_host_create`, `centreon_host_group_list`, `centreon_host_template_list` |
| `service_config_tools.go`   | Service configuration   | 13    | `centreon_service_create`, `centreon_service_group_list`, `centreon_service_template_list` |
| `infra_tools.go`            | Infrastructure          | 5     | `centreon_server_list`, `centreon_command_list`, `centreon_time_period_create`    |
| `user_tools.go`             | Users and contacts      | 6     | `centreon_user_list`, `centreon_contact_group_list`, `centreon_user_filter_create` |
| `notification_tools.go`     | Notification policies   | 2     | `centreon_notification_policy_host_get`, `centreon_notification_policy_service_get` |
| `status_tools.go`           | Platform status         | 1     | `centreon_platform_status`                                                        |
| `connection_tools.go`       | Connection testing      | 1     | `centreon_connection_test`                                                        |

**Total: 73 tools**

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
| `task ci`        | Full CI pipeline — check then build           |
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
