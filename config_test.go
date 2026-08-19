package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLoadConfig_Valid(t *testing.T) {
	t.Setenv("CENTREON_HOST", "https://centreon.example.com")
	t.Setenv("CENTREON_USERNAME", "admin")
	t.Setenv("CENTREON_PASSWORD", "secret")
	t.Setenv("CENTREON_TOKEN", "tok123")
	t.Setenv("CENTREON_PASSWORD_FILE", "")
	t.Setenv("CENTREON_TOKEN_FILE", "")
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
	t.Setenv("CENTREON_PASSWORD_FILE", "")
	t.Setenv("CENTREON_TOKEN_FILE", "")
	t.Setenv("CENTREON_ALLOW_SELF_SIGNED", "")
	t.Setenv("CENTREON_ALLOW_HTTP", "")
	t.Setenv("MCP_READ_ONLY", "")
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
	if cfg.ReadOnly {
		t.Error("expected default ReadOnly false")
	}
}

func TestLoadConfig_ReadOnly(t *testing.T) {
	base := func(t *testing.T) {
		t.Helper()
		t.Setenv("CENTREON_HOST", "https://centreon.example.com")
		t.Setenv("CENTREON_USERNAME", "admin")
		t.Setenv("CENTREON_PASSWORD", "secret")
		t.Setenv("CENTREON_TOKEN", "")
		t.Setenv("CENTREON_PASSWORD_FILE", "")
		t.Setenv("CENTREON_TOKEN_FILE", "")
	}

	t.Run("true enables read-only", func(t *testing.T) {
		base(t)
		t.Setenv("MCP_READ_ONLY", "true")
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !cfg.ReadOnly {
			t.Error("expected ReadOnly true")
		}
	})

	t.Run("invalid value is a fatal config error", func(t *testing.T) {
		base(t)
		t.Setenv("MCP_READ_ONLY", "yesplease")
		if _, err := LoadConfig(); err == nil {
			t.Error("expected an error for an invalid MCP_READ_ONLY value")
		}
	})
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
			t.Setenv("CENTREON_PASSWORD_FILE", "")
			t.Setenv("CENTREON_TOKEN_FILE", "")

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
	t.Setenv("CENTREON_PASSWORD_FILE", "")
	t.Setenv("CENTREON_TOKEN_FILE", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.Token != "my-api-token" {
		t.Errorf("expected Token my-api-token, got %q", cfg.Token)
	}
}

type secretFileCase struct {
	name         string
	fileContents string
	passwordVar  string
	tokenVar     string
	usePassFile  bool
	useTokenFile bool
	wantPassword string
	wantToken    string
}

func runSecretFileTest(t *testing.T, tt *secretFileCase) {
	t.Helper()
	t.Setenv("CENTREON_HOST", "https://centreon.example.com")
	t.Setenv("CENTREON_USERNAME", "admin")
	t.Setenv("CENTREON_PASSWORD", tt.passwordVar)
	t.Setenv("CENTREON_TOKEN", tt.tokenVar)
	t.Setenv("CENTREON_PASSWORD_FILE", "")
	t.Setenv("CENTREON_TOKEN_FILE", "")

	if tt.useTokenFile {
		t.Setenv("CENTREON_USERNAME", "")
		t.Setenv("CENTREON_PASSWORD", "")
		t.Setenv("CENTREON_TOKEN", "")
	}

	if tt.usePassFile {
		path := filepath.Join(t.TempDir(), "passwd")
		if err := os.WriteFile(path, []byte(tt.fileContents), 0o600); err != nil {
			t.Fatalf("write temp secret: %v", err)
		}
		t.Setenv("CENTREON_PASSWORD_FILE", path)
	}
	if tt.useTokenFile {
		path := filepath.Join(t.TempDir(), "token")
		if err := os.WriteFile(path, []byte(tt.fileContents), 0o600); err != nil {
			t.Fatalf("write temp secret: %v", err)
		}
		t.Setenv("CENTREON_TOKEN_FILE", path)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if tt.wantPassword != "" && cfg.Password != tt.wantPassword {
		t.Errorf("expected Password %q, got %q", tt.wantPassword, cfg.Password)
	}
	if tt.wantToken != "" && cfg.Token != tt.wantToken {
		t.Errorf("expected Token %q, got %q", tt.wantToken, cfg.Token)
	}
}

func TestLoadConfig_SecretFile(t *testing.T) {
	tests := []secretFileCase{
		{
			name:         "password from file, inline unset",
			fileContents: "filepw\n",
			usePassFile:  true,
			wantPassword: "filepw",
		},
		{
			name:         "file precedence over inline",
			fileContents: "filepw",
			passwordVar:  "inlinepw",
			usePassFile:  true,
			wantPassword: "filepw",
		},
		{
			name:         "token from file satisfies requirement",
			fileContents: "filetok\n",
			useTokenFile: true,
			wantToken:    "filetok",
		},
		{
			name:         "trailing LF trimmed",
			fileContents: "tok\n",
			useTokenFile: true,
			wantToken:    "tok",
		},
		{
			name:         "trailing CRLF trimmed",
			fileContents: "tok\r\n",
			useTokenFile: true,
			wantToken:    "tok",
		},
		{
			name:         "only one newline trimmed",
			fileContents: "tok\n\n",
			useTokenFile: true,
			wantToken:    "tok\n",
		},
		{
			name:         "trailing space preserved",
			fileContents: "tok \n",
			useTokenFile: true,
			wantToken:    "tok ",
		},
		{
			name:         "bare trailing CR preserved",
			fileContents: "tok\r",
			useTokenFile: true,
			wantToken:    "tok\r",
		},
		{
			name:         "unset _FILE leaves inline behavior",
			passwordVar:  "inlinepw",
			wantPassword: "inlinepw",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runSecretFileTest(t, &tt)
		})
	}
}

func testSecretFileMissing(t *testing.T) {
	t.Setenv("CENTREON_HOST", "https://centreon.example.com")
	t.Setenv("CENTREON_USERNAME", "admin")
	t.Setenv("CENTREON_PASSWORD", "")
	t.Setenv("CENTREON_TOKEN", "")
	t.Setenv("CENTREON_TOKEN_FILE", "")
	absentPath := filepath.Join(t.TempDir(), "absent")
	t.Setenv("CENTREON_PASSWORD_FILE", absentPath)

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for missing secret file, got nil")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "CENTREON_PASSWORD_FILE") {
		t.Errorf("expected error to name variable CENTREON_PASSWORD_FILE, got: %v", err)
	}
	if !strings.Contains(errStr, absentPath) {
		t.Errorf("expected error to contain path %q, got: %v", absentPath, err)
	}
	if !strings.Contains(errStr, "cannot read secret file") {
		t.Errorf("expected error to contain 'cannot read secret file', got: %v", err)
	}
}

func testSecretFileEmpty(t *testing.T) {
	t.Setenv("CENTREON_HOST", "https://centreon.example.com")
	t.Setenv("CENTREON_USERNAME", "")
	t.Setenv("CENTREON_PASSWORD", "")
	t.Setenv("CENTREON_TOKEN", "")
	t.Setenv("CENTREON_PASSWORD_FILE", "")
	emptyFile := filepath.Join(t.TempDir(), "empty_token")
	if err := os.WriteFile(emptyFile, []byte("\n"), 0o600); err != nil {
		t.Fatalf("write empty temp secret: %v", err)
	}
	t.Setenv("CENTREON_TOKEN_FILE", emptyFile)

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for empty secret file, got nil")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "CENTREON_TOKEN_FILE") {
		t.Errorf("expected error to name variable CENTREON_TOKEN_FILE, got: %v", err)
	}
	if !strings.Contains(errStr, emptyFile) {
		t.Errorf("expected error to contain path %q, got: %v", emptyFile, err)
	}
	if !strings.Contains(errStr, "is empty") {
		t.Errorf("expected error to contain 'is empty', got: %v", err)
	}
}

