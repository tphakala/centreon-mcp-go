package tools

import (
	"context"
	"strings"
	"testing"

	centreon "github.com/tphakala/centreon-go-client"
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
