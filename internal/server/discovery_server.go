package server

import (
	"context"
	"errors"
	"net"

	"github.com/grepplabs/loggo/zlog"
	discoveryv1alpha1 "github.com/grepplabs/talos-discovery/api/v1alpha1/server/pb"
	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	grpcrecovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"github.com/grepplabs/talos-discovery/internal/config"
	"github.com/grepplabs/talos-discovery/internal/state"
)

type DiscoveryServer struct {
	grpcServer *grpc.Server
}

func NewDiscoveryServer(ctx context.Context, state *state.State, discoveryConfig config.DiscoveryConfig, registry *prometheus.Registry) *DiscoveryServer {
	srvMetrics := grpcprom.NewServerMetrics(
		grpcprom.WithServerHandlingTimeHistogram(
			grpcprom.WithHistogramBuckets([]float64{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10}),
		),
	)
	registry.MustRegister(srvMetrics)

	loggingOpts := []logging.Option{
		logging.WithLogOnEvents(logging.FinishCall),
		logging.WithLevels(func(code codes.Code) logging.Level {
			//nolint:exhaustive
			switch code {
			case codes.OK:
				return logging.LevelInfo
			case codes.Canceled:
				return logging.LevelInfo
			case codes.DeadlineExceeded:
				return logging.LevelInfo
			default:
				return logging.DefaultServerCodeToLevel(code)
			}
		}),
	}
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			debugStartUnaryInterceptor(),
			logging.UnaryServerInterceptor(grpcInterceptorLogger(), loggingOpts...),
			normalizeContextUnaryServerInterceptor(),
			srvMetrics.UnaryServerInterceptor(),
			grpcrecovery.UnaryServerInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			debugStartStreamInterceptor(),
			logging.StreamServerInterceptor(grpcInterceptorLogger(), loggingOpts...),
			normalizeContextStreamServerInterceptor(),
			srvMetrics.StreamServerInterceptor(),
			grpcrecovery.StreamServerInterceptor(),
		),
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	reflection.Register(grpcServer)
	discoveryv1alpha1.RegisterClusterServer(grpcServer, newClusterServer(ctx, state, discoveryConfig))

	healthServer := health.NewServer()
	healthServer.SetServingStatus("sidero.discovery.server.Cluster", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)

	return &DiscoveryServer{
		grpcServer: grpcServer,
	}
}

func (s *DiscoveryServer) RunListener(ln net.Listener) error {
	return s.grpcServer.Serve(ln)
}

func grpcInterceptorLogger() logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		switch lvl {
		case logging.LevelDebug:
			zlog.DebugCw(ctx, msg, fields...)
		case logging.LevelInfo:
			zlog.InfoCw(ctx, msg, fields...)
		case logging.LevelWarn:
			zlog.WarnCw(ctx, msg, fields...)
		case logging.LevelError:
			zlog.ErrorCw(ctx, msg, fields...)
		default:
			zlog.InfoCw(ctx, msg, fields...)
		}
	})
}

func normalizeContextErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, err.Error())
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, err.Error())
	}
	return err
}

func normalizeContextUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		return resp, normalizeContextErr(err)
	}
}

func normalizeContextStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		err := handler(srv, ss)
		return normalizeContextErr(err)
	}
}

func debugStartUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		zlog.Debugw("started call", "grpc.method", info.FullMethod, "grpc.method_type", "unary")
		return handler(ctx, req)
	}
}

func debugStartStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		methodType := "server_stream"
		if info.IsClientStream && info.IsServerStream {
			methodType = "bidi_stream"
		} else if info.IsClientStream {
			methodType = "client_stream"
		}
		zlog.Debugw("started call", "grpc.method", info.FullMethod, "grpc.method_type", methodType)
		return handler(srv, ss)
	}
}
