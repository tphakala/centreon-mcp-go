package tools

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client/v2"
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
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, in)
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
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, CreateServiceSeverityInput{Name: "x", Level: 1, IconID: 1})
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
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, in)
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
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, UpdateServiceSeverityInput{ID: 1, Name: "x", Level: 1, IconID: 1})
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
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, IDInput{ID: 7})
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
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, IDInput{ID: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Error("expected error result")
	}
}

// ---- Service Get (macros via detail endpoint, #88) ----

// TestServiceGetHandlerFn_ReturnsMacrosFromDetail pins that centreon_service_get
// returns the per-service detail (which carries macros). Making the handler drop
// the detail (e.g. jsonResult(nil)) turns this red.
func TestServiceGetHandlerFn_ReturnsMacrosFromDetail(t *testing.T) {
	val := "https://runbook.example/ping"
	detail := &centreon.ServiceDetail{ID: 8, Name: "ping", Macros: []centreon.ServiceMacro{{Name: "URL", Value: &val}}}
	getDetail := func(_ context.Context, id int) (*centreon.ServiceDetail, error) {
		if id != 8 {
			t.Errorf("detail id = %d, want 8", id)
		}
		return detail, nil
	}
	handler := serviceGetHandlerFn(getDetail, testLogger(t))
	res, anyVal, err := handler(t.Context(), &mcp.CallToolRequest{}, IDInput{ID: 8})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", textOf(t, res))
	}
	got, ok := anyVal.(*centreon.ServiceDetail)
	if !ok {
		t.Fatalf("anyVal type = %T, want *centreon.ServiceDetail", anyVal)
	}
	if len(got.Macros) != 1 || got.Macros[0].Name != "URL" {
		t.Errorf("macros = %+v, want one macro named URL", got.Macros)
	}
	if !strings.Contains(textOf(t, res), `"macros"`) {
		t.Errorf("expected macros in JSON output, got: %s", textOf(t, res))
	}
}

// TestServiceGetHandlerFn_404GivesVersionHint pins that a plain 404 (resource
// missing, or the detail endpoint absent on an older Centreon, indistinguishable
// here) renders the ambiguous version hint. Swapping versionSensitiveReason for
// redact.Reason in the handler turns this red.
func TestServiceGetHandlerFn_404GivesVersionHint(t *testing.T) {
	getDetail := func(_ context.Context, _ int) (*centreon.ServiceDetail, error) {
		return nil, &centreon.APIError{HTTPStatus: http.StatusNotFound, Message: "Service not found"}
	}
	res, _, err := serviceGetHandlerFn(getDetail, testLogger(t))(t.Context(), &mcp.CallToolRequest{}, IDInput{ID: 8})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error result for a 404")
	}
	if !strings.Contains(textOf(t, res), "not found, or unsupported on this Centreon version") {
		t.Errorf("expected the ambiguous version hint, got: %s", textOf(t, res))
	}
}

// TestServiceGetHandlerFn_RouteNotFoundPreciseHint pins that a routing 404 (the
// detail endpoint is not registered on this Centreon) renders the precise "API
// route not present" wording, not the ambiguous one. Deleting the
// centreon.IsRouteNotFound branch in versionSensitiveReason turns this red.
func TestServiceGetHandlerFn_RouteNotFoundPreciseHint(t *testing.T) {
	getDetail := func(_ context.Context, _ int) (*centreon.ServiceDetail, error) {
		return nil, &centreon.APIError{HTTPStatus: http.StatusNotFound, Message: `No route found for "GET /centreon/api/latest/configuration/services/8"`}
	}
	res, _, err := serviceGetHandlerFn(getDetail, testLogger(t))(t.Context(), &mcp.CallToolRequest{}, IDInput{ID: 8})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error result for a routing 404")
	}
	txt := textOf(t, res)
	if !strings.Contains(txt, "API route not present") {
		t.Errorf("expected the precise routing hint, got: %s", txt)
	}
	if strings.Contains(txt, "not found, or") {
		t.Errorf("a routing 404 must use the precise wording, not the ambiguous one, got: %s", txt)
	}
}

// TestServiceGetHandler_SendsPath pins that serviceGetHandler wires
// client.Services.Get (GET /configuration/services/{id}), not some other method.
// Rewiring it (e.g. to client.Hosts.Get) changes the path and turns this red.
func TestServiceGetHandler_SendsPath(t *testing.T) {
	var gotPath string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":8,"name":"ping"}`))
	}))
	defer fake.Close()

	client, err := centreon.NewClient(fake.URL, centreon.WithAPIToken("t"))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	res, _, err := serviceGetHandler(client, testLogger(t))(t.Context(), &mcp.CallToolRequest{}, IDInput{ID: 8})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", textOf(t, res))
	}
	if !strings.HasSuffix(gotPath, "/configuration/services/8") {
		t.Errorf("path = %q, want suffix /configuration/services/8", gotPath)
	}
}
