# Poller Apply Tools Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `centreon_poller_apply` and `centreon_poller_apply_all` MCP tools to trigger generate-and-reload on one or all Centreon pollers.

**Architecture:** Both tools are simple wrappers around `client.MonitoringServers.GenerateAndReload` and `client.MonitoringServers.GenerateAndReloadAll`. They follow the existing mutation handler pattern: set tool name on context, log, call client, return `successResult` or `errorResult`. The single-poller tool uses a new `PollerApplyInput` struct; the all-pollers tool uses `struct{}` (same pattern as `connectionTestHandler`).

**Tech Stack:** Go, github.com/modelcontextprotocol/go-sdk/mcp, github.com/tphakala/centreon-go-client v1.6.0

---

## File Map

- Modify: `tools/infra_tools.go` — add `PollerApplyInput`, `pollerApplyHandler`, `pollerApplyAllHandler`, register both in `RegisterInfraTools`
- Create: `tools/infra_tools_test.go` — tests for both handlers using a table-driven approach with a mock client stub
- Modify: `README.md` — update infra_tools count 5→7, total 73→75, add new tool names to table row

---

### Task 1: Write failing tests

**Files:**
- Create: `tools/infra_tools_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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
```

> Note: `testLogger` and the `Fn` variants are helpers defined alongside the handlers. The test file will not compile until Task 2 adds these symbols.

- [ ] **Step 2: Run tests to verify they fail (won't compile yet)**

```bash
cd /Users/e909385/src/centreon-mcp-go && go test ./tools/... 2>&1 | head -20
```
Expected: compile error — `PollerApplyInput`, `pollerApplyHandlerFn`, `pollerApplyAllHandlerFn`, `testLogger` undefined.

- [ ] **Step 3: Commit the failing test**

```bash
cd /Users/e909385/src/centreon-mcp-go
git add tools/infra_tools_test.go
git commit -m "test: add failing tests for centreon_poller_apply tools"
```

---

### Task 2: Implement the handlers

**Files:**
- Modify: `tools/infra_tools.go`

- [ ] **Step 1: Add `PollerApplyInput` struct and handler helper types**

Add after the existing `TimePeriodDayInput` / `CreateTimePeriodInput` block in `tools/infra_tools.go`:

```go
// PollerApplyInput is the input for the centreon_poller_apply tool.
type PollerApplyInput struct {
	PollerID int `json:"pollerID" jsonschema:"Poller (monitoring server) ID"`
}
```

- [ ] **Step 2: Add `testLogger` helper in `tools/infra_tools_test.go`**

Add at the top of the test file (before `pollerApplyStub`):

```go
import "log/slog"

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.Default()
}
```

- [ ] **Step 3: Add `pollerApplyHandlerFn` and `pollerApplyAllHandlerFn` to `infra_tools.go`**

These are thin wrappers that accept function dependencies so they can be tested without a real client:

```go
func pollerApplyHandlerFn(
	fn func(context.Context, int) error,
	logger *slog.Logger,
) func(ctx context.Context, req *mcp.CallToolRequest, in PollerApplyInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in PollerApplyInput) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_poller_apply")
		logger.Info("centreon_poller_apply", "pollerID", in.PollerID)
		if err := fn(ctx, in.PollerID); err != nil {
			logger.Error("failed: centreon_poller_apply", "error", err, "pollerID", in.PollerID)
			res, anyVal := errorResult("failed to apply configuration for poller %d: %v", in.PollerID, err)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_poller_apply", "Applied configuration for poller %d", in.PollerID)
		return res, anyVal, nil
	}
}

func pollerApplyAllHandlerFn(
	fn func(context.Context) error,
	logger *slog.Logger,
) func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_poller_apply_all")
		logger.Info("centreon_poller_apply_all")
		if err := fn(ctx); err != nil {
			logger.Error("failed: centreon_poller_apply_all", "error", err)
			res, anyVal := errorResult("failed to apply configuration for all pollers: %v", err)
			return res, anyVal, nil
		}
		res, anyVal := successResult(logger, "centreon_poller_apply_all", "Applied configuration for all pollers")
		return res, anyVal, nil
	}
}
```

- [ ] **Step 4: Add public wrapper handlers used by `RegisterInfraTools`**

```go
func pollerApplyHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in PollerApplyInput) (*mcp.CallToolResult, any, error) {
	return pollerApplyHandlerFn(client.MonitoringServers.GenerateAndReload, logger)
}

func pollerApplyAllHandler(client *centreon.Client, logger *slog.Logger) func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
	return pollerApplyAllHandlerFn(client.MonitoringServers.GenerateAndReloadAll, logger)
}
```

- [ ] **Step 5: Register both tools in `RegisterInfraTools`**

Add to the end of `RegisterInfraTools` (before the closing brace):

```go
	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_poller_apply",
		Description: "Apply configuration (generate and reload) for a specific monitoring server (poller) by ID.",
	}, pollerApplyHandler(client, logger))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_poller_apply_all",
		Description: "Apply configuration (generate and reload) for all monitoring servers (pollers).",
	}, pollerApplyAllHandler(client, logger))
```

- [ ] **Step 6: Run tests**

```bash
cd /Users/e909385/src/centreon-mcp-go && go test ./tools/... -v -run TestPoller 2>&1
```
Expected: 4 tests PASS.

- [ ] **Step 7: Run full test suite**

```bash
cd /Users/e909385/src/centreon-mcp-go && task go:test 2>&1
```
Expected: all tests PASS, no race conditions.

- [ ] **Step 8: Commit**

```bash
cd /Users/e909385/src/centreon-mcp-go
git add tools/infra_tools.go tools/infra_tools_test.go
git commit -m "feat: add centreon_poller_apply and centreon_poller_apply_all tools (issue #18)"
```

---

### Task 3: Update README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update infra_tools row** — change count `5` → `7` and add new tool names:

Before:
```
| `infra_tools.go`            | Infrastructure          | 5     | `centreon_server_list`, `centreon_command_list`, `centreon_time_period_create`    |
```
After:
```
| `infra_tools.go`            | Infrastructure          | 7     | `centreon_server_list`, `centreon_command_list`, `centreon_poller_apply`, `centreon_poller_apply_all` |
```

- [ ] **Step 2: Update total count** — change `**Total: 73 tools**` → `**Total: 75 tools**`

- [ ] **Step 3: Run build to verify no regressions**

```bash
cd /Users/e909385/src/centreon-mcp-go && task go:build 2>&1
```
Expected: successful build, no errors.

- [ ] **Step 4: Commit**

```bash
cd /Users/e909385/src/centreon-mcp-go
git add README.md
git commit -m "docs: update tool count and table for poller apply tools"
```
