package main

import (
	"slices"
	"strings"
	"testing"
)

func TestLoadConfig_Valid(t *testing.T) {
	t.Setenv("CENTREON_HOST", "https://centreon.example.com")
	t.Setenv("CENTREON_USERNAME", "admin")
	t.Setenv("CENTREON_PASSWORD", "secret")
	t.Setenv("CENTREON_TOKEN", "tok123")
	t.Setenv("CENTREON_ALLOW_SELF_SIGNED", "true")
	t.Setenv("MCP_TRANSPORT", "http")
	t.Setenv("MCP_HTTP_PORT", "9090")
	t.Setenv("MCP_HTTP_HOST", "127.0.0.1")
	t.Setenv("AUTH_MODE", "gateway")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.Host != "https://centreon.example.com" {
		t.Errorf("expected Host https://centreon.example.com, got %q", cfg.Host)
	}
	if cfg.Username != "admin" {
		t.Errorf("expected Username admin, got %q", cfg.Username)
	}
	if cfg.Password != "secret" {
		t.Errorf("expected Password secret, got %q", cfg.Password)
	}
	if cfg.Token != "tok123" {
		t.Errorf("expected Token tok123, got %q", cfg.Token)
	}
	if !cfg.AllowSelfSigned {
		t.Error("expected AllowSelfSigned true")
	}
	if cfg.Transport != "http" {
		t.Errorf("expected Transport http, got %q", cfg.Transport)
	}
	if cfg.HTTPPort != 9090 {
		t.Errorf("expected HTTPPort 9090, got %d", cfg.HTTPPort)
	}
	if cfg.HTTPHost != "127.0.0.1" {
		t.Errorf("expected HTTPHost 127.0.0.1, got %q", cfg.HTTPHost)
	}
	if cfg.AuthMode != "gateway" {
		t.Errorf("expected AuthMode gateway, got %q", cfg.AuthMode)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel debug, got %q", cfg.LogLevel)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	t.Setenv("CENTREON_HOST", "https://centreon.example.com")
	t.Setenv("CENTREON_USERNAME", "admin")
	t.Setenv("CENTREON_PASSWORD", "secret")
	t.Setenv("CENTREON_TOKEN", "")
	t.Setenv("CENTREON_ALLOW_SELF_SIGNED", "")
	t.Setenv("MCP_TRANSPORT", "")
	t.Setenv("MCP_HTTP_PORT", "")
	t.Setenv("MCP_HTTP_HOST", "")
	t.Setenv("AUTH_MODE", "")
	t.Setenv("LOG_LEVEL", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.Transport != "stdio" {
		t.Errorf("expected default Transport stdio, got %q", cfg.Transport)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected default LogLevel info, got %q", cfg.LogLevel)
	}
	if cfg.HTTPHost != "0.0.0.0" {
		t.Errorf("expected default HTTPHost 0.0.0.0, got %q", cfg.HTTPHost)
	}
	if cfg.HTTPPort != 8080 {
		t.Errorf("expected default HTTPPort 8080, got %d", cfg.HTTPPort)
	}
	if cfg.AuthMode != "env" {
		t.Errorf("expected default AuthMode env, got %q", cfg.AuthMode)
	}
	if cfg.AllowSelfSigned {
		t.Error("expected default AllowSelfSigned false")
	}
}

func TestLoadConfig_MissingRequired(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		username string
		password string
		wantErr  string
	}{
		{"missing host", "", "admin", "secret", "CENTREON_HOST environment variable is required"},
		{"missing username", "https://h.com", "", "secret", "CENTREON_USERNAME environment variable is required (or set CENTREON_TOKEN)"},
		{"missing password", "https://h.com", "admin", "", "CENTREON_PASSWORD environment variable is required (or set CENTREON_TOKEN)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CENTREON_HOST", tt.host)
			t.Setenv("CENTREON_USERNAME", tt.username)
			t.Setenv("CENTREON_PASSWORD", tt.password)
			t.Setenv("CENTREON_TOKEN", "")

			_, err := LoadConfig()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestLoadConfig_TokenOnly(t *testing.T) {
	t.Setenv("CENTREON_HOST", "https://centreon.example.com")
	t.Setenv("CENTREON_USERNAME", "")
	t.Setenv("CENTREON_PASSWORD", "")
	t.Setenv("CENTREON_TOKEN", "my-api-token")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.Token != "my-api-token" {
		t.Errorf("expected Token my-api-token, got %q", cfg.Token)
	}
}

func TestLoadConfig_AllowedHosts(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    []string
		wantErr bool
	}{
		{"unset or empty yields no allowlist", "", nil, false},
		{"single host", "https://a.example.com", []string{"https://a.example.com"}, false},
		{
			"multiple hosts trimmed",
			" https://a.example.com , https://b.example.com ",
			[]string{"https://a.example.com", "https://b.example.com"},
			false,
		},
		{"empty entries dropped", "https://a.example.com,,", []string{"https://a.example.com"}, false},
		{"only separators and whitespace errors", " , , ", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CENTREON_HOST", "https://centreon.example.com")
			t.Setenv("CENTREON_USERNAME", "admin")
			t.Setenv("CENTREON_PASSWORD", "secret")
			t.Setenv("CENTREON_TOKEN", "")
			// Isolate from ambient values so the only error source is the allowlist.
			t.Setenv("MCP_HTTP_PORT", "")
			t.Setenv("MCP_TRANSPORT", "")
			t.Setenv("AUTH_MODE", "")
			t.Setenv("CENTREON_ALLOWED_HOSTS", tt.value)

			cfg, err := LoadConfig()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), "CENTREON_ALLOWED_HOSTS") {
					t.Errorf("expected error about CENTREON_ALLOWED_HOSTS, got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if !slices.Equal(cfg.AllowedHosts, tt.want) {
				t.Errorf("expected AllowedHosts %v, got %v", tt.want, cfg.AllowedHosts)
			}
		})
	}
}

func TestLoadConfig_HTTPPort(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    int
		wantErr bool
	}{
		{"unset defaults to 8080", "", defaultHTTPPort, false},
		{"valid port", "9090", 9090, false},
		{"non-numeric errors", "abc", 0, true},
		{"below range errors", "0", 0, true},
		{"above range errors", "70000", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CENTREON_HOST", "https://centreon.example.com")
			t.Setenv("CENTREON_TOKEN", "tok")
			t.Setenv("CENTREON_USERNAME", "")
			t.Setenv("CENTREON_PASSWORD", "")
			// Isolate from ambient values so the only error source is the port.
			t.Setenv("CENTREON_ALLOWED_HOSTS", "")
			t.Setenv("MCP_TRANSPORT", "")
			t.Setenv("AUTH_MODE", "")
			t.Setenv("MCP_HTTP_PORT", tt.value)

			cfg, err := LoadConfig()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), "MCP_HTTP_PORT") {
					t.Errorf("expected error about MCP_HTTP_PORT, got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if cfg.HTTPPort != tt.want {
				t.Errorf("HTTPPort = %d, want %d", cfg.HTTPPort, tt.want)
			}
		})
	}
}
