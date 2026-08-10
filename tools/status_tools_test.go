package tools

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client"
)

// errorCountHandler is a slog.Handler that counts records emitted at Error
// level or above. Safe for the concurrent logging the errgroup goroutines do.
type errorCountHandler struct{ count atomic.Int64 }

func (h *errorCountHandler) Enabled(context.Context, slog.Level) bool { return true }

// slog.Handler requires Record by value, so gocritic's hugeParam does not apply.
//
//nolint:gocritic // slog.Handler.Handle signature is fixed by the interface.
func (h *errorCountHandler) Handle(_ context.Context, r slog.Record) error {
	if r.Level >= slog.LevelError {
		h.count.Add(1)
	}
	return nil
}

func (h *errorCountHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *errorCountHandler) WithGroup(string) slog.Handler { return h }

// testStatusHost is the already-redacted display host threaded into the status
// handler under test; the handler must surface it verbatim.
const testStatusHost = "https://centreon.example.com"

// textOf extracts the first text content from a tool result.
func textOf(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("first content is %T, want *mcp.TextContent", res.Content[0])
	}
	return tc.Text
}

func TestPlatformStatusHandlerFn_CombinesResults(t *testing.T) {
	var hostsCalled, servicesCalled, serversCalled bool
	fetchHosts := func(_ context.Context) (*centreon.HostStatusCount, error) {
		hostsCalled = true
		return &centreon.HostStatusCount{Total: 11}, nil
	}
	fetchServices := func(_ context.Context) (*centreon.ServiceStatusCount, error) {
		servicesCalled = true
		return &centreon.ServiceStatusCount{Total: 22}, nil
	}
	fetchServers := func(_ context.Context) (*centreon.ListResponse[centreon.MonitoringServer], error) {
		serversCalled = true
		return &centreon.ListResponse[centreon.MonitoringServer]{
			Result: []centreon.MonitoringServer{{ID: 7, Name: "poller-7"}},
		}, nil
	}

	handler := platformStatusHandlerFn(fetchHosts, fetchServices, fetchServers, testLogger(t), testStatusHost)
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error result: %s", textOf(t, res))
	}
	if !hostsCalled || !servicesCalled || !serversCalled {
		t.Fatalf("not all fetchers called: hosts=%v services=%v servers=%v", hostsCalled, servicesCalled, serversCalled)
	}

	var got PlatformStatus
	if err := json.Unmarshal([]byte(textOf(t, res)), &got); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if got.Hosts == nil || got.Hosts.Total != 11 {
		t.Errorf("hosts not placed correctly: %+v", got.Hosts)
	}
	if got.Services == nil || got.Services.Total != 22 {
		t.Errorf("services not placed correctly: %+v", got.Services)
	}
	if got.Servers == nil || len(got.Servers.Result) != 1 || got.Servers.Result[0].Name != "poller-7" {
		t.Errorf("servers not placed correctly: %+v", got.Servers)
	}
	if got.Host != testStatusHost {
		t.Errorf("host = %q, want %q", got.Host, testStatusHost)
	}
}

// TestPlatformStatusHandlerFn_RunsConcurrently proves the three reads run in
// parallel: each fetcher blocks until every fetcher has started. A sequential
// implementation can never get all three started at once, so it fails to reach
// the barrier before the deadline. Regression to sequential turns this red.
func TestPlatformStatusHandlerFn_RunsConcurrently(t *testing.T) {
	started := make(chan struct{}, 3)
	release := make(chan struct{})
	gate := func() {
		started <- struct{}{}
		<-release
	}
	fetchHosts := func(_ context.Context) (*centreon.HostStatusCount, error) {
		gate()
		return &centreon.HostStatusCount{}, nil
	}
	fetchServices := func(_ context.Context) (*centreon.ServiceStatusCount, error) {
		gate()
		return &centreon.ServiceStatusCount{}, nil
	}
	fetchServers := func(_ context.Context) (*centreon.ListResponse[centreon.MonitoringServer], error) {
		gate()
		return &centreon.ListResponse[centreon.MonitoringServer]{}, nil
	}

	handler := platformStatusHandlerFn(fetchHosts, fetchServices, fetchServers, testLogger(t), testStatusHost)
	done := make(chan struct{})
	go func() {
		_, _, _ = handler(t.Context(), &mcp.CallToolRequest{}, struct{}{})
		close(done)
	}()

	deadline := time.After(2 * time.Second)
	for i := range 3 {
		select {
		case <-started:
		case <-deadline:
			close(release) // unblock any goroutine already waiting so the handler can finish
			t.Fatalf("fetchers did not run concurrently: only %d of 3 started before deadline", i)
		}
	}
	close(release)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not complete after release")
	}
}

