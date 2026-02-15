package web

import (
	"fmt"
	"log/slog"

	tlsclient "github.com/grepplabs/cert-source/tls/client"
	tlsclientconfig "github.com/grepplabs/cert-source/tls/client/config"
	"github.com/grepplabs/loggo/zlog"
	"github.com/grepplabs/talos-discovery/api/v1alpha1/server/pb"
	slogzap "github.com/samber/slog-zap/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/grepplabs/talos-discovery/internal/config"
)

type DiscoveryClient struct {
	conn   *grpc.ClientConn
	client pb.ClusterClient
}

func NewDiscoveryClient(cfg config.ClientConfig) (*DiscoveryClient, error) {
	var creds credentials.TransportCredentials
	if cfg.TLS.Enable {
		sl := slog.New(slogzap.Option{Logger: zlog.LogSink}.NewZapHandler())
		tlsClientConfigFunc, err := tlsclientconfig.GetTLSClientConfigFunc(sl, &cfg.TLS,
			tlsclient.WithTLSClientNextProtos([]string{"h2"}))
		if err != nil {
			return nil, fmt.Errorf("create tls client config: %w", err)
		}
		creds = credentials.NewTLS(tlsClientConfigFunc())
	} else {
		creds = insecure.NewCredentials()
	}
	conn, err := grpc.NewClient(cfg.Target, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, err
	}
	return NewDiscoveryClientWithConn(conn), nil
}

func NewDiscoveryClientWithConn(conn *grpc.ClientConn) *DiscoveryClient {
	return &DiscoveryClient{
		conn:   conn,
		client: pb.NewClusterClient(conn),
	}
}

func (c *DiscoveryClient) Close() {
	_ = c.conn.Close()
}
