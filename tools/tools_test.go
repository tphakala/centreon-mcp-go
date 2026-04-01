package tools

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestTextResult(t *testing.T) {
	res, anyVal := textResult("hello %s", "world")
	if anyVal != nil {
		t.Error("expected nil anyVal")
	}
	if res.IsError {
		t.Error("expected IsError false")
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}
	if tc.Text != "hello world" {
		t.Errorf("expected 'hello world', got %q", tc.Text)
	}
}

func TestErrorResult(t *testing.T) {
	res, anyVal := errorResult("fail: %d", 42)
	if anyVal != nil {
		t.Error("expected nil anyVal")
	}
	if !res.IsError {
		t.Error("expected IsError true")
	}
	tc := res.Content[0].(*mcp.TextContent)
	if tc.Text != "fail: 42" {
		t.Errorf("expected 'fail: 42', got %q", tc.Text)
	}
}

func TestJsonResult(t *testing.T) {
	data := map[string]string{"key": "value"}
	res, anyVal := jsonResult(data)
	if anyVal == nil {
		t.Error("expected non-nil anyVal")
	}
	if res.IsError {
		t.Error("expected IsError false")
	}
	tc := res.Content[0].(*mcp.TextContent)
	if tc.Text == "" {
		t.Error("expected non-empty text")
	}
}

func TestBuildListOptions_Defaults(t *testing.T) {
	in := ListInput{}
	opts := buildListOptions(in)
	// Should have at least the limit option
	if len(opts) == 0 {
		t.Error("expected at least one option")
	}
}

func TestBuildListOptions_ClampsMaxPageSize(t *testing.T) {
	in := ListInput{Limit: 500}
	opts := buildListOptions(in)
	// We can't inspect the options directly, but we can verify no panic
	if len(opts) == 0 {
		t.Error("expected at least one option")
	}
}