func testSecretFileNeverLeaks(t *testing.T) {
	const leakSecret = "leakmarker-secret"
	t.Setenv("CENTREON_HOST", "https://centreon.example.com")
	t.Setenv("CENTREON_USERNAME", "admin")
	t.Setenv("CENTREON_PASSWORD", "")
	t.Setenv("CENTREON_TOKEN", "")
	t.Setenv("CENTREON_TOKEN_FILE", "")
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte(leakSecret+"\n"), 0o600); err != nil {
		t.Fatalf("write temp secret: %v", err)
	}
	t.Setenv("CENTREON_PASSWORD_FILE", path)
	t.Setenv("MCP_HTTP_PORT", "invalid-port")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected port parsing error, got nil")
	}
	if strings.Contains(err.Error(), leakSecret) {
		t.Errorf("error leaked secret file contents: %v", err)
	}
}

func TestLoadConfig_SecretFileErrors(t *testing.T) {
	t.Run("missing file", testSecretFileMissing)
	t.Run("empty file", testSecretFileEmpty)
	t.Run("secret never leaks", testSecretFileNeverLeaks)
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

// TestValidateHostScheme pins the cleartext-credential guard (CWE-319) and the
// hostname requirement: https is accepted when it names a host, http only with the
// opt-in, and a missing or unsupported scheme, an unparseable URL, or a URL naming
// no hostname is rejected outright.
func TestValidateHostScheme(t *testing.T) {
	t.Parallel()
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
		// url.Error.Error() reprints the URL it failed on, so wrapping it whole would
		// undo the safeHost redaction in the very same message. These inputs fail
		// url.Parse (an unescaped character in the password), so they exercise the
		// parse-error branch, and notWant pins that the password stays out of it.
		{"parse error keeps the password out of the message", "https://admin:se^kret@centreon.example.com", false, "invalid host url", "se^kret"},
		{"parse error keeps a spaced password out of the message", "https://admin:se kret@centreon.example.com", false, "invalid host url", "se kret"},
		// A raw '@' after the authority is NOT rejected. It is indistinguishable
		// from a mis-encoded credential, so safeHost masks it, but rejecting the
		// host would refuse legitimate base URLs carrying an '@' in a path or
		// query. Redaction is the control here, not validation (issue #55).
		{"accepts a credential-free host with @ in the path", "https://centreon.example.com:8080/a@b", false, "", ""},
		{"accepts an @ in the path with no colon", "https://centreon.example.com/api@v1", false, "", ""},
		{"accepts a mis-encoded credential, redaction is the control", "https://admin:1234/secret@centreon.example.com", false, "", ""},
		// A URL naming no hostname is refused here rather than left to the client
		// (issue #58). For the empty-Host shapes centreon.NewClient rejects it itself,
		// with an error that formats the RAW base URL, and that error is logged, so a
		// credential-bearing form would otherwise reach the log in clear (CWE-532).
		// The port-only shapes are different: the client accepts them, so they are
		// rejected as a configuration error, not a leak. notWant pins that the
		// rejection message here stays redacted.
		{"scheme-only rejected", "https:", false, "must include a hostname", ""},
		// With the opt-in set too, so the cleartext flag cannot rescue a hostless https URL.
		{"empty authority rejected even with the http opt-in", "https://", true, "must include a hostname", ""},
		{"opaque form rejected", "https:centreon.example.com", false, "must include a hostname", ""},
		{"port-only authority rejected", "https://:9443", false, "must include a hostname", ""},
		{"userinfo-only authority rejected with password redacted", "https://admin:sekrit58@", false, "must include a hostname", "sekrit58"},
		{"userinfo with port-only authority rejected with password redacted", "https://admin:sekrit58@:9443", false, "must include a hostname", "sekrit58"},
		// The hostname check runs BEFORE the cleartext-http check inside the shared
		// http(s) arm, so a hostless http URL is reported as hostless whatever
		// CENTREON_ALLOW_HTTP says. Without these rows the two guards never co-occur,
		// and swapping their order leaves the suite green.
		{"hostless http rejected on the hostname, not the scheme", "http://", false, "must include a hostname", ""},
		{"hostless http rejected as hostless even with the opt-in", "http://:8080", true, "must include a hostname", ""},
		{"hostless http keeps the password out of the message", "http://admin:sekrit58@", false, "must include a hostname", "sekrit58"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
// logs or error messages, CWE-532), including the scheme-less user:pass@host form
// url.Redacted alone does not mask (issue #41) and the forms url.Parse silently
// mis-reads as host, port, path, query or fragment (issue #55). A host whose '@'
// cannot carry a password, and a bracketed IPv6 authority with no password span in
// its tail, are still returned byte for byte unchanged. These rows are examples;
// TestSafeHostNeverLeaksPassword sweeps the class.
func TestSafeHost(t *testing.T) {
	t.Parallel()
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
		// A '@' surviving after the authority is ambiguous: url.Parse's benign reading
		// (host centreon.example.com, path /p@th) is byte-for-byte the credential
		// reading (password sekret@centreon.example.com/p, host th), so safeHost fails
		// closed and masks to the last '@' rather than leak a password of that shape
		// (issue #62). The pre-colon text still greps. Same fail-closed rule the
		// ambiguous colon-and-@ rows below already apply.
		{"fails closed when a @ survives after the authority", "https://admin:sekret@centreon.example.com/p@th", "https://admin:xxxxx@th"},
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
		// url.Parse can mis-read intended userinfo without failing: an all-numeric
		// password prefix parses as a port, so admin:1234 becomes host:port and the
		// password tail lands in the path, query or fragment (issue #55).
		{"masks numeric-prefix password with a slash", "https://admin:1234/secret@centreon.example.com", "https://admin:xxxxx@centreon.example.com"},
		{"masks numeric-prefix password with a question mark", "https://admin:1234?secret@centreon.example.com", "https://admin:xxxxx@centreon.example.com"},
		{"masks numeric-prefix password with a hash", "https://admin:1234#secret@centreon.example.com", "https://admin:xxxxx@centreon.example.com"},
		{"masks numeric-prefix password ahead of a real host:port", "https://admin:1234/secret@centreon.example.com:8443/mon", "https://admin:xxxxx@centreon.example.com:8443/mon"},
		{"masks protocol-relative numeric-prefix password", "//admin:1234/secret@centreon.example.com", "//admin:xxxxx@centreon.example.com"},
		// The same mis-read happens from the username side: an unencoded '/', '?' or
		// '#' in the username makes url.Parse take the leading span as the host and
		// leave the whole credential in the path, query or fragment (issue #55).
		{"masks password behind a username with a slash", "https://us/er:secret@centreon.example.com", "https://us/er:xxxxx@centreon.example.com"},
		{"masks password behind a username with a question mark", "https://us?er:secret@centreon.example.com", "https://us?er:xxxxx@centreon.example.com"},
		{"masks password behind a username with a hash", "https://us#er:secret@centreon.example.com", "https://us#er:xxxxx@centreon.example.com"},
		// A ':' before a later raw '@' always admits a credential reading, so it is
		// masked whether or not url.Parse decoded the URL (issue #55).
		{"masks ambiguous colon and @ after a portless authority", "https://centreon.example.com/a:b@c", "https://centreon.example.com/a:xxxxx@c"},
		{"masks ambiguous colon and @ in an unparseable URL", "https://host/%zz/a:b@c", "https://host/%zz/a:xxxxx@c"},
		// A bracketed authority is a genuine IPv6 literal: '[' is an RFC 3986
		// gen-delim and invalid in userinfo, so it cannot be a mis-read credential.
		// A username that merely starts with '[' is not a bracketed authority.
		{"leaves IPv6 host with port and @ in path unchanged", "https://[2001:db8::1]:8443/x@y", "https://[2001:db8::1]:8443/x@y"},
		{"masks username starting with a bracket", "https://[user:secret@centreon.example.com", "https://[user:xxxxx@centreon.example.com"},
		// An email-style username makes url.Parse split the authority at the wrong
		// '@', so it reports a password-less userinfo and url.Redacted has nothing to
		// mask while the real password sits in the path, query or fragment (#55).
		{"masks password behind an email-style username", "https://user@corp.com:1234/secret@centreon.example.com", "https://user@corp.com:xxxxx@centreon.example.com"},
		{"masks password behind an email-style username in a query", "https://a@b:1234?secret@centreon.example.com", "https://a@b:xxxxx@centreon.example.com"},
		{"masks password behind an email-style username in a fragment", "https://a@b:1234#secret@centreon.example.com", "https://a@b:xxxxx@centreon.example.com"},
		// The opaque scheme:host form has no "//" authority marker, so the reparse
		// fallback would invent one out of "https:user" and mask the wrong span.
		{"masks opaque form with an email-style username", "https:user@corp.com:1234/secret@centreon.example.com", "https:xxxxx@centreon.example.com"},
		// A bracketed IPv6 authority cannot be userinfo, but its tail still can, so
		// the bracket exemption must not cover a password span after the authority.
		// The address' own colons must not be taken for the password delimiter
		// either, or the host collapses to "[" and the log line loses its target.
		{"masks a password span after a bracketed IPv6 authority", "https://[::1]/admin:secret@evil.example.com", "https://[::1]/admin:xxxxx@evil.example.com"},
		{"keeps a bracketed IPv6 host and port while masking the tail", "https://[2001:db8::1]:8443?owner:secret@evil.example.com", "https://[2001:db8::1]:xxxxx@evil.example.com"},
		{"keeps an IPv6 zone ID while masking the tail", "https://[fe80::1%25eth0]:8443/mon?u=a:secret@c", "https://[fe80::1%25eth0]:xxxxx@c"},
		// A bracket-prefixed username is not an IPv6 literal, so its first ':' is
		// still the delimiter.
		{"masks a bracket-prefixed username that is not an IP", "https://[notanip:secret]x@centreon.example.com", "https://[notanip:xxxxx@centreon.example.com"},
		// A real parsed password plus a second password span in the tail: Redacted
		// masks only the first, so this must fall through to the textual masker.
		{"masks a password span in the tail beside real userinfo", "https://admin:pw@centreon.example.com/x:secret@evil.example.com", "https://admin:xxxxx@evil.example.com"},
		// Issue #62, family 1: a password holding a raw '@' before a '/', '?' or '#'
		// makes url.Parse decode only a fragment as userinfo and read the rest as
		// host/path, so url.Redacted masks the fragment and leaves the rest verbatim.
		// safeHost must fall through to the textual masker, which masks to the last '@'.
		{"masks a raw @ password before a slash", "https://admin:se@cret/path@centreon.example.com", "https://admin:xxxxx@centreon.example.com"},
		{"masks a raw @ password before a slash, second host", "https://user:pass1@pass2/word@centreon.example.com", "https://user:xxxxx@centreon.example.com"},
		{"masks a raw @ password with a long tail", "https://admin:p@ssw0rdLONGSECRET/x@centreon.example.com", "https://admin:xxxxx@centreon.example.com"},
		// Issue #62, family 2: the delimiter can arrive percent-encoded, so there is no
		// raw '@' for the unchanged-return or the masker to key on; both decode it. The
		// encoded delimiter is kept verbatim in the output so the host still greps.
		{"masks a %40 delimiter after a slash", "https://admin:1234/SECRETpassword%40centreon.example.com", "https://admin:xxxxx%40centreon.example.com"},
		{"masks a %40 delimiter after a question mark", "https://admin:1234?SECRETpassword%40centreon.example.com", "https://admin:xxxxx%40centreon.example.com"},
		// A raw '@' in the password ahead of an encoded delimiter: the masker takes the
		// LATER encoded '@' as the delimiter, so the whole span collapses to xxxxx.
		{"masks a raw @ password ahead of an encoded delimiter", "https://admin:p@ssword%40centreon.example.com", "https://admin:xxxxx%40centreon.example.com"},
		// Encoded twice, so a "%40" substring test would miss it; peeled up to 4 layers.
		{"masks a double-encoded delimiter", "https://admin:1234/secret%2540centreon.example.com", "https://admin:xxxxx%2540centreon.example.com"},
		{"masks the scheme-less encoded-delimiter form", "admin:secret%40centreon.example.com", "admin:xxxxx%40centreon.example.com"},
		// Fail closed on an encoded delimiter behind a real port (the issue #61 trade:
		// the port is masked, the pre-colon host still greps), the encoded analog of
		// the ambiguous host:port-with-@ rows in this table.
		{"fails closed on an encoded delimiter behind a port", "https://centreon.example.com:8443/report%40q", "https://centreon.example.com:xxxxx%40q"},
		// Benign escapes with no password span before them must be left intact.
		{"leaves an encoded @ with no colon before it unchanged", "https://centreon.example.com/report%40q", "https://centreon.example.com/report%40q"},
		{"leaves an ordinary encoded space unchanged", "https://centreon.example.com/mon%20test", "https://centreon.example.com/mon%20test"},
		{"leaves a non-delimiter escape unchanged", "https://centreon.example.com/pw%4chost", "https://centreon.example.com/pw%4chost"},
		// A bracketed IPv6 authority is exempt, but an encoded password span in its
		// tail is not; a colon-free encoded '@' in the tail still is exempt.
		{"masks an encoded password span after a bracketed IPv6 authority", "https://[::1]/admin:secret%40evil.example.com", "https://[::1]/admin:xxxxx%40evil.example.com"},
		{"leaves an IPv6 host with an encoded @ and no colon in the tail unchanged", "https://[::1]/x%40y", "https://[::1]/x%40y"},
		// url.Parse rejects a '%40' inside an IPv6 zone (only '%25' is a valid zone
		// escape), so safeHost reaches the textual masker. The '%40' must NOT be taken
		// for a userinfo delimiter that masks from inside the address: the bracketed
		// span is a valid IPv6 literal, so there is no credential and the host is left
		// intact (issue #62; without the guard this masked to "[fe80:xxxxx%40en0]").
		{"leaves an IPv6 host with a %40 zone unchanged", "https://[fe80::1%40en0]:8443", "https://[fe80::1%40en0]:8443"},
		{"leaves an IPv6 host with a %40 zone and path unchanged", "https://[2001:db8::1%40x]/mon", "https://[2001:db8::1%40x]/mon"},
		// The unclosed-bracket exemption must not fail open: a valid IP followed by a
		// '%' and then a ':password' is NOT a zoned host, it hides a credential. Only
		// the whole post-'[' span parsing as an IP earns the exemption, so this masks
		// (masking a mis-parsed bracketed authority is the issue #61 trade, never a leak).
		{"masks a password after a fake zone in an unclosed IPv6 bracket", "https://[192.168.0.1%x:secret@host", "https://[192.168.0.1%x:xxxxx@host"},
		// The delimiter can be nested arbitrarily deep; the masker peels every layer,
		// so a bound cannot be outrun to leave the password verbatim (5 layers here).
		{"masks a five-layer encoded delimiter", "https://admin:secret%2525252540centreon.example.com", "https://admin:xxxxx%2525252540centreon.example.com"},
		{"masks a raw @ password ahead of a five-layer encoded delimiter", "https://admin:p@ssword%2525252540centreon.example.com", "https://admin:xxxxx%2525252540centreon.example.com"},
		// Issue #68: the delimiter's own hex digits can be encoded, so "@" can be
		// written "%25%34%30" ("%25"->'%', "%34"->'4', "%30"->'0'). lastAtDelimiter's
		// flat %(25)*40 scan cannot see that shape. Locating it exactly would mean
		// mapping an offset back through every decode round, so safeHost fails closed
		// and masks from the first userinfo colon instead, losing the host. That is the
		// issue #61 trade already made by the "behind a port" row above, and no
		// operator writes this form: validateHostScheme rejects it outright, so it is
		// only reachable through the caller-supplied X-Centreon-Host header.
		{"fails closed on a delimiter whose hex digits are encoded", "https://admin:secret%25%34%30centreon.example.com", "https://admin:xxxxx"},
		{"fails closed on a raw @ password ahead of a digit-encoded delimiter", "https://admin:p@ssword%25%34%30centreon.example.com", "https://admin:xxxxx"},
		{"fails closed on a partially digit-encoded delimiter", "https://admin:secret%254%30centreon.example.com", "https://admin:xxxxx"},
		// The decode is lenient: an unrelated malformed escape later in the string must
		// not stop the delimiter being found. url.PathUnescape cannot be used here
		// because it rejects the whole string on the first bad escape.
		{"fails closed on a digit-encoded delimiter beside a malformed escape", "https://admin:secret%25%34%30centreon.example.com/%zz", "https://admin:xxxxx"},
		// Nested past the decode bound: decoding is still making progress when the
		// rounds run out, so a delimiter could be hiding deeper and safeHost fails
		// closed rather than guess.
		{"fails closed on a delimiter nested past the decode bound", "https://admin:secret%25252525%25252534%25252530centreon.example.com", "https://admin:xxxxx"},
		// The mirror image: an escape run that decodes to "40" and never to '@' is not
		// a delimiter, so the input is left alone. Without this the decoder could be
		// made to treat any digit-bearing escape run as a credential delimiter.
		{"leaves a lookalike encoded run that decodes to 40 unchanged", "https://host/%zz/pw%2534%30q", "https://host/%zz/pw%2534%30q"},
		// Issue #75: the userinfo COLON can be percent-encoded too. Go splits userinfo
		// on a RAW ':' only, so "admin%3ASEKRIT@host" parses as a bare username with no
		// password and url.Redacted masks nothing. The username is dropped along with
		// the host here because the colon's position is only known after decoding, and
		// safeHost masks rather than maps the offset back.
		{"fails closed on an encoded userinfo colon with a raw delimiter", "https://admin%3Asekret@centreon.example.com", "https://xxxxx"},
		{"fails closed on an encoded userinfo colon with an encoded delimiter", "https://admin%3Asekret%40centreon.example.com", "https://xxxxx"},
		{"fails closed on a lowercase encoded userinfo colon", "https://admin%3asekret@centreon.example.com", "https://xxxxx"},
		{"fails closed on the scheme-less encoded-colon form", "admin%3Asekret@centreon.example.com", "xxxxx"},
		// The same shapes behind an explicit port and behind a bracketed IPv6 literal.
		// These are the mainstream deployment, not a corner case, and the first fix for
		// #75 missed them: maskAmbiguousAuthority located the username boundary with
		// userinfoColon over the WHOLE post-marker span, and on this path the userinfo
		// carries no raw ':' by construction, so the first raw colon it found was the
		// PORT colon. It masked the port and returned the credential verbatim. Every
		// row above uses a portless host, which is exactly why the gap survived.
		{"fails closed on an encoded userinfo colon behind a port", "https://admin%3Asekret@centreon.example.com:8443", "https://xxxxx"},
		{"fails closed on an encoded userinfo colon behind a port with an encoded delimiter", "https://admin%3Asekret%40centreon.example.com:8443", "https://xxxxx"},
		{"fails closed on the scheme-less encoded-colon form behind a port", "admin%3Asekret@centreon.example.com:8443", "xxxxx"},
		{"fails closed on an encoded userinfo colon before a bracketed IPv6 host", "https://uzer%3Asekret@[::1]", "https://xxxxx"},
		{"fails closed on an encoded userinfo colon before a bracketed IPv6 host with a port", "https://uzer%3Asekret@[2001:db8::1]:8443", "https://xxxxx"},
		// A raw ':' inside a bracketed literal must not disarm the encoded-separator
		// check. userinfoColon deliberately reports no separator for a bracketed
		// reading, so gating that check on "the userinfo holds no raw ':'" exempted
		// exactly the inputs whose colons belong to an address, and they leaked.
		{"fails closed on an encoded colon after a bracketed literal", "https://[::1]%3Asekret@centreon.example.com", "https://xxxxx"},
		{"fails closed on an encoded colon after a bracketed link-local literal", "https://[fe80::1]%3Asekret@centreon.example.com", "https://xxxxx"},
		// The bracketed-authority carve-out reaches its verdict through
		// tailCarriesPassword, which searched the tail for a RAW ':' and could not see
		// a delimiter whose digits were encoded. Both spellings must defeat it.
		{"masks an encoded colon in the tail of a bracketed authority", "https://[::1]/admin%3Asekret@centreon.example.com", "https://xxxxx"},
		{"masks a digit-encoded delimiter in the tail of a bracketed authority", "https://[::1]/p:sekret%25%34%30centreon.example.com", "https://[::1]/p:xxxxx"},
		// A delimiter without a separator is not a credential. These carry a '@' that
		// only decoding reveals, or exhaust the decode bound outright, and hold no
		// secret at all; masking them would drop a hostname to hide nothing.
		{"leaves a digit-encoded @ with no separator unchanged", "https://example.com/path/%25%34%30", "https://example.com/path/%25%34%30"},
		{"leaves a path nested past the decode bound unchanged", "https://centreon.example.com/100%25252525", "https://centreon.example.com/100%25252525"},
		// The accepted cost of that bound. Nesting past maxDecodeRounds is reported as
		// uncertainty, and combined with a ':' anywhere ahead of it the input is
		// indistinguishable from a credential whose delimiter is hiding deeper, so it
		// is masked. No secret is exposed; the operator loses the tail. This row exists
		// to make the trade deliberate rather than incidental.
		{"masks a credential-free URL whose only colon precedes an over-nested escape", "https://example.com/p:%25252525", "https://example.com/p:xxxxx"},
		// And the reason the separator search cannot simply be narrowed to the
		// authority to avoid that cost. Here the ':' is ALSO only in the path, the
		// delimiter is again reachable only by decoding past the bound, and the input
		// IS a credential: url.Parse mis-reads "user:pass" spanning a '/' (issue #55),
		// which is why the search spans the whole post-marker string. Restricting it to
		// the authority would return this verbatim and leak the password.
		{"masks a mis-parsed credential whose colon is only in the path", "https://x/admin:pw%25252525%25252534%25252530host", "https://x/admin:xxxxx"},
		// An encoded separator in the USERNAME moves the real boundary earlier than the
		// raw one Go split on, so url.Redacted masks only the fragment after the raw
		// ':' and echoes everything before it. redactedCoversCredential returned true
		// on hasPassword before consulting the encoded-separator guard at all, which
		// made this the last reachable shape of the family: validateHostScheme accepts
		// it, so it runs normally and reaches a log on every line.
		{"fails closed on an encoded separator ahead of a raw one", "https://admin%3Asekret:x@centreon.example.com", "https://xxxxx"},
		{"fails closed on an encoded separator ahead of a raw one behind a port", "https://admin%3Asekret:x@centreon.example.com:8443", "https://xxxxx"},
		// Pins the region the encoded-separator search covers. Widening it to the whole
		// userinfo makes a bracketed literal's own colons read as a separator and masks
		// these to "https://xxxxx", which is why the search starts past the bracket.
		// Four pre-existing bracketed rows also fail under that widening, so these are
		// the direct statement of the invariant rather than its only guard.
		{"leaves a bracketed IPv6 userinfo with no separator unchanged", "https://[::1]@centreon.example.com", "https://[::1]@centreon.example.com"},
		{"leaves a longer bracketed IPv6 userinfo with no separator unchanged", "https://[2001:db8::1]@centreon.example.com", "https://[2001:db8::1]@centreon.example.com"},
		// Pins the delimiter half of maskAmbiguousAuthority's guard, which the rest of
		// the suite leaves deletable: dropping it keeps the username span whenever a
		// credential delimiter precedes the colon, which is where the port colon and
		// the real credential both live.
		{"fails closed on an encoded separator inside a bracketed span before a port", "https://[::1%3Asekret]@centreon.example.com:8443/x%25%34%30", "https://xxxxx"},
		// Control isolating the cause to the colon: encoding a character in the
		// USERNAME changes nothing, because the separator is still raw.
		{"masks normally when only a username character is encoded", "https://ad%6Din:sekret@centreon.example.com", "https://admin:xxxxx@centreon.example.com"},
		// A userinfo with no colon in any encoding has no password span, so it is left
		// alone. This is the row that stops the encoded-colon guard from turning every
		// bare username into a fail-closed mask.
		{"leaves a username-only userinfo unchanged", "https://user@centreon.example.com", "https://user@centreon.example.com"},
		// url.Redacted canonicalises the userinfo, so the escape is decoded here. That
		// is normalisation of a username, not a leak: no colon means no password span.
		{"leaves a username-only userinfo with an encoded character intact", "https://us%65r@centreon.example.com", "https://user@centreon.example.com"},
		// Issue #59 item 5: pins lastAtDelimiter's last-'@'-wins rule through the one
		// path that can never migrate to url.Redacted or the reparse. The "%zz" forces
		// url.Parse to fail and the second '@' makes safeHost skip the reparse, so only
		// the textual masker can produce this output. Swapping LastIndexByte for
		// IndexByte in lastAtDelimiter yields ".../a:xxxxx@c@d" instead.
		{"masks to the last raw @ in an unparseable URL", "https://host/%zz/a:b@c@d", "https://host/%zz/a:xxxxx@d"},
		// Issue #59 item 6: pins the scheme-less reparse block. Deleting it drops
		// safeHost to the textual masker, which cannot canonicalise the host, so the
		// output keeps the raw "é" instead of percent-encoding it.
		{"masks scheme-less userinfo with a non-ASCII host via the reparse", "admin:pw@centréon.example.com", "admin:xxxxx@centr%C3%A9on.example.com"},
		// The issue #61 log-integrity trade extended to an encoded delimiter: once a
		// real userinfo is decoded but a %40 survives in the tail, the input is the
		// same ambiguous shape as a password whose own delimiter is encoded (compare
		// "admin:x@SEKRIT/p%40host", where SEKRIT is password material). safeHost
		// cannot tell them apart, so it fails closed and masks to the encoded '@',
		// losing the host rather than risk leaking a password. A '%40' in the path or
		// query of a Centreon base URL does not occur in practice, so this costs a log
		// only on inputs that do not arise; it never leaks.
		{"fails closed on an encoded @ in the path beside real userinfo", "https://admin:secret@centreon.example.com/api%40v1", "https://admin:xxxxx%40v1"},
		{"fails closed on an encoded @ in the query beside real userinfo", "https://admin:secret@centreon.example.com?q=1%402", "https://admin:xxxxx%402"},
		// A "//" inside the password used to pose as the authority marker, so the
		// textual masker treated the real password as the authority and returned the
		// host unmasked. Only a leading "//" or one behind a valid scheme counts now.
		{"masks a scheme-less password containing //", "admin:pw//secret@centreon.example.com", "admin:xxxxx@centreon.example.com"},
		{"masks a scheme-less password containing ://", "admin:pw://secret@centreon.example.com", "admin:xxxxx@centreon.example.com"},
		{"masks a base64-style password containing //", "admin:aB9//xY2z@centreon.example.com", "admin:xxxxx@centreon.example.com"},
		// Credential-free hosts must not be corrupted where that is decidable: a
		// later '@' with no earlier ':' has no password span, and a bracketed IPv6
		// authority can never be userinfo. A ':' followed by '@' after the authority
		// is genuinely ambiguous and fails closed instead (issue #55).
		//
		// These two rows changed with #55. "https://centreon.example.com:8080/a@b" is
		// byte-for-byte indistinguishable from user "centreon.example.com", password
		// "8080/a", host "b", so redaction fails closed; the text before the first
		// ':' survives, so hostname greps still match.
		{"masks ambiguous host:port with @ in path", "https://centreon.example.com:8080/a@b", "https://centreon.example.com:xxxxx@b"},
		{"masks ambiguous : and @ in query", "https://centreon.example.com:8443/x?a=b:c@d", "https://centreon.example.com:xxxxx@d"},
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
			t.Parallel()
			if got := safeHost(tt.host); got != tt.want {
				t.Errorf("safeHost(%q) = %q, want %q", tt.host, got, tt.want)
			}
		})
	}
}

