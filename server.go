package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client/v2"
	"github.com/tphakala/centreon-mcp-go/internal/redact"
	"github.com/tphakala/centreon-mcp-go/tools"
	"golang.org/x/sync/errgroup"
)

const (
	serverInstructions = "Centreon MCP Server. Provides tools for monitoring hosts and services, managing downtimes and acknowledgements, configuring hosts/services/groups/templates, and platform administration. Use centreon_monitoring_* for real-time status, centreon_resource_* for bulk operations, centreon_downtime_* and centreon_acknowledgement_* for per-resource management, and centreon_host_*/centreon_service_* for configuration."

	readTimeout       = 30 * time.Second
	readHeaderTimeout = 10 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 120 * time.Second
	shutdownTimeout   = 15 * time.Second
	tokenCacheTTL     = 50 * time.Minute
	httpClientTimeout = 30 * time.Second

	// maxRedirects caps a redirect chain, matching net/http's default policy that
	// noCrossHostRedirect replaces: installing a custom CheckRedirect removes the
	// stdlib's own 10-redirect cap, so the policy must re-impose one.
	maxRedirects = 10

	authModeEnv     = "env"
	authModeGateway = "gateway"
	transportStdio  = "stdio"
	transportHTTP   = "http"

	// shutdownLogoutTimeout bounds the best-effort logout of Centreon sessions on
	// graceful shutdown (both the env-mode shared client and the gateway token
	// cache). It is a budget separate from shutdownTimeout so a slow
	// httpServer.Shutdown cannot leave the logout with an already-expired context.
	shutdownLogoutTimeout = 10 * time.Second
	// gatewayLogoutConcurrency caps how many session logouts run at once during
	// the shutdown drain, so a large cache does not open thousands of sockets.
	gatewayLogoutConcurrency = 16
)

// buildServer creates an MCP server with all tools registered. displayHostName
// is the Centreon host the status and connection tools report; the caller must
// already have passed it through displayHost (which strips all userinfo), as
// buildServer does not sanitize.
func buildServer(client *centreon.Client, logger *slog.Logger, displayHostName string) *mcp.Server {
	s := mcp.NewServer(
		&mcp.Implementation{Name: "centreon-mcp-go", Version: version},
		&mcp.ServerOptions{Instructions: serverInstructions},
	)
	tools.RegisterAll(s, client, logger, displayHostName)
	return s
}

// noCrossHostRedirect is an http.Client CheckRedirect policy that refuses any
// redirect whose target host differs from the original request's host. The
// centreon client sets X-AUTH-TOKEN on every request, and Go does NOT strip
// custom headers on cross-origin redirects (unlike Authorization or Cookie), so
// without this guard a compromised or misconfigured Centreon host could redirect
// an API call to another origin and leak the session token (CWE-522).
//
// The comparison anchors on via[0] (the original request), so every hop must
// stay on the original host; a chain cannot be walked off-host one same-looking
// hop at a time. It compares Hostname() only, ignoring scheme and port, so
// same-host behaviours such as trailing-slash normalisation and an http->https
// upgrade keep working. The match is an exact hostname (case-insensitive),
// intentionally stricter than net/http's domain-or-subdomain rule: a redirect to
// a different subdomain is refused (fail-closed for a credential header). A
// same-host https->http downgrade is still allowed here; cleartext transport is
// out of scope for this fix and owned by issue #28.
//
// net/http invokes CheckRedirect only while following a redirect, so via always
// holds at least the original request; the len(via)==0 guard is defensive
// against any future direct caller.
func noCrossHostRedirect(req *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	origin := via[0].URL.Hostname()
	if target := req.URL.Hostname(); !strings.EqualFold(target, origin) {
		// Wrap the sentinel so redact.Reason can classify this guard across the
		// *url.Error that http.Client.Do wraps it in, without importing this
		// package. The sentinel's own text still contains "cross-host redirect".
		return fmt.Errorf("%w from %q to %q", redact.ErrCrossHostRedirect, origin, target)
	}
	return nil
}

