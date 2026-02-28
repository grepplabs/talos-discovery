package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/grepplabs/talos-discovery/internal/metrics"
	"github.com/grepplabs/talos-discovery/internal/runutil"
	"github.com/grepplabs/talos-discovery/internal/web"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/grepplabs/loggo/zlog"
	"github.com/grepplabs/talos-discovery/internal/config"
	"github.com/grepplabs/talos-discovery/internal/state"
)

func StartDiscoveryServer(cfg config.ServiceCommandConfig) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := state.NewState(state.WithSnapshot(cfg.DiscoveryConfig.SnapshotPath))
	if err != nil {
		return fmt.Errorf("failed to create state: %w", err)
	}
	registry := metrics.NewRegistry()
	discoveryServer := NewDiscoveryServer(ctx, st, cfg.DiscoveryConfig, registry)

	var group run.Group
	runutil.AddListenerServer("discovery", &group, cfg.Server, discoveryServer.RunListener)

	if cfg.WebEnable {
		webServer, err := newEmbeddedWebServer(ctx, &group, cfg, discoveryServer, registry)
		if err != nil {
			return fmt.Errorf("failed to initialize embedded web: %w", err)
		}
		runutil.AddListenerServer("web", &group, cfg.WebServer, webServer.RunListener)
		runutil.AddGracefulShutdownActor(&group, ctx, "embedded web server", 2*time.Second, webServer.Shutdown)
	}

	addCleanupActor(&group, ctx, st, cfg.DiscoveryConfig)
	addSnapshotActor(&group, ctx, st, cfg.DiscoveryConfig)
	runutil.AddContextCancelActor(&group, ctx, stop)
	return group.Run()
}

func newEmbeddedWebServer(ctx context.Context, group *run.Group, cfg config.ServiceCommandConfig, discoveryServer *DiscoveryServer, registry *prometheus.Registry) (*web.WebServer, error) {
	opts := []web.WebServerOption{web.WithMetrics(registry)}
	target := strings.TrimSpace(cfg.WebDiscoveryClient.Target)
	if target == "" || strings.EqualFold(target, config.InMemoryTransport) {
		ln := addBufconnDiscoveryListener(group, discoveryServer.RunListener)
		conn, err := newBufconnDiscoveryClientConn(ln)
		if err != nil {
			_ = ln.Close()
			return nil, err
		}
		client := web.NewDiscoveryClientWithConn(conn)
		opts = append(opts, web.WithClient(client))
	} else {
		opts = append(opts, web.WithClientConfig(cfg.WebDiscoveryClient))
	}
	return web.NewWebServer(ctx, registry, opts...)
}

func addBufconnDiscoveryListener(group *run.Group, runWithListener func(net.Listener) error) *bufconn.Listener {
	const bufconnSize = 1024 * 1024

	ln := bufconn.Listen(bufconnSize)
	group.Add(func() error {
		return runWithListener(ln)
	}, func(error) {
		_ = ln.Close()
	})
	return ln
}

func newBufconnDiscoveryClientConn(ln *bufconn.Listener) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return ln.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("failed to create bufconn gRPC client: %w", err)
	}
	return conn, nil
}

func addCleanupActor(group *run.Group, ctx context.Context, st *state.State, cfg config.DiscoveryConfig) {
	if cfg.CleanupInterval <= 0 {
		return
	}
	group.Add(func() error {
		ticker := time.NewTicker(cfg.CleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				st.Cleanup()
			}
		}
	}, func(error) {
	})
}

func addSnapshotActor(group *run.Group, ctx context.Context, st *state.State, cfg config.DiscoveryConfig) {
	if cfg.SnapshotInterval <= 0 || cfg.SnapshotPath == "" {
		return
	}
	group.Add(func() error {
		ticker := time.NewTicker(cfg.SnapshotInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				if err := st.SaveSnapshot(cfg.SnapshotPath); err != nil {
					zlog.Errorf("snapshot on shutdown failed: %v", err)
				}
				return nil
			case <-ticker.C:
				if err := st.SaveSnapshot(cfg.SnapshotPath); err != nil {
					zlog.Errorf("snapshot failed: %v", err)
				}
			}
		}
	}, func(error) {
	})
}