// TestPlatformStatusHandlerFn_ReportsError checks that whichever single read
// fails, the error result names that specific call. Each of the three branches
// emits a distinct message, so all three are exercised to catch a copy-paste
// slip that routes one branch through another's wording.
func TestPlatformStatusHandlerFn_ReportsError(t *testing.T) {
	okHosts := func(context.Context) (*centreon.HostStatusCount, error) {
		return &centreon.HostStatusCount{}, nil
	}
	okServices := func(context.Context) (*centreon.ServiceStatusCount, error) {
		return &centreon.ServiceStatusCount{}, nil
	}
	okServers := func(context.Context) (*centreon.ListResponse[centreon.MonitoringServer], error) {
		return &centreon.ListResponse[centreon.MonitoringServer]{}, nil
	}

	tests := []struct {
		name        string
		failing     string
		wantMessage string
	}{
		{"hosts", "hosts", "host status counts"},
		{"services", "services", "service status counts"},
		{"servers", "servers", "monitoring servers"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetchHosts, fetchServices, fetchServers := okHosts, okServices, okServers
			switch tt.failing {
			case "hosts":
				fetchHosts = func(context.Context) (*centreon.HostStatusCount, error) {
					return nil, errors.New("boom")
				}
			case "services":
				fetchServices = func(context.Context) (*centreon.ServiceStatusCount, error) {
					return nil, errors.New("boom")
				}
			case "servers":
				fetchServers = func(context.Context) (*centreon.ListResponse[centreon.MonitoringServer], error) {
					return nil, errors.New("boom")
				}
			}

			handler := platformStatusHandlerFn(fetchHosts, fetchServices, fetchServers, testLogger(t), testStatusHost)
			res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, struct{}{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !res.IsError {
				t.Fatal("expected error result")
			}
			text := textOf(t, res)
			if !strings.Contains(text, tt.wantMessage) {
				t.Errorf("error result should identify the failing call %q, got: %q", tt.wantMessage, text)
			}
			if strings.Contains(text, testStatusHost) {
				t.Errorf("error result should not include the host, got: %q", text)
			}
		})
	}
}

// TestPlatformStatusHandlerFn_SuppressesCancellationLogs pins that a single
// genuine failure produces a single Error log line. When one read fails,
// errgroup cancels the shared context and the two in-flight sibling reads
// return context.Canceled; those induced cancellations must not be logged at
// Error level, or one logical failure inflates into three ERROR lines.
func TestPlatformStatusHandlerFn_SuppressesCancellationLogs(t *testing.T) {
	handler := &errorCountHandler{}
	logger := slog.New(handler)

	// services fails for real; hosts and servers block until the shared context
	// is cancelled (which errgroup does on the first failure) and then return
	// that cancellation error, reproducing the induced-cancellation case.
	fetchHosts := func(ctx context.Context) (*centreon.HostStatusCount, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	fetchServices := func(_ context.Context) (*centreon.ServiceStatusCount, error) {
		return nil, errors.New("real failure")
	}
	fetchServers := func(ctx context.Context) (*centreon.ListResponse[centreon.MonitoringServer], error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	h := platformStatusHandlerFn(fetchHosts, fetchServices, fetchServers, logger, testStatusHost)
	res, _, err := h(t.Context(), &mcp.CallToolRequest{}, struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result")
	}
	if got := handler.count.Load(); got != 1 {
		t.Errorf("expected exactly 1 Error log line (the real failure), got %d", got)
	}
	if text := textOf(t, res); !strings.Contains(text, "service status counts") {
		t.Errorf("error should name the real failing call, got: %q", text)
	}
}
