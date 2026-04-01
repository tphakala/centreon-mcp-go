package main

import "testing"

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
