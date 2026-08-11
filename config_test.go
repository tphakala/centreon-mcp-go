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
		// A URL with no hostname cannot be used at all, so validation refuses it
		// rather than leaving it to the client (issue #58). centreon.NewClient
		// rejects it with an error that formats the RAW base URL, and that error is
		// logged, so a credential-bearing form would otherwise reach the log in
		// clear (CWE-532). notWant pins that the rejection here stays redacted.
		{"scheme-only rejected", "https:", true, "must include a hostname", ""},
		{"empty authority rejected", "https://", true, "must include a hostname", ""},
		{"opaque form rejected", "https:centreon.example.com", true, "must include a hostname", ""},
		{"port-only authority rejected", "https://:9443", false, "must include a hostname", ""},
		{"userinfo-only authority rejected with password redacted", "https://admin:sekrit58@", false, "must include a hostname", "sekrit58"},
		{"userinfo with port-only authority rejected with password redacted", "https://admin:sekrit58@:9443", false, "must include a hostname", "sekrit58"},
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
// logs or error messages, CWE-532), including the scheme-less user:pass@host form
// url.Redacted alone does not mask (issue #41) and the forms url.Parse silently
// mis-reads as host, port, path, query or fragment (issue #55). A host whose '@'
// cannot carry a password, and a bracketed IPv6 authority with no password span in
// its tail, are still returned byte for byte unchanged. These rows are examples;
// TestSafeHostNeverLeaksPassword sweeps the class.
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
	grid := credentialHostGrid()
	// Without this the whole test passes vacuously if the builder ever returns
	// nothing, which is the failure mode a pure absence assertion cannot see.
	if len(grid) < 1000 {
		t.Fatalf("credentialHostGrid() returned %d hosts, expected a full grid", len(grid))
	}

	failures := 0
	for _, host := range grid {
		// Positive control: the input must actually carry the marker, otherwise
		// asserting its absence from the output proves nothing.
		if !strings.Contains(host, passwordMarker) {
			t.Fatalf("grid host %q does not contain the marker, the builder is wrong", host)
		}
		if got := safeHost(host); strings.Contains(got, passwordMarker) {
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
}

// passwordMarker appears only in password position in the credentialHostGrid
// inputs, so finding it in safeHost output is unambiguously a leak.
const passwordMarker = "SEKRIT"

// credentialHostGrid builds credential-shaped host URLs by combining a scheme, a
// username, a password holding passwordMarker, a host and a trailing path, query or
// fragment. It covers the placements url.Parse mis-reads: an unencoded '/', '?' or
// '#' in either the username or the password, an all-numeric password prefix that
// parses as a port, an email-style username that moves the authority split onto the
// wrong '@', and a second password span in the tail (issue #55).
func credentialHostGrid() []string {
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
	}
	hosts := []string{
		"centreon.example.com", "centreon.example.com:8443",
		"[2001:db8::1]:8443", "[::1]",
	}
	tails := []string{"", "/mon", "?q=1", "#f", "/a@b", "/d:" + passwordMarker + "@f"}

	userinfos := make([]string, 0, len(usernames)*len(passwords))
	for _, username := range usernames {
		for _, password := range passwords {
			userinfos = append(userinfos, username+":"+password)
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
				grid = append(grid, scheme+userinfo+"@"+target)
			}
		}
	}
	return grid
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
		// An '@' that url.Parse did not account for as the whole credential means the
		// parsed authority may be the credential rather than the host, so echoing it
		// can hand a fragment of the operator's password to a client (issue #57).
		// The first row expected "https://admin:1234" before that fix, where 1234 is
		// the first segment of the password the operator typed.
		{"fails closed on the numeric-prefix credential form", "https://admin:1234/secret@centreon.example.com", redactedHostPlaceholder},
		{"fails closed on the short-username credential form", "https://us/er:secret@centreon.example.com", redactedHostPlaceholder},
		{"fails closed on the multi-at credential form", "https://user@corp.example:1234/Sekr1tPass@10.0.0.5", redactedHostPlaceholder},
		// A colon-free credential is the same hazard with no password span to find:
		// url.Parse reads "mysecret" as the host, and safeHost leaves the string
		// untouched because what it sees is a username, which it keeps by design.
		{"fails closed on a colon-free credential mis-read as the host", "https://mysecret/password@centreon.example.com", redactedHostPlaceholder},
		// scheme://X/Y@Z cannot be told apart from userinfo X/Y plus host Z without
		// resolving names, so a legitimate '@' in a path or query fails closed too.
		// displayHost discards the path and query anyway, so nothing is lost beyond
		// naming the host.
		{"fails closed on an at sign in the path", "https://centreon.example.com/api@v1", redactedHostPlaceholder},
		{"fails closed on an at sign in the query", "https://centreon.example.com?q=a@b", redactedHostPlaceholder},
		{"fails closed when a port colon precedes a path at sign", "https://centreon.example.com:8080/a@b", redactedHostPlaceholder},
		// '[' is invalid in userinfo, so a bracketed authority cannot itself be a
		// credential and stays trusted even with an '@' later in the URL.
		{"keeps a bracketed ipv6 host", "https://[::1]:8443", "https://[::1]:8443"},
		{"keeps a bracketed ipv6 host despite an at sign in the path", "https://[::1]:8443/path@x", "https://[::1]:8443"},
		{"keeps a host whose password is empty", "https://user:@centreon.example.com", "https://centreon.example.com"},
		{"keeps a host whose password is percent-encoded", "https://user:p%40ss@centreon.example.com", "https://centreon.example.com"},
		{"fails closed on empty", "", redactedHostPlaceholder},
		{"fails closed on a hostless authority", "https://:8443", redactedHostPlaceholder},
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
