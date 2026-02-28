package runutil

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"time"

	tlsserver "github.com/grepplabs/cert-source/tls/server"
	tlsserverconfig "github.com/grepplabs/cert-source/tls/server/config"
	"github.com/grepplabs/loggo/zlog"
	"github.com/grepplabs/talos-discovery/internal/config"
	"github.com/oklog/run"
	slogzap "github.com/samber/slog-zap/v2"
)

func AddContextCancelActor(group *run.Group, ctx context.Context, stop context.CancelFunc) {
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

func AddListenerServer(name string, group *run.Group, cfg config.ServerConfig, runWithListener func(net.Listener) error) {
	var ln net.Listener
	group.Add(func() error {
		listener, err := buildListener(cfg)
		if err != nil {
			return fmt.Errorf("error building listener: %w", err)
		}
		ln = listener

		msg := fmt.Sprintf("starting %s service", name)
		if cfg.TLS.Enable {
			msg = fmt.Sprintf("starting TLS %s service", name)
		}
		zlog.Infof("%s on %s (version: %s)", msg, cfg.Addr, config.GetBuildVersion())

		return runWithListener(ln)
	}, func(error) {
		if ln != nil {
			_ = ln.Close()
		}
	})
}

func AddGracefulShutdownActor(group *run.Group, ctx context.Context, component string, timeout time.Duration, shutdown func(context.Context) error) {
	group.Add(func() error {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown %s: %w", component, err)
		}
		return nil
	}, func(error) {})
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
	tlsConfig, err := tlsserverconfig.GetServerTLSConfig(logger, &cfg.TLS,
		tlsserver.WithTLSServerNextProtos([]string{"h2"}))
	if err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("error creating TLS server config: %w", err)
	}
	return tls.NewListener(ln, tlsConfig), nil
}
