package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestHostAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		host    string
		allowed []string
		want    bool
	}{
		{"nil allowlist accepts any host", "https://any.example.com", nil, true},
		{"empty allowlist accepts any host", "https://any.example.com", []string{}, true},
		{"host present in allowlist", "https://centreon.example.com", []string{"https://centreon.example.com"}, true},
		{"host present among several", "https://b.example.com", []string{"https://a.example.com", "https://b.example.com"}, true},
		{"host absent from allowlist", "https://evil.example.com", []string{"https://centreon.example.com"}, false},
		{"exact match rejects surrounding whitespace", "  https://centreon.example.com  ", []string{"https://centreon.example.com"}, false},
		{"match is case-sensitive", "https://Centreon.Example.com", []string{"https://centreon.example.com"}, false},
		{"empty host with non-empty allowlist is rejected", "", []string{"https://centreon.example.com"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := hostAllowed(tt.host, tt.allowed); got != tt.want {
				t.Errorf("hostAllowed(%q, %v) = %v, want %v", tt.host, tt.allowed, got, tt.want)
			}
		})
	}
}

// TestGatewayServer_HostAllowlist exercises the enforcement point itself, not just
// the hostAllowed helper: gatewayServer must reject a host off the allowlist and a
// cleartext scheme, and admit an allowed one. Since #79 an accepted host must answer
// the token-validation read, so the accept cases point at a fake that accepts any
// token. The reject cases assert on the specific rejection-reason LOG, not only a
// nil server: deleting the allowlist or scheme check would fall through to token
// validation, which also returns nil against the unreachable reject host, so a bare
// nil check could not tell the two rejections apart (or catch either check's
// removal).
func TestGatewayServer_HostAllowlist(t *testing.T) {
	cache := NewTokenCache(time.Minute)

	// A fake Centreon that accepts any token, so the accept cases reach a real
	// (loopback http, hence AllowHTTP) host and clear token validation.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /centreon/api/latest/monitoring/hosts/status", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})
	fake := httptest.NewServer(mux)
	defer fake.Close()

	discard := slog.New(slog.DiscardHandler)
	newReq := func(host string) *http.Request {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", http.NoBody)
		r.Header.Set("X-Centreon-Host", host)
		r.Header.Set("X-Centreon-Token", "dummy-token")
		return r
	}

	t.Run("host not in allowlist is rejected", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, nil))
		cfg := &Config{AllowedHosts: []string{"https://allowed.example.com"}}
		if s := gatewayServer(newReq("https://evil.example.com"), cfg, cache, logger, nil); s != nil {
			t.Error("expected nil server for host not in allowlist, got non-nil")
		}
		if !strings.Contains(buf.String(), "not in allowlist") {
			t.Errorf("expected the allowlist-rejection log (deleting the allowlist check must be caught here), got: %s", buf.String())
		}
	})

	t.Run("host in allowlist is accepted", func(t *testing.T) {
		cfg := &Config{AllowHTTP: true, AllowedHosts: []string{fake.URL}}
		if s := gatewayServer(newReq(fake.URL), cfg, cache, discard, nil); s == nil {
			t.Error("expected non-nil server for allowed host, got nil")
		}
	})

	t.Run("empty allowlist accepts any host", func(t *testing.T) {
		cfg := &Config{AllowHTTP: true}
		if s := gatewayServer(newReq(fake.URL), cfg, cache, discard, nil); s == nil {
			t.Error("expected non-nil server when allowlist is empty, got nil")
		}
	})

	t.Run("http host rejected when AllowHTTP is false", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, nil))
		cfg := &Config{}
		if s := gatewayServer(newReq("http://plain.example.com"), cfg, cache, logger, nil); s != nil {
			t.Error("expected nil server for cleartext http host without CENTREON_ALLOW_HTTP, got non-nil")
		}
		if !strings.Contains(buf.String(), "host rejected") {
			t.Errorf("expected the scheme-rejection log (deleting the scheme check must be caught here), got: %s", buf.String())
		}
	})

	t.Run("http host accepted when AllowHTTP is true", func(t *testing.T) {
		cfg := &Config{AllowHTTP: true}
		if s := gatewayServer(newReq(fake.URL), cfg, cache, discard, nil); s == nil {
			t.Error("expected non-nil server for http host with CENTREON_ALLOW_HTTP, got nil")
		}
	})
}

// TestGatewayServer_RejectsOversizedHostHeader pins issue #76: gatewayServer must
// reject an X-Centreon-Host header longer than maxHostHeaderBytes BEFORE it reaches
// safeHost or any credential handling, and must never log the host content (logging
// even a "redacted" oversized host is the O(len) redaction cost the cap exists to
// defend against). Input shapes are varied deliberately (padding location, escape
// placement, allowlisted vs not) because this file's history shows a single fixed
// shape lets fail-open bugs survive a sabotage pass.
func TestGatewayServer_RejectsOversizedHostHeader(t *testing.T) {
	t.Parallel()

	const marker = "oversized-host-marker-must-not-be-logged"

	newReq := func(host string) *http.Request {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", http.NoBody)
		r.Header.Set("X-Centreon-Host", host)
		r.Header.Set("X-Centreon-Token", "dummy-token")
		return r
	}

	tests := []struct {
		name        string
		host        string
		allowlisted bool
	}{
		{
			name: "just over the cap, padding in path, escape at end",
			host: "https://" + marker + ".example.com/" + strings.Repeat("a", maxHostHeaderBytes) + "%",
		},
		{
			name: "one megabyte, padding in query, escape mid-string",
			host: "https://" + marker + ".example.com/?q=" + strings.Repeat("b", 512<<10) + "%25" + strings.Repeat("c", 512<<10),
		},
		{
			// The host equals an allowlist entry: without the length gate the flow
			// would reach hostAllowed (true) and build a server, so this pins that the
			// cap precedes the allowlist check.
			name:        "oversized host that is on the allowlist is still rejected",
			host:        "https://" + marker + ".example.com/" + strings.Repeat("d", maxHostHeaderBytes),
			allowlisted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if len(tt.host) <= maxHostHeaderBytes {
				t.Fatalf("test host is not oversized: len=%d cap=%d", len(tt.host), maxHostHeaderBytes)
			}
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
			cache := NewTokenCache(time.Minute)

			cfg := &Config{}
			if tt.allowlisted {
				cfg.AllowedHosts = []string{tt.host}
			}

			if srv := gatewayServer(newReq(tt.host), cfg, cache, logger, nil); srv != nil {
				t.Fatal("expected nil server for oversized X-Centreon-Host, got non-nil")
			}
			if out := buf.String(); strings.Contains(out, marker) {
				t.Errorf("oversized host content must not be logged (DoS cost / CWE-532), got: %s", out)
			}
		})
	}
}

