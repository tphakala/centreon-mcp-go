package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDoctorReport_HealthyTokenMode(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /centreon/api/latest/monitoring/hosts/status", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})
	mux.HandleFunc("GET /centreon/api/latest/platform/versions", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"web": map[string]any{
				"version": "24.10.3",
				"major":   "24",
				"minor":   "10",
				"fix":     "3",
			},
			"modules": map[string]any{},
			"widgets": map[string]any{},
		})
	})
	fake := httptest.NewServer(mux)
	defer fake.Close()

	cfg := &Config{
		Host:      fake.URL,
		Token:     "tok",
		AllowHTTP: true,
		Transport: transportStdio,
		AuthMode:  authModeEnv,
	}

	var buf bytes.Buffer
	code := doctorReport(t.Context(), cfg, fake.Client(), &buf)
	if code != doctorExitHealthy {
		t.Fatalf("expected exit code %d, got %d. Output:\n%s", doctorExitHealthy, code, buf.String())
	}

	out := buf.String()
	if !strings.Contains(out, "config: ok") {
		t.Errorf("expected output to contain 'config: ok', got:\n%s", out)
	}
	if !strings.Contains(out, "connectivity: ok") {
		t.Errorf("expected output to contain 'connectivity: ok', got:\n%s", out)
	}
	if !strings.Contains(out, "credentials: ok") {
		t.Errorf("expected output to contain 'credentials: ok', got:\n%s", out)
	}
	if !strings.Contains(out, "centreon version: 24.10.3") {
		t.Errorf("expected output to contain 'centreon version: 24.10.3', got:\n%s", out)
	}
	if !strings.Contains(out, "doctor: healthy") {
		t.Errorf("expected output to contain 'doctor: healthy', got:\n%s", out)
	}
}

func TestDoctorReport_HealthyPasswordMode(t *testing.T) {
	var logoutCalled atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("POST /centreon/api/latest/login", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"security": map[string]any{
				"token": "tok-1",
			},
		})
	})
	mux.HandleFunc("GET /centreon/api/latest/logout", func(w http.ResponseWriter, _ *http.Request) {
		logoutCalled.Store(true)
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /centreon/api/latest/platform/versions", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"web": map[string]any{
				"version": "24.10.3",
				"major":   "24",
				"minor":   "10",
				"fix":     "3",
			},
			"modules": map[string]any{},
			"widgets": map[string]any{},
		})
	})
	fake := httptest.NewServer(mux)
	defer fake.Close()

	cfg := &Config{
		Host:      fake.URL,
		Username:  "admin",
		Password:  "secret",
		AllowHTTP: true,
		Transport: transportStdio,
		AuthMode:  authModeEnv,
	}

	var buf bytes.Buffer
	code := doctorReport(t.Context(), cfg, fake.Client(), &buf)
	if code != doctorExitHealthy {
		t.Fatalf("expected exit code %d, got %d. Output:\n%s", doctorExitHealthy, code, buf.String())
	}
	if !logoutCalled.Load() {
		t.Error("expected logout to be called during healthy password doctor check")
	}

	out := buf.String()
	if !strings.Contains(out, "doctor: healthy") {
		t.Errorf("expected output to contain 'doctor: healthy', got:\n%s", out)
	}
}

