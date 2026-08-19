package tools

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client/v2"
)

// checkToolTDQS asserts the tier-A invariants for one tool: a substantive
// description, annotations carrying a title, no em or en dashes, and a trailing
// read/write marker consistent with the readOnly hint (so the description never
// contradicts the annotation, a TDQS hard-gate failure).
func checkToolTDQS(t *testing.T, tool *mcp.Tool) {
	t.Helper()
	const (
		minDescLen      = 40
		emDash     rune = 0x2014
		enDash     rune = 0x2013
	)
	name := tool.Name

	if len(strings.TrimSpace(tool.Description)) < minDescLen {
		t.Errorf("%s: description too short to be specific: %q", name, tool.Description)
	}
	if tool.Annotations == nil {
		t.Errorf("%s: missing annotations", name)
		return
	}
	if strings.TrimSpace(tool.Annotations.Title) == "" {
		t.Errorf("%s: missing annotation title", name)
	}
	for _, field := range []string{tool.Description, tool.Annotations.Title} {
		if strings.ContainsRune(field, emDash) || strings.ContainsRune(field, enDash) {
			t.Errorf("%s: contains em/en dash: %q", name, field)
		}
	}

	readOnly := tool.Annotations.ReadOnlyHint
	endsRead := strings.HasSuffix(tool.Description, "Read-only.")
	endsWrite := strings.HasSuffix(tool.Description, "Writes to Centreon.")
	switch {
	case !endsRead && !endsWrite:
		t.Errorf("%s: description missing Read-only./Writes to Centreon. marker", name)
	case readOnly && endsWrite:
		t.Errorf("%s: readOnly tool claims Writes to Centreon", name)
	case !readOnly && endsRead:
		t.Errorf("%s: write tool claims Read-only", name)
	}
}

// TestTDQS_ToolDefinitionsQuality lists every registered tool over a real
// in-memory MCP session (confirming annotations serialize end to end) and
// checks the tier-A invariants on each. A zero-value client is enough because
// registration only binds handler method values; tools/list never calls them.
func TestTDQS_ToolDefinitionsQuality(t *testing.T) {
	const wantTools = 91

	ctx := t.Context()

	s := mcp.NewServer(&mcp.Implementation{Name: "centreon-mcp-go", Version: "test"}, nil)
	RegisterAll(s, &centreon.Client{}, nil, "https://tdqs.example.com")

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	ss, err := s.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })

	c := mcp.NewClient(&mcp.Implementation{Name: "tdqs-test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	count := 0
	for tool, iterErr := range cs.Tools(ctx, nil) {
		if iterErr != nil {
			t.Fatalf("listing tools: %v", iterErr)
		}
		count++
		checkToolTDQS(t, tool)
	}

	if count < wantTools {
		t.Errorf("expected at least %d registered tools, got %d", wantTools, count)
	}
}