// TestRunHTTP_RejectsOversizedRequestHeaders pins issue #76: runHTTP must set
// MaxHeaderBytes so the server answers a request whose headers exceed the cap with
// 431 instead of buffering up to Go's 1 MiB default. The oversize is placed in a
// single header in one subtest and spread across many in another, since
// MaxHeaderBytes bounds the TOTAL header size, not one field.
func TestRunHTTP_RejectsOversizedRequestHeaders(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	port := freePort(t)
	cfg := &Config{
		Host:      "https://unused.example.com", // required field; unused in gateway mode
		Transport: transportHTTP,
		AuthMode:  authModeGateway,
		HTTPHost:  "127.0.0.1",
		HTTPPort:  port,
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- runHTTP(ctx, cfg, logger, nil) }()

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForHealth(t, base+"/health")

	tests := []struct {
		name    string
		setHdrs func(*http.Request)
	}{
		{
			name: "bulk in one X-Centreon-Host header",
			setHdrs: func(r *http.Request) {
				r.Header.Set("X-Centreon-Host", strings.Repeat("z", maxHeaderBytes+(8<<10)))
			},
		},
		{
			name: "bulk spread across many headers",
			setHdrs: func(r *http.Request) {
				chunk := strings.Repeat("z", 8<<10)
				for i := 0; i < (maxHeaderBytes/(8<<10))+2; i++ {
					r.Header.Set(fmt.Sprintf("X-Junk-%d", i), chunk)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/health", http.NoBody)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			tt.setHdrs(req)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusRequestHeaderFieldsTooLarge {
				t.Errorf("expected 431 for oversized headers, got %d", resp.StatusCode)
			}
		})
	}

	// A normal request stays under the cap and still succeeds.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/health", http.NoBody)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("normal /health request should return 200, got %d", resp.StatusCode)
	}

	cancel()
	select {
	case <-errCh:
	case <-time.After(15 * time.Second):
		t.Fatal("runHTTP did not return within 15s after context cancel")
	}
}

// TestGatewayServer_HostCapBoundaryIsExclusive pins that the #76 length gate is
// '>', not '>=': a host of exactly maxHostHeaderBytes must pass the gate. We prove
// it passed by observing that it reaches the NEXT check (the allowlist) rather than
// the "header too long" rejection.
func TestGatewayServer_HostCapBoundaryIsExclusive(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	host := "https://boundary.example.com/"
	host += strings.Repeat("a", maxHostHeaderBytes-len(host))
	if len(host) != maxHostHeaderBytes {
		t.Fatalf("host must be exactly the cap: len=%d cap=%d", len(host), maxHostHeaderBytes)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", http.NoBody)
	req.Header.Set("X-Centreon-Host", host)
	req.Header.Set("X-Centreon-Token", "tok")

	// A non-empty allowlist that does NOT contain the host, so the expected outcome
	// past the length gate is an allowlist rejection.
	cfg := &Config{AllowedHosts: []string{"https://a-different-allowed-host.example.com"}}
	if s := gatewayServer(req, cfg, NewTokenCache(time.Minute), logger, nil); s != nil {
		t.Fatal("expected nil (rejected by allowlist), got non-nil")
	}

	out := buf.String()
	if strings.Contains(out, "too long") {
		t.Errorf("a host of exactly the cap must NOT trip the length gate, got: %s", out)
	}
	if !strings.Contains(out, "not in allowlist") {
		t.Errorf("a host of exactly the cap should reach the allowlist check, got: %s", out)
	}
}

// TestGatewayServer_ValidatesTokenBeforeBuild pins issue #79: a caller-supplied
// X-Centreon-Token must be validated against Centreon BEFORE gatewayServer builds
// the (expensive) tool registry, so an unauthenticated caller cannot force a
// per-request registry build. Inputs are varied (token value, allowlist state) so
// no single hidden constant carries the test.
func TestGatewayServer_ValidatesTokenBeforeBuild(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	const goodToken = "valid-tok-3"

	var statusHits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("GET /centreon/api/latest/monitoring/hosts/status", func(w http.ResponseWriter, r *http.Request) {
		statusHits.Add(1)
		if r.Header.Get("X-AUTH-TOKEN") != goodToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})
	fake := httptest.NewServer(mux)
	defer fake.Close()

	newReq := func(token string) *http.Request {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", http.NoBody)
		r.Header.Set("X-Centreon-Host", fake.URL)
		r.Header.Set("X-Centreon-Token", token)
		return r
	}

	tests := []struct {
		name      string
		token     string
		allowlist []string
		wantNil   bool
	}{
		{"bad token is rejected before build (empty allowlist)", "wrong-tok", nil, true},
		{"bad token rejected even when the host is allowlisted", "another-wrong-tok", []string{fake.URL}, true},
		{"valid token builds a server", goodToken, []string{fake.URL}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statusHits.Store(0)
			cfg := &Config{AllowHTTP: true, AllowedHosts: tt.allowlist}
			s := gatewayServer(newReq(tt.token), cfg, NewTokenCache(time.Minute), logger, nil)
			if tt.wantNil && s != nil {
				t.Fatal("expected nil server for a token that fails validation, got non-nil")
			}
			if !tt.wantNil && s == nil {
				t.Fatal("expected non-nil server for a valid token, got nil")
			}
			if got := statusHits.Load(); got != 1 {
				t.Errorf("expected exactly one token-validation read against Centreon, got %d", got)
			}
		})
	}
}

