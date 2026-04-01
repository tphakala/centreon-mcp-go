package main

import (
	"fmt"
	"os"
	"strconv"
)

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

	switch cfg.Transport { //nolint:goconst // validation uses literal values
	case "stdio", "http":
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

	portStr := os.Getenv("MCP_HTTP_PORT")
	if portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return Config{}, fmt.Errorf("invalid MCP_HTTP_PORT value %q: %w", portStr, err)
		}
		if port < 1 || port > 65535 {
			return Config{}, fmt.Errorf("MCP_HTTP_PORT must be between 1 and 65535, got %d", port)
		}
		cfg.HTTPPort = port
	}
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = 8080
	}

	selfSigned := os.Getenv("CENTREON_ALLOW_SELF_SIGNED")
	if selfSigned != "" {
		v, err := strconv.ParseBool(selfSigned)
		if err != nil {
			return Config{}, fmt.Errorf("invalid CENTREON_ALLOW_SELF_SIGNED value %q: expected true/false", selfSigned)
		}
		cfg.AllowSelfSigned = v
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
