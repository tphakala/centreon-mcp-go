package tools

import (
	"net"
	"net/http"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client/v2"
)

// failRoundTripper fails every request with a resolver error, so a client call
// deterministically yields a credential-bearing *url.Error without real DNS.
type failRoundTripper struct{}

func (failRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, &net.DNSError{Err: "no such host", Name: leakUser, IsNotFound: true}
}

// credBaseURL is a base URL whose password holds a '/', so url.Parse reads
// leakUser as the host and the credential survives into every request URL and
// the resulting *url.Error (the #63/#71 leak shape).
const credBaseURL = "https://" + leakUser + ":1234/" + leakPassword + "@centreon.invalid"

// TestToolHandlers_ErrorNeverEchoesCredential is the structural regression guard
// for the per-handler redact.Reason sinks (#63, #71). Each get/create/update/
// delete/cancel handler carries its OWN inline sink (same pattern, independent
// code), so the shared-choke-point tests (commonListHandler, logReadError) do
// not cover them. This drives one representative own-sink handler per otherwise
// untested client subsystem against a real centreon.Client whose base URL
// carries a credential and whose every request fails, and asserts the
// client-facing response never contains the credential. Reverting redact.Reason
// in any covered handler turns its row red.
//
// Not exhaustive: it samples a handful of the per-handler sinks. Full per-sink
// coverage (every get/create/update/delete/cancel handler, plus the operations,
// user, service and infra families) is tracked as a follow-up (issue #73).
func TestToolHandlers_ErrorNeverEchoesCredential(t *testing.T) {
	client, err := centreon.NewClient(
		credBaseURL,
		centreon.WithAPIToken("tok"), // a token makes the client issue the request directly (no login round trip)
		centreon.WithHTTPClient(&http.Client{Transport: failRoundTripper{}}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	log := testLogger(t)
	ctx := t.Context()
	req := &mcp.CallToolRequest{}

	cases := []struct {
		name string
		run  func() (*mcp.CallToolResult, any, error)
	}{
		{"acknowledgement_get", func() (*mcp.CallToolResult, any, error) {
			return acknowledgementGetHandler(client, log)(ctx, req, IDInput{ID: 1})
		}},
		{"downtime_get", func() (*mcp.CallToolResult, any, error) {
			return downtimeGetHandler(client, log)(ctx, req, IDInput{ID: 1})
		}},
		{"monitoring_host_get", func() (*mcp.CallToolResult, any, error) {
			return monitoringHostGetHandler(client, log)(ctx, req, IDInput{ID: 1})
		}},
		{"monitoring_resource_host_get", func() (*mcp.CallToolResult, any, error) {
			return monitoringResourceHostGetHandler(client, log)(ctx, req, IDInput{ID: 1})
		}},
		{"notification_policy_host_get", func() (*mcp.CallToolResult, any, error) {
			return notificationPolicyHostGetHandler(client, log)(ctx, req, HostIDInput{HostID: 1})
		}},
		{"host_get", func() (*mcp.CallToolResult, any, error) {
			// The failing transport yields a *url.Error (not a 404), so the
			// detail-error sink runs and no fallback is attempted.
			return hostGetHandler(client, log)(ctx, req, IDInput{ID: 1})
		}},
		{"monitoring_service_metrics", func() (*mcp.CallToolResult, any, error) {
			return monitoringServiceMetricsHandler(client, log)(ctx, req, HostServiceInput{HostID: 1, ServiceID: 1})
		}},
		{"monitoring_service_timeline", func() (*mcp.CallToolResult, any, error) {
			return monitoringServiceTimelineHandler(client, log)(ctx, req, MonitoringHostServiceListInput{HostID: 1, ServiceID: 1})
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, _, err := tc.run()
			if err != nil {
				t.Fatalf("handler returned a Go error: %v", err)
			}
			if !res.IsError {
				t.Fatal("expected an error result from the failing client")
			}
			assertNoCredential(t, textOf(t, res))
		})
	}
}
