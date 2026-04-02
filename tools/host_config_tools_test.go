package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client"
)

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
	res, anyVal, err := handler(context.Background(), &mcp.CallToolRequest{}, IDInput{ID: 3})
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
	res, _, err := handler(context.Background(), &mcp.CallToolRequest{}, IDInput{ID: 99})
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
	res, _, err := handler(context.Background(), &mcp.CallToolRequest{}, CreateHostCategoryInput{Name: "web", Alias: "Web Servers"})
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
	res, _, err := handler(context.Background(), &mcp.CallToolRequest{}, CreateHostCategoryInput{Name: "x"})
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
	res, _, err := handler(context.Background(), &mcp.CallToolRequest{}, UpdateHostCategoryInput{ID: 8, Name: "linux", Alias: "Linux"})
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
	res, _, err := handler(context.Background(), &mcp.CallToolRequest{}, UpdateHostCategoryInput{ID: 1, Name: "x"})
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
	res, _, err := handler(context.Background(), &mcp.CallToolRequest{}, IDInput{ID: 11})
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
	res, _, err := handler(context.Background(), &mcp.CallToolRequest{}, IDInput{ID: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Error("expected error result")
	}
}