// TestGatewayServer_CachesTokenValidation pins the perf half of the #79 containment:
// a repeated valid token reuses the cached validation and does NOT re-hit Centreon,
// while a DISTINCT token validates separately (the token is part of the cache key,
// guarding the fail-open shape where the cache would match any token).
func TestGatewayServer_CachesTokenValidation(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	const (
		tokA = "cache-tok-A"
		tokB = "cache-tok-B"
	)

	var statusHits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("GET /centreon/api/latest/monitoring/hosts/status", func(w http.ResponseWriter, r *http.Request) {
		statusHits.Add(1)
		if tok := r.Header.Get("X-AUTH-TOKEN"); tok != tokA && tok != tokB {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})
	fake := httptest.NewServer(mux)
	defer fake.Close()

	cfg := &Config{AllowHTTP: true}
	cache := NewTokenCache(time.Minute)

	newReq := func(token string) *http.Request {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", http.NoBody)
		r.Header.Set("X-Centreon-Host", fake.URL)
		r.Header.Set("X-Centreon-Token", token)
		return r
	}

	// Same token twice: the second request reuses the cached validation.
	if s := gatewayServer(newReq(tokA), cfg, cache, logger, nil); s == nil {
		t.Fatal("request 1: expected non-nil server for a valid token")
	}
	if s := gatewayServer(newReq(tokA), cfg, cache, logger, nil); s == nil {
		t.Fatal("request 2: expected non-nil server for a valid token")
	}
	if got := statusHits.Load(); got != 1 {
		t.Fatalf("the same token twice should validate against Centreon exactly once, got %d", got)
	}

	// A distinct token must not match the cached entry, so it validates separately.
	if s := gatewayServer(newReq(tokB), cfg, cache, logger, nil); s == nil {
		t.Fatal("request 3: expected non-nil server for a second valid token")
	}
	if got := statusHits.Load(); got != 2 {
		t.Fatalf("a distinct token must validate separately (token is in the cache key), got %d validations", got)
	}
}

// TestGatewayServer_RedactsUserinfoInHostLogs pins issue #41: a gateway host URL
// can embed userinfo (https://user:pass@host), so every gateway host log field
// must go through safeHost (url.Redacted) or an embedded credential leaks into the
// logs (CWE-532). It drives representative reachable rejection paths; all gateway
// host log fields share the same safeHost(host) wrapper.
func TestGatewayServer_RedactsUserinfoInHostLogs(t *testing.T) {
	t.Parallel()

	const secret = "s3cr3tpw"

	newReq := func(host string, withToken bool) *http.Request {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", http.NoBody)
		r.Header.Set("X-Centreon-Host", host)
		if withToken {
			r.Header.Set("X-Centreon-Token", "tok")
		}
		return r
	}

	allowlist := &Config{AllowedHosts: []string{"https://allowed.example.com"}}

	tests := []struct {
		name string
		cfg  *Config
		req  *http.Request
		// wantMask is the redacted userinfo the log line must show. It varies by
		// row because url.Parse splits the authority at a different '@' depending
		// on the shape, so a single literal cannot cover them all.
		wantMask string
	}{
		// The host-not-in-allowlist path logs before validateHostScheme, so it sees
		// the raw header for both the standard and the scheme-less credential form
		// (the scheme-less form is the one url.Redacted alone would miss, issue #41).
		{"scheme-ful host not in allowlist", allowlist, newReq("https://gwuser:"+secret+"@evil.example.com", true), "gwuser:xxxxx@"},
		{"scheme-less host not in allowlist", allowlist, newReq("gwuser:"+secret+"@evil.example.com", true), "gwuser:xxxxx@"},
		// Issue #55: url.Parse mis-reads these as host:port or as a password-less
		// userinfo, so url.Redacted masks nothing. This log runs before
		// validateHostScheme, so safeHost is the only control on this sink and it is
		// the one place an attacker supplies the header.
		{"numeric-prefix password not in allowlist", allowlist, newReq("https://gwuser:1234/"+secret+"@evil.example.com", true), "gwuser:xxxxx@"},
		{"email-style username not in allowlist", allowlist, newReq("https://gwuser@corp.com:1234/"+secret+"@evil.example.com", true), "gwuser@corp.com:xxxxx@"},
		// A "//" inside the password posed as the authority marker, so the textual
		// masker returned this host unmasked straight into the log.
		{"password containing // not in allowlist", allowlist, newReq("gwuser:pw//"+secret+"@evil.example.com", true), "gwuser:xxxxx@"},
		// empty allowlist + no credential headers -> missing-credentials path.
		{"missing credentials", &Config{}, newReq("https://gwuser:"+secret+"@evil.example.com", false), "gwuser:xxxxx@"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
			cache := NewTokenCache(time.Minute)

			if srv := gatewayServer(tt.req, tt.cfg, cache, logger, nil); srv != nil {
				t.Fatal("expected nil server for a rejected gateway request")
			}

			out := buf.String()
			if out == "" {
				t.Fatal("expected a gateway log line, got none")
			}
			if strings.Contains(out, secret) {
				t.Errorf("log leaked embedded password (CWE-532): %s", out)
			}
			if !strings.Contains(out, tt.wantMask) {
				t.Errorf("expected redacted host containing %q in log, got: %s", tt.wantMask, out)
			}
		})
	}
}

// TestLogoutCachedToken_SendsTokenToLogoutEndpoint pins that logoutCachedToken
// invalidates a specific Centreon session: it must call GET /logout with the
// cached token in the X-AUTH-TOKEN header (the client sends its stored token),
// so the right server session is torn down.
func TestLogoutCachedToken_SendsTokenToLogoutEndpoint(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	var logoutCount atomic.Int32
	var gotToken atomic.Value
	gotToken.Store("")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /centreon/api/latest/logout", func(w http.ResponseWriter, r *http.Request) {
		logoutCount.Add(1)
		gotToken.Store(r.Header.Get("X-AUTH-TOKEN"))
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	logoutCachedToken(t.Context(), srv.URL, "tok-xyz", logger, nil)

	if n := logoutCount.Load(); n != 1 {
		t.Fatalf("expected exactly 1 logout call, got %d", n)
	}
	if tok, _ := gotToken.Load().(string); tok != "tok-xyz" {
		t.Errorf("logout must send the cached token as X-AUTH-TOKEN, got %q", tok)
	}
}

// roundTripFunc adapts a function to http.RoundTripper so a test can observe the
// request an http.Client is about to send.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestLogoutClientBounded_BoundsLogoutWithDeadline pins issue #38: the stdio
// shutdown logout must run on a bounded, cancel-immune context. Graceful shutdown
// cancels the parent ctx, so the logout runs on context.WithoutCancel to survive
// that cancellation, but it must still carry a deadline (shutdownLogoutTimeout) so
// an unreachable Centreon cannot block process exit. The test drives a client
// whose transport records whether the logout request carried a deadline and
// asserts one is present even though the parent ctx is already cancelled. The
// http.Client sets no Timeout of its own, so the only possible source of a request
// deadline is the bounded logout context under test.
func TestLogoutClientBounded_BoundsLogoutWithDeadline(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	var logoutCount atomic.Int32
	var hadDeadline atomic.Bool

	mux := http.NewServeMux()
	mux.HandleFunc("GET /centreon/api/latest/logout", func(w http.ResponseWriter, _ *http.Request) {
		logoutCount.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if _, ok := r.Context().Deadline(); ok {
				hadDeadline.Store(true)
			}
			return http.DefaultTransport.RoundTrip(r)
		}),
	}

	client, err := newCentreonClient(srv.URL, &Config{Token: "tok-xyz"}, logger, httpClient)
	if err != nil {
		t.Fatalf("newCentreonClient: %v", err)
	}

	// Simulate graceful shutdown: the parent context is already cancelled when the
	// deferred logout fires.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	logoutClientBounded(ctx, client, logger)

	if n := logoutCount.Load(); n != 1 {
		t.Fatalf("bounded logout must still call GET /logout once despite a cancelled parent ctx, got %d", n)
	}
	if !hadDeadline.Load() {
		t.Error("stdio shutdown logout must bound the logout with a deadline (issue #38); the request carried none")
	}
}

