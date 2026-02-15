package web

import (
	"context"
	"net"
	"testing"

	pb "github.com/grepplabs/talos-discovery/api/v1alpha1/server/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type testClusterServer struct {
	pb.UnimplementedClusterServer
	listFunc  func(context.Context, *pb.ListRequest) (*pb.ListResponse, error)
	watchFunc func(*pb.WatchRequest, grpc.ServerStreamingServer[pb.WatchResponse]) error
}

func (s *testClusterServer) List(ctx context.Context, req *pb.ListRequest) (*pb.ListResponse, error) {
	if s.listFunc == nil {
		return nil, status.Error(codes.Unimplemented, "list not implemented")
	}
	return s.listFunc(ctx, req)
}

func (s *testClusterServer) Watch(req *pb.WatchRequest, stream grpc.ServerStreamingServer[pb.WatchResponse]) error {
	if s.watchFunc == nil {
		return status.Error(codes.Unimplemented, "watch not implemented")
	}
	return s.watchFunc(req, stream)
}

func newBufconnDiscoveryClient(t *testing.T, srv pb.ClusterServer) *DiscoveryClient {
	t.Helper()

	const bufSize = 1024 * 1024
	ln := bufconn.Listen(bufSize)

	grpcSrv := grpc.NewServer()
	pb.RegisterClusterServer(grpcSrv, srv)

	go func() {
		_ = grpcSrv.Serve(ln)
	}()

	conn, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return ln.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		grpcSrv.Stop()
		_ = ln.Close()
		t.Fatalf("create bufconn grpc client: %v", err)
	}

	t.Cleanup(func() {
		_ = conn.Close()
		grpcSrv.Stop()
		_ = ln.Close()
	})

	return NewDiscoveryClientWithConn(conn)
}