// TestSafeHostNeverLeaksPassword sweeps a constructed grid of credential-shaped
// hosts and asserts the invariant the row table can only sample: a marker placed in
// password position never survives safeHost. Both issues in this area (#41 and #55)
// turned up shapes the hand-written rows missed, so the class is pinned by varying
// every placement that moves the authority split rather than by enumeration. A
// constructed grid is used rather than go test -fuzz because it is deterministic,
// needs no corpus in the repo, and runs in ordinary CI; a fuzz target could build
// the same oracle from separate username and password arguments, so the oracle is
// not the reason.
func TestSafeHostNeverLeaksPassword(t *testing.T) {
	t.Parallel()
	// Each family is asserted to contribute rows BEFORE being swept. Checking only
	// the first builder's length, as this test used to, left every later family free
	// to contribute nothing while the absence assertion still reported success: a
	// vacuous pass is exactly the failure mode a pure absence assertion cannot see,
	// and appending to an already-large slice hides it.
	families := []struct {
		name string
		rows []string
	}{
		// The raw '@' credential form, and the percent-encoded delimiter of issue #62
		// once and twice encoded. The encoded ones carry no raw '@' at all, so before
		// that fix safeHost either returned them unchanged or masked to a non-existent
		// raw '@'. The display test deliberately does not get the encoded joins.
		{"raw @ delimiter", credentialHostGrid()},
		{"%40 delimiter", credentialHostGridJoined("%40")},
		{"%2540 delimiter", credentialHostGridJoined("%2540")},
		// Issue #68: the delimiter's hex digits themselves encoded, whole and partial.
		// Neither matches the flat %(25)*40 shape, so the masker found no delimiter.
		{"%25%34%30 delimiter", credentialHostGridJoined("%25%34%30")},
		{"%254%30 delimiter", credentialHostGridJoined("%254%30")},
		// Issue #75: the userinfo SEPARATOR encoded. Go splits userinfo on a raw ':'
		// only, so these parse as a bare username with no password and url.Redacted
		// masks nothing. Swept with both delimiter spellings and with the separator
		// nested twice, because the guards for this family are spelled three different
		// ways across maskAuthorityPassword, redactedCoversCredential and
		// reparsedAuthority. credentialGridHosts supplies the ports and bracketed IPv6
		// literals that the hand-written rows for this family originally all missed.
		{"%3A separator", credentialHostGridSeparated("%3A", "@")},
		{"%3A separator, %40 delimiter", credentialHostGridSeparated("%3A", "%40")},
		{"%253A separator", credentialHostGridSeparated("%253A", "@")},
	}
	var grid []string
	for _, f := range families {
		if len(f.rows) < 1000 {
			t.Fatalf("family %q contributed %d hosts, expected a full grid", f.name, len(f.rows))
		}
		grid = append(grid, f.rows...)
	}

	fragmentRows := 0
	failures := 0
	for _, host := range grid {
		// Positive control: the input must actually carry the marker, otherwise
		// asserting its absence from the output proves nothing.
		if !strings.Contains(host, passwordMarker) {
			t.Fatalf("grid host %q does not contain the marker, the builder is wrong", host)
		}
		if strings.Contains(host, passwordFragment) {
			fragmentRows++
		}
		if got := safeHost(host); strings.Contains(got, passwordMarker) || strings.Contains(got, passwordFragment) {
			failures++
			// Cap the output: a regression here leaks thousands of rows at once.
			if failures <= 20 {
				t.Errorf("safeHost(%q) = %q, which leaks the password", host, got)
			}
		}
	}
	if failures > 20 {
		t.Errorf("safeHost leaked the password in %d of %d grid hosts", failures, len(grid))
	}
	// Positive control for the fragment half of the assertion above. Without it the
	// fragment check passes vacuously if the marker-before-delimiter passwords are
	// ever dropped from the builder, which is exactly the blindspot issue #65 item 8
	// records: every original grid password put the marker last, so an output that
	// echoed the span BEFORE the marker satisfied a marker-only assertion.
	if fragmentRows < 1000 {
		t.Fatalf("only %d grid hosts carry the fragment, the builder lost its marker-before-delimiter passwords", fragmentRows)
	}
}

