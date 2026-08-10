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
	t.Setenv("CENTREON_ALLOW_HTTP", "")
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
	if cfg.AllowHTTP {
		t.Error("expected default AllowHTTP false")
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

// TestValidateHostScheme pins the cleartext-credential guard (CWE-319): https is
// always accepted, http only with the opt-in, and a missing or unsupported
// scheme (or an unparseable URL) is rejected outright.
func TestValidateHostScheme(t *testing.T) {
	tests := []struct {
		name      string
		host      string
		allowHTTP bool
		wantErr   string // substring to find; "" means expect nil
		notWant   string // substring the error must NOT contain (credential redaction)
	}{
		{"https accepted", "https://centreon.example.com", false, "", ""},
		{"https uppercase scheme accepted", "HTTPS://centreon.example.com", false, "", ""},
		{"https ipv6 accepted", "https://[::1]:8443", false, "", ""},
		{"https with userinfo accepted", "https://admin:sekret@centreon.example.com", false, "", ""},
		{"http rejected by default", "http://centreon.example.com", false, "CWE-319", ""},
		{"http uppercase rejected by default", "HTTP://centreon.example.com", false, "CWE-319", ""},
		{"http accepted with opt-in", "http://centreon.example.com", true, "", ""},
		{"http userinfo rejected with password redacted", "http://admin:sekret@centreon.example.com", false, "CWE-319", "sekret"},
		{"missing scheme rejected", "centreon.example.com", false, "must include a scheme", ""},
		{"unsupported scheme rejected", "ftp://centreon.example.com", false, "unsupported scheme", ""},
		{"host:port without scheme rejected as unsupported", "centreon.example.com:443", false, "unsupported scheme", ""},
		{"unparseable host rejected", "127.0.0.1:8080", false, "invalid host url", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHostScheme(tt.host, tt.allowHTTP)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("validateHostScheme(%q, %v) = %v, want nil", tt.host, tt.allowHTTP, err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Errorf("validateHostScheme(%q, %v) = %v, want error containing %q", tt.host, tt.allowHTTP, err, tt.wantErr)
			}
			if tt.notWant != "" && err != nil && strings.Contains(err.Error(), tt.notWant) {
				t.Errorf("validateHostScheme(%q) error must not leak %q, got %v", tt.host, tt.notWant, err)
			}
		})
	}
}

