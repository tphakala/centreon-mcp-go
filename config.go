package main

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// defaultHTTPPort is the listen port used when MCP_HTTP_PORT is unset.
const defaultHTTPPort = 8080

// URL schemes accepted for a Centreon host. These name a URL scheme and are
// deliberately separate from the transport* constants (which name the MCP
// transport mode), even though "http" is the same string in both.
const (
	schemeHTTPS = "https"
	schemeHTTP  = "http"
)

// Config holds the server configuration.
type Config struct {
	Host            string
	Username        string
	Password        string
	Token           string
	AllowSelfSigned bool
	AllowHTTP       bool
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

	cfg.Transport = envOr("MCP_TRANSPORT", transportStdio)
	cfg.LogLevel = envOr("LOG_LEVEL", "info")
	cfg.AuthMode = envOr("AUTH_MODE", authModeEnv)

	switch cfg.Transport {
	case transportStdio, transportHTTP:
	default:
		return Config{}, fmt.Errorf("invalid MCP_TRANSPORT value %q: expected stdio/http", cfg.Transport)
	}

	switch cfg.AuthMode {
	case authModeEnv, authModeGateway:
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

	cfg.AllowSelfSigned, err = parseBoolEnv("CENTREON_ALLOW_SELF_SIGNED")
	if err != nil {
		return Config{}, err
	}
	cfg.AllowHTTP, err = parseBoolEnv("CENTREON_ALLOW_HTTP")
	if err != nil {
		return Config{}, err
	}

	// Reject a cleartext http:// CENTREON_HOST (CWE-319) unless the operator opts
	// in. In HTTP gateway mode CENTREON_HOST is an unused placeholder and the real
	// hosts arrive per request via X-Centreon-Host, so its scheme is not validated
	// here (gatewayServer validates each header value instead).
	if !gatewayMode(&cfg) {
		if err := validateHostScheme(cfg.Host, cfg.AllowHTTP); err != nil {
			return Config{}, fmt.Errorf("CENTREON_HOST: %w", err)
		}
	}

	allowedHosts, err := loadAllowedHosts(cfg.AllowHTTP)
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
func loadAllowedHosts(allowHTTP bool) ([]string, error) {
	raw := os.Getenv("CENTREON_ALLOWED_HOSTS")
	if raw == "" {
		return nil, nil
	}
	hosts := parseAllowedHosts(raw)
	if len(hosts) == 0 {
		return nil, fmt.Errorf("CENTREON_ALLOWED_HOSTS is set but contains no valid host entries")
	}
	for _, h := range hosts {
		if err := validateHostScheme(h, allowHTTP); err != nil {
			return nil, fmt.Errorf("CENTREON_ALLOWED_HOSTS: %w", err)
		}
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

// parseBoolEnv reads a boolean environment variable. An unset or empty value
// yields false with no error; any other value is parsed with strconv.ParseBool.
func parseBoolEnv(name string) (bool, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return false, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("invalid %s value %q: expected true/false", name, raw)
	}
	return v, nil
}

// gatewayMode reports whether the server runs in HTTP gateway mode: http
// transport with gateway auth. This is the only mode where the effective
// Centreon hosts arrive per request via the X-Centreon-Host header (so
// CENTREON_HOST is an unused placeholder) and the only mode that enforces the
// CENTREON_ALLOWED_HOSTS allowlist. Every other mode (stdio, or HTTP with env
// auth) uses CENTREON_HOST directly and never enforces the allowlist.
func gatewayMode(cfg *Config) bool {
	return cfg.Transport == transportHTTP && cfg.AuthMode == authModeGateway
}

// validateHostScheme rejects a Centreon host URL whose scheme would send
// credentials in cleartext. Credentials (username/password, or the X-AUTH-TOKEN
// header once authenticated) ride every request to the host, so an http:// target
// exposes them on the wire (CWE-319). https is always accepted; http only when the
// operator opts in via CENTREON_ALLOW_HTTP. A missing or non-http(s) scheme, or an
// unparseable URL, is rejected outright (fail closed). url.Parse normalises the
// scheme to lowercase, so the cases below are matched in lowercase.
func validateHostScheme(host string, allowHTTP bool) error {
	u, err := url.Parse(host)
	if err != nil {
		return fmt.Errorf("invalid host url %q: %w", safeHost(host), err)
	}
	switch u.Scheme {
	case schemeHTTPS:
		return nil
	case schemeHTTP:
		if allowHTTP {
			return nil
		}
		return fmt.Errorf("host %q uses http, which sends credentials in cleartext (CWE-319): use https or set CENTREON_ALLOW_HTTP=true", safeHost(host))
	case "":
		return fmt.Errorf("host %q must include a scheme (https://...)", safeHost(host))
	default:
		return fmt.Errorf("host %q has unsupported scheme %q: use https (or http with CENTREON_ALLOW_HTTP=true)", safeHost(host), u.Scheme)
	}
}

// safeHost masks the password in a standard scheme://user:pass@host URL so it can
// be put into an error message or log field without leaking an embedded credential
// (CWE-532). It uses url.Redacted, so it covers the realistic case where a
// credential is written into the host URL; a scheme-less or unparseable host is
// returned unchanged, which validateHostScheme rejects anyway. This handles the
// surfaces this file adds; the pre-existing gateway host log lines are tracked in
// a separate issue.
func safeHost(host string) string {
	u, err := url.Parse(host)
	if err != nil {
		return host
	}
	return u.Redacted()
}