// namesAnIntendedHost reports whether out is exactly a scheme plus one of the
// authorities credentialHostGrid builds its inputs around. It reads
// credentialGridHosts, the same list the grid is built from, so the two cannot drift
// apart, and it requires the scheme so that a bare "://host" does not pass.
func namesAnIntendedHost(out string) bool {
	for _, host := range credentialGridHosts {
		for _, scheme := range []string{"https", "http"} {
			if out == scheme+"://"+host {
				return true
			}
		}
	}
	return false
}

// TestDisplayHostNamesOnlyARealHost drives the same credential grid through
// displayHost and asserts the invariant issue #48 actually needs: whatever comes back
// either is the placeholder or names one of the hosts the grid was built from, never a
// span of the credential.
//
// It is a stronger assertion than TestSafeHostNeverLeaksPassword's marker check, which
// cannot see a leak of PART of a password: a displayHost with no fail-closed guard
// echoes many grid rows as an output that names no real host (a username or a password
// prefix url.Parse mis-read as the host), and many of those carry no passwordMarker, so
// a marker-only assertion would have stayed green through them. What it does not do is
// reproduce the specific leaks the
// hand-written TestDisplayHost rows were written from; those rows are the
// reproduction, and this is the guard against the next shape.
func TestDisplayHostNamesOnlyARealHost(t *testing.T) {
	t.Parallel()
	// Only the raw-'@' grid. The "%40"/"%2540" joins the safeHost sweep adds are
	// covered by TestDisplayHostEncodedJoinsNeverEchoACredential: since the issue #70
	// fix they fail closed on every row, so they cannot contribute to the echoed>0
	// availability control this test keeps.
	grid := credentialHostGrid()
	if len(grid) < 1000 {
		t.Fatalf("credentialHostGrid() returned %d hosts, expected a full grid", len(grid))
	}

	failures, echoed := 0, 0
	for _, host := range grid {
		got := displayHost(host)
		if got == redactedHostPlaceholder {
			continue
		}
		echoed++
		if strings.Contains(got, passwordMarker) || !namesAnIntendedHost(got) {
			failures++
			if failures <= 20 {
				t.Errorf("displayHost(%q) = %q, which names something other than the host", host, got)
			}
		}
	}
	if failures > 20 {
		t.Errorf("displayHost named a non-host in %d of %d grid hosts", failures, len(grid))
	}
	// Positive control. Every row failing closed would satisfy the assertion above
	// vacuously, so require that the grid still exercises the echoing path.
	if echoed == 0 {
		t.Error("no grid host was echoed, so the invariant above proved nothing")
	}
}

