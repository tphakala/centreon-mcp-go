package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client"
)

// ---- Service Severity ----

func TestServiceSeverityCreateHandlerFn_Success(t *testing.T) {
	var calledReq centreon.CreateServiceSeverityRequest
	fn := func(_ context.Context, req centreon.CreateServiceSeverityRequest) (int, error) {
		calledReq = req
		return 20, nil
	}
	handler := serviceSeverityCreateHandlerFn(fn, testLogger(t))
	in := CreateServiceSeverityInput{Name: "high", Alias: "High", Level: 2, IconID: 4}
	res, _, err := handler(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Errorf("expected success, got error: %v", res.Content)
	}
	if calledReq.Name != "high" || calledReq.Level != 2 || calledReq.IconID != 4 {
		t.Errorf("unexpected request: %+v", calledReq)
	}
	if calledReq.Alias != "High" {
		t.Errorf("expected alias='High', got %q", calledReq.Alias)
	}
}

func TestServiceSeverityCreateHandlerFn_Error(t *testing.T) {
	fn := func(_ context.Context, _ centreon.CreateServiceSeverityRequest) (int, error) {
		return 0, errors.New("duplicate")
	}
	handler := serviceSeverityCreateHandlerFn(fn, testLogger(t))
	res, _, err := handler(context.Background(), &mcp.CallToolRequest{}, CreateServiceSeverityInput{Name: "x", Level: 1, IconID: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Error("expected error result")
	}
}

func TestServiceSeverityUpdateHandlerFn_Success(t *testing.T) {
	var calledID int
	var calledReq centreon.UpdateServiceSeverityRequest
	fn := func(_ context.Context, id int, req centreon.UpdateServiceSeverityRequest) error {
		calledID = id
		calledReq = req
		return nil
	}
	handler := serviceSeverityUpdateHandlerFn(fn, testLogger(t))
	in := UpdateServiceSeverityInput{ID: 5, Name: "critical", Alias: "Critical", Level: 1, IconID: 9}
	res, _, err := handler(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Errorf("expected success, got error: %v", res.Content)
	}
	if calledID != 5 {
		t.Errorf("expected calledID=5, got %d", calledID)
	}
	if calledReq.Level != 1 || calledReq.IconID != 9 {
		t.Errorf("unexpected request: %+v", calledReq)
	}
	if calledReq.Name != "critical" || calledReq.Alias != "Critical" {
		t.Errorf("unexpected name/alias: name=%q alias=%q", calledReq.Name, calledReq.Alias)
	}
}

func TestServiceSeverityUpdateHandlerFn_Error(t *testing.T) {
	fn := func(_ context.Context, _ int, _ centreon.UpdateServiceSeverityRequest) error {
		return errors.New("not found")
	}
	handler := serviceSeverityUpdateHandlerFn(fn, testLogger(t))
	res, _, err := handler(context.Background(), &mcp.CallToolRequest{}, UpdateServiceSeverityInput{ID: 1, Name: "x", Level: 1, IconID: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Error("expected error result")
	}
}

func TestServiceSeverityDeleteHandlerFn_Success(t *testing.T) {
	var calledID int
	fn := func(_ context.Context, id int) error {
		calledID = id
		return nil
	}
	handler := serviceSeverityDeleteHandlerFn(fn, testLogger(t))
	res, _, err := handler(context.Background(), &mcp.CallToolRequest{}, IDInput{ID: 7})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Errorf("expected success, got error: %v", res.Content)
	}
	if calledID != 7 {
		t.Errorf("expected calledID=7, got %d", calledID)
	}
}

func TestServiceSeverityDeleteHandlerFn_Error(t *testing.T) {
	fn := func(_ context.Context, _ int) error {
		return errors.New("in use")
	}
	handler := serviceSeverityDeleteHandlerFn(fn, testLogger(t))
	res, _, err := handler(context.Background(), &mcp.CallToolRequest{}, IDInput{ID: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Error("expected error result")
	}
}
