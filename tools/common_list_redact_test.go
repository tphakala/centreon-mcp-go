package tools

import (
	"context"
	"strings"
	"testing"

	centreon "github.com/tphakala/centreon-go-client/v2"
)

// TestCommonListHandler_ErrorNeverEchoesCredential pins the single sink shared
// by every list tool that routes through commonListHandler (#63, #71): a
// client-call error carrying the base-URL credential must be classified before
// it reaches the log or the response. Changing the redact.Reason call in
// commonListHandler to pass the raw err turns this red.
func TestCommonListHandler_ErrorNeverEchoesCredential(t *testing.T) {
	requester := func(_ context.Context, _ ...centreon.ListOption) (*centreon.ListResponse[centreon.MonitoringServer], error) {
		return nil, misparsedClientErr()
	}

	res, _, err := commonListHandler(t.Context(), testLogger(t), "centreon_host_list", ListInput{}, requester)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result")
	}
	text := textOf(t, res)
	assertNoCredential(t, text)
	if !strings.Contains(text, "centreon_host_list") || !strings.Contains(text, "DNS lookup failed") {
		t.Errorf("want the tool named and the reason classified, got: %q", text)
	}
}

// TestCommonListHandler_NilResultSerializesAsArray pins #85: a successful list
// whose underlying Result slice is nil (a 204, a body with result:null, or an
// omitted result) must serialize as "result": [] so a client never has to
// special-case null.
func TestCommonListHandler_NilResultSerializesAsArray(t *testing.T) {
	requester := func(_ context.Context, _ ...centreon.ListOption) (*centreon.ListResponse[centreon.MonitoringServer], error) {
		return &centreon.ListResponse[centreon.MonitoringServer]{Result: nil}, nil
	}

	res, _, err := commonListHandler(t.Context(), testLogger(t), "centreon_host_list", ListInput{}, requester)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %q", textOf(t, res))
	}
	text := textOf(t, res)
	if strings.Contains(text, `"result": null`) {
		t.Errorf("nil Result must not serialize as null, got: %q", text)
	}
	if !strings.Contains(text, `"result": []`) {
		t.Errorf("nil Result must serialize as an empty array, got: %q", text)
	}
}