// TestDrainAndLogout_LogsOutEveryCachedTokenAndEmptiesCache pins the #5 shutdown
// behaviour end to end: every token in the cache is logged out on the Centreon
// server and the cache is left empty.
func TestDrainAndLogout_LogsOutEveryCachedTokenAndEmptiesCache(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	var mu sync.Mutex
	seen := map[string]bool{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /centreon/api/latest/logout", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen[r.Header.Get("X-AUTH-TOKEN")] = true
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cache := newTokenCache(50*time.Minute, 10)
	cache.Set(srv.URL, "admin", "p1", "tok-1")
	cache.Set(srv.URL, "admin", "p2", "tok-2")
	cache.Set(srv.URL, "admin", "p3", "tok-3")

	drainAndLogout(t.Context(), cache, logger, nil)

	mu.Lock()
	defer mu.Unlock()
	for _, tok := range []string{"tok-1", "tok-2", "tok-3"} {
		if !seen[tok] {
			t.Errorf("token %q was not logged out", tok)
		}
	}
	if n := len(cache.entries); n != 0 {
		t.Errorf("drainAndLogout should empty the cache, %d entries remain", n)
	}
}

// TestDrainAndLogout_SwallowsLogoutFailuresAndEmptiesCache pins the best-effort
// contract: when the Centreon logout endpoint fails, drainAndLogout still
// attempts every cached session, still empties the cache, and never fails. A
// failing logout must not abort the remaining ones.
func TestDrainAndLogout_SwallowsLogoutFailuresAndEmptiesCache(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	var attempts atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("GET /centreon/api/latest/logout", func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cache := newTokenCache(50*time.Minute, 10)
	cache.Set(srv.URL, "admin", "p1", "tok-1")
	cache.Set(srv.URL, "admin", "p2", "tok-2")
	cache.Set(srv.URL, "admin", "p3", "tok-3")

	drainAndLogout(t.Context(), cache, logger, nil)

	if got := attempts.Load(); got != 3 {
		t.Errorf("every cached session should be attempted despite failures: got %d, want 3", got)
	}
	if n := len(cache.entries); n != 0 {
		t.Errorf("drainAndLogout should empty the cache even when logouts fail, %d entries remain", n)
	}
}

// TestRunHTTP_GatewayLogsOutCachedSessionsOnShutdown is the end-to-end guard for
// issue #5: it starts the real HTTP server in gateway mode, drives one gateway
// request that logs in and caches a token, cancels the context, and asserts that
// runHTTP does not return until the cached Centreon session has been logged out.
// This pins the shutdownDone wait: without it, runHTTP would return before the
// drain completes and the logout would be lost.
func TestRunHTTP_GatewayLogsOutCachedSessionsOnShutdown(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	var logins, logouts atomic.Int32
	var loggedOutToken atomic.Value
	loggedOutToken.Store("")

	fakeMux := http.NewServeMux()
	fakeMux.HandleFunc("POST /centreon/api/latest/login", func(w http.ResponseWriter, _ *http.Request) {
		n := logins.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"security": map[string]any{"token": fmt.Sprintf("tok-%d", n)}})
	})
	fakeMux.HandleFunc("GET /centreon/api/latest/logout", func(w http.ResponseWriter, r *http.Request) {
		logouts.Add(1)
		loggedOutToken.Store(r.Header.Get("X-AUTH-TOKEN"))
		w.WriteHeader(http.StatusOK)
	})
	fake := httptest.NewServer(fakeMux)
	defer fake.Close()

	port := freePort(t)
	cfg := &Config{
		Host:      fake.URL, // required field; unused in gateway mode (host comes from headers)
		Transport: transportHTTP,
		AuthMode:  authModeGateway,
		HTTPHost:  "127.0.0.1",
		HTTPPort:  port,
		AllowHTTP: true, // fake Centreon (httptest) is http loopback
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- runHTTP(ctx, cfg, logger, nil) }()

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForHealth(t, base+"/health")

	// One gateway request: the gateway logs in on the fake Centreon and caches the
	// token. The login runs synchronously while building the per-request server.
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build /mcp request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("X-Centreon-Host", fake.URL)
	req.Header.Set("X-Centreon-Username", "admin")
	req.Header.Set("X-Centreon-Password", "secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	_ = resp.Body.Close()

	if got := logins.Load(); got != 1 {
		t.Fatalf("gateway request should trigger exactly one login, got %d", got)
	}

	// Graceful shutdown: runHTTP must not return until the drain has completed.
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("runHTTP returned error on shutdown: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("runHTTP did not return within 15s after context cancel")
	}

	if got := logouts.Load(); got != 1 {
		t.Fatalf("cached session should be logged out before runHTTP returns, got %d logouts", got)
	}
	if tok, _ := loggedOutToken.Load().(string); tok != "tok-1" {
		t.Errorf("shutdown logout should target the cached token, got %q", tok)
	}
}

// TestRunHTTP_LogsOutEnvClientOnBindError pins that when the HTTP listener fails
// to start, runHTTP still logs out an env-mode client that already logged in at
// startup (rather than leaking that Centreon session) and returns the bind error.
func TestRunHTTP_LogsOutEnvClientOnBindError(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	var logouts atomic.Int32
	fakeMux := http.NewServeMux()
	fakeMux.HandleFunc("POST /centreon/api/latest/login", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"security": map[string]any{"token": "tok-env"}})
	})
	fakeMux.HandleFunc("GET /centreon/api/latest/logout", func(w http.ResponseWriter, _ *http.Request) {
		logouts.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	fake := httptest.NewServer(fakeMux)
	defer fake.Close()

	// Hold a port open so runHTTP's ListenAndServe fails to bind it.
	var lc net.ListenConfig
	occupied, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupy port: %v", err)
	}
	defer func() { _ = occupied.Close() }()
	addr, ok := occupied.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address is not *net.TCPAddr: %T", occupied.Addr())
	}

	cfg := &Config{
		Host:      fake.URL,
		Username:  "admin",
		Password:  "secret",
		Transport: transportHTTP,
		AuthMode:  authModeEnv,
		HTTPHost:  "127.0.0.1",
		HTTPPort:  addr.Port,
	}

	if err := runHTTP(t.Context(), cfg, logger, nil); err == nil {
		t.Fatal("runHTTP should return the bind error, got nil")
	}
	if got := logouts.Load(); got != 1 {
		t.Errorf("env client should be logged out on bind error, got %d logouts", got)
	}
}

