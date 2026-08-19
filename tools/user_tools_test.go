package tools

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client/v2"
)

// ---- User update: 25.10 read-only classification (#87) ----

// TestUserUpdateHandler_RouteNotFoundReportsReadOnly pins that a routing 404
// (the PATCH user route is unregistered, e.g. on Centreon 25.10) is reported as
// the read-only-API case rather than a bare "HTTP 404". Deleting the
// centreon.IsRouteNotFound branch in userUpdateHandler turns this red.
func TestUserUpdateHandler_RouteNotFoundReportsReadOnly(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":404,"message":"No route found for \"PATCH /centreon/api/latest/configuration/users/5\""}`))
	}))
	defer fake.Close()

	client, err := centreon.NewClient(fake.URL, centreon.WithAPIToken("t"))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	name := "renamed"
	res, _, err := userUpdateHandler(client, testLogger(t))(t.Context(), &mcp.CallToolRequest{}, UpdateUserInput{ID: 5, Name: &name})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error result for the 25.10 routing 404")
	}
	if !strings.Contains(textOf(t, res), "read-only") {
		t.Errorf("expected the read-only explanation, got: %s", textOf(t, res))
	}
}

// TestUserUpdateHandler_Non404ErrorClassified pins that an error which is NOT a
// routing 404 (here a 500) skips the read-only branch and is classified by
// redact.Reason. Widening the IsRouteNotFound branch to swallow every error, or
// dropping the redact.Reason fallthrough, turns this red.
func TestUserUpdateHandler_Non404ErrorClassified(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer fake.Close()

	client, err := centreon.NewClient(fake.URL, centreon.WithAPIToken("t"))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	name := "renamed"
	res, _, err := userUpdateHandler(client, testLogger(t))(t.Context(), &mcp.CallToolRequest{}, UpdateUserInput{ID: 5, Name: &name})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error result for a 500")
	}
	txt := textOf(t, res)
	if !strings.Contains(txt, "HTTP 500") {
		t.Errorf("a non-route error should be classified by redact.Reason, got: %s", txt)
	}
	if strings.Contains(txt, "read-only") {
		t.Errorf("a 500 must not be reported as the read-only route case, got: %s", txt)
	}
}

// TestUserUpdateHandler_SendsPatchAndReportsSuccess pins that a successful update
// sends a PATCH to the per-user route and reports the updated id. Rewiring the
// handler to a different method, id, or route turns this red.
func TestUserUpdateHandler_SendsPatchAndReportsSuccess(t *testing.T) {
	var gotMethod, gotPath string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer fake.Close()

	client, err := centreon.NewClient(fake.URL, centreon.WithAPIToken("t"))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	name := "renamed"
	res, _, err := userUpdateHandler(client, testLogger(t))(t.Context(), &mcp.CallToolRequest{}, UpdateUserInput{ID: 5, Name: &name})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", textOf(t, res))
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", gotMethod)
	}
	if !strings.HasSuffix(gotPath, "/configuration/users/5") {
		t.Errorf("path = %q, want suffix /configuration/users/5", gotPath)
	}
	if !strings.Contains(textOf(t, res), "Updated user 5") {
		t.Errorf("expected success text, got: %s", textOf(t, res))
	}
}
