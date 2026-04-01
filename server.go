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
	"strconv"
	"time"

	centreon "github.com/tphakala/centreon-go-client"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tphakala/centreon-mcp-go/tools"
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
	authModeEnv       = "env"
	transportStdio    = "stdio"
	transportHTTP     = "http"
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
			"transport": "http",
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

	go func() {
		<-ctx.Done()
		shutdownCtx, cancelShutdown := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancelShutdown()
		_ = httpServer.Shutdown(shutdownCtx)

		if sharedClient != nil && cfg.Token == "" {
			_ = sharedClient.Logout(shutdownCtx)
			logger.Info("centreon client logged out")
		}
	}()

	logger.Info("centreon-mcp-go HTTP server listening", "addr", addr, "authMode", cfg.AuthMode)
	if err := httpServer.ListenAndServe(); errors.Is(err, http.ErrServerClosed) {
		return nil
	} else {
		return err
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

	gwCfg := &Config{
		Host:            host,
		AllowSelfSigned: cfg.AllowSelfSigned,
	}

	switch {
	case token != "":
		gwCfg.Token = token
	case username != "" && password != "":
		if cached, ok := tokenCache.Get(host, username); ok {
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
	}

	logger.Debug("gateway: created per-request client", "host", host)
	return buildServer(client, logger)
}
