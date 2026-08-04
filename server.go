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
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	centreon "github.com/tphakala/centreon-go-client"
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
	selfSignedTimeout = 30 * time.Second

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

// buildServer creates an MCP server with all tools registered.
func buildServer(client *centreon.Client, logger *slog.Logger) *mcp.Server {
	s := mcp.NewServer(
		&mcp.Implementation{Name: "centreon-mcp-go", Version: version},
		&mcp.ServerOptions{Instructions: serverInstructions},
	)
	tools.RegisterAll(s, client, logger)
	return s
}

// selfSignedHTTPClient creates a reusable HTTP client that accepts self-signed certificates.
// Called once at startup; the returned client is shared across all requests.
func selfSignedHTTPClient() (*http.Client, error) {
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("unexpected default transport type")
	}
	transport := defaultTransport.Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // user-requested self-signed cert support
	return &http.Client{
		Timeout:   selfSignedTimeout,
		Transport: transport,
	}, nil
}

// newCentreonClient creates a centreon.Client with the given config.
// httpClient may be nil; if non-nil it is used for all HTTP requests (e.g. self-signed TLS).
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

// run starts the server with the configured transport.
func run(ctx context.Context, cfg *Config, logger *slog.Logger) error {
	logger.Info("centreon-mcp-go starting", "version", version, "transport", cfg.Transport)

	// Create self-signed HTTP client once, shared across all requests.
	var httpClient *http.Client
	if cfg.AllowSelfSigned {
		var err error
		httpClient, err = selfSignedHTTPClient()
		if err != nil {
			return fmt.Errorf("creating self-signed HTTP client: %w", err)
		}
	}

	switch cfg.Transport {
	case transportStdio:
		return runStdio(ctx, cfg, logger, httpClient)
	case transportHTTP:
		return runHTTP(ctx, cfg, logger, httpClient)
	default:
		return fmt.Errorf("unknown transport %q: expected \"stdio\" or \"http\"", cfg.Transport)
	}
}

// runStdio starts the MCP server on stdin/stdout.
func runStdio(ctx context.Context, cfg *Config, logger *slog.Logger, httpClient *http.Client) error {
	client, err := newCentreonClient(cfg.Host, cfg, logger, httpClient)
	if err != nil {
		return fmt.Errorf("creating centreon client: %w", err)
	}

	if cfg.Token == "" {
		if err := client.Login(ctx); err != nil {
			return fmt.Errorf("centreon login: %w", err)
		}
		defer func() {
			_ = client.Logout(context.WithoutCancel(ctx))
		}()
		logger.Info("centreon client authenticated", "host", cfg.Host)
	}

	s := buildServer(client, logger)
	logger.Info("centreon-mcp-go ready", "transport", "stdio")
	return s.Run(ctx, &mcp.StdioTransport{})
}

// runHTTP starts the MCP server over HTTP.
func runHTTP(ctx context.Context, cfg *Config, logger *slog.Logger, httpClient *http.Client) error {
	var sharedClient *centreon.Client
	var tokenCache *TokenCache

	if cfg.AuthMode == authModeEnv {
		client, err := newCentreonClient(cfg.Host, cfg, logger, httpClient)
		if err != nil {
			return fmt.Errorf("creating centreon client: %w", err)
		}
		if cfg.Token == "" {
			if err := client.Login(ctx); err != nil {
				return fmt.Errorf("centreon login: %w", err)
			}
			logger.Info("centreon client authenticated", "host", cfg.Host)
		}
		sharedClient = client
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
			return buildServer(sharedClient, logger)
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
			logger.Debug("centreon client logout failed", "error", err)
		} else {
			logger.Info("centreon client logged out")
		}
	}
	if tokenCache != nil {
		drainAndLogout(logoutCtx, tokenCache, cfg, logger, httpClient)
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
		logger.Error("gateway: host not in allowlist", "host", host)
		return nil
	}

	gwCfg := &Config{
		Host:            host,
		AllowSelfSigned: cfg.AllowSelfSigned,
	}

	switch {
	case token != "":
		gwCfg.Token = token
	case username != "" && password != "":
		if cached, ok := tokenCache.Get(host, username, password); ok {
			logger.Debug("gateway: using cached token", "host", host)
			gwCfg.Token = cached
		} else {
			gwCfg.Username = username
			gwCfg.Password = password
		}
	default:
		logger.Error("gateway: missing credentials", "host", host)
		return nil
	}

	client, err := newCentreonClient(host, gwCfg, logger, httpClient)
	if err != nil {
		logger.Error("gateway: failed to create client", "host", host, "error", err)
		return nil
	}

	if gwCfg.Token == "" {
		if err := client.Login(r.Context()); err != nil {
			logger.Error("gateway: authentication failed", "host", host, "error", err)
			return nil
		}
		// Cache the token for subsequent requests
		if tok := client.Token(); tok != "" {
			tokenCache.Set(host, username, password, tok)
			logger.Debug("gateway: cached token after login", "host", host)
		}
	}

	logger.Debug("gateway: created per-request client", "host", host)
	return buildServer(client, logger)
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
func drainAndLogout(ctx context.Context, tc *TokenCache, cfg *Config, logger *slog.Logger, httpClient *http.Client) {
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
			logoutCachedToken(ctx, ct.Host, ct.Token, cfg, logger, httpClient)
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
func logoutCachedToken(ctx context.Context, host, token string, cfg *Config, logger *slog.Logger, httpClient *http.Client) {
	if token == "" {
		return
	}
	gwCfg := &Config{Host: host, Token: token, AllowSelfSigned: cfg.AllowSelfSigned}
	client, err := newCentreonClient(host, gwCfg, nil, httpClient)
	if err != nil {
		logger.Debug("gateway: failed to build client for token logout", "host", host, "error", err)
		return
	}
	if err := client.Logout(ctx); err != nil {
		logger.Debug("gateway: token logout failed", "host", host, "error", err)
		return
	}
	logger.Debug("gateway: logged out cached token", "host", host)
}