// TestSafeHost pins that safeHost masks a userinfo password (so it cannot reach
// logs or error messages, CWE-532) across every form that can carry one, including
// the scheme-less user:pass@host form url.Redacted alone does not mask (issue #41),
// while leaving a credential-free host byte for byte unchanged.
func TestSafeHost(t *testing.T) {
	tests := []struct {
		name string
		host string
		want string
	}{
		{"masks password in http userinfo", "http://admin:sekret@centreon.example.com", "http://admin:xxxxx@centreon.example.com"},
		{"masks password in https userinfo", "https://admin:sekret@centreon.example.com", "https://admin:xxxxx@centreon.example.com"},
		// Scheme-less/opaque forms: url.Parse leaves no userinfo for url.Redacted, so
		// these leaked before #41. safeHost reparses (or masks textually) instead.
		{"masks scheme-less userinfo", "admin:sekret@centreon.example.com", "admin:xxxxx@centreon.example.com"},
		{"masks scheme-less userinfo with path", "admin:sekret@centreon.example.com/mon", "admin:xxxxx@centreon.example.com/mon"},
		{"masks leading //authority userinfo", "//admin:sekret@centreon.example.com", "//admin:xxxxx@centreon.example.com"},
		{"masks empty-scheme ://authority userinfo", "://admin:sekret@centreon.example.com", "://admin:xxxxx@centreon.example.com"},
		{"masks userinfo with port and path", "https://admin:sekret@centreon.example.com:8443/mon?q=1", "https://admin:xxxxx@centreon.example.com:8443/mon?q=1"},
		{"masks userinfo when the path also has @", "https://admin:sekret@centreon.example.com/p@th", "https://admin:xxxxx@centreon.example.com/p@th"},
		// url.Parse fails outright on the bad percent-escape, so the textual fallback
		// must still redact rather than return the raw credential.
		{"masks userinfo in an unparseable URL", "https://admin:sekret@centreon.example.com/%zz", "https://admin:xxxxx@centreon.example.com/%zz"},
		// A password containing an unescaped '/', '?' or '#' breaks url.Parse and must
		// not fail open (base64 passwords routinely contain '/').
		{"masks password with a slash", "https://admin:aB9/xY2z@centreon.example.com/api", "https://admin:xxxxx@centreon.example.com/api"},
		{"masks scheme-less password with a slash", "admin:aB9/xY2z@centreon.example.com", "admin:xxxxx@centreon.example.com"},
		{"masks password with a question mark", "https://admin:pa?ss@centreon.example.com", "https://admin:xxxxx@centreon.example.com"},
		{"masks password with a hash", "https://admin:pa#ss@centreon.example.com", "https://admin:xxxxx@centreon.example.com"},
		{"masks password beginning with a slash", "https://admin:/sekret@centreon.example.com", "https://admin:xxxxx@centreon.example.com"},
		{"masks scheme-less password beginning with a slash", "admin:/sekret@centreon.example.com", "admin:xxxxx@centreon.example.com"},
		{"masks scheme-less password beginning with a question mark", "admin:?sekret@centreon.example.com", "admin:xxxxx@centreon.example.com"},
		// Credential-free hosts whose path or query legitimately contains ':' or '@'
		// must never be corrupted.
		{"leaves host:port with @ in path unchanged", "https://centreon.example.com:8080/a@b", "https://centreon.example.com:8080/a@b"},
		{"leaves : and @ in query unchanged", "https://centreon.example.com:8443/x?a=b:c@d", "https://centreon.example.com:8443/x?a=b:c@d"},
		{"leaves IPv6 host with @ in path unchanged", "https://[2001:db8::1]/x@y", "https://[2001:db8::1]/x@y"},
		{"leaves scheme-less host with @ in path unchanged", "centreon.example.com/a@b", "centreon.example.com/a@b"},
		{"leaves plain host unchanged", "https://centreon.example.com", "https://centreon.example.com"},
		{"leaves @ in query unchanged", "https://centreon.example.com/path?x=a@b.com", "https://centreon.example.com/path?x=a@b.com"},
		{"leaves userinfo without a password unchanged", "https://user@centreon.example.com", "https://user@centreon.example.com"},
		{"leaves empty string unchanged", "", ""},
		{"passes through unparseable host", "127.0.0.1:8080", "127.0.0.1:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := safeHost(tt.host); got != tt.want {
				t.Errorf("safeHost(%q) = %q, want %q", tt.host, got, tt.want)
			}
		})
	}
}

// TestDisplayHost pins that displayHost reduces a host URL to scheme://host[:port]
// for a tool response, stripping the username as well as the password (issue #48
// requires no username/password/token in the response), unlike safeHost which
// keeps the username for log greps.
func TestDisplayHost(t *testing.T) {
	tests := []struct {
		name string
		host string
		want string
	}{
		{"strips username and password", "https://gwuser:sekret@centreon.example.com", "https://centreon.example.com"},
		{"strips username-only userinfo", "https://gwuser@centreon.example.com", "https://centreon.example.com"},
		{"strips userinfo over http", "http://admin:sekret@centreon.example.com", "http://centreon.example.com"},
		{"keeps host and port", "https://centreon.example.com:8443", "https://centreon.example.com:8443"},
		{"strips path and query", "https://centreon.example.com:8443/mon?q=1", "https://centreon.example.com:8443"},
		{"strips userinfo with port and path", "https://admin:sekret@centreon.example.com:8443/mon", "https://centreon.example.com:8443"},
		{"leaves plain host unchanged", "https://centreon.example.com", "https://centreon.example.com"},
		// The numeric-prefix userinfo form (issue #55) is mis-parsed by url.Parse as
		// host:port; displayHost still does not leak the "secret" password span, and
		// this malformed form is not reachable through the success-gated tool sink.
		{"does not leak password on the numeric-prefix form", "https://admin:1234/secret@centreon.example.com", "https://admin:1234"},
		{"fails closed on empty", "", "(redacted host)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := displayHost(tt.host); got != tt.want {
				t.Errorf("displayHost(%q) = %q, want %q", tt.host, got, tt.want)
			}
		})
	}
}

