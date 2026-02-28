package web

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/grepplabs/talos-discovery/internal/config"
	"github.com/grepplabs/talos-discovery/internal/metrics"
	"github.com/grepplabs/talos-discovery/internal/runutil"
	"github.com/oklog/run"
)

func StartDiscoveryWeb(cfg config.WebCommandConfig) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	registry := metrics.NewRegistry()
	webServer, err := NewWebServer(ctx, registry, WithClientConfig(cfg.WebDiscoveryClient), WithMetrics(registry))
	if err != nil {
		return fmt.Errorf("start discovery web server: %w", err)
	}
	var group run.Group
	runutil.AddContextCancelActor(&group, ctx, stop)
	runutil.AddListenerServer("web", &group, cfg.WebServer, webServer.RunListener)
	runutil.AddGracefulShutdownActor(&group, ctx, "web server", 2*time.Second, webServer.Shutdown)
	return group.Run()
}
