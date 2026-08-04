package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
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

// TestGatewayServer_HostAllowlist exercises the enforcement point itself, not
// just the hostAllowed helper: it confirms gatewayServer rejects a host that is
// not on the allowlist (returns nil) and admits one that is. A token is
// supplied so the token branch is taken and no network login is attempted.
func TestGatewayServer_HostAllowlist(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	cache := NewTokenCache(time.Minute)

	newReq := func(host string) *http.Request {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", http.NoBody)
		r.Header.Set("X-Centreon-Host", host)
		r.Header.Set("X-Centreon-Token", "dummy-token")
		return r
	}

	t.Run("host not in allowlist is rejected", func(t *testing.T) {
		cfg := &Config{AllowedHosts: []string{"https://allowed.example.com"}}
		if srv := gatewayServer(newReq("https://evil.example.com"), cfg, cache, logger, nil); srv != nil {
			t.Error("expected nil server for host not in allowlist, got non-nil")
		}
	})

	t.Run("host in allowlist is accepted", func(t *testing.T) {
		cfg := &Config{AllowedHosts: []string{"https://allowed.example.com"}}
		if srv := gatewayServer(newReq("https://allowed.example.com"), cfg, cache, logger, nil); srv == nil {
			t.Error("expected non-nil server for allowed host, got nil")
		}
	})

	t.Run("empty allowlist accepts any host", func(t *testing.T) {
		cfg := &Config{}
		if srv := gatewayServer(newReq("https://anything.example.com"), cfg, cache, logger, nil); srv == nil {
			t.Error("expected non-nil server when allowlist is empty, got nil")
		}
	})
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

	cfg := &Config{} // no allowlist restriction
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
