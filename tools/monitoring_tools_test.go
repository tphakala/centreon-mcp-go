package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client"
)

// applyOpts applies functional list options to a fresh ListOptions so tests can
// inspect the concrete search filter, sort map, page and limit that would be sent.
func applyOpts(opts []centreon.ListOption) *centreon.ListOptions {
	o := &centreon.ListOptions{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// searchJSON marshals the built search filter to compact JSON for comparison.
func searchJSON(t *testing.T, o *centreon.ListOptions) string {
	t.Helper()
	if o.Search == nil {
		t.Fatal("expected a search filter, got nil")
	}
	b, err := json.Marshal(o.Search.Build())
	if err != nil {
		t.Fatalf("marshal search filter: %v", err)
	}
	return string(b)
}

func TestBuildMonitoringResourceListOptions_NoFiltersNoSearchParam(t *testing.T) {
	opts, err := buildMonitoringResourceListOptions(&MonitoringResourceListInput{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	o := applyOpts(opts)
	if o.Search != nil {
		t.Errorf("expected no search filter, got %v", o.Search.Build())
	}
	if o.SortBy != nil {
		t.Errorf("expected no sort, got %v", o.SortBy)
	}
	if o.Limit != 10 {
		t.Errorf("limit = %d, want 10", o.Limit)
	}
}

func TestBuildMonitoringResourceListOptions_SearchIsNameLike(t *testing.T) {
	opts, err := buildMonitoringResourceListOptions(&MonitoringResourceListInput{Search: "web"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := searchJSON(t, applyOpts(opts))
	want := `{"name":{"$lk":"%web%"}}`
	if got != want {
		t.Errorf("search = %s, want %s", got, want)
	}
}

func TestBuildMonitoringResourceListOptions_HostNameUsesHName(t *testing.T) {
	opts, err := buildMonitoringResourceListOptions(&MonitoringResourceListInput{HostName: "db01"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := searchJSON(t, applyOpts(opts))
	want := `{"h.name":{"$lk":"%db01%"}}`
	if got != want {
		t.Errorf("search = %s, want %s", got, want)
	}
}

func TestBuildMonitoringResourceListOptions_MonitoringServerIsEq(t *testing.T) {
	opts, err := buildMonitoringResourceListOptions(&MonitoringResourceListInput{MonitoringServer: "Central"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := searchJSON(t, applyOpts(opts))
	want := `{"monitoring_server_name":{"$eq":"Central"}}`
	if got != want {
		t.Errorf("search = %s, want %s", got, want)
	}
}

func TestBuildMonitoringResourceListOptions_MultipleFiltersAndCombined(t *testing.T) {
	opts, err := buildMonitoringResourceListOptions(&MonitoringResourceListInput{Search: "web", HostName: "db01"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := searchJSON(t, applyOpts(opts))
	want := `{"$and":[{"name":{"$lk":"%web%"}},{"h.name":{"$lk":"%db01%"}}]}`
	if got != want {
		t.Errorf("search = %s, want %s", got, want)
	}
}

func TestBuildMonitoringResourceListOptions_SortStatusDesc(t *testing.T) {
	opts, err := buildMonitoringResourceListOptions(&MonitoringResourceListInput{SortBy: "status", SortOrder: "DESC"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	o := applyOpts(opts)
	want := map[string]string{"status_severity_code": "DESC"}
	if !reflect.DeepEqual(o.SortBy, want) {
		t.Errorf("sortBy = %v, want %v", o.SortBy, want)
	}
}

func TestBuildMonitoringResourceListOptions_SortDefaultsAscending(t *testing.T) {
	opts, err := buildMonitoringResourceListOptions(&MonitoringResourceListInput{SortBy: "name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	o := applyOpts(opts)
	want := map[string]string{"name": "ASC"}
	if !reflect.DeepEqual(o.SortBy, want) {
		t.Errorf("sortBy = %v, want %v", o.SortBy, want)
	}
}

func TestBuildMonitoringResourceListOptions_SortFieldCaseInsensitive(t *testing.T) {
	// A mixed-case sort field must resolve via case-folding rather than erroring.
	opts, err := buildMonitoringResourceListOptions(&MonitoringResourceListInput{SortBy: "Status"})
	if err != nil {
		t.Fatalf("mixed-case sortBy should resolve, got error: %v", err)
	}
	if got := applyOpts(opts).SortBy; len(got) != 1 {
		t.Errorf("expected exactly one sort key, got %v", got)
	}
}

func TestBuildMonitoringResourceListOptions_SortOrderCaseInsensitive(t *testing.T) {
	opts, err := buildMonitoringResourceListOptions(&MonitoringResourceListInput{SortBy: "name", SortOrder: "desc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	o := applyOpts(opts)
	want := map[string]string{"name": "DESC"}
	if !reflect.DeepEqual(o.SortBy, want) {
		t.Errorf("sortBy = %v, want %v", o.SortBy, want)
	}
}

func TestBuildMonitoringResourceListOptions_InvalidInput(t *testing.T) {
	cases := []struct {
		name string
		in   MonitoringResourceListInput
	}{
		{"unknown sortBy", MonitoringResourceListInput{SortBy: "bogus"}},
		{"sortOrder without sortBy", MonitoringResourceListInput{SortOrder: "ASC"}},
		{"bad sortOrder", MonitoringResourceListInput{SortBy: "name", SortOrder: "sideways"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := buildMonitoringResourceListOptions(&tc.in); err == nil {
				t.Errorf("expected error for %s", tc.name)
			}
		})
	}
}

func TestBuildMonitoringResourceListOptions_WhitespaceInputsIgnored(t *testing.T) {
	opts, err := buildMonitoringResourceListOptions(&MonitoringResourceListInput{Search: "   ", HostName: "\t", MonitoringServer: " "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o := applyOpts(opts); o.Search != nil {
		t.Errorf("expected no search filter for whitespace-only inputs, got %v", o.Search.Build())
	}
}

func TestMonitoringResourceListHandler_SendsSearchAndSortParams(t *testing.T) {
	var gotPath, gotSearch, gotSort string
	var calls atomic.Int64
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		gotPath = r.URL.Path
		gotSearch = r.URL.Query().Get("search")
		gotSort = r.URL.Query().Get("sort_by")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":[],"meta":{"page":1,"limit":30,"total":0}}`))
	}))
	defer fake.Close()

	client, err := centreon.NewClient(fake.URL, centreon.WithAPIToken("t"))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	handler := monitoringResourceListHandler(client, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, MonitoringResourceListInput{Search: "web", SortBy: "name"})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", textOf(t, res))
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 API call, got %d", calls.Load())
	}
	if !strings.HasSuffix(gotPath, "/monitoring/resources") {
		t.Errorf("path = %q, want suffix /monitoring/resources", gotPath)
	}
	if gotSearch != `{"name":{"$lk":"%web%"}}` {
		t.Errorf("search param = %q", gotSearch)
	}
	if gotSort != `{"name":"ASC"}` {
		t.Errorf("sort_by param = %q", gotSort)
	}
}

func TestMonitoringResourceListHandler_InvalidInputSkipsAPI(t *testing.T) {
	var calls atomic.Int64
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"result":[],"meta":{}}`))
	}))
	defer fake.Close()

	client, err := centreon.NewClient(fake.URL, centreon.WithAPIToken("t"))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	handler := monitoringResourceListHandler(client, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, MonitoringResourceListInput{SortBy: "name", SortOrder: "sideways"})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError for invalid input")
	}
	if !strings.Contains(textOf(t, res), "sideways") {
		t.Errorf("error should mention the bad value, got %q", textOf(t, res))
	}
	if calls.Load() != 0 {
		t.Errorf("expected 0 API calls for invalid input, got %d", calls.Load())
	}
}

func TestWrapLikePattern(t *testing.T) {
	cases := []struct{ in, want string }{
		{"web", "%web%"},
		{"%web%", "%web%"},
		{"%web", "%web%"},
		{"web%", "%web%"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := wrapLikePattern(tc.in); got != tc.want {
			t.Errorf("wrapLikePattern(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBuildMonitoringListOptions_NeverAddsSearchOrSort(t *testing.T) {
	o := applyOpts(buildMonitoringListOptions(MonitoringListInput{Page: 2, Limit: 10}))
	if o.Search != nil {
		t.Errorf("shared monitoring builder must not set search, got %v", o.Search.Build())
	}
	if o.SortBy != nil {
		t.Errorf("shared monitoring builder must not set sort, got %v", o.SortBy)
	}
	if o.Limit != 10 || o.Page != 2 {
		t.Errorf("limit/page = %d/%d, want 10/2", o.Limit, o.Page)
	}
}
