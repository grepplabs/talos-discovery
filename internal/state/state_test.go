package state

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// zeroTTL returns a TTL that is already expired by the time it is used.
func zeroTTL() time.Duration { return -1 * time.Second }

func TestCluster_Prune_RemovesExpiredAffiliate(t *testing.T) {
	c := NewCluster("test-cluster")

	err := c.UpdateAffiliate(context.Background(), AffiliateUpdateRequest{
		ID:   "node-1",
		TTL:  zeroTTL(),
		Data: []byte("some-data"),
	})
	require.NoError(t, err)

	affiliates, err := c.ListAffiliates(context.Background())
	require.NoError(t, err)
	require.Len(t, affiliates, 1)

	c.Prune()

	affiliates, err = c.ListAffiliates(context.Background())
	require.NoError(t, err)
	assert.Empty(t, affiliates, "expired affiliate should be pruned")
}

func TestCluster_Prune_RemovesExpiredAffiliate_NotifiesSubscribers(t *testing.T) {
	c := NewCluster("test-cluster")

	snapshot, sub, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)
	require.Empty(t, snapshot)
	defer sub.Close()

	err = c.UpdateAffiliate(context.Background(), AffiliateUpdateRequest{
		ID:   "node-1",
		TTL:  zeroTTL(),
		Data: []byte("some-data"),
	})
	require.NoError(t, err)
	// drain the upsert event
	<-sub.Events()

	c.Prune()

	select {
	case ev := <-sub.Events():
		assert.Equal(t, AffiliateEventDelete, ev.Type)
		assert.Equal(t, "node-1", ev.ID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for delete event from Prune")
	}
}

func TestCluster_Prune_RemovesExpiredEndpoints(t *testing.T) {
	c := NewCluster("test-cluster")

	endpoint1 := []byte("10.0.0.1")
	endpoint2 := []byte("10.0.0.2")

	// Add endpoint1 with an already-expired TTL.
	err := c.UpdateAffiliate(context.Background(), AffiliateUpdateRequest{
		ID:        "node-1",
		TTL:       zeroTTL(),
		Endpoints: [][]byte{endpoint1},
	})
	require.NoError(t, err)

	// Add endpoint2 with a long TTL — also refreshes the affiliate expiry.
	err = c.UpdateAffiliate(context.Background(), AffiliateUpdateRequest{
		ID:        "node-1",
		TTL:       time.Hour,
		Endpoints: [][]byte{endpoint2},
	})
	require.NoError(t, err)

	c.Prune()

	affiliates, err := c.ListAffiliates(context.Background())
	require.NoError(t, err)
	require.Len(t, affiliates, 1)

	eps := affiliates[0].Endpoints
	require.Len(t, eps, 1, "only the non-expired endpoint should remain")
	assert.Equal(t, endpoint2, eps[0])
}

func TestCluster_Prune_RemovesExpiredEndpoints_NotifiesSubscribers(t *testing.T) {
	c := NewCluster("test-cluster")

	snapshot, sub, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)
	require.Empty(t, snapshot)
	defer sub.Close()

	endpoint1 := []byte("10.0.0.1")
	endpoint2 := []byte("10.0.0.2")

	err = c.UpdateAffiliate(context.Background(), AffiliateUpdateRequest{
		ID:        "node-1",
		TTL:       zeroTTL(),
		Endpoints: [][]byte{endpoint1},
	})
	require.NoError(t, err)
	<-sub.Events() // drain upsert

	err = c.UpdateAffiliate(context.Background(), AffiliateUpdateRequest{
		ID:        "node-1",
		TTL:       time.Hour,
		Endpoints: [][]byte{endpoint2},
	})
	require.NoError(t, err)
	<-sub.Events() // drain upsert

	c.Prune()

	select {
	case ev := <-sub.Events():
		assert.Equal(t, AffiliateEventUpsert, ev.Type)
		require.Len(t, ev.Endpoints, 1)
		assert.Equal(t, endpoint2, ev.Endpoints[0])
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for upsert event from Prune")
	}
}

func TestState_Cleanup_RemovesExpiredCluster(t *testing.T) {
	s, err := NewState()
	require.NoError(t, err)
	c := s.ClusterFor("test-cluster")

	// Force cluster expiry by backdating it.
	c.expiry.Store(time.Now().Add(-time.Second).UnixNano())

	s.Cleanup()

	_, exists := s.GetCluster("test-cluster")
	assert.False(t, exists, "expired empty cluster should be removed by Cleanup")
}
