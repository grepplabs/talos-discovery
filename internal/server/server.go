package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	tlsserverconfig "github.com/grepplabs/cert-source/tls/server/config"
	"github.com/grepplabs/loggo/zlog"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	slogzap "github.com/samber/slog-zap/v2"

	"github.com/grepplabs/talos-discovery/internal/config"
	"github.com/grepplabs/talos-discovery/internal/state"
)

func StartDiscoveryServer(cfg config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	st, err := state.NewState(state.WithSnapshot(cfg.DiscoveryConfig.SnapshotPath))
	if err != nil {
		return fmt.Errorf("failed to create state: %w", err)
	}
	registry := newRegistry()
	discoveryServer := NewDiscoveryServer(ctx, st, cfg.DiscoveryConfig, registry)

	var group run.Group
	addListenerServer(&group, cfg, discoveryServer.RunListener)
	addCleanupActor(&group, ctx, st, cfg.DiscoveryConfig)
	addSnapshotActor(&group, ctx, st, cfg.DiscoveryConfig)
	addContextCancelActor(&group, ctx, stop)
	return group.Run()
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

func addListenerServer(group *run.Group, cfg config.Config, runWithListener func(net.Listener) error) {
	var ln net.Listener
	group.Add(func() error {
		listener, err := buildListener(cfg.Server)
		if err != nil {
			return fmt.Errorf("error building listener: %w", err)
		}
		ln = listener

		msg := "starting discovery service"
		if cfg.Server.TLS.Enable {
			msg = "starting TLS discovery server"
		}
		zlog.Infof("%s on %s (version: %s)", msg, cfg.Server.Addr, getVersion())

		return runWithListener(ln)
	}, func(error) {
		if ln != nil {
			_ = ln.Close()
		}
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

func addContextCancelActor(group *run.Group, ctx context.Context, stop context.CancelFunc) {
	group.Add(
		func() error {
			<-ctx.Done()
			zlog.Infof("stop received, shutting down")
			return nil
		},
		func(error) {
			stop()
		},
	)
}

func buildListener(cfg config.ServerConfig) (net.Listener, error) {
	//nolint:noctx
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("error listening on %s: %w", cfg.Addr, err)
	}
	if !cfg.TLS.Enable {
		return ln, nil
	}
	logger := slog.New(slogzap.Option{Logger: zlog.LogSink}.NewZapHandler())
	tlsConfig, err := tlsserverconfig.GetServerTLSConfig(logger, &cfg.TLS)
	if err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("error creating TLS server config: %w", err)
	}
	return tls.NewListener(ln, tlsConfig), nil
}

func newRegistry() *prometheus.Registry {
	registerer := prometheus.NewRegistry()
	registerer.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return registerer
}

func getVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		return bi.Main.Version
	}
	return config.Version
}
