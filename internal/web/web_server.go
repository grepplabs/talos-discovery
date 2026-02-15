package web

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/grepplabs/loggo/zlog"
	"github.com/grepplabs/talos-discovery/internal/config"
	"github.com/grepplabs/talos-discovery/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type WebServerOption func(*webServerOptions)
type webServerOptions struct {
	discoveryClient *DiscoveryClient
	clientConfig    config.ClientConfig
	metricsRegistry *prometheus.Registry
}

type WebServer struct {
	engine          *gin.Engine
	httpServer      *http.Server
	discoveryClient *DiscoveryClient
}

func WithMetrics(registry *prometheus.Registry) WebServerOption {
	return func(opts *webServerOptions) {
		opts.metricsRegistry = registry
	}
}

func WithClient(client *DiscoveryClient) WebServerOption {
	return func(opts *webServerOptions) {
		opts.discoveryClient = client
	}
}

func WithClientConfig(clientConfig config.ClientConfig) WebServerOption {
	return func(opts *webServerOptions) {
		opts.clientConfig = clientConfig
	}
}

func NewWebServer(ctx context.Context, registry *prometheus.Registry, opts ...WebServerOption) (*WebServer, error) {
	serverOpts := &webServerOptions{}
	for _, opt := range opts {
		opt(serverOpts)
	}

	client := serverOpts.discoveryClient
	if client == nil {
		if serverOpts.clientConfig.Target == "" {
			return nil, errors.New("missing discovery client configuration: provide WithClient or WithClientConfig")
		}
		var err error
		client, err = NewDiscoveryClient(serverOpts.clientConfig)
		if err != nil {
			return nil, fmt.Errorf("create discovery client: %w", err)
		}
	}

	watchManager, err := NewDiscoveryWatchManager(ctx, client, registry)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("create discovery watch manager: %w", err)
	}
	engine := newEngine()
	//nolint:contextcheck
	engine = addDiscoveryEndpoints(engine, client, watchManager)

	if serverOpts.metricsRegistry != nil {
		engine.GET("/metrics", metrics.NewHandlerWithConfig(metrics.HandlerConfig{
			Gatherer: serverOpts.metricsRegistry,
		}))
	}

	return &WebServer{
		engine: engine,
		httpServer: &http.Server{
			Handler:           engine,
			ReadHeaderTimeout: 5 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
		discoveryClient: client,
	}, nil
}

func newEngine() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	logger := zlog.LogSink.WithOptions(zap.WithCaller(false)).With(zap.String("server", "web"))
	engine := gin.New()
	engine.Use(ginzap.GinzapWithConfig(logger, &ginzap.Config{
		TimeFormat: time.RFC3339,
		SkipPaths:  []string{"/healthz", "/readyz", "/metrics"},
	}))
	engine.Use(ginzap.RecoveryWithZap(logger, true))
	addHealthEndpoints(engine)
	return engine
}

func (s *WebServer) RunListener(l net.Listener) error {
	err := s.httpServer.Serve(l)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *WebServer) Shutdown(ctx context.Context) error {
	err := s.httpServer.Shutdown(ctx)
	s.discoveryClient.Close()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func addHealthEndpoints(engine *gin.Engine) {
	// health and readiness
	engine.GET("/healthz", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	engine.GET("/readyz", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
}
