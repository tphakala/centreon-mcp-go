package tools

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client/v2"
)

// TestHostCategoryGetHandlerFn_ErrorNeverEchoesCredential pins one config
// handler end to end (#63, #71): a mis-parsed base-URL credential must reach
// neither the response nor the log. Each swept handler has its OWN inline
// redact.Reason sink (same pattern, independent code), so this guards
// hostCategoryGetHandlerFn specifically; the other per-handler sinks across the
// tools package are exercised by TestToolHandlers_ErrorNeverEchoesCredential
// (redact_sweep_test.go). Changing the redact.Reason call in
// hostCategoryGetHandlerFn to pass the raw err turns this red.
func TestHostCategoryGetHandlerFn_ErrorNeverEchoesCredential(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	fn := func(_ context.Context, _ int) (*centreon.HostCategory, error) {
		return nil, misparsedClientErr()
	}
	handler := hostCategoryGetHandlerFn(fn, logger)
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, IDInput{ID: 99})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result")
	}
	text := textOf(t, res)
	assertNoCredential(t, text)
	assertNoCredential(t, buf.String())
	if !strings.Contains(text, "host category 99") || !strings.Contains(text, "DNS lookup failed") {
		t.Errorf("want the target identified and the reason classified, got: %q", text)
	}
}

// ---- Host Category ----

func TestHostCategoryGetHandlerFn_Success(t *testing.T) {
	want := &centreon.HostCategory{ID: 3, Name: "linux", Alias: "Linux Servers", IsActivated: true}
	fn := func(_ context.Context, id int) (*centreon.HostCategory, error) {
		if id != 3 {
			t.Errorf("expected id=3, got %d", id)
		}
		return want, nil
	}
	handler := hostCategoryGetHandlerFn(fn, testLogger(t))
	res, anyVal, err := handler(t.Context(), &mcp.CallToolRequest{}, IDInput{ID: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Errorf("expected success, got error: %v", res.Content)
	}
	got, ok := anyVal.(*centreon.HostCategory)
	if !ok {
		t.Fatalf("expected *centreon.HostCategory, got %T", anyVal)
	}
	if got.ID != want.ID || got.Name != want.Name {
		t.Errorf("expected %+v, got %+v", want, got)
	}
}

func TestHostCategoryGetHandlerFn_Error(t *testing.T) {
	fn := func(_ context.Context, _ int) (*centreon.HostCategory, error) {
		return nil, errors.New("not found")
	}
	handler := hostCategoryGetHandlerFn(fn, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, IDInput{ID: 99})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Error("expected error result")
	}
}

