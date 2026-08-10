package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client"
	"golang.org/x/sync/errgroup"
)

// PlatformStatus combines host status counts, service status counts, and monitoring servers.
type PlatformStatus struct {
	// Host is the configured Centreon host, with any embedded credentials
	// stripped (the caller passes it through displayHost); it names the instance
	// the counts below describe.
	Host     string                                            `json:"host"`
	Hosts    *centreon.HostStatusCount                         `json:"hosts"`
	Services *centreon.ServiceStatusCount                      `json:"services"`
	Servers  *centreon.ListResponse[centreon.MonitoringServer] `json:"servers"`
}

// RegisterStatusTools registers all platform status tools. host is the
// credential-redacted Centreon host reported in the response; the caller
// supplies it already redacted (see RegisterAll).
func RegisterStatusTools(s *mcp.Server, client *centreon.Client, logger *slog.Logger, host string) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "centreon_platform_status",
		Description: "Return a combined live platform overview in one call: host status counts (up/down/unreachable), service status counts (ok/warning/critical/unknown), and the list of monitoring servers. The response also names the configured Centreon host (credentials redacted). Use this for an at-a-glance health snapshot; for the individual pieces use centreon_monitoring_host_status_counts, centreon_monitoring_service_status_counts, or centreon_server_list. Read-only.",
		Annotations: readOnlyTool("Platform status"),
	}, platformStatusHandler(client, logger, host))
}

// logReadError logs a failed platform-status read at Error level, unless the
// read was cancelled (context.Canceled): either a sibling read failed first and
// errgroup cancelled this one, or the caller cancelled the request. A real
// timeout (context.DeadlineExceeded) is not a cancellation and is still logged.
// It then returns the wrapped error for errgroup. Skipping cancelled reads keeps
// one logical failure to one error line instead of three.
func logReadError(logger *slog.Logger, part, msg string, err error) error {
	if !errors.Is(err, context.Canceled) {
		logger.Error("failed: centreon_platform_status ("+part+")", "error", err)
	}
	return fmt.Errorf("%s: %w", msg, err)
}

func platformStatusHandler(client *centreon.Client, logger *slog.Logger, host string) func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
	return platformStatusHandlerFn(
		client.MonitoringHosts.StatusCounts,
		client.MonitoringServices.StatusCounts,
		func(ctx context.Context) (*centreon.ListResponse[centreon.MonitoringServer], error) {
			return client.MonitoringServers.List(ctx)
		},
		logger,
		host,
	)
}

// platformStatusHandlerFn runs the three independent status reads concurrently
// and combines them, so total latency is the slowest single call rather than
// their sum. Taking the reads as function values keeps it unit-testable without
// a live Centreon client.
func platformStatusHandlerFn(
	fetchHosts func(context.Context) (*centreon.HostStatusCount, error),
	fetchServices func(context.Context) (*centreon.ServiceStatusCount, error),
	fetchServers func(context.Context) (*centreon.ListResponse[centreon.MonitoringServer], error),
	logger *slog.Logger,
	host string,
) func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		ctx = centreon.WithToolName(ctx, "centreon_platform_status")
		logger.Debug("centreon_platform_status")

		var (
			hosts    *centreon.HostStatusCount
			services *centreon.ServiceStatusCount
			servers  *centreon.ListResponse[centreon.MonitoringServer]
		)
		// errgroup cancels the shared context on the first failure so the other
		// two reads can abort early. Each goroutine writes only its own variable,
		// and g.Wait() establishes the happens-before for reading them afterwards.
		g, gctx := errgroup.WithContext(ctx)
		g.Go(func() error {
			h, err := fetchHosts(gctx)
			if err != nil {
				return logReadError(logger, "hosts", "failed to get host status counts", err)
			}
			hosts = h
			return nil
		})
		g.Go(func() error {
			s, err := fetchServices(gctx)
			if err != nil {
				return logReadError(logger, "services", "failed to get service status counts", err)
			}
			services = s
			return nil
		})
		g.Go(func() error {
			srv, err := fetchServers(gctx)
			if err != nil {
				return logReadError(logger, "servers", "failed to get monitoring servers", err)
			}
			servers = srv
			return nil
		})
		if err := g.Wait(); err != nil {
			res, anyVal := errorResult("%v", err)
			return res, anyVal, nil
		}

		status := PlatformStatus{
			Host:     host,
			Hosts:    hosts,
			Services: services,
			Servers:  servers,
		}
		res, anyVal := jsonResult(status)
		return res, anyVal, nil
	}
}