func runBadCredentialsTest(t *testing.T, statusCode int, useToken bool) {
	t.Helper()
	mux := http.NewServeMux()
	if useToken {
		mux.HandleFunc("GET /centreon/api/latest/monitoring/hosts/status", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(statusCode)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    statusCode,
				"message": "auth error",
			})
		})
	} else {
		mux.HandleFunc("POST /centreon/api/latest/login", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(statusCode)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    statusCode,
				"message": "auth error",
			})
		})
	}
	fake := httptest.NewServer(mux)
	defer fake.Close()

	cfg := &Config{
		Host:      fake.URL,
		AllowHTTP: true,
		Transport: transportStdio,
		AuthMode:  authModeEnv,
	}
	if useToken {
		cfg.Token = "bad-tok"
	} else {
		cfg.Username = "admin"
		cfg.Password = "bad-pass"
	}

	var buf bytes.Buffer
	code := doctorReport(t.Context(), cfg, fake.Client(), &buf)
	if code != doctorExitBadCredentials {
		t.Fatalf("expected exit code %d, got %d. Output:\n%s", doctorExitBadCredentials, code, buf.String())
	}

	out := buf.String()
	wantReason := fmt.Sprintf("credentials: FAIL: centreon API error (HTTP %d)", statusCode)
	if !strings.Contains(out, wantReason) {
		t.Errorf("expected output to contain %q, got:\n%s", wantReason, out)
	}
	if !strings.Contains(out, "connectivity: ok (API reached)") {
		t.Errorf("expected output to contain 'connectivity: ok (API reached)', got:\n%s", out)
	}
	if !strings.Contains(out, "doctor: unhealthy") {
		t.Errorf("expected output to contain 'doctor: unhealthy', got:\n%s", out)
	}
}

func TestDoctorReport_BadCredentials(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		useToken   bool
	}{
		{"token mode 401", http.StatusUnauthorized, true},
		{"token mode 403", http.StatusForbidden, true},
		{"password mode 401", http.StatusUnauthorized, false},
		{"password mode 403", http.StatusForbidden, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runBadCredentialsTest(t, tt.statusCode, tt.useToken)
		})
	}
}

func TestDoctorReport_Unreachable(t *testing.T) {
	cfg := &Config{
		Host:      "https://unreachable.example.com",
		Token:     "tok",
		Transport: transportStdio,
		AuthMode:  authModeEnv,
	}

	var buf bytes.Buffer
	code := doctorReport(t.Context(), cfg, failDNSClient(), &buf)
	if code != doctorExitUnreachable {
		t.Fatalf("expected exit code %d, got %d. Output:\n%s", doctorExitUnreachable, code, buf.String())
	}

	out := buf.String()
	if !strings.Contains(out, "connectivity: FAIL:") {
		t.Errorf("expected output to contain 'connectivity: FAIL:', got:\n%s", out)
	}
	if !strings.Contains(out, "credentials: skipped (platform unreachable)") {
		t.Errorf("expected output to contain 'credentials: skipped (platform unreachable)', got:\n%s", out)
	}
	if !strings.Contains(out, "doctor: unhealthy") {
		t.Errorf("expected output to contain 'doctor: unhealthy', got:\n%s", out)
	}
}

func TestDoctorReport_VersionUnavailableStaysHealthy(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /centreon/api/latest/monitoring/hosts/status", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})
	mux.HandleFunc("GET /centreon/api/latest/platform/versions", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    404,
			"message": "not found",
		})
	})
	fake := httptest.NewServer(mux)
	defer fake.Close()

	cfg := &Config{
		Host:      fake.URL,
		Token:     "tok",
		AllowHTTP: true,
		Transport: transportStdio,
		AuthMode:  authModeEnv,
	}

	var buf bytes.Buffer
	code := doctorReport(t.Context(), cfg, fake.Client(), &buf)
	if code != doctorExitHealthy {
		t.Fatalf("expected exit code %d, got %d. Output:\n%s", doctorExitHealthy, code, buf.String())
	}

	out := buf.String()
	if !strings.Contains(out, "centreon version: unavailable") {
		t.Errorf("expected output to contain 'centreon version: unavailable', got:\n%s", out)
	}
	if !strings.Contains(out, "doctor: healthy") {
		t.Errorf("expected output to contain 'doctor: healthy', got:\n%s", out)
	}
}

func TestDoctorReport_NeverPrintsSecret(t *testing.T) {
	cfg := &Config{
		Host:      misparsedHost,
		Token:     leakMarker,
		Transport: transportStdio,
		AuthMode:  authModeEnv,
	}

	var buf bytes.Buffer
	doctorReport(t.Context(), cfg, failDNSClient(), &buf)

	out := buf.String()
	if strings.Contains(out, leakMarker) {
		t.Errorf("doctor output leaked secret token (CWE-532): %s", out)
	}
	if strings.Contains(out, "admin:1234") {
		t.Errorf("doctor output leaked misparsedHost userinfo: %s", out)
	}
}