// TestDisplayHostEncodedJoinsNeverEchoACredential drives the encoded-delimiter grids
// ("%40" and its double-encoded spelling "%2540") through displayHost.
// TestDisplayHostNamesOnlyARealHost deliberately omits these joins; before the issue
// #70 fix the "%2540" join echoed a credential fragment in 64 rows, exactly the shapes
// where the encoded delimiter landed inside the authority url.Parse chose (u.Host) with
// no '/', '?' or '#' after it, where the old tail-only guard never looked. With the
// guard fed afterAuthorityDelimiter every row either fails closed or names a real grid
// host. There is no echoed>0 control here: the encoded joins legitimately fail closed on
// every row, and the raw-'@' grid in TestDisplayHostNamesOnlyARealHost keeps the
// availability control.
func TestDisplayHostEncodedJoinsNeverEchoACredential(t *testing.T) {
	t.Parallel()
	for _, join := range []string{"%40", "%2540"} {
		grid := credentialHostGridJoined(join)
		if len(grid) < 1000 {
			t.Fatalf("credentialHostGridJoined(%q) returned %d hosts, expected a full grid", join, len(grid))
		}
		failures := 0
		for _, host := range grid {
			got := displayHost(host)
			if got == redactedHostPlaceholder {
				continue
			}
			if strings.Contains(got, passwordMarker) || !namesAnIntendedHost(got) {
				failures++
				if failures <= 20 {
					t.Errorf("displayHost(%q) = %q, which names something other than the host", host, got)
				}
			}
		}
		if failures > 20 {
			t.Errorf("join %q: displayHost named a non-host in %d of %d grid hosts", join, failures, len(grid))
		}
	}
}

