// Package redact turns an error into a short, credential-safe description fit
// for a server log or an MCP tool response (CWE-532).
//
// A Centreon base URL can carry userinfo (user:pass@host), from CENTREON_HOST
// or, in gateway mode, from a caller-supplied X-Centreon-Host header. When a
// request against it fails, net/http builds a *url.Error whose text embeds that
// base URL, and url.Error.Error masks only what url.Parse decoded as userinfo:
// a mis-parsed authority (https://admin:1234/pw@host, where url.Parse read
// "admin" as the host and decoded no userinfo) prints the whole typed password,
// and even a well-formed URL leaks the username. Wrapped resolver, TLS, and
// upstream API errors can carry the same bytes. Issues #63 (server log) and #71
// (client response) both need one routine that keeps those bytes out of a sink.
package redact

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"

	centreon "github.com/tphakala/centreon-go-client/v2"
)

// ErrCrossHostRedirect is the sentinel the redirect guard wraps so Reason can
// classify a refused cross-host redirect without importing the guard's package.
var ErrCrossHostRedirect = errors.New("refusing cross-host redirect")

// Reason maps err to a short, fixed description safe for logs and MCP tool
// responses. The hard invariant that makes it fail closed: it never copies a
// string field out of the error chain. It calls no err.Error(), and reads no
// APIError.Message, net.DNSError.Name, or url.Error.URL, because any of those
// can hold the request URL and the credential typed into it, including spans
// url.Parse never decoded as userinfo. Only compile-time strings and the trusted
// HTTP status integer are emitted; an unrecognised error falls through to a
// constant. A new error type nobody anticipated therefore lands on the
// fail-closed default rather than leaking, and adding a leak takes a deliberate
// edit that reads a string field, not an omission.
func Reason(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.Canceled):
		return "request cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return "request timed out"
	case errors.Is(err, ErrCrossHostRedirect):
		return "cross-host redirect refused"
	}

	// Concrete upstream and network types first, so a *net.DNSError wrapped in a
	// *url.Error is classified by the resolver failure ("DNS lookup failed")
	// rather than by the generic *url.Error branch below. Only the integer
	// HTTPStatus is surfaced from an APIError; its Message is the upstream
	// response body, remote-controlled (in gateway mode by an attacker-chosen
	// host), so it is dropped.
	if e, ok := errors.AsType[*centreon.APIError](err); ok {
		return fmt.Sprintf("centreon API error (HTTP %d)", e.HTTPStatus)
	}
	if _, ok := errors.AsType[*centreon.NotFoundError](err); ok {
		return "resource not found"
	}
	if _, ok := errors.AsType[*net.DNSError](err); ok {
		return "DNS lookup failed"
	}
	if _, ok := errors.AsType[*tls.CertificateVerificationError](err); ok {
		return "TLS certificate verification failed"
	}

	// A timeout can arrive as any net.Error (including a *url.Error whose inner
	// error timed out), so check the interface before the concrete URL branches.
	if ne, ok := errors.AsType[net.Error](err); ok && ne.Timeout() {
		return "network timeout"
	}

	// Op is a stdlib constant, never request data, so it is safe to read.
	if e, ok := errors.AsType[*url.Error](err); ok {
		if e.Op == "parse" {
			return "invalid URL"
		}
		return "network error"
	}
	if _, ok := errors.AsType[*net.OpError](err); ok {
		return "network error"
	}

	return "request failed"
}
