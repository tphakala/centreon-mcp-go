package tools

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client/v2"
)

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.Default()
}

// pollerApplyStub implements the generate-and-reload calls for testing.
type pollerApplyStub struct {
	applyErr    error
	applyAllErr error
	calledID    int
	calledAll   bool
}

func (s *pollerApplyStub) generateAndReload(_ context.Context, id int) error {
	s.calledID = id
	return s.applyErr
}

func (s *pollerApplyStub) generateAndReloadAll(_ context.Context) error {
	s.calledAll = true
	return s.applyAllErr
}

func TestPollerApplyHandler_Success(t *testing.T) {
	stub := &pollerApplyStub{}
	handler := pollerApplyHandlerFn(stub.generateAndReload, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, PollerApplyInput{PollerID: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Errorf("expected success, got error result: %v", res.Content)
	}
	if stub.calledID != 42 {
		t.Errorf("expected calledID=42, got %d", stub.calledID)
	}
}

func TestPollerApplyHandler_Error(t *testing.T) {
	stub := &pollerApplyStub{applyErr: errors.New("server down")}
	handler := pollerApplyHandlerFn(stub.generateAndReload, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, PollerApplyInput{PollerID: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Error("expected error result")
	}
}

func TestPollerApplyAllHandler_Success(t *testing.T) {
	stub := &pollerApplyStub{}
	handler := pollerApplyAllHandlerFn(stub.generateAndReloadAll, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Errorf("expected success, got error result: %v", res.Content)
	}
	if !stub.calledAll {
		t.Error("expected generateAndReloadAll to be called")
	}
}

func TestPollerApplyAllHandler_Error(t *testing.T) {
	stub := &pollerApplyStub{applyAllErr: errors.New("timeout")}
	handler := pollerApplyAllHandlerFn(stub.generateAndReloadAll, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Error("expected error result")
	}
}

func TestTimePeriodUpdateHandlerFn_Success(t *testing.T) {
	var calledID int
	var calledReq *centreon.UpdateTimePeriodRequest
	fn := func(_ context.Context, id int, req *centreon.UpdateTimePeriodRequest) error {
		calledID = id
		calledReq = req
		return nil
	}
	handler := timePeriodUpdateHandlerFn(fn, testLogger(t))
	in := UpdateTimePeriodInput{
		ID:        7,
		Name:      "workhours",
		Alias:     "Work Hours",
		Days:      []TimePeriodDayInput{{Day: 1, TimeRange: "08:00-17:00"}},
		Templates: []int{1, 2},
	}
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Errorf("expected success, got error: %v", res.Content)
	}
	if calledID != 7 {
		t.Errorf("expected calledID=7, got %d", calledID)
	}
	if calledReq == nil || calledReq.Name != "workhours" {
		t.Errorf("unexpected request: %+v", calledReq)
	}
	if len(calledReq.Days) != 1 || calledReq.Days[0].Day != 1 {
		t.Errorf("unexpected days: %+v", calledReq.Days)
	}
	if calledReq.Alias != "Work Hours" {
		t.Errorf("expected alias='Work Hours', got %q", calledReq.Alias)
	}
	if len(calledReq.Templates) != 2 || calledReq.Templates[0] != 1 {
		t.Errorf("unexpected templates: %+v", calledReq.Templates)
	}
}

func TestTimePeriodUpdateHandlerFn_Error(t *testing.T) {
	fn := func(_ context.Context, _ int, _ *centreon.UpdateTimePeriodRequest) error {
		return errors.New("not found")
	}
	handler := timePeriodUpdateHandlerFn(fn, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, UpdateTimePeriodInput{ID: 1, Name: "x", Days: nil})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Error("expected error result")
	}
}

func TestTimePeriodDeleteHandlerFn_Success(t *testing.T) {
	var calledID int
	fn := func(_ context.Context, id int) error {
		calledID = id
		return nil
	}
	handler := timePeriodDeleteHandlerFn(fn, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, IDInput{ID: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Errorf("expected success, got error: %v", res.Content)
	}
	if calledID != 5 {
		t.Errorf("expected calledID=5, got %d", calledID)
	}
}

func TestTimePeriodDeleteHandlerFn_Error(t *testing.T) {
	fn := func(_ context.Context, _ int) error {
		return errors.New("in use")
	}
	handler := timePeriodDeleteHandlerFn(fn, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, IDInput{ID: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Error("expected error result")
	}
}
