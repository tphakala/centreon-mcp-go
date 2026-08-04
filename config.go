package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// defaultHTTPPort is the listen port used when MCP_HTTP_PORT is unset.
const defaultHTTPPort = 8080

// Config holds the server configuration.
type Config struct {
	Host            string
	Username        string
	Password        string
	Token           string
	AllowSelfSigned bool
	Transport       string
	HTTPPort        int
	HTTPHost        string
	AuthMode        string
	LogLevel        string
	// AllowedHosts restricts the X-Centreon-Host header in gateway mode.
	// Empty means no restriction (any host accepted).
	AllowedHosts []string
}

// LoadConfig reads configuration from environment variables.
func LoadConfig() (Config, error) {
	cfg := Config{
		Host:     os.Getenv("CENTREON_HOST"),
		Username: os.Getenv("CENTREON_USERNAME"),
		Password: os.Getenv("CENTREON_PASSWORD"),
		Token:    os.Getenv("CENTREON_TOKEN"),
		HTTPHost: os.Getenv("MCP_HTTP_HOST"),
	}

	if cfg.Host == "" {
		return Config{}, fmt.Errorf("CENTREON_HOST environment variable is required")
	}

	// Either token or username+password must be set
	if cfg.Token == "" {
		if cfg.Username == "" {
			return Config{}, fmt.Errorf("CENTREON_USERNAME environment variable is required (or set CENTREON_TOKEN)")
		}
		if cfg.Password == "" {
			return Config{}, fmt.Errorf("CENTREON_PASSWORD environment variable is required (or set CENTREON_TOKEN)")
		}
	}

	cfg.Transport = envOr("MCP_TRANSPORT", "stdio")
	cfg.LogLevel = envOr("LOG_LEVEL", "info")
	cfg.AuthMode = envOr("AUTH_MODE", "env")

	switch cfg.Transport {
	case "stdio", transportHTTP:
	default:
		return Config{}, fmt.Errorf("invalid MCP_TRANSPORT value %q: expected stdio/http", cfg.Transport)
	}

	switch cfg.AuthMode {
	case "env", "gateway":
	default:
		return Config{}, fmt.Errorf("invalid AUTH_MODE value %q: expected env/gateway", cfg.AuthMode)
	}

	if cfg.HTTPHost == "" {
		cfg.HTTPHost = "0.0.0.0"
	}

	port, err := loadHTTPPort()
	if err != nil {
		return Config{}, err
	}
	cfg.HTTPPort = port

	selfSigned := os.Getenv("CENTREON_ALLOW_SELF_SIGNED")
	if selfSigned != "" {
		v, err := strconv.ParseBool(selfSigned)
		if err != nil {
			return Config{}, fmt.Errorf("invalid CENTREON_ALLOW_SELF_SIGNED value %q: expected true/false", selfSigned)
		}
		cfg.AllowSelfSigned = v
	}

	allowedHosts, err := loadAllowedHosts()
	if err != nil {
		return Config{}, err
	}
	cfg.AllowedHosts = allowedHosts

	return cfg, nil
}

// loadHTTPPort resolves the MCP_HTTP_PORT value, defaulting to 8080 when unset.
func loadHTTPPort() (int, error) {
	portStr := os.Getenv("MCP_HTTP_PORT")
	if portStr == "" {
		return defaultHTTPPort, nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid MCP_HTTP_PORT value %q: %w", portStr, err)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("MCP_HTTP_PORT must be between 1 and 65535, got %d", port)
	}
	return port, nil
}

// loadAllowedHosts reads and parses the CENTREON_ALLOWED_HOSTS allowlist.
// An unset or empty variable yields a nil slice (no restriction), matching
// os.Getenv, which cannot distinguish the two. A variable set to a non-empty
// value that has no valid entries after trimming (e.g. " , , ") is a
// misconfiguration and returns an error.
func loadAllowedHosts() ([]string, error) {
	raw := os.Getenv("CENTREON_ALLOWED_HOSTS")
	if raw == "" {
		return nil, nil
	}
	hosts := parseAllowedHosts(raw)
	if len(hosts) == 0 {
		return nil, fmt.Errorf("CENTREON_ALLOWED_HOSTS is set but contains no valid host entries")
	}
	return hosts, nil
}

// parseAllowedHosts splits a comma-separated host list, trimming whitespace
// and dropping empty entries.
func parseAllowedHosts(raw string) []string {
	parts := strings.Split(raw, ",")
	hosts := make([]string, 0, len(parts))
	for _, p := range parts {
		if h := strings.TrimSpace(p); h != "" {
			hosts = append(hosts, h)
		}
	}
	return hosts
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