// newHTTPClient builds the HTTP client shared by every Centreon request. It
// always installs noCrossHostRedirect so the X-AUTH-TOKEN header cannot leak
// across a cross-host redirect (see that function). When allowSelfSigned is set
// it clones http.DefaultTransport and disables TLS verification for self-signed
// Centreon instances; cloning (rather than mutating the shared default) both
// avoids polluting other users of http.DefaultTransport and preserves settings
// like ForceAttemptHTTP2.
func newHTTPClient(allowSelfSigned bool) (*http.Client, error) {
	hc := &http.Client{
		Timeout:       httpClientTimeout,
		CheckRedirect: noCrossHostRedirect,
	}
	if allowSelfSigned {
		defaultTransport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return nil, fmt.Errorf("unexpected default transport type")
		}
		transport := defaultTransport.Clone()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // user-requested self-signed cert support
		hc.Transport = transport
	}
	return hc, nil
}

// newCentreonClient creates a centreon.Client with the given config. In
// production httpClient is always the newHTTPClient product, so the cross-host
// redirect guard and shared timeout apply to every request; it may be nil in
// tests, in which case the dependency's default client (no redirect guard) is used.
//
// It reads only the auth credentials from cfg (Token, or Username/Password); the
// host comes from the host parameter. It deliberately does not read
// cfg.AllowSelfSigned: self-signed TLS is applied once on httpClient by
// newHTTPClient. Building a fresh per-client transport from that flag here would
// drop httpClient's cross-host redirect guard and reopen the X-AUTH-TOKEN leak
// (CWE-522, #36); if it must vary per client, clone httpClient and keep its
// CheckRedirect.
func newCentreonClient(host string, cfg *Config, logger *slog.Logger, httpClient *http.Client) (*centreon.Client, error) {
	opts := []centreon.Option{}
	if cfg.Token != "" {
		opts = append(opts, centreon.WithAPIToken(cfg.Token))
	} else {
		opts = append(opts, centreon.WithCredentials(cfg.Username, cfg.Password))
	}
	if logger != nil {
		opts = append(opts, centreon.WithLogger(logger))
	}
	if httpClient != nil {
		opts = append(opts, centreon.WithHTTPClient(httpClient))
	}
	return centreon.NewClient(host, opts...)
}

// warnIfAllowlistIneffective emits a startup warning when CENTREON_ALLOWED_HOSTS
// is configured but the effective mode will never consult it. The allowlist is
// only enforced in gateway mode (http transport with gateway auth); in every
// other mode a set value is silently a no-op, so without this signal an operator
// can believe they have restricted the reachable hosts when they have not. This
// is a least-surprise warning, not a security control: outside gateway mode
// there is no per-request X-Centreon-Host to restrict. It mirrors the gateway
// allowlist status lines emitted in runHTTP.
func warnIfAllowlistIneffective(cfg *Config, logger *slog.Logger) {
	if len(cfg.AllowedHosts) == 0 || gatewayMode(cfg) {
		return
	}
	logger.Warn("CENTREON_ALLOWED_HOSTS is set but is only enforced in gateway mode (AUTH_MODE=gateway with MCP_TRANSPORT=http); it has no effect with the current transport/auth mode",
		"transport", cfg.Transport, "authMode", cfg.AuthMode)
}

// run starts the server with the configured transport.
func run(ctx context.Context, cfg *Config, logger *slog.Logger) error {
	logger.Info("centreon-mcp-go starting", "version", version, "transport", cfg.Transport)
	warnIfAllowlistIneffective(cfg, logger)

	// One HTTP client, shared across all requests. It always installs the
	// cross-host redirect guard (X-AUTH-TOKEN must not leak, CWE-522) and, when
	// configured, self-signed TLS support. Building it here means every mode
	// (stdio, env HTTP, gateway per-request, shutdown logout) gets the guard.
	httpClient, err := newHTTPClient(cfg.AllowSelfSigned)
	if err != nil {
		return fmt.Errorf("creating HTTP client: %w", err)
	}

	switch cfg.Transport {
	case transportStdio:
		return runStdio(ctx, cfg, logger, httpClient, &mcp.StdioTransport{})
	case transportHTTP:
		return runHTTP(ctx, cfg, logger, httpClient)
	default:
		return fmt.Errorf("unknown transport %q: expected \"stdio\" or \"http\"", cfg.Transport)
	}
}

