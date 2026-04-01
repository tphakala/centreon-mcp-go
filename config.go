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

	if cfg.HTTPHost == "" {
		cfg.HTTPHost = "0.0.0.0"
	}

	portStr := os.Getenv("MCP_HTTP_PORT")
	if portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err == nil {
			cfg.HTTPPort = port
		}
	}
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = 8080
	}

	selfSigned := os.Getenv("CENTREON_ALLOW_SELF_SIGNED")
	if selfSigned != "" {
		v, err := strconv.ParseBool(selfSigned)
		if err == nil {
			cfg.AllowSelfSigned = v
		}
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
