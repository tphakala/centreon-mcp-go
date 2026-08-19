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
	centreon "github.com/tphakala/centreon-go-client/v2"
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

// ---- Resource list array filters (#52) ----

func TestBuildMonitoringResourceListOptions_ArrayFilters(t *testing.T) {
	in := &MonitoringResourceListInput{
		ResourceTypes:     []string{"service"},
		Statuses:          []string{"CRITICAL", "WARNING"},
		StatusTypes:       []string{"hard"},
		States:            []string{"unhandled_problems"},
		HostGroups:        []string{"Linux"},
		ServiceGroups:     []string{"HTTP"},
		HostCategories:    []string{"prod"},
		ServiceCategories: []string{"web"},
	}
	opts, err := buildMonitoringResourceListOptions(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := applyOpts(opts).ArrayFilters
	want := map[string][]string{
		"types":                  {"service"},
		"statuses":               {"CRITICAL", "WARNING"},
		"status_types":           {"hard"},
		"states":                 {"unhandled_problems"},
		"hostgroup_names":        {"Linux"},
		"servicegroup_names":     {"HTTP"},
		"host_category_names":    {"prod"},
		"service_category_names": {"web"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ArrayFilters = %#v, want %#v", got, want)
	}
}

func TestBuildMonitoringResourceListOptions_ArrayFiltersDropBlanks(t *testing.T) {
	in := &MonitoringResourceListInput{
		Statuses:   []string{" ", ""},
		HostGroups: []string{"  Linux  ", ""},
	}
	opts, err := buildMonitoringResourceListOptions(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	af := applyOpts(opts).ArrayFilters
	if _, ok := af["statuses"]; ok {
		t.Errorf("blank-only statuses must emit no filter, got %v", af["statuses"])
	}
	if got := af["hostgroup_names"]; !reflect.DeepEqual(got, []string{"Linux"}) {
		t.Errorf("hostgroup_names = %v, want [Linux] (trimmed, blank dropped)", got)
	}
}

// ---- Service metrics (#50) ----

func TestMonitoringServiceMetricsHandler_ReturnsMetrics(t *testing.T) {
	var gotPath string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":1,"name":"rta","unit":"ms","current_value":12.5,"warning_high_threshold":100,"critical_high_threshold":200}]`))
	}))
	defer fake.Close()

	client, err := centreon.NewClient(fake.URL, centreon.WithAPIToken("t"))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	handler := monitoringServiceMetricsHandler(client, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, HostServiceInput{HostID: 3, ServiceID: 8})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", textOf(t, res))
	}
	if !strings.HasSuffix(gotPath, "/monitoring/hosts/3/services/8/metrics") {
		t.Errorf("path = %q, want suffix /monitoring/hosts/3/services/8/metrics", gotPath)
	}
	// Assert the whole metric contract, not just the name, so a regression in any
	// mapped field (id, unit, value, thresholds) is caught. The payload is read
	// from the fenced text content (read tools emit no structured copy).
	var got []centreon.Metric
	unmarshalFenced(t, res, &got)
	if len(got) != 1 {
		t.Fatalf("len(metrics) = %d, want 1", len(got))
	}
	m := got[0]
	if m.ID != 1 || m.Name != "rta" || m.Unit != "ms" {
		t.Errorf("id/name/unit = %d/%q/%q, want 1/\"rta\"/\"ms\"", m.ID, m.Name, m.Unit)
	}
	for _, f := range []struct {
		name string
		got  *float64
		want float64
	}{
		{"current_value", m.CurrentValue, 12.5},
		{"warning_high_threshold", m.WarningHighThreshold, 100},
		{"critical_high_threshold", m.CriticalHighThreshold, 200},
	} {
		if f.got == nil || *f.got != f.want {
			t.Errorf("%s = %v, want %v", f.name, f.got, f.want)
		}
	}
}

