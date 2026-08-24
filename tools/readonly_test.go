package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client/v2"
)

// connectServerSession connects an in-memory client to an already-built server
// and returns the client session, cleaned up when the test ends.
func connectServerSession(t *testing.T, ctx context.Context, s *mcp.Server) *mcp.ClientSession {
	t.Helper()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	ss, err := s.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })

	c := mcp.NewClient(&mcp.Implementation{Name: "readonly-test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// registerAllSession builds a server with every tool registered against client,
// in the given read-only mode, and returns a connected client session.
func registerAllSession(t *testing.T, ctx context.Context, client *centreon.Client, readOnly bool) *mcp.ClientSession {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "readonly-test", Version: "test"}, nil)
	RegisterAll(s, client, nil, "https://ro.example.com", readOnly)
	return connectServerSession(t, ctx, s)
}

// TestReadOnly_MutatingToolsRefuse pins the #81 acceptance path: with read-only
// mode on, every mutating tool refuses with a clear tool-level error naming the
// tool, and the underlying Centreon API is never contacted.
func TestReadOnly_MutatingToolsRefuse(t *testing.T) {
	var calls atomic.Int64
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		t.Errorf("read-only mode must not contact Centreon, but a request reached the backend")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer fake.Close()

	client, err := centreon.NewClient(fake.URL, centreon.WithAPIToken("t"))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	ctx := t.Context()
	cs := registerAllSession(t, ctx, client, true)

	// One representative per mutating category, including the two reclassified
	// operations tools (submit, check) and the fleet-wide poller reload.
	cases := []struct {
		name string
		args map[string]any
	}{
		{"centreon_poller_apply_all", map[string]any{}},
		{"centreon_host_delete", map[string]any{"id": 1}},
		{"centreon_service_delete", map[string]any{"id": 1}},
		{"centreon_downtime_cancel", map[string]any{"id": 1}},
		{"centreon_time_period_delete", map[string]any{"id": 1}},
		{"centreon_acknowledgement_host_cancel", map[string]any{"hostID": 1}},
		{"centreon_resource_submit", map[string]any{"type": "host", "id": 1, "status": 0, "output": "x"}},
		{"centreon_resource_check", map[string]any{"type": "host", "id": 1}},
		// A createTool-annotated write tool, so the refuse path is proven for the
		// create annotation too, not only update and delete.
		{"centreon_resource_comment", map[string]any{"type": "host", "id": 1, "comment": "x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
			if err != nil {
				t.Fatalf("CallTool returned a protocol error: %v", err)
			}
			if !res.IsError {
				t.Fatalf("expected a tool-level error, got success")
			}
			text := textOf(t, res)
			if !strings.Contains(text, "read-only mode") {
				t.Errorf("refusal message missing 'read-only mode': %q", text)
			}
			if !strings.Contains(text, tc.name) {
				t.Errorf("refusal message does not name the tool %q: %q", tc.name, text)
			}
		})
	}
	if calls.Load() != 0 {
		t.Errorf("Centreon backend was contacted %d times in read-only mode, want 0", calls.Load())
	}
}

// TestReadOnly_ReadToolsStillWork pins that read tools are unaffected by
// read-only mode and their output is still fenced as untrusted data.
func TestReadOnly_ReadToolsStillWork(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":[],"meta":{"page":1,"limit":30,"total":0}}`))
	}))
	defer fake.Close()

	client, err := centreon.NewClient(fake.URL, centreon.WithAPIToken("t"))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	ctx := t.Context()
	cs := registerAllSession(t, ctx, client, true)

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "centreon_host_list", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("CallTool returned a protocol error: %v", err)
	}
	if res.IsError {
		t.Fatalf("read tool must work in read-only mode, got error: %s", textOf(t, res))
	}
	if !strings.Contains(textOf(t, res), UntrustedBegin) {
		t.Errorf("read tool output is not fenced")
	}
}

// TestReadOnly_ToolListIdentical pins that read-only mode changes only handlers,
// not the advertised tool surface: names, the full annotations, the input schema,
// and the description are identical in both modes, so a client caching tools/list
// sees no difference.
func TestReadOnly_ToolListIdentical(t *testing.T) {
	ctx := t.Context()
	// The whole advertised surface per tool, not just a couple of fields, so a
	// cross-mode divergence in any annotation, the schema, or the description is
	// caught.
	type toolFace struct {
		Annotations *mcp.ToolAnnotations
		Schema      string
		Description string
	}
	collect := func(readOnly bool) map[string]toolFace {
		cs := registerAllSession(t, ctx, &centreon.Client{}, readOnly)
		got := map[string]toolFace{}
		for tool, err := range cs.Tools(ctx, nil) {
			if err != nil {
				t.Fatalf("listing tools: %v", err)
			}
			var schema string
			if tool.InputSchema != nil {
				b, mErr := json.Marshal(tool.InputSchema)
				if mErr != nil {
					t.Fatalf("marshal input schema for %s: %v", tool.Name, mErr)
				}
				schema = string(b)
			}
			got[tool.Name] = toolFace{tool.Annotations, schema, tool.Description}
		}
		return got
	}
	rw := collect(false)
	ro := collect(true)
	if len(rw) != len(ro) {
		t.Fatalf("tool count differs: read-write %d, read-only %d", len(rw), len(ro))
	}
	for name, a := range rw {
		b, ok := ro[name]
		if !ok {
			t.Errorf("tool %q present in read-write but missing in read-only", name)
			continue
		}
		if !reflect.DeepEqual(a, b) {
			t.Errorf("tool %q surface differs across modes:\n read-write: %+v\n read-only:  %+v", name, a, b)
		}
	}
}

