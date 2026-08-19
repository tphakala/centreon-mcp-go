package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	centreon "github.com/tphakala/centreon-go-client/v2"
	"github.com/tphakala/centreon-mcp-go/internal/redact"
)

const (
	doctorExitHealthy        = 0
	doctorExitConfigError    = 1
	doctorExitUnreachable    = 2
	doctorExitBadCredentials = 3
)

// runDoctor implements the "doctor" subcommand: load config, build the same
// guarded HTTP client the server uses, verify connectivity and credentials,
// and report the Centreon version. It prints one line per check to out and
// returns the process exit code. It never prints a password or token: doctor
// formats its own host lines through displayHost (which strips all userinfo), a
// config-load failure is surfaced as LoadConfig returns it (which never carries
// a password or token, though a host it names keeps its username via safeHost),
// and upstream request errors are classified through redact.Reason and the typed
// HTTPStatus field.
func runDoctor(ctx context.Context, out io.Writer) int {
	cfg, err := LoadConfig()
	if err != nil {
		_, _ = fmt.Fprintf(out, "config: FAIL: %v\n", err)
		_, _ = fmt.Fprintln(out, "doctor: unhealthy")
		return doctorExitConfigError
	}
	httpClient, err := newHTTPClient(cfg.AllowSelfSigned)
	if err != nil {
		_, _ = fmt.Fprintf(out, "config: FAIL: creating HTTP client: %v\n", err)
		_, _ = fmt.Fprintln(out, "doctor: unhealthy")
		return doctorExitConfigError
	}
	return doctorReport(ctx, &cfg, httpClient, out)
}

// doctorReport runs the connectivity and credential checks against an already
// loaded config. Split from runDoctor so tests can inject a config and a
// transport-faked HTTP client without touching the environment.
func doctorReport(ctx context.Context, cfg *Config, httpClient *http.Client, out io.Writer) int {
	_, _ = fmt.Fprintf(out, "config: ok (host %s, transport %s, auth mode %s)\n", displayHost(cfg.Host), cfg.Transport, cfg.AuthMode)

	if gatewayMode(cfg) {
		_, _ = fmt.Fprintln(out, "connectivity: skipped (gateway mode: credentials arrive per request)")
		_, _ = fmt.Fprintln(out, "credentials: skipped (gateway mode)")
		_, _ = fmt.Fprintln(out, "doctor: healthy (config only)")
		return doctorExitHealthy
	}

	client, err := newCentreonClient(cfg.Host, cfg, nil, httpClient)
	if err != nil {
		_, _ = fmt.Fprintf(out, "config: FAIL: cannot create client for host %s: %s\n", displayHost(cfg.Host), redact.Reason(err))
		_, _ = fmt.Fprintln(out, "doctor: unhealthy")
		return doctorExitConfigError
	}

	var authErr error
	if cfg.Token == "" {
		authErr = client.Login(ctx)
		if authErr == nil {
			defer logoutClientBounded(ctx, client, slog.New(slog.DiscardHandler))
		}
	} else {
		authErr = checkCredentials(ctx, client)
	}

	if authErr != nil {
		if isAuthRejected(authErr) {
			_, _ = fmt.Fprintln(out, "connectivity: ok (API reached)")
			_, _ = fmt.Fprintf(out, "credentials: FAIL: %s\n", redact.Reason(authErr))
			_, _ = fmt.Fprintln(out, "doctor: unhealthy")
			return doctorExitBadCredentials
		}
		_, _ = fmt.Fprintf(out, "connectivity: FAIL: %s\n", redact.Reason(authErr))
		_, _ = fmt.Fprintln(out, "credentials: skipped (platform unreachable)")
		_, _ = fmt.Fprintln(out, "doctor: unhealthy")
		return doctorExitUnreachable
	}
	_, _ = fmt.Fprintln(out, "connectivity: ok")
	_, _ = fmt.Fprintln(out, "credentials: ok")

	if versions, err := client.Platform.Versions(ctx); err != nil {
		_, _ = fmt.Fprintf(out, "centreon version: unavailable (%s)\n", redact.Reason(err))
	} else {
		_, _ = fmt.Fprintf(out, "centreon version: %s\n", versions.Web.Version)
	}
	_, _ = fmt.Fprintln(out, "doctor: healthy")
	return doctorExitHealthy
}

// isAuthRejected reports whether err is an authentication rejection from the
// Centreon API (HTTP 401 or 403). It reads only the trusted HTTPStatus
// integer of a typed *centreon.APIError, never error text, the same
// fail-closed discipline as internal/redact.Reason and isNotFoundStatus, so
// it cannot leak a credential and an unknown error classifies as unreachable
// rather than as a credential verdict.
func isAuthRejected(err error) bool {
	if apiErr, ok := errors.AsType[*centreon.APIError](err); ok {
		return apiErr.HTTPStatus == http.StatusUnauthorized || apiErr.HTTPStatus == http.StatusForbidden
	}
	return false
}
