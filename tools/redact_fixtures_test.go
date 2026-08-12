package tools

import (
	"errors"
	"net"
	"net/url"
	"strings"
	"testing"
)

// Shared fixtures for the credential-leak tests across the tools package
// (issues #63 and #71). Every tool handler that surfaces a Centreon client-call
// error must route it through redact.Reason so a credential embedded in the
// base URL never reaches the client response or the log.

const (
	// leakPassword is the password marker planted in a mis-parsed base URL.
	leakPassword = "s3cr3tpw"
	// leakUser is the username fragment url.Parse mis-reads as the host.
	leakUser = "leakuser"
)

// misparsedClientErr reproduces the mis-parsed-authority leak measured in #63
// and #71: a '/' inside the password makes url.Parse read leakUser as the host
// and decode no userinfo, so http.Client prints the whole typed credential in
// its *url.Error and the wrapped *net.DNSError.Name is the username fragment.
func misparsedClientErr() error {
	return &url.Error{
		Op:  "Get",
		URL: "https://" + leakUser + ":1234/" + leakPassword + "@centreon.invalid/monitoring/hosts",
		Err: &net.DNSError{Err: "no such host", Name: leakUser, IsNotFound: true},
	}
}

// wellFormedClientErr carries a correctly-formed userinfo URL. In production Go's
// stripPassword would mask the password on such a URL but leave the username, so
// an unredacted sink leaks at least leakUser. This fixture hardcodes the raw URL
// into *url.Error.URL (so both fragments are literally present); the point is
// that redact.Reason must classify it so neither fragment reaches a sink.
func wellFormedClientErr() error {
	return &url.Error{
		Op:  "Get",
		URL: "https://" + leakUser + ":" + leakPassword + "@centreon.example.com/monitoring/hosts",
		Err: errors.New("dial tcp: connection refused"),
	}
}

// assertNoCredential fails when s carries either planted credential fragment.
func assertNoCredential(t *testing.T, s string) {
	t.Helper()
	if strings.Contains(s, leakPassword) {
		t.Errorf("leaked password marker (CWE-532): %q", s)
	}
	if strings.Contains(s, leakUser) {
		t.Errorf("leaked username (CWE-532): %q", s)
	}
}
