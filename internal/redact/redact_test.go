package redact_test

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"testing"

	centreon "github.com/tphakala/centreon-go-client/v2"
	"github.com/tphakala/centreon-mcp-go/internal/redact"
)

// secret is the password marker planted throughout the fixtures. No Reason
// output may ever contain it, nor the username fragment "admin".
const (
	secret = "s3cr3tpw"
	user   = "admin"
)

// misparsedErr reproduces the exact leak measured in issues #63 and #71: a base
// URL whose password holds a '/', so url.Parse reads "admin" as the host and
// never decodes userinfo. http.Client.Do wraps the resolver failure in a
// *url.Error whose URL field prints the whole typed credential, and the wrapped
// *net.DNSError.Name is the credential fragment "admin".
func misparsedErr() error {
	return &url.Error{
		Op:  "Get",
		URL: "https://admin:1234/" + secret + "@centreon.invalid/login",
		Err: &net.DNSError{Err: "no such host", Name: user, IsNotFound: true},
	}
}

// timeoutError is a net.Error whose message embeds the secret and which reports a
// timeout, so it exercises the net.Error timeout branch without matching an
// earlier concrete-type branch.
type timeoutError struct{}

func (timeoutError) Error() string   { return "dial https://" + user + ":" + secret + "@h: i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return false }

func TestReason(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"canceled", fmt.Errorf("wrap: %w", context.Canceled), "request cancelled"},
		{"deadline", fmt.Errorf("wrap: %w", context.DeadlineExceeded), "request timed out"},
		{
			"cross-host redirect",
			fmt.Errorf("outer: %w", fmt.Errorf("%w from %q to %q", redact.ErrCrossHostRedirect, "a", "b")),
			"cross-host redirect refused",
		},
		{
			"api error drops message",
			&centreon.APIError{HTTPStatus: 401, Message: "login as " + user + ":" + secret},
			"centreon API error (HTTP 401)",
		},
		{
			"not found",
			&centreon.NotFoundError{Resource: "host", ID: 7},
			"resource not found",
		},
		{"misparsed dns", misparsedErr(), "DNS lookup failed"},
		{
			"tls verification",
			&url.Error{Op: "Get", URL: "https://" + user + ":" + secret + "@h/x", Err: &tls.CertificateVerificationError{}},
			"TLS certificate verification failed",
		},
		{
			"parse error",
			&url.Error{Op: "parse", URL: "https://u:" + secret + "@", Err: errors.New("missing host")},
			"invalid URL",
		},
		{"net timeout", timeoutError{}, "network timeout"},
		{
			"generic op error",
			&url.Error{Op: "Get", URL: "https://" + user + ":" + secret + "@h/x", Err: &net.OpError{Op: "dial", Err: errors.New("connection refused")}},
			"network error",
		},
		{
			// Bare *net.OpError (not wrapped in a *url.Error): exercises the
			// standalone OpError branch, which the wrapped row above skips.
			"bare op error",
			&net.OpError{Op: "dial", Err: errors.New("connection refused")},
			"network error",
		},
		{
			"unknown fails closed",
			errors.New(`Get "https://` + user + ":" + secret + `@x": boom`),
			"request failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := redact.Reason(tt.err)
			if got != tt.want {
				t.Fatalf("Reason() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestReason_NeverEmitsCredential is the masking-is-active control: every error
// shape that can carry a credential is fed through Reason and the output must
// contain neither the password marker nor the username fragment. Changing any
// classifier branch to return err.Error() (or a raw URL/Name field) turns this
// red. (Deleting a branch does not: the fixture then falls through to another
// credential-safe constant; the sibling TestReason equality table catches a
// deleted or misrouted branch.)
func TestReason_NeverEmitsCredential(t *testing.T) {
	t.Parallel()
	errs := []error{
		misparsedErr(),
		&centreon.APIError{HTTPStatus: 500, Message: "boom " + user + ":" + secret},
		&url.Error{Op: "Get", URL: "https://" + user + ":" + secret + "@h/api", Err: &net.DNSError{Name: user}},
		&url.Error{Op: "Get", URL: "https://" + user + ":" + secret + "@h/api", Err: errors.New("dial tcp: connection refused")},
		&net.DNSError{Err: "no such host", Name: user + ":" + secret},
		timeoutError{},
		fmt.Errorf("centreon: create request: %w", &url.Error{Op: "parse", URL: "https://" + user + ":" + secret + "@"}),
	}
	for i, err := range errs {
		got := redact.Reason(err)
		if strings.Contains(got, secret) {
			t.Errorf("case %d: Reason() = %q leaked password marker %q", i, got, secret)
		}
		if strings.Contains(got, user) {
			t.Errorf("case %d: Reason() = %q leaked username %q", i, got, user)
		}
	}
}