// freePort reserves an ephemeral TCP port and releases it, returning the number
// so runHTTP can bind it. A small TOCTOU window is acceptable for a local test.
func freePort(t *testing.T) int {
	t.Helper()
	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve free port: %v", err)
	}
	addr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		_ = l.Close()
		t.Fatalf("listener address is not *net.TCPAddr: %T", l.Addr())
	}
	port := addr.Port
	_ = l.Close()
	return port
}

// waitForHealth polls the /health endpoint until it returns 200 or the deadline
// elapses, so the test only proceeds once runHTTP is actually serving.
func waitForHealth(t *testing.T, url string) {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
		if err != nil {
			t.Fatalf("build health request: %v", err)
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("server did not become healthy within 10s")
}

// TestGatewayServer_TokenCachePinsPassword is an end-to-end regression guard
// for #26: it drives gatewayServer against a fake Centreon login endpoint and
// proves the request password actually flows into the cache key. It counts real
// login attempts: the correct password logs in once and is then served from
// cache (no second login), while a wrong password must NOT reuse the cached
// token, so it forces a fresh login that the fake server rejects, and the
// request is denied. If gatewayServer passed anything other than the real
// password into Get/Set, the wrong-password request would reuse the cached
// token and this test would fail.
func TestGatewayServer_TokenCachePinsPassword(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	const goodPass = "correct-horse"
	var loginCount int32

	mux := http.NewServeMux()
	mux.HandleFunc("POST /centreon/api/latest/login", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&loginCount, 1)
		var body struct {
			Security struct {
				Credentials struct {
					Login    string `json:"login"`
					Password string `json:"password"`
				} `json:"credentials"`
			} `json:"security"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if body.Security.Credentials.Password != goodPass {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 1, "message": "bad credentials"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"security": map[string]any{"token": "tok-123"}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := &Config{AllowHTTP: true} // no allowlist restriction; httptest server is http loopback
	cache := NewTokenCache(time.Minute)

	req := func(pass string) *http.Request {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", http.NoBody)
		r.Header.Set("X-Centreon-Host", srv.URL)
		r.Header.Set("X-Centreon-Username", "admin")
		r.Header.Set("X-Centreon-Password", pass)
		return r
	}

	// Request 1: correct password -> one real login, token cached, server built.
	if s := gatewayServer(req(goodPass), cfg, cache, logger, nil); s == nil {
		t.Fatal("request 1 (correct password): expected a server, got nil")
	}
	if n := atomic.LoadInt32(&loginCount); n != 1 {
		t.Fatalf("request 1: expected exactly 1 login, got %d", n)
	}

	// Request 2: same correct password -> cache hit, no new login.
	if s := gatewayServer(req(goodPass), cfg, cache, logger, nil); s == nil {
		t.Fatal("request 2 (cached): expected a server, got nil")
	}
	if n := atomic.LoadInt32(&loginCount); n != 1 {
		t.Fatalf("request 2: expected cache hit (still 1 login), got %d", n)
	}

	// Request 3: wrong password -> cache miss -> fresh login -> 401 -> denied.
	if s := gatewayServer(req("wrong-pass"), cfg, cache, logger, nil); s != nil {
		t.Error("request 3 (wrong password): expected nil (denied), got a server")
	}
	if n := atomic.LoadInt32(&loginCount); n != 2 {
		t.Fatalf("request 3: wrong password must force a fresh login (expected 2 total), got %d", n)
	}
}

// callToolText extracts the first text content from a tool result.
func callToolText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("first content is %T, want *mcp.TextContent", res.Content[0])
	}
	return tc.Text
}

// TestGatewayServer_ToolResponseReportsRedactedHost is an end-to-end guard for
// #48: it drives gatewayServer with an X-Centreon-Host that embeds credentials,
// then calls centreon_connection_test over an in-memory MCP session and asserts
// the response names the per-request host with ALL userinfo stripped, leaking
// neither the username, the password, nor the token. This pins both the gateway
// threading and the display-host sanitization at the gateway buildServer call
// site: passing safeHost(host) leaks the username, passing the raw host leaks the
// password, and passing cfg.Host (the gateway placeholder) drops the loopback
// host:port. Each regression turns this red.
func TestGatewayServer_ToolResponseReportsRedactedHost(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	const (
		secret = "sup3r-s3cret-pass"
		token  = "tok-abc"
	)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /centreon/api/latest/monitoring/hosts/status", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	hostPort := strings.TrimPrefix(srv.URL, "http://")
	hostWithCreds := "http://gwuser:" + secret + "@" + hostPort

	cfg := &Config{AllowHTTP: true} // empty allowlist accepts any host; loopback http
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", http.NoBody)
	req.Header.Set("X-Centreon-Host", hostWithCreds)
	req.Header.Set("X-Centreon-Token", token)

	s := gatewayServer(req, cfg, NewTokenCache(time.Minute), logger, nil)
	if s == nil {
		t.Fatal("gatewayServer returned nil")
	}

	ctx := t.Context()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	ss, err := s.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })

	c := mcp.NewClient(&mcp.Implementation{Name: "issue48-test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "centreon_connection_test"})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if res.IsError {
		t.Fatalf("connection test reported an error: %s", callToolText(t, res))
	}

	text := callToolText(t, res)
	if want := "http://" + hostPort; !strings.Contains(text, want) {
		t.Errorf("response should name the per-request host %q, got: %q", want, text)
	}
	if strings.Contains(text, "gwuser") {
		t.Errorf("response leaked the username, got: %q", text)
	}
	if strings.Contains(text, secret) {
		t.Errorf("response leaked the password, got: %q", text)
	}
	if strings.Contains(text, token) {
		t.Errorf("response leaked the token, got: %q", text)
	}
}

// --- Issue #36: cross-host redirects must not leak the X-AUTH-TOKEN header ---

// TestNoCrossHostRedirect pins the redirect policy directly: redirects that stay
// on the original request host are allowed (so trailing-slash normalisation,
// http->https upgrade, and same-host port changes keep working), and a redirect
// to a different host is refused. The anchor is the ORIGINAL request (via[0]),
// not the previous hop, so a chain cannot be walked off the original host one
// same-looking hop at a time.
func TestNoCrossHostRedirect(t *testing.T) {
	t.Parallel()

	req := func(rawURL string) *http.Request {
		return httptest.NewRequestWithContext(t.Context(), http.MethodGet, rawURL, http.NoBody)
	}

	tests := []struct {
		name    string
		target  string
		via     []*http.Request
		wantErr string // substring to find; "" means expect nil
	}{
		{"same host is allowed", "http://centreon.example.com/api", []*http.Request{req("http://centreon.example.com/login")}, ""},
		{"http to https on same host is allowed", "https://centreon.example.com/api", []*http.Request{req("http://centreon.example.com/login")}, ""},
		{"same host different port is allowed", "https://centreon.example.com:8443/api", []*http.Request{req("http://centreon.example.com:8080/login")}, ""},
		{"host match is case-insensitive", "http://Centreon.Example.COM/api", []*http.Request{req("http://centreon.example.com/login")}, ""},
		{"empty via is allowed defensively", "http://centreon.example.com/api", nil, ""},
		{"different host is refused", "http://evil.example.com/api", []*http.Request{req("http://centreon.example.com/login")}, "cross-host redirect"},
		// Discriminates the via[0] anchor from a previous-hop anchor: the last hop
		// matches the target, so anchoring on the previous hop would WRONGLY allow
		// this; only anchoring on the original host (via[0]) refuses it.
		{"anchored on the original host across a hostname change", "http://mid.example.com/x", []*http.Request{req("http://centreon.example.com/"), req("http://mid.example.com/a")}, "cross-host redirect"},
		// Host-spoof via userinfo: url.Parse puts "centreon.example.com" in the
		// userinfo and the real host is evil.example.com. A naive raw-string or
		// substring check on the URL would be fooled by the userinfo; Hostname()
		// returns the real host, so the guard must still refuse.
		{"userinfo in the target does not spoof the host", "http://centreon.example.com@evil.example.com/", []*http.Request{req("http://centreon.example.com/login")}, "cross-host redirect"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := noCrossHostRedirect(req(tt.target), tt.via)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("noCrossHostRedirect(%s) = %v, want nil", tt.target, err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Errorf("noCrossHostRedirect(%s) = %v, want error containing %q", tt.target, err, tt.wantErr)
			}
		})
	}
}

// TestNoCrossHostRedirect_CapsChain pins that the policy stops a chain after
// maxRedirects hops, matching net/http's default cap that a custom CheckRedirect
// would otherwise remove. The target is same-host so ONLY the length cap can
// produce the error: if the cap check is missing, the host check passes and the
// function wrongly returns nil.
func TestNoCrossHostRedirect_CapsChain(t *testing.T) {
	t.Parallel()

	via := make([]*http.Request, maxRedirects)
	for i := range via {
		via[i] = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://centreon.example.com/", http.NoBody)
	}
	err := noCrossHostRedirect(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://centreon.example.com/next", http.NoBody), via)
	if err == nil || !strings.Contains(err.Error(), "stopped after") {
		t.Errorf("noCrossHostRedirect with %d prior hops = %v, want a redirect-cap error", maxRedirects, err)
	}
}

// TestNewHTTPClient_Wiring pins that every client the server builds carries the
// redirect policy and the shared timeout, and that the self-signed variant
// disables TLS verification on a cloned transport.
func TestNewHTTPClient_Wiring(t *testing.T) {
	t.Parallel()

	plain, err := newHTTPClient(false)
	if err != nil {
		t.Fatalf("newHTTPClient(false): %v", err)
	}
	if plain.CheckRedirect == nil {
		t.Error("plain client must set CheckRedirect so redirects are policed")
	}
	if plain.Timeout != httpClientTimeout {
		t.Errorf("plain client timeout = %v, want %v", plain.Timeout, httpClientTimeout)
	}

	selfSigned, err := newHTTPClient(true)
	if err != nil {
		t.Fatalf("newHTTPClient(true): %v", err)
	}
	if selfSigned.CheckRedirect == nil {
		t.Error("self-signed client must also set CheckRedirect")
	}
	transport, ok := selfSigned.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("self-signed client transport = %T, want *http.Transport", selfSigned.Transport)
	}
	if transport.TLSClientConfig == nil || !transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("self-signed client must skip TLS verification")
	}
}

// isolatedRedirectClient returns the guarded HTTP client that newHTTPClient(false)
// builds, but on its OWN transport rather than the shared http.DefaultTransport.
// The redirect tests run with t.Parallel() and each starts an httptest.Server, and
// httptest.Server.Close() calls http.DefaultTransport.CloseIdleConnections()
// (verified in Go 1.26 net/http/httptest/server.go). A sibling test's deferred
// Close() can therefore tear down an in-flight request this test is running on the
// shared default transport, surfacing "transport connection broken: http:
// CloseIdleConnections called" instead of the CheckRedirect error the test asserts
// (issue #54). An isolated transport removes that cross-test interference, and
// disabling keep-alives keeps a connection from lingering; CheckRedirect, the guard
// under test, is set on the client by newHTTPClient and is left untouched.
func isolatedRedirectClient(t *testing.T) *http.Client {
	t.Helper()
	hc, err := newHTTPClient(false)
	if err != nil {
		t.Fatalf("newHTTPClient: %v", err)
	}
	hc.Transport = &http.Transport{DisableKeepAlives: true}
	return hc
}

// TestNewHTTPClient_FollowsSameHostRedirect proves the policy does not
// over-block: a same-host 302 is still followed to completion through the real
// client newHTTPClient builds.
func TestNewHTTPClient_FollowsSameHostRedirect(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/final", http.StatusFound)
	})
	mux.HandleFunc("/final", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	hc := isolatedRedirectClient(t)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/start", http.NoBody)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		t.Fatalf("same-host redirect should be followed, got error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200 after same-host redirect, got %d", resp.StatusCode)
	}
}

// TestNewHTTPClient_BlocksCrossHostRedirect proves the token-leak vector is
// closed end to end: a redirect to a different host is refused before the client
// dials the target, so the X-AUTH-TOKEN header never reaches another origin.
// other.invalid is unresolvable by RFC 2606, so the test only passes if
// CheckRedirect rejects the hop before any dial or DNS lookup is attempted.
func TestNewHTTPClient_BlocksCrossHostRedirect(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://other.invalid/steal", http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	hc := isolatedRedirectClient(t)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/start", http.NoBody)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := hc.Do(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatal("cross-host redirect must be refused, got nil error")
	}
	if !strings.Contains(err.Error(), "cross-host redirect") {
		t.Errorf("want a cross-host redirect error, got: %v", err)
	}
}

// TestNewHTTPClient_StopsRedirectLoop proves a same-host redirect loop is bounded
// by the maxRedirects cap through the real client instead of spinning forever.
func TestNewHTTPClient_StopsRedirectLoop(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/loop", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop", http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	hc := isolatedRedirectClient(t)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/loop", http.NoBody)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := hc.Do(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatal("same-host redirect loop must be stopped, got nil error")
	}
	if !strings.Contains(err.Error(), "stopped after") {
		t.Errorf("want a redirect-cap error, got: %v", err)
	}
}

// TestRun_EnvLoginRefusesCrossHostRedirect pins the always-supply-client change:
// run must build the guarded HTTP client for every mode, so an env-mode startup
// login whose endpoint redirects to another host fails with the redirect guard
// rather than following the redirect and leaking the credentials. The login
// fails before any listener binds, so no port is needed.
func TestRun_EnvLoginRefusesCrossHostRedirect(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.DiscardHandler)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /centreon/api/latest/login", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://other.invalid/", http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := &Config{
		Host:      srv.URL,
		Username:  "admin",
		Password:  "secret",
		Transport: transportHTTP,
		AuthMode:  authModeEnv,
		HTTPHost:  "127.0.0.1",
	}

	err := run(t.Context(), cfg, logger)
	if err == nil {
		t.Fatal("run should fail when the startup login redirects cross-host, got nil")
	}
	if !strings.Contains(err.Error(), "centreon login") || !strings.Contains(err.Error(), "cross-host redirect") {
		t.Errorf("want a login error caused by the cross-host redirect guard, got: %v", err)
	}
}

// TestNewCentreonClient_AllowSelfSignedDoesNotBypassRedirectGuard pins issue #39:
// newCentreonClient must route every request through the shared, redirect-guarded
// httpClient it is given and must NOT read cfg.AllowSelfSigned. Self-signed TLS is
// applied once on that shared client by newHTTPClient; wiring cfg.AllowSelfSigned
// into a per-client transport here would build a second client that bypasses the
// cross-host redirect guard and reopen the X-AUTH-TOKEN leak (CWE-522, #36). So
// even with AllowSelfSigned set, a cross-host login redirect must still be refused.
func TestNewCentreonClient_AllowSelfSignedDoesNotBypassRedirectGuard(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /centreon/api/latest/login", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://other.invalid/steal", http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	httpClient := isolatedRedirectClient(t)

	// AllowSelfSigned is deliberately set: newCentreonClient must ignore it and use
	// the guarded httpClient, so the cross-host redirect is still refused.
	client, err := newCentreonClient(srv.URL, &Config{Username: "admin", Password: "secret", AllowSelfSigned: true}, nil, httpClient)
	if err != nil {
		t.Fatalf("newCentreonClient: %v", err)
	}

	err = client.Login(t.Context())
	if err == nil {
		t.Fatal("login must refuse the cross-host redirect via the shared guarded client, got nil")
	}
	if !strings.Contains(err.Error(), "cross-host redirect") {
		t.Errorf("want a cross-host redirect error (shared guard in effect despite AllowSelfSigned), got: %v", err)
	}
}

// TestWarnIfAllowlistIneffective pins issue #31: a configured
// CENTREON_ALLOWED_HOSTS is only consulted in gateway mode (http transport with
// gateway auth). In every other mode a set value is silently a no-op, so startup
// must emit exactly one Warn line to signal the least-surprise gap; in the
// enforcing mode, and whenever no allowlist is set, it must stay silent.
func TestWarnIfAllowlistIneffective(t *testing.T) {
	t.Parallel()

	const marker = "CENTREON_ALLOWED_HOSTS is set but"
	allowlist := []string{"https://a.example.com"}

	tests := []struct {
		name      string
		hosts     []string
		transport string
		authMode  string
		wantWarn  bool
	}{
		{"set, stdio+env: inert, warns", allowlist, transportStdio, authModeEnv, true},
		{"set, http+env: inert, warns", allowlist, transportHTTP, authModeEnv, true},
		{"set, stdio+gateway: inert, warns", allowlist, transportStdio, authModeGateway, true},
		{"set, http+gateway: enforced, silent", allowlist, transportHTTP, authModeGateway, false},
		{"unset, http+gateway: silent", nil, transportHTTP, authModeGateway, false},
		{"unset, stdio+env: silent", nil, transportStdio, authModeEnv, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
			cfg := &Config{AllowedHosts: tt.hosts, Transport: tt.transport, AuthMode: tt.authMode}

			warnIfAllowlistIneffective(cfg, logger)

			wantCount := 0
			if tt.wantWarn {
				wantCount = 1
			}
			if gotCount := strings.Count(buf.String(), marker); gotCount != wantCount {
				t.Errorf("warn lines = %d, want %d; log=%q", gotCount, wantCount, buf.String())
			}
		})
	}
}

// TestRunHTTP_EnvToolResponseReportsRedactedHost pins the env-auth display sink in
// runHTTP (envDisplayHost = displayHost(cfg.Host), issue #64): a CENTREON_HOST that
// embeds user:pass must reach a tool response with all userinfo stripped. It drives the
// real runHTTP server over the streamable HTTP transport, so it dies both when the call
// site stops calling displayHost (the raw credential appears in the response) and when
// displayHost degenerates to the placeholder (the loopback host:port disappears).
func TestRunHTTP_EnvToolResponseReportsRedactedHost(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	const secret = "sup3r-s3cret-pass"

	fakeMux := http.NewServeMux()
	fakeMux.HandleFunc("GET /centreon/api/latest/monitoring/hosts/status", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})
	fake := httptest.NewServer(fakeMux)
	defer fake.Close()

	hostPort := strings.TrimPrefix(fake.URL, "http://")
	hostWithCreds := "http://envuser:" + secret + "@" + hostPort

	port := freePort(t)
	cfg := &Config{
		Host:      hostWithCreds,
		Token:     "tok-env", // token auth: no login round-trip, no shutdown logout
		Transport: transportHTTP,
		AuthMode:  authModeEnv,
		HTTPHost:  "127.0.0.1",
		HTTPPort:  port,
		AllowHTTP: true, // fake Centreon (httptest) is http loopback
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- runHTTP(ctx, cfg, logger, nil) }()

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForHealth(t, base+"/health")

	c := mcp.NewClient(&mcp.Implementation{Name: "issue64-env-test", Version: "0"}, nil)
	// DisableStandaloneSSE: this test only needs request/response (one CallTool), and
	// leaving the persistent server-initiated SSE stream open would block
	// httpServer.Shutdown on cancel until it times out.
	cs, err := c.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: base + "/mcp", DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "centreon_connection_test"})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if res.IsError {
		t.Fatalf("connection test reported an error: %s", callToolText(t, res))
	}
	text := callToolText(t, res)
	if want := "http://" + hostPort; !strings.Contains(text, want) {
		t.Errorf("response should name the env host %q, got: %q", want, text)
	}
	if strings.Contains(text, "envuser") {
		t.Errorf("response leaked the username, got: %q", text)
	}
	if strings.Contains(text, secret) {
		t.Errorf("response leaked the password, got: %q", text)
	}

	cancel()
	select {
	case <-errCh:
	case <-time.After(15 * time.Second):
		t.Fatal("runHTTP did not return within 15s after context cancel")
	}
}

// TestRunStdio_ToolResponseReportsRedactedHost pins the stdio display sink in runStdio
// (buildServer(client, logger, displayHost(cfg.Host)), issue #64): a CENTREON_HOST that
// embeds user:pass must reach a tool response with all userinfo stripped. runStdio takes
// its transport as a parameter, so the test drives it over an in-memory transport pair
// (the same seam TestGatewayServer_ToolResponseReportsRedactedHost uses) rather than
// mutating the process's os.Stdin/os.Stdout. It dies when the call site stops calling
// displayHost (the raw credential appears in the response) and when displayHost
// degenerates to the placeholder (the loopback host:port disappears).
func TestRunStdio_ToolResponseReportsRedactedHost(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	const secret = "sup3r-s3cret-pass"

	fakeMux := http.NewServeMux()
	fakeMux.HandleFunc("GET /centreon/api/latest/monitoring/hosts/status", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})
	fake := httptest.NewServer(fakeMux)
	defer fake.Close()

	hostPort := strings.TrimPrefix(fake.URL, "http://")
	hostWithCreds := "http://stdiouser:" + secret + "@" + hostPort

	cfg := &Config{
		Host:      hostWithCreds,
		Token:     "tok-stdio", // token auth: no login/logout round-trips
		Transport: transportStdio,
		AllowHTTP: true,
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel() // guarantees runStdio returns on every exit path, including t.Fatalf

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	errCh := make(chan error, 1)
	go func() { errCh <- runStdio(ctx, cfg, logger, nil, serverTransport) }()

	c := mcp.NewClient(&mcp.Implementation{Name: "issue64-stdio-test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "centreon_connection_test"})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if res.IsError {
		t.Fatalf("connection test reported an error: %s", callToolText(t, res))
	}
	text := callToolText(t, res)
	if want := "http://" + hostPort; !strings.Contains(text, want) {
		t.Errorf("response should name the configured host %q, got: %q", want, text)
	}
	if strings.Contains(text, "stdiouser") {
		t.Errorf("response leaked the username, got: %q", text)
	}
	if strings.Contains(text, secret) {
		t.Errorf("response leaked the password, got: %q", text)
	}

	cancel()
	select {
	case <-errCh:
	case <-time.After(15 * time.Second):
		t.Fatal("runStdio did not return within 15s after context cancel")
	}
}

func TestRunStdio_TokenPreflightRejectsBadToken(t *testing.T) {
	fakeMux := http.NewServeMux()
	fakeMux.HandleFunc("GET /centreon/api/latest/monitoring/hosts/status", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    401,
			"message": "invalid token",
		})
	})
	fake := httptest.NewServer(fakeMux)
	defer fake.Close()

	cfg := &Config{
		Host:      fake.URL,
		Token:     "bad-token",
		AllowHTTP: true,
	}

	err := runStdio(t.Context(), cfg, slog.New(slog.DiscardHandler), nil, nil)
	if err == nil {
		t.Fatal("expected error on bad token preflight, got nil")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "centreon token validation failed") {
		t.Errorf("expected error to contain 'centreon token validation failed', got: %v", err)
	}
	if !strings.Contains(errStr, "HTTP 401") {
		t.Errorf("expected error to contain 'HTTP 401', got: %v", err)
	}
	if strings.Contains(errStr, "bad-token") {
		t.Errorf("error leaked token (CWE-532): %v", err)
	}
}

func TestRunStdio_TokenPreflightSuccessProceeds(t *testing.T) {
	var statusHits atomic.Int32

	fakeMux := http.NewServeMux()
	fakeMux.HandleFunc("GET /centreon/api/latest/monitoring/hosts/status", func(w http.ResponseWriter, _ *http.Request) {
		statusHits.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})
	fake := httptest.NewServer(fakeMux)
	defer fake.Close()

	cfg := &Config{
		Host:      fake.URL,
		Token:     "valid-tok",
		Transport: transportStdio,
		AllowHTTP: true,
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	errCh := make(chan error, 1)
	go func() { errCh <- runStdio(ctx, cfg, slog.New(slog.DiscardHandler), nil, serverTransport) }()

	c := mcp.NewClient(&mcp.Implementation{Name: "preflight-test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	_ = cs.Close()

	cancel()
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("runStdio returned error: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("runStdio did not return within 15s after context cancel")
	}

	if got := statusHits.Load(); got != 1 {
		t.Errorf("expected status endpoint to be called exactly once during preflight, got %d", got)
	}
}

func TestRunHTTP_EnvTokenPreflightRejectsBadToken(t *testing.T) {
	fakeMux := http.NewServeMux()
	fakeMux.HandleFunc("GET /centreon/api/latest/monitoring/hosts/status", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    401,
			"message": "invalid token",
		})
	})
	fake := httptest.NewServer(fakeMux)
	defer fake.Close()

	port := freePort(t)
	cfg := &Config{
		Host:      fake.URL,
		Transport: transportHTTP,
		AuthMode:  authModeEnv,
		Token:     "bad-token",
		AllowHTTP: true,
		HTTPHost:  "127.0.0.1",
		HTTPPort:  port,
	}

	err := runHTTP(t.Context(), cfg, slog.New(slog.DiscardHandler), nil)
	if err == nil {
		t.Fatal("expected error on bad token preflight, got nil")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "centreon token validation failed") {
		t.Errorf("expected error to contain 'centreon token validation failed', got: %v", err)
	}
	if strings.Contains(errStr, "bad-token") {
		t.Errorf("error leaked token (CWE-532): %v", err)
	}
}

func TestRunHTTP_GatewayModeSkipsStartupPreflight(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	var statusHits, logins atomic.Int32

	fakeMux := http.NewServeMux()
	fakeMux.HandleFunc("GET /centreon/api/latest/monitoring/hosts/status", func(w http.ResponseWriter, _ *http.Request) {
		statusHits.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})
	fakeMux.HandleFunc("POST /centreon/api/latest/login", func(w http.ResponseWriter, _ *http.Request) {
		n := logins.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"security": map[string]any{"token": fmt.Sprintf("tok-%d", n)}})
	})
	fake := httptest.NewServer(fakeMux)
	defer fake.Close()

	port := freePort(t)
	cfg := &Config{
		Host:      fake.URL,
		Transport: transportHTTP,
		AuthMode:  authModeGateway,
		HTTPHost:  "127.0.0.1",
		HTTPPort:  port,
		AllowHTTP: true,
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- runHTTP(ctx, cfg, logger, nil) }()

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForHealth(t, base+"/health")

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("runHTTP returned error on shutdown: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("runHTTP did not return within 15s after context cancel")
	}

	if got := statusHits.Load(); got != 0 {
		t.Errorf("gateway mode should not hit status endpoint on startup, got %d hits", got)
	}
	if got := logins.Load(); got != 0 {
		t.Errorf("gateway mode should not hit login endpoint on startup, got %d logins", got)
	}
}