// TestMonitoringServiceMetricsHandler_EmptyArrayNotNull pins that a service with
// no performance data (the client maps its 404 "metrics not found" to a nil
// slice) is surfaced as an empty JSON array, not null. Removing the nil-to-empty
// normalization turns this red.
func TestMonitoringServiceMetricsHandler_EmptyArrayNotNull(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":404,"message":"metrics not found"}`))
	}))
	defer fake.Close()

	client, err := centreon.NewClient(fake.URL, centreon.WithAPIToken("t"))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	handler := monitoringServiceMetricsHandler(client, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, HostServiceInput{HostID: 3, ServiceID: 8})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if res.IsError {
		t.Fatalf("a no-perfdata 404 must not be an error, got: %s", textOf(t, res))
	}
	if got := unwrapUntrusted(t, textOf(t, res)); got != "[]" {
		t.Errorf("output = %q, want []", got)
	}
}

// TestMonitoringServiceMetricsHandler_VersionHintOnOther404 pins that a 404 the
// client does NOT swallow (e.g. a route absent on an older Centreon) is reported
// with the version hint from versionSensitiveReason.
func TestMonitoringServiceMetricsHandler_VersionHintOnOther404(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":404,"message":"No route found"}`))
	}))
	defer fake.Close()

	client, err := centreon.NewClient(fake.URL, centreon.WithAPIToken("t"))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	handler := monitoringServiceMetricsHandler(client, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, HostServiceInput{HostID: 3, ServiceID: 8})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error result for a non-perfdata 404")
	}
	if !strings.Contains(textOf(t, res), "unsupported on this Centreon version") {
		t.Errorf("expected version hint, got: %s", textOf(t, res))
	}
}

// ---- Service timeline (#51) ----

func TestMonitoringServiceTimelineHandler_SendsPathAndPaging(t *testing.T) {
	var gotPath, gotLimit, gotPage string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotLimit = r.URL.Query().Get("limit")
		gotPage = r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":[],"meta":{"page":2,"limit":10,"total":0}}`))
	}))
	defer fake.Close()

	client, err := centreon.NewClient(fake.URL, centreon.WithAPIToken("t"))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	handler := monitoringServiceTimelineHandler(client, testLogger(t))
	res, _, err := handler(t.Context(), &mcp.CallToolRequest{}, MonitoringHostServiceListInput{HostID: 3, ServiceID: 8, Page: 2, Limit: 10})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", textOf(t, res))
	}
	if !strings.HasSuffix(gotPath, "/monitoring/hosts/3/services/8/timeline") {
		t.Errorf("path = %q, want suffix /monitoring/hosts/3/services/8/timeline", gotPath)
	}
	if gotLimit != "10" || gotPage != "2" {
		t.Errorf("paging params: limit=%q page=%q, want 10/2", gotLimit, gotPage)
	}
}

func TestVersionSensitiveReason_404GivesVersionHint(t *testing.T) {
	got := versionSensitiveReason(&centreon.APIError{HTTPStatus: http.StatusNotFound})
	if !strings.Contains(got, "unsupported on this Centreon version") {
		t.Errorf("404 reason = %q, want version hint", got)
	}
	if strings.Contains(got, "API route not present") {
		t.Errorf("a bare 404 (no route message) must use the ambiguous wording, got %q", got)
	}
	// A routing 404 the client classified gets the precise wording.
	route := versionSensitiveReason(&centreon.APIError{HTTPStatus: http.StatusNotFound, Message: `No route found for "GET /x"`})
	if !strings.Contains(route, "API route not present") {
		t.Errorf("routing 404 reason = %q, want precise route hint", route)
	}
	if strings.Contains(route, "not found, or") {
		t.Errorf("routing 404 must not use the ambiguous wording, got %q", route)
	}
	// A non-404 falls through to redact.Reason.
	if got := versionSensitiveReason(&centreon.APIError{HTTPStatus: http.StatusInternalServerError}); !strings.Contains(got, "HTTP 500") {
		t.Errorf("500 reason = %q, want HTTP 500 classification", got)
	}
}