// TestRedactedHostPlaceholderLiteral pins the sentinel's literal value. Every other
// assertion writes it as the constant, so the client-facing string could otherwise be
// changed to anything, including something that reads as a host or a credential, with
// the whole suite still green.
func TestRedactedHostPlaceholderLiteral(t *testing.T) {
	t.Parallel()
	if redactedHostPlaceholder != "(redacted host)" {
		t.Errorf("redactedHostPlaceholder = %q, want %q", redactedHostPlaceholder, "(redacted host)")
	}
}

// TestSafeHostBoundsAllocationOnNestedEscapes pins the DoS property, not the
// redaction. An earlier design peeled one percent-encoding layer per candidate
// position, rebuilding the string each time; that was measured at 1.15GB allocated
// per call on a 64KB nested input, roughly 17500x the input (issue #62). safeHost
// now decodes the whole span a fixed number of times instead, so allocation is a
// small multiple of the input at ANY nesting depth. Both cases below measure 7.9x;
// the 20x bound is deliberately loose so machine and Go-version differences cannot
// flake it. Replacing maxDecodeRounds in decodesToContain with an unbounded loop is
// the change this is meant to catch, and doing so measures 17632x (1.156GB on a
// 64KB input), reproducing the issue #62 figure almost exactly.
// Deliberately NOT t.Parallel: testing.Benchmark derives AllocedBytesPerOp from
// process-wide runtime.MemStats deltas, so every allocation a concurrently running
// parallel test makes inside the benchmark window is charged to this one. Measured
// under the CI command (go test -race ./...) with t.Parallel on, the ratio rose
// from 7.9x to 15.5x against the 20x bound, which a slower runner would push over.
// Serial, it measures 7.88x with no run-to-run spread.
func TestSafeHostBoundsAllocationOnNestedEscapes(t *testing.T) {
	cases := []struct {
		name string
		host string
	}{
		// Re-encoding '%' appends two bytes per layer ("%25" -> "%2525" -> "%252525"),
		// so a 64KB input nests about 32000 layers deep and every round of decoding
		// makes progress. This is the shape that must NOT be decoded to exhaustion.
		// It deliberately does not end in "40": a flat %(25)*40 delimiter is found by
		// escapeEncodesAt without decoding at all, which would short-circuit the very
		// path this test exists to bound.
		{"deeply nested %25 layers", "https://admin:secret%" + strings.Repeat("25", 32*1024) + "x"},
		{"deeply nested layers with a real delimiter", "https://admin:secret%" + strings.Repeat("25", 32*1024) + "%34%30host"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			//nolint:thelper // this is the benchmark body itself, not a helper.
			res := testing.Benchmark(func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					got = safeHost(tt.host)
				}
			})
			// Positive control: a benchmark that never ran would report 0 B/op and pass
			// the bound vacuously.
			if res.N == 0 || got == "" {
				t.Fatalf("benchmark did not run (N=%d, output %q)", res.N, got)
			}
			if strings.Contains(got, "secret") {
				t.Errorf("safeHost leaked the password on %s: %.80q", tt.name, got)
			}
			if limit := int64(len(tt.host)) * 20; res.AllocedBytesPerOp() > limit {
				t.Errorf("safeHost allocated %d B/op on a %d byte input (%.1fx), want at most %d B/op",
					res.AllocedBytesPerOp(), len(tt.host),
					float64(res.AllocedBytesPerOp())/float64(len(tt.host)), limit)
			}
		})
	}
}

// TestValidateHostSchemeErrorsNeverEchoACredential pins that a REJECTION message
// cannot reprint the credential it is rejecting. Every message already quotes
// safeHost's masked form, but Go's own parse reason quotes the offending span too
// ("invalid port \":<password>\" after host"), so unwrapping one layer and
// formatting it put the password back into the very sentence that had just masked
// it (CWE-532). The messages reach a log at main.go for CENTREON_HOST and at the
// gateway host-rejected line for a caller-supplied X-Centreon-Host header.
func TestValidateHostSchemeErrorsNeverEchoACredential(t *testing.T) {
	t.Parallel()
	grid := credentialHostGrid()
	grid = append(grid, credentialHostGridJoined("%40")...)
	grid = append(grid, credentialHostGridJoined("%25%34%30")...)
	grid = append(grid, credentialHostGridSeparated("%3A", "@")...)
	if len(grid) < 1000 {
		t.Fatalf("grid returned %d hosts, expected a full grid", len(grid))
	}

	rejected, failures := 0, 0
	for _, host := range grid {
		err := validateHostScheme(host, true)
		if err == nil {
			continue
		}
		rejected++
		if msg := err.Error(); strings.Contains(msg, passwordMarker) || strings.Contains(msg, passwordFragment) {
			failures++
			if failures <= 10 {
				t.Errorf("validateHostScheme(%q) error leaks the password:\n  %s", host, msg)
			}
		}
	}
	if failures > 10 {
		t.Errorf("validateHostScheme leaked the password in %d of %d rejection messages", failures, rejected)
	}
	// Positive control: an assertion over rejection messages proves nothing if
	// nothing in the grid is actually rejected.
	if rejected < 1000 {
		t.Fatalf("only %d grid hosts were rejected, too few to prove anything", rejected)
	}
}