// TestLoadConfig_HostScheme pins that CENTREON_HOST is scheme-checked at load,
// gated by CENTREON_ALLOW_HTTP, and skipped only in HTTP gateway mode where the
// host is an unused placeholder. It also pins that stdio+gateway still validates,
// so the skip requires BOTH the http transport and gateway auth mode.
func TestLoadConfig_HostScheme(t *testing.T) {
	tests := []struct {
		name          string
		host          string
		transport     string
		authMode      string
		allowHTTP     string
		wantErr       string // "" = no error
		wantAllowHTTP bool
	}{
		{"https host accepted", "https://h.example.com", "", "", "", "", false},
		{"http host rejected by default", "http://h.example.com", "", "", "", "CENTREON_HOST", false},
		{"http host accepted with opt-in", "http://h.example.com", "", "", "true", "", true},
		{"invalid allow-http value errors", "https://h.example.com", "", "", "banana", "CENTREON_ALLOW_HTTP", false},
		{"gateway mode skips http host check", "http://placeholder.example.com", "http", "gateway", "", "", false},
		{"http transport env auth still validates http host", "http://h.example.com", "http", "env", "", "CENTREON_HOST", false},
		{"stdio with gateway auth still validates http host", "http://h.example.com", "stdio", "gateway", "", "CENTREON_HOST", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CENTREON_HOST", tt.host)
			t.Setenv("CENTREON_TOKEN", "tok")
			t.Setenv("CENTREON_USERNAME", "")
			t.Setenv("CENTREON_PASSWORD", "")
			t.Setenv("CENTREON_ALLOWED_HOSTS", "")
			t.Setenv("MCP_HTTP_PORT", "")
			t.Setenv("MCP_TRANSPORT", tt.transport)
			t.Setenv("AUTH_MODE", tt.authMode)
			t.Setenv("CENTREON_ALLOW_HTTP", tt.allowHTTP)

			cfg, err := LoadConfig()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("LoadConfig() error = %v, want error containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadConfig() unexpected error: %v", err)
			}
			if cfg.AllowHTTP != tt.wantAllowHTTP {
				t.Errorf("AllowHTTP = %v, want %v", cfg.AllowHTTP, tt.wantAllowHTTP)
			}
		})
	}
}

// TestLoadConfig_AllowedHostsScheme pins that each CENTREON_ALLOWED_HOSTS entry
// is scheme-checked at load, gated by CENTREON_ALLOW_HTTP.
func TestLoadConfig_AllowedHostsScheme(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		allowHTTP string
		wantErr   string // "" = no error
	}{
		{"all https accepted", "https://a.example.com,https://b.example.com", "", ""},
		{"http entry rejected by default", "https://a.example.com,http://b.example.com", "", "CENTREON_ALLOWED_HOSTS"},
		{"http entry accepted with opt-in", "https://a.example.com,http://b.example.com", "true", ""},
		{"scheme-less entry rejected", "a.example.com", "", "CENTREON_ALLOWED_HOSTS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CENTREON_HOST", "https://centreon.example.com")
			t.Setenv("CENTREON_TOKEN", "tok")
			t.Setenv("CENTREON_USERNAME", "")
			t.Setenv("CENTREON_PASSWORD", "")
			t.Setenv("MCP_TRANSPORT", "")
			t.Setenv("AUTH_MODE", "")
			t.Setenv("MCP_HTTP_PORT", "")
			t.Setenv("CENTREON_ALLOW_HTTP", tt.allowHTTP)
			t.Setenv("CENTREON_ALLOWED_HOSTS", tt.value)

			_, err := LoadConfig()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("LoadConfig() error = %v, want error containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadConfig() unexpected error: %v", err)
			}
		})
	}
}
