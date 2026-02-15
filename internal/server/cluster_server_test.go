package server

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	discoveryv1alpha1 "github.com/siderolabs/discovery-api/api/v1alpha1/server/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/grepplabs/talos-discovery/internal/config"
	"github.com/grepplabs/talos-discovery/internal/state"
)

func TestHello_ValidClusterID(t *testing.T) {
	env := newTestEnv(t)
	resp, err := env.client.Hello(context.Background(), &discoveryv1alpha1.HelloRequest{
		ClusterId: "cluster-1",
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestHello_EmptyClusterID(t *testing.T) {
	env := newTestEnv(t)
	_, err := env.client.Hello(context.Background(), &discoveryv1alpha1.HelloRequest{})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestHello_RedirectEndpoint(t *testing.T) {
	env := newTestEnv(t, config.DiscoveryConfig{RedirectEndpoint: "other.example.com:443"})
	resp, err := env.client.Hello(context.Background(), &discoveryv1alpha1.HelloRequest{
		ClusterId: "cluster-1",
	})
	require.NoError(t, err)
	require.NotNil(t, resp.GetRedirect())
	assert.Equal(t, "other.example.com:443", resp.GetRedirect().GetEndpoint())
}

func TestHello_NoRedirectEndpoint(t *testing.T) {
	env := newTestEnv(t)
	resp, err := env.client.Hello(context.Background(), &discoveryv1alpha1.HelloRequest{
		ClusterId: "cluster-1",
	})
	require.NoError(t, err)
	assert.Nil(t, resp.GetRedirect())
}

func TestHello_ClientIPFromXRealIP(t *testing.T) {
	env := newTestEnv(t)
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-real-ip", "1.2.3.4"))
	resp, err := env.client.Hello(ctx, &discoveryv1alpha1.HelloRequest{ClusterId: "cluster-1"})
	require.NoError(t, err)
	assert.Equal(t, []byte{1, 2, 3, 4}, resp.GetClientIp())
}

func TestHello_ClientIPFromXForwardedFor(t *testing.T) {
	env := newTestEnv(t)
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-forwarded-for", "5.6.7.8, 9.10.11.12"))
	resp, err := env.client.Hello(ctx, &discoveryv1alpha1.HelloRequest{ClusterId: "cluster-1"})
	require.NoError(t, err)
	assert.Equal(t, []byte{5, 6, 7, 8}, resp.GetClientIp())
}

func TestAffiliateUpdate_Valid(t *testing.T) {
	env := newTestEnv(t)
	_, err := env.client.AffiliateUpdate(context.Background(), affiliateUpdateReq("cluster-1", "node-1"))
	require.NoError(t, err)
}

func TestAffiliateUpdate_EmptyClusterID(t *testing.T) {
	env := newTestEnv(t)
	req := affiliateUpdateReq("", "node-1")
	_, err := env.client.AffiliateUpdate(context.Background(), req)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestAffiliateUpdate_EmptyAffiliateID(t *testing.T) {
	env := newTestEnv(t)
	req := affiliateUpdateReq("cluster-1", "")
	_, err := env.client.AffiliateUpdate(context.Background(), req)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestAffiliateUpdate_DataTooBig(t *testing.T) {
	env := newTestEnv(t)
	req := affiliateUpdateReq("cluster-1", "node-1")
	req.AffiliateData = make([]byte, state.AffiliateDataMax+1)
	_, err := env.client.AffiliateUpdate(context.Background(), req)
	assert.Equal(t, codes.OutOfRange, status.Code(err))
}

func TestAffiliateUpdate_EndpointTooBig(t *testing.T) {
	env := newTestEnv(t)
	req := affiliateUpdateReq("cluster-1", "node-1")
	req.AffiliateEndpoints = [][]byte{make([]byte, state.AffiliateEndpointMax+1)}
	_, err := env.client.AffiliateUpdate(context.Background(), req)
	assert.Equal(t, codes.OutOfRange, status.Code(err))
}

func TestAffiliateUpdate_TTLNil(t *testing.T) {
	env := newTestEnv(t)
	req := affiliateUpdateReq("cluster-1", "node-1")
	req.Ttl = nil
	_, err := env.client.AffiliateUpdate(context.Background(), req)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestAffiliateUpdate_TTLTooLong(t *testing.T) {
	env := newTestEnv(t)
	req := affiliateUpdateReq("cluster-1", "node-1")
	req.Ttl = durationpb.New(state.TTLMax + time.Second)
	_, err := env.client.AffiliateUpdate(context.Background(), req)
	assert.Equal(t, codes.OutOfRange, status.Code(err))
}

func TestAffiliateUpdate_TooManyAffiliates(t *testing.T) {
	env := newTestEnv(t)

	for i := 0; i < state.AffiliatesMax; i++ {
		id := fmt.Sprintf("node-%d", i)
		req := affiliateUpdateReq("cluster-1", id)
		_, err := env.client.AffiliateUpdate(context.Background(), req)
		require.NoError(t, err)
	}

	req := affiliateUpdateReq("cluster-1", "overflow")
	_, err := env.client.AffiliateUpdate(context.Background(), req)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
}

func TestAffiliateDelete_ExistingAffiliate(t *testing.T) {
	env := newTestEnv(t)

	_, err := env.client.AffiliateUpdate(context.Background(), affiliateUpdateReq("cluster-1", "node-1"))
	require.NoError(t, err)

	_, err = env.client.AffiliateDelete(context.Background(), &discoveryv1alpha1.AffiliateDeleteRequest{
		ClusterId:   "cluster-1",
		AffiliateId: "node-1",
	})
	require.NoError(t, err)

	// Verify via List.
	resp, err := env.client.List(context.Background(), &discoveryv1alpha1.ListRequest{ClusterId: "cluster-1"})
	require.NoError(t, err)
	assert.Empty(t, resp.GetAffiliates())
}

func TestAffiliateDelete_NonExistentAffiliate(t *testing.T) {
	env := newTestEnv(t)
	_, err := env.client.AffiliateDelete(context.Background(), &discoveryv1alpha1.AffiliateDeleteRequest{
		ClusterId:   "cluster-1",
		AffiliateId: "ghost",
	})
	require.NoError(t, err, "deleting non-existent affiliate should be a no-op")
}

func TestAffiliateDelete_NonExistentCluster(t *testing.T) {
	env := newTestEnv(t)
	_, err := env.client.AffiliateDelete(context.Background(), &discoveryv1alpha1.AffiliateDeleteRequest{
		ClusterId:   "no-such-cluster",
		AffiliateId: "node-1",
	})
	require.NoError(t, err, "deleting from a non-existent cluster should be a no-op")
}

func TestAffiliateDelete_EmptyClusterID(t *testing.T) {
	env := newTestEnv(t)
	_, err := env.client.AffiliateDelete(context.Background(), &discoveryv1alpha1.AffiliateDeleteRequest{
		AffiliateId: "node-1",
	})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestAffiliateDelete_EmptyAffiliateID(t *testing.T) {
	env := newTestEnv(t)
	_, err := env.client.AffiliateDelete(context.Background(), &discoveryv1alpha1.AffiliateDeleteRequest{
		ClusterId: "cluster-1",
	})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestList_EmptyCluster(t *testing.T) {
	env := newTestEnv(t)
	resp, err := env.client.List(context.Background(), &discoveryv1alpha1.ListRequest{ClusterId: "cluster-1"})
	require.NoError(t, err)
	assert.Empty(t, resp.GetAffiliates())
}

func TestList_NonExistentCluster(t *testing.T) {
	env := newTestEnv(t)
	resp, err := env.client.List(context.Background(), &discoveryv1alpha1.ListRequest{ClusterId: "no-such"})
	require.NoError(t, err)
	assert.Empty(t, resp.GetAffiliates())
}

func TestList_ReturnsAffiliates(t *testing.T) {
	env := newTestEnv(t)

	_, err := env.client.AffiliateUpdate(context.Background(), affiliateUpdateReq("cluster-1", "node-1"))
	require.NoError(t, err)
	_, err = env.client.AffiliateUpdate(context.Background(), affiliateUpdateReq("cluster-1", "node-2"))
	require.NoError(t, err)

	resp, err := env.client.List(context.Background(), &discoveryv1alpha1.ListRequest{ClusterId: "cluster-1"})
	require.NoError(t, err)
	assert.Len(t, resp.GetAffiliates(), 2)
}

func TestList_EmptyClusterID(t *testing.T) {
	env := newTestEnv(t)
	_, err := env.client.List(context.Background(), &discoveryv1alpha1.ListRequest{})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestList_AffiliateFields(t *testing.T) {
	env := newTestEnv(t)

	req := affiliateUpdateReq("cluster-1", "node-1")
	req.AffiliateData = []byte("my-data")
	req.AffiliateEndpoints = [][]byte{[]byte("10.0.0.1")}
	_, err := env.client.AffiliateUpdate(context.Background(), req)
	require.NoError(t, err)

	resp, err := env.client.List(context.Background(), &discoveryv1alpha1.ListRequest{ClusterId: "cluster-1"})
	require.NoError(t, err)
	require.Len(t, resp.GetAffiliates(), 1)

	aff := resp.GetAffiliates()[0]
	assert.Equal(t, "node-1", aff.GetId())
	assert.Equal(t, []byte("my-data"), aff.GetData())
	require.Len(t, aff.GetEndpoints(), 1)
	assert.Equal(t, []byte("10.0.0.1"), aff.GetEndpoints()[0])
}

func recvWithTimeout(t *testing.T, stream grpc.ServerStreamingClient[discoveryv1alpha1.WatchResponse]) (*discoveryv1alpha1.WatchResponse, error) {
	t.Helper()
	ch := make(chan struct {
		resp *discoveryv1alpha1.WatchResponse
		err  error
	}, 1)
	go func() {
		resp, err := stream.Recv()
		ch <- struct {
			resp *discoveryv1alpha1.WatchResponse
			err  error
		}{resp, err}
	}()
	select {
	case res := <-ch:
		return res.resp, res.err
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Watch response")
		return nil, nil
	}
}

func TestWatch_EmptyClusterID(t *testing.T) {
	env := newTestEnv(t)
	stream, err := env.client.Watch(context.Background(), &discoveryv1alpha1.WatchRequest{})
	require.NoError(t, err)
	_, err = recvWithTimeout(t, stream)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestWatch_EmptySnapshot_NoInitialMessage(t *testing.T) {
	env := newTestEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := env.client.Watch(ctx, &discoveryv1alpha1.WatchRequest{ClusterId: "cluster-1"})
	require.NoError(t, err)

	// No affiliates exist — the server should not send an initial snapshot message.
	// Cancel and expect a cancellation error rather than a snapshot.
	cancel()
	_, err = recvWithTimeout(t, stream)
	assert.Equal(t, codes.Canceled, status.Code(err))
}

func TestWatch_ReceivesInitialSnapshot(t *testing.T) {
	env := newTestEnv(t)

	_, err := env.client.AffiliateUpdate(context.Background(), affiliateUpdateReq("cluster-1", "node-1"))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := env.client.Watch(ctx, &discoveryv1alpha1.WatchRequest{ClusterId: "cluster-1"})
	require.NoError(t, err)

	resp, err := recvWithTimeout(t, stream)
	require.NoError(t, err)
	require.Len(t, resp.GetAffiliates(), 1)
	assert.Equal(t, "node-1", resp.GetAffiliates()[0].GetId())
	assert.False(t, resp.GetDeleted())
}

func TestWatch_ReceivesUpsertEvent(t *testing.T) {
	env := newTestEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := env.client.Watch(ctx, &discoveryv1alpha1.WatchRequest{ClusterId: "cluster-1"})
	require.NoError(t, err)

	// Trigger an update after the watcher is established.
	_, err = env.client.AffiliateUpdate(context.Background(), affiliateUpdateReq("cluster-1", "node-1"))
	require.NoError(t, err)

	resp, err := recvWithTimeout(t, stream)
	require.NoError(t, err)
	assert.False(t, resp.GetDeleted())
	require.Len(t, resp.GetAffiliates(), 1)
	assert.Equal(t, "node-1", resp.GetAffiliates()[0].GetId())
}

func TestWatch_ReceivesDeleteEvent(t *testing.T) {
	env := newTestEnv(t)

	_, err := env.client.AffiliateUpdate(context.Background(), affiliateUpdateReq("cluster-1", "node-1"))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := env.client.Watch(ctx, &discoveryv1alpha1.WatchRequest{ClusterId: "cluster-1"})
	require.NoError(t, err)

	// Drain the initial snapshot.
	_, err = recvWithTimeout(t, stream)
	require.NoError(t, err)

	// Delete the affiliate.
	_, err = env.client.AffiliateDelete(context.Background(), &discoveryv1alpha1.AffiliateDeleteRequest{
		ClusterId:   "cluster-1",
		AffiliateId: "node-1",
	})
	require.NoError(t, err)

	resp, err := recvWithTimeout(t, stream)
	require.NoError(t, err)
	assert.True(t, resp.GetDeleted())
	require.Len(t, resp.GetAffiliates(), 1)
	assert.Equal(t, "node-1", resp.GetAffiliates()[0].GetId())
}

func TestWatch_CancelledContextClosesStream(t *testing.T) {
	env := newTestEnv(t)
	ctx, cancel := context.WithCancel(context.Background())

	stream, err := env.client.Watch(ctx, &discoveryv1alpha1.WatchRequest{ClusterId: "cluster-1"})
	require.NoError(t, err)

	cancel()

	_, err = recvWithTimeout(t, stream)
	assert.Equal(t, codes.Canceled, status.Code(err))
}

func TestWatch_ServerStopClosesStream(t *testing.T) {
	env := newTestEnv(t)

	stream, err := env.client.Watch(context.Background(), &discoveryv1alpha1.WatchRequest{ClusterId: "cluster-1"})
	require.NoError(t, err)

	// srv.Stop() forcefully closes all active server-side streams, which is
	// the correct way to simulate a server shutdown. Cancelling the server
	// context only affects the internal serverStop channel, which Watch does
	// not select on.
	env.srv.Stop()

	_, err = recvWithTimeout(t, stream)
	code := status.Code(err)
	assert.True(t, code == codes.Canceled || code == codes.Unavailable,
		"expected stream to close on server stop, got: %v", err)
}

// --- extractClientIP ---

func TestExtractClientIP_XRealIP(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-real-ip", "1.2.3.4"))
	addr := extractClientIP(ctx)
	assert.True(t, addr.IsValid())
	assert.Equal(t, "1.2.3.4", addr.String())
}

func TestExtractClientIP_XForwardedFor_Single(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-forwarded-for", "5.6.7.8"))
	addr := extractClientIP(ctx)
	assert.Equal(t, "5.6.7.8", addr.String())
}

func TestExtractClientIP_XForwardedFor_Chain(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-forwarded-for", "5.6.7.8, 9.10.11.12"))
	addr := extractClientIP(ctx)
	assert.Equal(t, "5.6.7.8", addr.String(), "should use the first (client) IP in the chain")
}

func TestExtractClientIP_XForwardedFor_WithSpaces(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-forwarded-for", "  5.6.7.8  "))
	addr := extractClientIP(ctx)
	assert.Equal(t, "5.6.7.8", addr.String())
}

func TestExtractClientIP_XRealIPTakesPrecedence(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-real-ip", "1.2.3.4",
		"x-forwarded-for", "5.6.7.8",
	))
	addr := extractClientIP(ctx)
	assert.Equal(t, "1.2.3.4", addr.String(), "x-real-ip should take precedence over x-forwarded-for")
}

func TestExtractClientIP_InvalidXRealIP_FallsBackToXForwardedFor(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-real-ip", "not-an-ip",
		"x-forwarded-for", "5.6.7.8",
	))
	addr := extractClientIP(ctx)
	assert.Equal(t, "5.6.7.8", addr.String())
}

func TestExtractClientIP_PeerAddress(t *testing.T) {
	ctx := peer.NewContext(context.Background(), &peer.Peer{
		Addr: &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 12345},
	})
	addr := extractClientIP(ctx)
	assert.Equal(t, "192.168.1.1", addr.String())
}

func TestExtractClientIP_NoMetadata_Nopeer(t *testing.T) {
	addr := extractClientIP(context.Background())
	assert.False(t, addr.IsValid())
}

func TestExtractClientIP_InvalidXForwardedFor_FallsBackToPeer(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-forwarded-for", "not-an-ip"))
	ctx = peer.NewContext(ctx, &peer.Peer{
		Addr: &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 9000},
	})
	addr := extractClientIP(ctx)
	assert.Equal(t, "10.0.0.1", addr.String())
}

func TestToProtoAffiliate(t *testing.T) {
	info := state.AffiliateInfo{
		ID:        "node-1",
		Data:      []byte("data"),
		Endpoints: [][]byte{[]byte("10.0.0.1")},
	}
	proto := toProtoAffiliate(info)
	assert.Equal(t, "node-1", proto.GetId())
	assert.Equal(t, []byte("data"), proto.GetData())
	require.Len(t, proto.GetEndpoints(), 1)
	assert.Equal(t, []byte("10.0.0.1"), proto.GetEndpoints()[0])
}

func TestToProtoAffiliate_NilEndpoints(t *testing.T) {
	info := state.AffiliateInfo{ID: "node-1"}
	proto := toProtoAffiliate(info)
	assert.Equal(t, "node-1", proto.GetId())
	assert.Nil(t, proto.GetEndpoints())
}

// test harness

type testEnv struct {
	srv    *grpc.Server
	client discoveryv1alpha1.ClusterClient
	conn   *grpc.ClientConn
	stop   func()
}

func newTestEnv(t *testing.T, cfg ...config.DiscoveryConfig) *testEnv {
	t.Helper()

	st, err := state.NewState()
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())

	discoveryCfg := config.DiscoveryConfig{}
	if len(cfg) > 0 {
		discoveryCfg = cfg[0]
	}

	srv := grpc.NewServer()
	cs := newClusterServer(ctx, st, discoveryCfg)
	discoveryv1alpha1.RegisterClusterServer(srv, cs)

	//nolint:noctx
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func() { _ = srv.Serve(lis) }()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	t.Cleanup(func() {
		cancel()
		conn.Close()
		srv.GracefulStop()
	})

	return &testEnv{
		client: discoveryv1alpha1.NewClusterClient(conn),
		srv:    srv,
		conn:   conn,
		stop:   cancel,
	}
}

func validTTL() *durationpb.Duration { return durationpb.New(time.Minute) }

func affiliateUpdateReq(clusterID, affiliateID string) *discoveryv1alpha1.AffiliateUpdateRequest {
	return &discoveryv1alpha1.AffiliateUpdateRequest{
		ClusterId:     clusterID,
		AffiliateId:   affiliateID,
		AffiliateData: []byte("data"),
		Ttl:           validTTL(),
	}
}
