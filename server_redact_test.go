package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// leakMarker is the password planted in a mis-parsed host URL across these
// tests. The mis-parsed shape (a '/' inside the password) is the one from
// issues #63 and #71: url.Parse reads "admin" as the host and decodes no
// userinfo, so http.Client prints the whole typed password in its *url.Error.
const leakMarker = "s3cr3tpw"

// misparsedHost is a base URL whose password holds a '/', so the credential
// survives into the request URL and any resulting *url.Error.
const misparsedHost = "https://admin:1234/" + leakMarker + "@centreon.invalid"

// failDNSClient is an http.Client whose transport fails every request with a
// resolver error, so a login or logout deterministically produces the
// credential-bearing *url.Error without touching real DNS.
func failDNSClient() *http.Client {
	return &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, &net.DNSError{Err: "no such host", Name: "admin", IsNotFound: true}
	})}
}

// logRecordByMsg returns the first JSON log record in buf whose "msg" equals
// want. It lets a test inspect one specific sink's attributes while ignoring
// the centreon client's own request-failure log line (the upstream residual
// tphakala/centreon-go-client#42, out of scope here), which this repo cannot
// intercept because the client formats and emits it directly.
func logRecordByMsg(t *testing.T, buf *bytes.Buffer, want string) map[string]any {
	t.Helper()
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if rec["msg"] == want {
			return rec
		}
	}
	t.Fatalf("no log record with msg=%q in: %s", want, buf.String())
	return nil
}

// TestRunStdio_LoginErrorRedacted pins that a failed startup login does not
// carry the credential out through the error returned to main (CWE-532, #63).
// It asserts on the return value, not a log buffer, so the client's own request
// logging cannot confound it.
func TestRunStdio_LoginErrorRedacted(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.DiscardHandler)
	cfg := &Config{Host: misparsedHost, Username: "admin", Password: leakMarker}

	err := runStdio(t.Context(), cfg, logger, failDNSClient(), nil)
	if err == nil {
		t.Fatal("want a login error, got nil")
	}
	if strings.Contains(err.Error(), leakMarker) {
		t.Errorf("login error leaked credential (CWE-532): %v", err)
	}
	if !strings.Contains(err.Error(), "centreon login") {
		t.Errorf("want the login-failure wrap to stay identifiable, got: %v", err)
	}
}

// TestRunStdio_TokenPreflightErrorRedacted pins that a failed startup token
// preflight does not carry the credential out through the error returned to main
// (CWE-532, #63, #82).
func TestRunStdio_TokenPreflightErrorRedacted(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.DiscardHandler)
	cfg := &Config{Host: misparsedHost, Token: "tok"}

	err := runStdio(t.Context(), cfg, logger, failDNSClient(), nil)
	if err == nil {
		t.Fatal("want a preflight error, got nil")
	}
	if strings.Contains(err.Error(), leakMarker) {
		t.Errorf("token preflight error leaked credential (CWE-532): %v", err)
	}
	if !strings.Contains(err.Error(), "centreon token validation failed") {
		t.Errorf("want the token-validation-failure wrap to stay identifiable, got: %v", err)
	}
}

// TestLogoutCachedToken_LogNeverLeaksCredential pins that the gateway
// token-logout Debug sinks (#63) do not print the credential-bearing host URL.
// logoutCachedToken builds its client with a nil logger, so the buffer holds
// only this function's own lines; no client residual can confound it.
func TestLogoutCachedToken_LogNeverLeaksCredential(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	logoutCachedToken(t.Context(), misparsedHost, "tok", logger, failDNSClient())

	out := buf.String()
	if strings.Contains(out, leakMarker) {
		t.Errorf("token-logout log leaked credential (CWE-532): %s", out)
	}
	if !strings.Contains(out, "gateway: token logout failed") {
		t.Errorf("want the logout-failure line, got: %s", out)
	}
}

// TestGatewayServer_AuthFailureLogRedactsError pins the highest-concern #63
// sink: gateway auth failure, where the host is the caller-supplied
// X-Centreon-Host header. It inspects only the "gateway: authentication failed"
// record, so the client's separate request-failure line (#42) does not mask a
// regression in our sink.
func TestGatewayServer_AuthFailureLogRedactsError(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", http.NoBody)
	r.Header.Set("X-Centreon-Host", misparsedHost)
	r.Header.Set("X-Centreon-Username", "admin")
	r.Header.Set("X-Centreon-Password", leakMarker)

	cache := NewTokenCache(time.Minute)
	if srv := gatewayServer(r, &Config{}, cache, logger, failDNSClient()); srv != nil {
		t.Fatal("want nil server on auth failure")
	}

	rec := logRecordByMsg(t, &buf, "gateway: authentication failed")
	errVal, _ := rec["error"].(string)
	if strings.Contains(errVal, leakMarker) {
		t.Errorf("auth-failure error attribute leaked credential (CWE-532): %q", errVal)
	}
	if errVal != "DNS lookup failed" {
		t.Errorf("auth-failure error attribute = %q, want the classified reason %q", errVal, "DNS lookup failed")
	}
	if hostVal, _ := rec["host"].(string); strings.Contains(hostVal, leakMarker) {
		t.Errorf("auth-failure host attribute leaked credential (CWE-532): %q", hostVal)
	}
}
