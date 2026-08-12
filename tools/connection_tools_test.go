package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client"
)

// TestConnectionTestHandlerFn_SuccessReportsHost pins that a successful
// connection test names the configured host in its text result. The host is
// passed in already redacted (the caller runs it through safeHost), so the
// handler prints it verbatim.
func TestConnectionTestHandlerFn_SuccessReportsHost(t *testing.T) {
	fetch := func(context.Context) (*centreon.HostStatusCount, error) {
		return &centreon.HostStatusCount{}, nil
	}

	handler := connectionTestHandlerFn(fetch, testLogger(t), "https://centreon.example.com")
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error result: %s", textOf(t, res))
	}
	if got, want := textOf(t, res), "Connection successful (host: https://centreon.example.com)"; got != want {
		t.Errorf("result text = %q, want %q", got, want)
	}
}

// TestConnectionTestHandlerFn_ErrorOmitsHost pins that a failed connection test
// reports the error without the host string, so the failure path cannot surface
// the host. Credential redaction on the success path is covered by the gateway
// end-to-end test in the main package.
func TestConnectionTestHandlerFn_ErrorOmitsHost(t *testing.T) {
	fetch := func(context.Context) (*centreon.HostStatusCount, error) {
		return nil, errors.New("boom")
	}

	handler := connectionTestHandlerFn(fetch, testLogger(t), "https://centreon.example.com")
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result")
	}
	if got, want := textOf(t, res), "connection failed: request failed"; got != want {
		t.Errorf("result text = %q, want %q", got, want)
	}
}

// TestConnectionTestHandlerFn_ErrorNeverEchoesCredential pins #71 for this tool:
// a client-call error carrying the base-URL credential must be classified before
// it reaches the response, so neither the mis-parsed password nor the username
// leaks. Deleting the redact.Reason call at connection_tools.go:37 turns this red.
func TestConnectionTestHandlerFn_ErrorNeverEchoesCredential(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"mis-parsed authority", misparsedClientErr(), "connection failed: DNS lookup failed"},
		{"well-formed userinfo leaks username", wellFormedClientErr(), "connection failed: network error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fetch := func(context.Context) (*centreon.HostStatusCount, error) {
				return nil, tc.err
			}
			handler := connectionTestHandlerFn(fetch, testLogger(t), "https://centreon.example.com")
			res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, struct{}{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !res.IsError {
				t.Fatal("expected error result")
			}
			text := textOf(t, res)
			assertNoCredential(t, text)
			if text != tc.want {
				t.Errorf("result text = %q, want %q", text, tc.want)
			}
		})
	}
}