// TestAddTool_ReadOnlyPolicyKeyedOffAnnotation pins the choke point: in read-only
// mode a read-annotated tool still runs, a mutating tool is refused without
// running, and a tool with NO annotations fails closed (refused).
func TestAddTool_ReadOnlyPolicyKeyedOffAnnotation(t *testing.T) {
	ctx := t.Context()
	s := mcp.NewServer(&mcp.Implementation{Name: "addtool-test", Version: "0"}, nil)
	r := &Registrar{server: s, readOnly: true}

	var readRan, mutRan, noAnnRan atomic.Bool
	mk := func(ran *atomic.Bool) func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
		return func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
			ran.Store(true)
			res, v := textResult("ran")
			return res, v, nil
		}
	}
	addTool(r, &mcp.Tool{Name: "t_read", Description: "read", Annotations: readOnlyTool("read")}, mk(&readRan))
	addTool(r, &mcp.Tool{Name: "t_mut", Description: "mut", Annotations: updateTool("mut")}, mk(&mutRan))
	addTool(r, &mcp.Tool{Name: "t_noann", Description: "noann", Annotations: nil}, mk(&noAnnRan))

	cs := connectServerSession(t, ctx, s)

	readRes, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "t_read"})
	if err != nil {
		t.Fatalf("CallTool t_read: %v", err)
	}
	if readRes.IsError || !readRan.Load() {
		t.Errorf("read-annotated tool must run in read-only mode (isError=%v, ran=%v)", readRes.IsError, readRan.Load())
	}

	for _, tc := range []struct {
		name string
		ran  *atomic.Bool
	}{
		{"t_mut", &mutRan},
		{"t_noann", &noAnnRan},
	} {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tc.name})
		if err != nil {
			t.Fatalf("CallTool %s: %v", tc.name, err)
		}
		if !res.IsError {
			t.Errorf("%s must be refused in read-only mode", tc.name)
		}
		if tc.ran.Load() {
			t.Errorf("%s handler ran but should have been short-circuited", tc.name)
		}
	}
}

// TestAnnotations_SubmitAndCheckAreDestructive pins the #81 reclassification: the
// two tools that overwrite or force-refresh live status must carry a destructive,
// non-read-only annotation so a client's own guardrails treat them correctly.
func TestAnnotations_SubmitAndCheckAreDestructive(t *testing.T) {
	ctx := t.Context()
	cs := registerAllSession(t, ctx, &centreon.Client{}, false)

	want := map[string]bool{"centreon_resource_submit": false, "centreon_resource_check": false}
	for tool, err := range cs.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("listing tools: %v", err)
		}
		if _, ok := want[tool.Name]; !ok {
			continue
		}
		want[tool.Name] = true
		if tool.Annotations == nil {
			t.Errorf("%s has no annotations", tool.Name)
			continue
		}
		if tool.Annotations.ReadOnlyHint {
			t.Errorf("%s is annotated read-only, want destructive", tool.Name)
		}
		if tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint {
			t.Errorf("%s is not annotated destructive", tool.Name)
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("tool %q was not found among registered tools", name)
		}
	}
}

// TestAnnotations_ForceCheckIsNotIdempotent pins #91 item 1: forcing an on-demand
// check schedules a fresh check on every call, so centreon_resource_check must NOT
// advertise IdempotentHint (a client honoring it could auto-retry and trigger
// repeated checks). centreon_resource_submit overwrites to a fixed state and is
// genuinely idempotent, so it keeps IdempotentHint.
func TestAnnotations_ForceCheckIsNotIdempotent(t *testing.T) {
	ctx := t.Context()
	cs := registerAllSession(t, ctx, &centreon.Client{}, false)

	wantIdempotent := map[string]bool{"centreon_resource_check": false, "centreon_resource_submit": true}
	seen := map[string]bool{}
	for tool, err := range cs.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("listing tools: %v", err)
		}
		want, ok := wantIdempotent[tool.Name]
		if !ok {
			continue
		}
		seen[tool.Name] = true
		if tool.Annotations == nil {
			t.Errorf("%s has no annotations", tool.Name)
			continue
		}
		if tool.Annotations.IdempotentHint != want {
			t.Errorf("%s IdempotentHint = %v, want %v", tool.Name, tool.Annotations.IdempotentHint, want)
		}
	}
	for name := range wantIdempotent {
		if !seen[name] {
			t.Errorf("tool %q was not found among registered tools", name)
		}
	}
}