// runStdio starts the MCP server on the given transport (mcp.StdioTransport in
// production; an in-memory transport in tests). Taking the transport as a parameter
// keeps the stdio display sink testable end to end without mutating the process's
// os.Stdin/os.Stdout.
func runStdio(ctx context.Context, cfg *Config, logger *slog.Logger, httpClient *http.Client, transport mcp.Transport) error {
	client, err := newCentreonClient(cfg.Host, cfg, logger, httpClient)
	if err != nil {
		return fmt.Errorf("creating centreon client for host %s: %s", safeHost(cfg.Host), redact.Reason(err))
	}

	if cfg.Token == "" {
		if err := client.Login(ctx); err != nil {
			return fmt.Errorf("centreon login failed (host %s): %s", safeHost(cfg.Host), redact.Reason(err))
		}
		defer logoutClientBounded(ctx, client, logger)
		logger.Info("centreon client authenticated", "host", safeHost(cfg.Host))
	}

	s := buildServer(client, logger, displayHost(cfg.Host))
	logger.Info("centreon-mcp-go ready", "transport", "stdio")
	return s.Run(ctx, transport)
}

// logoutClientBounded logs client out during shutdown on a fresh, cancel-immune,
// time-bounded context. It is the stdio counterpart to gracefulShutdown's
// shared-client logout: the ctx passed to runStdio is cancelled on shutdown, so the
// logout runs on context.WithoutCancel to survive that cancellation, but it must
// still be bounded by shutdownLogoutTimeout so an unreachable or hung Centreon
// cannot block process exit (issue #38).
//
// It debug-logs and swallows its own logout error so best-effort cleanup never
// fails the process, and logs success at info to match gracefulShutdown's
// shared-client path and pair with runStdio's login-success line. It reuses the
// logger-bearing client from runStdio (like gracefulShutdown's shared client,
// unlike logoutCachedToken's nil-logger client), so a failing shutdown logout can
// still surface a line from the centreon client's own request logging.
func logoutClientBounded(ctx context.Context, client *centreon.Client, logger *slog.Logger) {
	logoutCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownLogoutTimeout)
	defer cancel()
	if err := client.Logout(logoutCtx); err != nil {
		logger.Debug("centreon client logout failed", "error", redact.Reason(err))
	} else {
		logger.Info("centreon client logged out")
	}
}