// passwordMarker appears only in password position in the credentialHostGrid
// inputs, so finding it in safeHost output is unambiguously a leak.
const passwordMarker = "SEKRIT"

// passwordFragment is a second sentinel placed BEFORE passwordMarker in some grid
// passwords, so the sweep can see a partial leak. Issue #65 item 8: every original
// grid password carried the marker at or near its end, so an output that echoed the
// password span preceding the marker satisfied a marker-only absence assertion. The
// two safeHost families closed in issue #62 both leaked that way and kept the grid
// green. Digit-only on purpose: it keeps the all-numeric prefix shape that makes
// url.Parse read a password as a port (issue #55), and it collides with nothing in
// credentialGridHosts, whose only digits are the 8443 port and the IPv6 literals.
const passwordFragment = "31337"

// credentialGridHosts are the authorities credentialHostGrid builds every input
// around. namesAnIntendedHost reads this same list, so the grid and the assertion
// over it cannot drift apart. Every entry must be a plain authority: a
// credential-shaped entry added here would silently be blessed as a legitimate host.
var credentialGridHosts = []string{
	"centreon.example.com", "centreon.example.com:8443",
	"[2001:db8::1]:8443", "[::1]",
}

// credentialHostGrid builds credential-shaped host URLs by combining a scheme, a
// username, a password holding passwordMarker, a host and a trailing path, query or
// fragment. It covers the placements url.Parse mis-reads: an unencoded '/', '?' or
// '#' in either the username or the password, an all-numeric password prefix that
// parses as a port, an email-style username that moves the authority split onto the
// wrong '@', and a second password span in the tail (issue #55).
func credentialHostGrid() []string { return credentialHostGridJoined("@") }

// credentialHostGridJoined is credentialHostGrid parameterised by the bytes that
// join the userinfo to the target. The raw "@" join is the credential form
// url.Parse decodes and is what the display test and the raw-'@' half of the
// safeHost sweep use. "%40" (and its multiply-encoded spelling "%2540") is the
// percent-encoded delimiter of issue #62: it carries no raw '@' at all, so the
// unchanged-return and the textual masker both have to decode it to find the
// password span. Only the safeHost sweep feeds the encoded joins; the display
// test deliberately does not (see the note there).
func credentialHostGridJoined(join string) []string {
	return credentialHostGridSeparated(":", join)
}

// credentialHostGridSeparated is credentialHostGridJoined with the userinfo
// SEPARATOR parameterised as well as the delimiter. Until issue #75 the grid built
// every userinfo as username+":"+password, so the separator was the one axis it
// held constant, and the whole encoded-separator family ("%3A") was reachable only
// through hand-written table rows. That gap hid a Critical leak: the first #75 fix
// masked correctly for a portless host and echoed the entire credential whenever
// the host carried a port, because every hand-written row happened to be portless.
// The lesson is issue #65 item 8 at a different axis: ask which dimension the grid
// holds constant and whether the root cause can vary it.
func credentialHostGridSeparated(sep, join string) []string {
	schemes := []string{"https://", "http://", "//", "://", "", "https:"}
	usernames := []string{
		"admin", "us/er", "us?er", "us#er", "us.er", "[user", "us[er",
		"user@corp.com", "a@b",
	}
	// A password BEGINNING with "://" is deliberately absent: behind a scheme-shaped
	// username that is byte-for-byte a scheme://authority URL whose userinfo holds a
	// username and no password, which safeHost never masks. safeHost documents it.
	// Every other "//" placement is covered, including mid-password, which is the
	// realistic base64 case.
	passwords := []string{
		passwordMarker, "1234/" + passwordMarker, "1234?" + passwordMarker,
		"1234#" + passwordMarker, "/" + passwordMarker, "?" + passwordMarker,
		"#" + passwordMarker, "aB9/" + passwordMarker, passwordMarker + "/tail",
		"1234/sec:" + passwordMarker, "pw/" + passwordMarker, "p@" + passwordMarker,
		"pw//" + passwordMarker, "pw://" + passwordMarker, "aB9//" + passwordMarker,
		// Marker AFTER an internal '@' and before a '/', '?' or '#'. url.Parse decodes
		// the pre-'@' fragment as userinfo and reads the marker as the host, so
		// url.Redacted masks only the fragment and the marker leaks; the marker-only
		// assertion can finally see the family-1 class (issue #62).
		"x@" + passwordMarker + "/p", "x@" + passwordMarker + "?q", "x@" + passwordMarker + "#f",
		// Marker-before-delimiter passwords (issue #65 item 8). The fragment leads, so
		// an output that echoes only the span ahead of the marker still trips the
		// sweep. The last two put the fragment after the marker and after an internal
		// raw '@', the geometry where the masker picks the earlier delimiter and leaves
		// the rest of the password standing.
		passwordFragment + "/" + passwordMarker, passwordFragment + "?" + passwordMarker,
		passwordFragment + "#" + passwordMarker, passwordMarker + "/" + passwordFragment,
		"p@" + passwordFragment + passwordMarker,
		// Fragment before a RAW ':', marker after it. Combined with an encoded
		// separator this is the only shape that exposes the last #75 family: Go splits
		// on the raw colon, so url.Redacted masks the marker and echoes everything in
		// front of it, and a marker-only assertion sees a correctly redacted string.
		// Without this row the 238464-row sweep passes while the password leaks.
		passwordFragment + ":" + passwordMarker,
	}
	hosts := credentialGridHosts
	tails := []string{"", "/mon", "?q=1", "#f", "/a@b", "/d:" + passwordMarker + "@f"}

	userinfos := make([]string, 0, len(usernames)*len(passwords))
	for _, username := range usernames {
		for _, password := range passwords {
			userinfos = append(userinfos, username+sep+password)
		}
	}

	targets := make([]string, 0, len(hosts)*len(tails))
	for _, host := range hosts {
		for _, tail := range tails {
			targets = append(targets, host+tail)
		}
	}

	grid := make([]string, 0, len(schemes)*len(userinfos)*len(targets))
	for _, scheme := range schemes {
		for _, userinfo := range userinfos {
			for _, target := range targets {
				grid = append(grid, scheme+userinfo+join+target)
			}
		}
	}
	return grid
}

