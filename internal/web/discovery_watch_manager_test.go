package web

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	pb "github.com/siderolabs/discovery-api/api/v1alpha1/server/pb"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestDiscoveryWatchManagerSubscribeUnsubscribeLifecycle(t *testing.T) {
	reg := prometheus.NewRegistry()
	client := newBufconnDiscoveryClient(t, &testClusterServer{
		watchFunc: func(_ *pb.WatchRequest, stream grpc.ServerStreamingServer[pb.WatchResponse]) error {
			<-stream.Context().Done()
			return status.Error(codes.Canceled, "stream canceled")
		},
	})
	manager, err := NewDiscoveryWatchManager(context.Background(), client, reg)
	require.NoError(t, err)

	ch1 := manager.Subscribe("cluster-1")
	ch2 := manager.Subscribe("cluster-1")
	require.NotNil(t, ch1)
	require.NotNil(t, ch2)
	require.Len(t, manager.entries, 1)
	require.Equal(t, 2, manager.entries["cluster-1"].refs)
	require.InDelta(t, 1, testutil.ToFloat64(manager.metrics.activeDiscoveryWatchers), 0)
	require.InDelta(t, 2, testutil.ToFloat64(manager.metrics.activeSubscribers), 0)

	manager.Unsubscribe("cluster-1", ch1)
	require.Len(t, manager.entries, 1)
	require.Equal(t, 1, manager.entries["cluster-1"].refs)
	require.InDelta(t, float64(1), testutil.ToFloat64(manager.metrics.activeDiscoveryWatchers), 0)
	require.InDelta(t, float64(1), testutil.ToFloat64(manager.metrics.activeSubscribers), 0)

	manager.Unsubscribe("cluster-1", ch2)
	require.Empty(t, manager.entries)
	require.InDelta(t, float64(0), testutil.ToFloat64(manager.metrics.activeDiscoveryWatchers), 0)
	require.InDelta(t, float64(0), testutil.ToFloat64(manager.metrics.activeSubscribers), 0)
}

func TestWatchDiscoveryBroadcastsEvent(t *testing.T) {
	reg := prometheus.NewRegistry()
	m, err := newWatchManagerMetrics(reg)
	require.NoError(t, err)
	hub := NewHub(m)
	sub := hub.Subscribe()
	defer hub.Unsubscribe(sub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := newBufconnDiscoveryClient(t, &testClusterServer{
		watchFunc: func(_ *pb.WatchRequest, stream grpc.ServerStreamingServer[pb.WatchResponse]) error {
			if err := stream.Send(&pb.WatchResponse{
				Affiliates: []*pb.Affiliate{
					{Id: "node-1", Data: []byte("data"), Endpoints: [][]byte{[]byte("ep")}},
				},
			}); err != nil {
				return err
			}
			<-stream.Context().Done()
			return status.Error(codes.Canceled, "stream canceled")
		},
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		watchDiscovery(ctx, client.client, "cluster-1", hub, m)
	}()

	select {
	case evt := <-sub:
		var env affiliatesEnvelope
		require.NoError(t, json.Unmarshal([]byte(evt.Data), &env))
		require.Len(t, env.Affiliates, 1)
		require.Equal(t, "node-1", env.Affiliates[0].ID)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watch event")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watch loop did not stop after cancel")
	}

	require.InDelta(t, float64(1), testutil.ToFloat64(m.watchEvents), 0)
}