// runHTTP starts the MCP server over HTTP.
func runHTTP(ctx context.Context, cfg *Config, logger *slog.Logger, httpClient *http.Client) error {
	var sharedClient *centreon.Client
	var tokenCache *TokenCache
	// envDisplayHost is the redacted host the tools report in env mode; computed
	// once here rather than per request. In gateway mode the per-request host is
	// used instead (see gatewayServer), so this stays empty.
	var envDisplayHost string

	if cfg.AuthMode == authModeEnv {
		client, err := newCentreonClient(cfg.Host, cfg, logger, httpClient)
		if err != nil {
			return fmt.Errorf("creating centreon client for host %s: %s", safeHost(cfg.Host), redact.Reason(err))
		}
		if cfg.Token == "" {
			if err := client.Login(ctx); err != nil {
				return fmt.Errorf("centreon login failed (host %s): %s", safeHost(cfg.Host), redact.Reason(err))
			}
			logger.Info("centreon client authenticated", "host", safeHost(cfg.Host))
		}
		sharedClient = client
		envDisplayHost = displayHost(cfg.Host)
	} else {
		tokenCache = NewTokenCache(tokenCacheTTL)
		if len(cfg.AllowedHosts) == 0 {
			logger.Warn("gateway: no host allowlist configured; X-Centreon-Host accepts any value (set CENTREON_ALLOWED_HOSTS to restrict)")
		} else {
			logger.Info("gateway: host allowlist active", "count", len(cfg.AllowedHosts))
		}
	}

	getServer := func(r *http.Request) *mcp.Server {
		if cfg.AuthMode == authModeEnv {
			return buildServer(sharedClient, logger, envDisplayHost)
		}
		return gatewayServer(r, cfg, tokenCache, logger, httpClient)
	}

	mcpHandler := mcp.NewStreamableHTTPHandler(getServer, &mcp.StreamableHTTPOptions{
		Logger: logger,
	})

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":    "ok",
			"transport": transportHTTP,
			"authMode":  cfg.AuthMode,
			"version":   version,
		})
	})

	addr := net.JoinHostPort(cfg.HTTPHost, strconv.Itoa(cfg.HTTPPort))
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadTimeout:       readTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	// serveCtx lets the shutdown goroutine run cleanup on a ListenAndServe startup
	// error (e.g. a failed bind) as well as on a signal. In env mode the shared
	// client has already logged in by this point, so that session must still be
	// logged out even if the listener never comes up.
	serveCtx, cancelServe := context.WithCancel(ctx)
	defer cancelServe()

	shutdownDone := make(chan struct{})
	go gracefulShutdown(serveCtx, httpServer, sharedClient, tokenCache, cfg, logger, httpClient, shutdownDone)

	logger.Info("centreon-mcp-go HTTP server listening", "addr", addr, "authMode", cfg.AuthMode)
	err := httpServer.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		// A startup error (e.g. a failed bind) returns here without ctx being
		// cancelled. Signal the shutdown goroutine so it still runs cleanup (an
		// env-mode client may already be logged in), wait for it, then return err.
		cancelServe()
		<-shutdownDone
		return err
	}
	// ListenAndServe returns ErrServerClosed as soon as Shutdown is *called*, not
	// when it completes. Wait for the shutdown goroutine so in-flight requests are
	// drained and cached sessions are logged out before the process exits.
	<-shutdownDone
	return nil
}

// gracefulShutdown waits for serveCtx to be cancelled (a signal or a startup
// error), shuts the HTTP server down, then logs out the Centreon sessions the
// server holds: the env-mode shared client or every session in the gateway token
// cache. It closes done when finished so runHTTP can wait for cleanup before
// returning.
func gracefulShutdown(serveCtx context.Context, httpServer *http.Server, sharedClient *centreon.Client, tokenCache *TokenCache, cfg *Config, logger *slog.Logger, httpClient *http.Client, done chan<- struct{}) {
	defer close(done)
	<-serveCtx.Done()

	shutdownCtx, cancelShutdown := context.WithTimeout(context.WithoutCancel(serveCtx), shutdownTimeout)
	defer cancelShutdown()
	_ = httpServer.Shutdown(shutdownCtx)

	// Best-effort session logout runs on a budget separate from Shutdown's, so a
	// slow Shutdown cannot leave the logout with an already-expired context. Both
	// modes use it, so the two paths stay consistent. Shutdown has waited only up to
	// shutdownTimeout; if it timed out, a straggler request may still hold a cached
	// token, but the process is exiting, so logging it out is acceptable.
	logoutCtx, cancelLogout := context.WithTimeout(context.WithoutCancel(serveCtx), shutdownLogoutTimeout)
	defer cancelLogout()

	if sharedClient != nil && cfg.Token == "" {
		if err := sharedClient.Logout(logoutCtx); err != nil {
			logger.Debug("centreon client logout failed", "error", redact.Reason(err))
		} else {
			logger.Info("centreon client logged out")
		}
	}
	if tokenCache != nil {
		drainAndLogout(logoutCtx, tokenCache, logger, httpClient)
	}
}