func TestHostCategoryCreateHandlerFn_Success(t *testing.T) {
	var calledReq centreon.CreateHostCategoryRequest
	fn := func(_ context.Context, req centreon.CreateHostCategoryRequest) (int, error) {
		calledReq = req
		return 42, nil
	}
	handler := hostCategoryCreateHandlerFn(fn, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, CreateHostCategoryInput{Name: "web", Alias: "Web Servers"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Errorf("expected success, got error: %v", res.Content)
	}
	if calledReq.Name != "web" {
		t.Errorf("expected name=web, got %q", calledReq.Name)
	}
	if calledReq.Alias != "Web Servers" {
		t.Errorf("expected alias='Web Servers', got %q", calledReq.Alias)
	}
}

func TestHostCategoryCreateHandlerFn_Error(t *testing.T) {
	fn := func(_ context.Context, _ centreon.CreateHostCategoryRequest) (int, error) {
		return 0, errors.New("duplicate")
	}
	handler := hostCategoryCreateHandlerFn(fn, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, CreateHostCategoryInput{Name: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Error("expected error result")
	}
}

func TestHostCategoryUpdateHandlerFn_Success(t *testing.T) {
	var calledID int
	var calledReq centreon.UpdateHostCategoryRequest
	fn := func(_ context.Context, id int, req centreon.UpdateHostCategoryRequest) error {
		calledID = id
		calledReq = req
		return nil
	}
	handler := hostCategoryUpdateHandlerFn(fn, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, UpdateHostCategoryInput{ID: 8, Name: "linux", Alias: "Linux"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Errorf("expected success, got error: %v", res.Content)
	}
	if calledID != 8 {
		t.Errorf("expected calledID=8, got %d", calledID)
	}
	if calledReq.Name != "linux" {
		t.Errorf("expected name=linux, got %q", calledReq.Name)
	}
	if calledReq.Alias != "Linux" {
		t.Errorf("expected alias='Linux', got %q", calledReq.Alias)
	}
}

func TestHostCategoryUpdateHandlerFn_Error(t *testing.T) {
	fn := func(_ context.Context, _ int, _ centreon.UpdateHostCategoryRequest) error {
		return errors.New("not found")
	}
	handler := hostCategoryUpdateHandlerFn(fn, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, UpdateHostCategoryInput{ID: 1, Name: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Error("expected error result")
	}
}

func TestHostCategoryDeleteHandlerFn_Success(t *testing.T) {
	var calledID int
	fn := func(_ context.Context, id int) error {
		calledID = id
		return nil
	}
	handler := hostCategoryDeleteHandlerFn(fn, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, IDInput{ID: 11})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Errorf("expected success, got error: %v", res.Content)
	}
	if calledID != 11 {
		t.Errorf("expected calledID=11, got %d", calledID)
	}
}

func TestHostCategoryDeleteHandlerFn_Error(t *testing.T) {
	fn := func(_ context.Context, _ int) error {
		return errors.New("in use")
	}
	handler := hostCategoryDeleteHandlerFn(fn, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, IDInput{ID: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Error("expected error result")
	}
}

// ---- Host Severity ----

func TestHostSeverityGetHandlerFn_Success(t *testing.T) {
	want := &centreon.HostSeverity{ID: 2, Name: "critical", Level: 1, IconID: 5}
	fn := func(_ context.Context, id int) (*centreon.HostSeverity, error) {
		if id != 2 {
			t.Errorf("expected id=2, got %d", id)
		}
		return want, nil
	}
	handler := hostSeverityGetHandlerFn(fn, testLogger(t))
	res, anyVal, err := handler(t.Context(), &mcp.CallToolRequest{}, IDInput{ID: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Errorf("expected success, got error: %v", res.Content)
	}
	got, ok := anyVal.(*centreon.HostSeverity)
	if !ok {
		t.Fatalf("expected *centreon.HostSeverity, got %T", anyVal)
	}
	if got.ID != want.ID || got.Name != want.Name {
		t.Errorf("expected %+v, got %+v", want, got)
	}
}

func TestHostSeverityGetHandlerFn_Error(t *testing.T) {
	fn := func(_ context.Context, _ int) (*centreon.HostSeverity, error) {
		return nil, errors.New("not found")
	}
	handler := hostSeverityGetHandlerFn(fn, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, IDInput{ID: 99})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Error("expected error result")
	}
}

func TestHostSeverityCreateHandlerFn_Success(t *testing.T) {
	var calledReq centreon.CreateHostSeverityRequest
	fn := func(_ context.Context, req centreon.CreateHostSeverityRequest) (int, error) {
		calledReq = req
		return 10, nil
	}
	handler := hostSeverityCreateHandlerFn(fn, testLogger(t))
	in := CreateHostSeverityInput{Name: "high", Alias: "High", Level: 2, IconID: 3}
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Errorf("expected success, got error: %v", res.Content)
	}
	if calledReq.Name != "high" || calledReq.Level != 2 || calledReq.IconID != 3 {
		t.Errorf("unexpected request: %+v", calledReq)
	}
	if calledReq.Alias != "High" {
		t.Errorf("expected alias='High', got %q", calledReq.Alias)
	}
}

func TestHostSeverityCreateHandlerFn_Error(t *testing.T) {
	fn := func(_ context.Context, _ centreon.CreateHostSeverityRequest) (int, error) {
		return 0, errors.New("duplicate")
	}
	handler := hostSeverityCreateHandlerFn(fn, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, CreateHostSeverityInput{Name: "x", Level: 1, IconID: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Error("expected error result")
	}
}

func TestHostSeverityUpdateHandlerFn_Success(t *testing.T) {
	var calledID int
	var calledReq centreon.UpdateHostSeverityRequest
	fn := func(_ context.Context, id int, req centreon.UpdateHostSeverityRequest) error {
		calledID = id
		calledReq = req
		return nil
	}
	handler := hostSeverityUpdateHandlerFn(fn, testLogger(t))
	in := UpdateHostSeverityInput{ID: 4, Name: "critical", Alias: "Critical", Level: 1, IconID: 7}
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Errorf("expected success, got error: %v", res.Content)
	}
	if calledID != 4 {
		t.Errorf("expected calledID=4, got %d", calledID)
	}
	if calledReq.Level != 1 || calledReq.IconID != 7 {
		t.Errorf("unexpected request: %+v", calledReq)
	}
	if calledReq.Name != "critical" || calledReq.Alias != "Critical" {
		t.Errorf("unexpected name/alias: name=%q alias=%q", calledReq.Name, calledReq.Alias)
	}
}

func TestHostSeverityUpdateHandlerFn_Error(t *testing.T) {
	fn := func(_ context.Context, _ int, _ centreon.UpdateHostSeverityRequest) error {
		return errors.New("not found")
	}
	handler := hostSeverityUpdateHandlerFn(fn, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, UpdateHostSeverityInput{ID: 1, Name: "x", Level: 1, IconID: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Error("expected error result")
	}
}

func TestHostSeverityDeleteHandlerFn_Success(t *testing.T) {
	var calledID int
	fn := func(_ context.Context, id int) error {
		calledID = id
		return nil
	}
	handler := hostSeverityDeleteHandlerFn(fn, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, IDInput{ID: 6})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Errorf("expected success, got error: %v", res.Content)
	}
	if calledID != 6 {
		t.Errorf("expected calledID=6, got %d", calledID)
	}
}

func TestHostSeverityDeleteHandlerFn_Error(t *testing.T) {
	fn := func(_ context.Context, _ int) error {
		return errors.New("in use")
	}
	handler := hostSeverityDeleteHandlerFn(fn, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, IDInput{ID: 9})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Error("expected error result")
	}
}