// TestDisplayHost pins two contracts. First, displayHost reduces a host URL to
// scheme://host[:port] for a tool response, stripping the username as well as the
// password (issue #48 requires no username/password/token in the response), unlike
// safeHost which keeps the username for log greps. Second, it fails closed when a
// '@' survives after the authority, because url.Parse may then have read part of a
// credential as the host (issue #57).
func TestDisplayHost(t *testing.T) {
	t.Parallel()
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
		// A '@' surviving after the authority means url.Parse may have read part of a
		// credential as the host, so echoing it can hand a fragment of the operator's
		// password to a client (issue #57). Against origin/main, where displayHost had
		// no guard at all, the first three rows echoed "https://admin:1234",
		// "https://us" and "https://corp.example:1234"; the fourth and fifth survived
		// the first attempt at the guard and echoed "https://ssword" and
		// "https://mysecret".
		{"fails closed on the numeric-prefix credential form", "https://admin:1234/secret@centreon.example.com", redactedHostPlaceholder},
		{"fails closed on the short-username credential form", "https://us/er:secret@centreon.example.com", redactedHostPlaceholder},
		{"fails closed on the multi-at credential form", "https://user@corp.example:1234/Sekr1tPass@10.0.0.5", redactedHostPlaceholder},
		// A password holding a '@' before a '/' is the shape that survived the first
		// attempt at this fix: url.Parse decodes userinfo "admin:p" and host "ssword",
		// so a predicate that trusted any decoded password echoed the second half of
		// the real one.
		{"fails closed when the password holds an at sign before a slash", "https://admin:p@ssword/x@centreon.example.com", redactedHostPlaceholder},
		// A colon-free credential is the same hazard with no password span to find.
		// url.Parse reads "mysecret" as the host; safeHost leaves this input unchanged
		// because maskAuthorityPassword finds no ':' in the "mysecret/password" span,
		// so a predicate keyed on safeHost's output echoed the host as well.
		{"fails closed on a colon-free credential mis-read as the host", "https://mysecret/password@centreon.example.com", redactedHostPlaceholder},
		// The delimiter can also arrive percent-encoded, which a raw-'@' test misses
		// while url.Parse still ends the authority at the '/', '?' or '#'. All three
		// delimiters the doc comment names are covered.
		{"fails closed on an encoded at sign after a slash", "https://admin:1234/secret%40centreon.example.com", redactedHostPlaceholder},
		{"fails closed on an encoded at sign after a question mark", "https://admin:1234?secret%40centreon.example.com", redactedHostPlaceholder},
		{"fails closed on an encoded at sign after a hash", "https://admin:1234#secret%40centreon.example.com", redactedHostPlaceholder},
		// Encoded more than once, so a substring test for "%40" does not see it. The
		// third row needs more unescape passes than the guard performs, and fails closed
		// because the tail was still escaped when the bound ran out.
		{"fails closed on a double-encoded at sign", "https://admin:1234/secret%2540centreon.example.com", redactedHostPlaceholder},
		{"fails closed on a triple-encoded at sign", "https://admin:1234/secret%252540centreon.example.com", redactedHostPlaceholder},
		{"fails closed when the encoding outruns the unescape bound", "https://admin:1234/secret%25252540centreon.example.com", redactedHostPlaceholder},
		// The encoded delimiter can also sit, still encoded, INSIDE the span url.Parse
		// accepted as the host, with no '/', '?' or '#' after it (issue #70). authorityTail
		// is empty for these, so the guard scans afterAuthorityDelimiter's span (which
		// includes the authority) instead. Before that switch the first row echoed
		// "https://SEKRIT%40centreon.example.com", the second half of the password
		// "p@SEKRIT", to the client.
		{"fails closed on a double-encoded at sign inside the authority", "https://admin:p@SEKRIT%2540centreon.example.com", redactedHostPlaceholder},
		{"fails closed on a triple-encoded at sign inside the authority", "https://admin:p@SEKRIT%252540centreon.example.com", redactedHostPlaceholder},
		// The single-encoded spelling never reaches the guard: net/url rejects "%40" as a
		// host escape, so it fails closed at the parse branch. The #68 shape "%25%34%30"
		// (the delimiter's own hex digits encoded) is rejected there too, on "%34". Both
		// pin that the client path is robust to these encodings; #68 is only the safeHost
		// log path.
		{"fails closed on a single-encoded at sign inside the authority", "https://admin:p@SEKRIT%40centreon.example.com", redactedHostPlaceholder},
		{"fails closed on a hex-digit-encoded at sign inside the authority", "https://admin:p@SEKRIT%25%34%30centreon.example.com", redactedHostPlaceholder},
		// A percent escape in the hostname itself now fails closed too: the guard span
		// includes the authority, "%25" decodes to a lone '%', and the next unescape
		// errors, which tailCarriesAtSign refuses. Losing this exotic but legal host
		// spelling is the accepted over-redaction cost of the #70 fix.
		{"fails closed on a percent escape in the host itself", "https://cent%25reon.example.com", redactedHostPlaceholder},
		// A malformed escape cannot be ruled out as a delimiter either. It has to sit in
		// the QUERY to pin that branch: in a path or fragment url.Parse rejects the URL
		// itself, so those inputs fail closed one guard earlier and prove nothing here.
		{"fails closed on a malformed escape in the query", "https://admin:1234?secret%4%30centreon.example.com", redactedHostPlaceholder},
		{"fails closed on a doubled percent in the query", "https://admin:1234?secret%%3440centreon.example.com", redactedHostPlaceholder},
		// No rule can tell scheme://X/Y@Z apart from userinfo X/Y plus host Z without
		// resolving names, so a legitimate '@' after the authority fails closed too.
		// displayHost discards the path, query and fragment anyway, so only the host
		// name is lost.
		{"fails closed on an at sign in the path", "https://centreon.example.com/api@v1", redactedHostPlaceholder},
		{"fails closed on an at sign in the query", "https://centreon.example.com?q=a@b", redactedHostPlaceholder},
		{"fails closed on an at sign in the fragment", "https://centreon.example.com#a@b", redactedHostPlaceholder},
		{"fails closed on an at sign in the path of a host with a port", "https://centreon.example.com:8080/a@b", redactedHostPlaceholder},
		// An unparseable URL fails closed on the error, not on the hostname. Without a
		// row here that half of the guard is unpinned, since every other input parses.
		{"fails closed on an unparseable url", "https://admin:se^kret@centreon.example.com", redactedHostPlaceholder},
		{"fails closed on a url with a space", "https://admin:se kret@centreon.example.com", redactedHostPlaceholder},
		// Unescaping the tail rather than scanning it for '%' is what keeps an ordinary
		// encoded path trusted. "%4c" is a valid escape for 'L', so the second row
		// holds no delimiter in any encoding.
		{"keeps a host whose path holds an ordinary escape", "https://centreon.example.com/mon%20test", "https://centreon.example.com"},
		{"keeps a host whose path escape is not a delimiter", "https://centreon.example.com/pw%4chost", "https://centreon.example.com"},
		// url.Parse requires the address part of a bracketed body to parse as an IP
		// literal, so such an authority is the host and not a mis-read credential. It
		// therefore stays trusted even when a '@' survives after it, which the second
		// row pins.
		{"keeps a bracketed ipv6 host", "https://[::1]:8443", "https://[::1]:8443"},
		{"keeps a bracketed ipv6 host despite an at sign in the path", "https://[::1]:8443/path@x", "https://[::1]:8443"},
		// An '@' INSIDE the authority needs no guard: url.Parse splits at the last one,
		// so the earlier ones belong to the userinfo it decoded and the host is sound.
		// These pin the availability half, which a rule counting every '@' would break.
		{"keeps a host whose password holds an at sign", "https://admin:p@ssword@centreon.example.com", "https://centreon.example.com"},
		{"keeps a host whose password holds an at sign, with a path", "https://admin:p@ssword@centreon.example.com/centreon", "https://centreon.example.com"},
		{"keeps a host whose password is empty", "https://user:@centreon.example.com", "https://centreon.example.com"},
		{"keeps a host whose password is percent-encoded", "https://user:p%40ss@centreon.example.com", "https://centreon.example.com"},
		{"fails closed on empty", "", redactedHostPlaceholder},
		{"fails closed on an authority holding only a port", "https://:8443", redactedHostPlaceholder},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := displayHost(tt.host); got != tt.want {
				t.Errorf("displayHost(%q) = %q, want %q", tt.host, got, tt.want)
			}
		})
	}
}

// TestLoadConfig_HostScheme pins that CENTREON_HOST is validated at load, for both
// its scheme (gated by CENTREON_ALLOW_HTTP) and the presence of a hostname, and
// that the check is skipped only in HTTP gateway mode where the host is an unused
// placeholder. It also pins that stdio+gateway still validates, so the skip requires
// BOTH the http transport and gateway auth mode. The hostname rows are here to pin
// the wiring rather than the predicate, which TestValidateHostScheme already covers:
// they fail if this call site stops validating CENTREON_HOST, and the gateway row
// fails if the skip stops covering the hostname check as well as the scheme.
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
		{"hostless host rejected", "https://", "", "", "", "CENTREON_HOST", false},
		{"port-only host rejected", "https://:8443", "", "", "", "CENTREON_HOST", false},
		{"credential-only host rejected", "https://admin:sekrit58@", "", "", "", "CENTREON_HOST", false},
		{"gateway mode skips the hostname check too", "https://", "http", "gateway", "", "", false},
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

// TestLoadConfig_AllowedHostsScheme pins that each CENTREON_ALLOWED_HOSTS entry is
// validated at load, for its scheme (gated by CENTREON_ALLOW_HTTP) and for naming a
// hostname. Note this runs in every mode, so a hostless entry aborts startup even
// where the allowlist itself is never consulted.
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
		{"hostless entry rejected", "https://a.example.com,https://", "", "CENTREON_ALLOWED_HOSTS"},
		{"port-only entry rejected", "https://:8443", "", "CENTREON_ALLOWED_HOSTS"},
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