// gatewayServer creates a per-request MCP server from gateway headers.
func gatewayServer(r *http.Request, cfg *Config, tokenCache *TokenCache, logger *slog.Logger, httpClient *http.Client) *mcp.Server {
	host := r.Header.Get("X-Centreon-Host")
	username := r.Header.Get("X-Centreon-Username")
	password := r.Header.Get("X-Centreon-Password")
	token := r.Header.Get("X-Centreon-Token")

	if host == "" {
		logger.Error("gateway: missing X-Centreon-Host header")
		return nil
	}

	if !hostAllowed(host, cfg.AllowedHosts) {
		logger.Error("gateway: host not in allowlist", "host", safeHost(host))
		return nil
	}

	if err := validateHostScheme(host, cfg.AllowHTTP); err != nil {
		logger.Error("gateway: host rejected", "host", safeHost(host), "error", err)
		return nil
	}

	gwCfg := &Config{}

	switch {
	case token != "":
		gwCfg.Token = token
	case username != "" && password != "":
		if cached, ok := tokenCache.Get(host, username, password); ok {
			logger.Debug("gateway: using cached token", "host", safeHost(host))
			gwCfg.Token = cached
		} else {
			gwCfg.Username = username
			gwCfg.Password = password
		}
	default:
		logger.Error("gateway: missing credentials", "host", safeHost(host))
		return nil
	}

	client, err := newCentreonClient(host, gwCfg, logger, httpClient)
	if err != nil {
		logger.Error("gateway: failed to create client", "host", safeHost(host), "error", redact.Reason(err))
		return nil
	}

	if gwCfg.Token == "" {
		if err := client.Login(r.Context()); err != nil {
			logger.Error("gateway: authentication failed", "host", safeHost(host), "error", redact.Reason(err))
			return nil
		}
		// Cache the token for subsequent requests
		if tok := client.Token(); tok != "" {
			tokenCache.Set(host, username, password, tok)
			logger.Debug("gateway: cached token after login", "host", safeHost(host))
		}
	}

	logger.Debug("gateway: created per-request client", "host", safeHost(host))
	return buildServer(client, logger, displayHost(host))
}

// hostAllowed reports whether host may be used in gateway mode. An empty
// allowlist disables the check and accepts any host. Otherwise the host must
// match an allowlist entry exactly. The comparison uses the raw header value
// (net/http already strips surrounding whitespace from header values) so the
// value that is validated here is identical to the one used to build the client.
func hostAllowed(host string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	return slices.Contains(allowed, host)
}

// drainAndLogout empties the gateway token cache and logs out each cached
// Centreon session. It runs only during graceful shutdown, after
// httpServer.Shutdown has returned. Shutdown has waited up to shutdownTimeout for
// in-flight handlers; if it did not time out, no request still holds any of these
// tokens and logging them out cannot break a live request. Logouts run
// concurrently, bounded by gatewayLogoutConcurrency and by ctx; once ctx is done
// the loop stops spawning further attempts that would only fail immediately.
func drainAndLogout(ctx context.Context, tc *TokenCache, logger *slog.Logger, httpClient *http.Client) {
	tokens := tc.Drain()
	if len(tokens) == 0 {
		return
	}

	var g errgroup.Group
	g.SetLimit(gatewayLogoutConcurrency)
	for _, ct := range tokens {
		if ctx.Err() != nil {
			break
		}
		g.Go(func() error {
			logoutCachedToken(ctx, ct.Host, ct.Token, logger, httpClient)
			return nil
		})
	}
	_ = g.Wait()
	logger.Info("gateway: token cache drained", "count", len(tokens))
}

// logoutCachedToken best-effort invalidates a single cached gateway session by
// building a token-authenticated client for its host and calling Logout, which
// sends the token as X-AUTH-TOKEN to the Centreon logout endpoint. Failures are
// logged at debug and swallowed: shutdown cleanup must never fail the process.
// The cleanup client is built WITHOUT the shared logger on purpose: the centreon
// client logs every failed request at Error, so on a cancelled or failing
// shutdown logout it would emit dependency-layer Error noise that contradicts
// this function's own debug-and-swallow handling.
func logoutCachedToken(ctx context.Context, host, token string, logger *slog.Logger, httpClient *http.Client) {
	if token == "" {
		return
	}
	gwCfg := &Config{Token: token}
	client, err := newCentreonClient(host, gwCfg, nil, httpClient)
	if err != nil {
		logger.Debug("gateway: failed to build client for token logout", "host", safeHost(host), "error", redact.Reason(err))
		return
	}
	if err := client.Logout(ctx); err != nil {
		logger.Debug("gateway: token logout failed", "host", safeHost(host), "error", redact.Reason(err))
		return
	}
	logger.Debug("gateway: logged out cached token", "host", safeHost(host))
}
