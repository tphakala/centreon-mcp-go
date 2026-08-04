package main

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
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
