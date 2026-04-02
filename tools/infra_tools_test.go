package tools

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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
	res, _, err := handler(context.Background(), &mcp.CallToolRequest{}, PollerApplyInput{PollerID: 42})
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
	res, _, err := handler(context.Background(), &mcp.CallToolRequest{}, PollerApplyInput{PollerID: 1})
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
	res, _, err := handler(context.Background(), &mcp.CallToolRequest{}, struct{}{})
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
	res, _, err := handler(context.Background(), &mcp.CallToolRequest{}, struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Error("expected error result")
	}
}