func TestDoctorReport_GatewayModeSkipsChecks(t *testing.T) {
	cfg := &Config{
		Host:      "https://placeholder.example",
		Transport: transportHTTP,
		AuthMode:  authModeGateway,
		Token:     "tok",
	}

	client := &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			t.Fatal("unexpected HTTP request made in gateway mode")
			return nil, errors.New("unexpected HTTP request made in gateway mode")
		}),
	}

	var buf bytes.Buffer
	code := doctorReport(t.Context(), cfg, client, &buf)
	if code != doctorExitHealthy {
		t.Fatalf("expected exit code %d, got %d. Output:\n%s", doctorExitHealthy, code, buf.String())
	}

	out := buf.String()
	if !strings.Contains(out, "connectivity: skipped (gateway mode: credentials arrive per request)") {
		t.Errorf("expected connectivity skipped line, got:\n%s", out)
	}
	if !strings.Contains(out, "credentials: skipped (gateway mode)") {
		t.Errorf("expected credentials skipped line, got:\n%s", out)
	}
	if !strings.Contains(out, "doctor: healthy (config only)") {
		t.Errorf("expected 'doctor: healthy (config only)', got:\n%s", out)
	}
}

func TestRunDoctor_ConfigErrorExitsOne(t *testing.T) {
	t.Setenv("CENTREON_HOST", "")
	t.Setenv("CENTREON_USERNAME", "")
	t.Setenv("CENTREON_PASSWORD", "")
	t.Setenv("CENTREON_TOKEN", "")
	t.Setenv("CENTREON_PASSWORD_FILE", "")
	t.Setenv("CENTREON_TOKEN_FILE", "")

	var buf bytes.Buffer
	code := runDoctor(t.Context(), &buf)
	if code != doctorExitConfigError {
		t.Fatalf("expected exit code %d, got %d", doctorExitConfigError, code)
	}

	out := buf.String()
	if !strings.HasPrefix(out, "config: FAIL:") {
		t.Errorf("expected output to start with 'config: FAIL:', got:\n%s", out)
	}
}

func TestRunDoctor_TokenFromFileHealthy(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /centreon/api/latest/monitoring/hosts/status", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-AUTH-TOKEN"); got != "tok" {
			t.Errorf("expected X-AUTH-TOKEN 'tok', got %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})
	mux.HandleFunc("GET /centreon/api/latest/platform/versions", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"web": map[string]any{
				"version": "24.10.3",
				"major":   "24",
				"minor":   "10",
				"fix":     "3",
			},
			"modules": map[string]any{},
			"widgets": map[string]any{},
		})
	})
	fake := httptest.NewServer(mux)
	defer fake.Close()

	tokenPath := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenPath, []byte("tok\n"), 0o600); err != nil {
		t.Fatalf("write temp token: %v", err)
	}

	t.Setenv("CENTREON_HOST", fake.URL)
	t.Setenv("CENTREON_ALLOW_HTTP", "true")
	t.Setenv("CENTREON_USERNAME", "")
	t.Setenv("CENTREON_PASSWORD", "")
	t.Setenv("CENTREON_TOKEN", "")
	t.Setenv("CENTREON_PASSWORD_FILE", "")
	t.Setenv("CENTREON_TOKEN_FILE", tokenPath)
	t.Setenv("MCP_TRANSPORT", "stdio")
	t.Setenv("AUTH_MODE", "env")

	var buf bytes.Buffer
	code := runDoctor(t.Context(), &buf)
	if code != doctorExitHealthy {
		t.Fatalf("expected exit code %d, got %d. Output:\n%s", doctorExitHealthy, code, buf.String())
	}
}
