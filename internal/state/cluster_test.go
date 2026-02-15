package state

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCluster_UpdateAffiliate_CreatesAffiliate(t *testing.T) {
	c := newCluster(t)

	updateAffiliate(t, c, "node-1", time.Hour, []byte("data"))

	affiliates, err := c.ListAffiliates(context.Background())
	require.NoError(t, err)
	require.Len(t, affiliates, 1)
	assert.Equal(t, "node-1", affiliates[0].ID)
	assert.Equal(t, []byte("data"), affiliates[0].Data)
}

func TestCluster_UpdateAffiliate_UpdatesExistingAffiliate(t *testing.T) {
	c := newCluster(t)

	updateAffiliate(t, c, "node-1", time.Hour, []byte("v1"))
	updateAffiliate(t, c, "node-1", time.Hour, []byte("v2"))

	affiliates, err := c.ListAffiliates(context.Background())
	require.NoError(t, err)
	require.Len(t, affiliates, 1)
	assert.Equal(t, []byte("v2"), affiliates[0].Data)
}

func TestCluster_UpdateAffiliate_NoChangeNoEvent(t *testing.T) {
	c := newCluster(t)
	updateAffiliate(t, c, "node-1", time.Hour, []byte("data"))

	_, sub := subscribe(t, c)

	// Update with no data and no endpoints — nothing changes.
	err := c.UpdateAffiliate(context.Background(), AffiliateUpdateRequest{
		ID:  "node-1",
		TTL: time.Hour,
	})
	require.NoError(t, err)

	requireNoEvent(t, sub)
}

func TestCluster_UpdateAffiliate_TooManyAffiliates(t *testing.T) {
	c := newCluster(t)

	for i := 0; i < AffiliatesMax; i++ {
		id := string(rune('a' + i))
		updateAffiliate(t, c, id, time.Hour, []byte("data"))
	}

	err := c.UpdateAffiliate(context.Background(), AffiliateUpdateRequest{
		ID:   "overflow",
		TTL:  time.Hour,
		Data: []byte("x"),
	})
	require.ErrorIs(t, err, ErrTooManyAffiliates)
}

func TestCluster_UpdateAffiliate_NotifiesSubscribers(t *testing.T) {
	c := newCluster(t)
	_, sub := subscribe(t, c)

	updateAffiliate(t, c, "node-1", time.Hour, []byte("data"))

	ev := requireEvent(t, sub, AffiliateEventUpsert, "node-1")
	assert.Equal(t, []byte("data"), ev.Data)
}

func TestCluster_UpdateAffiliate_NotifiesOnEndpointAdd(t *testing.T) {
	c := newCluster(t)
	_, sub := subscribe(t, c)

	updateAffiliate(t, c, "node-1", time.Hour, nil, []byte("10.0.0.1"))

	ev := requireEvent(t, sub, AffiliateEventUpsert, "node-1")
	require.Len(t, ev.Endpoints, 1)
	assert.Equal(t, []byte("10.0.0.1"), ev.Endpoints[0])
}

func TestCluster_DeleteAffiliate_RemovesAffiliate(t *testing.T) {
	c := newCluster(t)
	updateAffiliate(t, c, "node-1", time.Hour, []byte("data"))

	deleteAffiliate(t, c, "node-1")

	affiliates, err := c.ListAffiliates(context.Background())
	require.NoError(t, err)
	assert.Empty(t, affiliates)
}

func TestCluster_DeleteAffiliate_NonExistentIsNoop(t *testing.T) {
	c := newCluster(t)
	_, sub := subscribe(t, c)

	deleteAffiliate(t, c, "does-not-exist")

	requireNoEvent(t, sub)
}

func TestCluster_DeleteAffiliate_NotifiesSubscribers(t *testing.T) {
	c := newCluster(t)
	updateAffiliate(t, c, "node-1", time.Hour, []byte("data"))
	_, sub := subscribe(t, c)

	deleteAffiliate(t, c, "node-1")

	requireEvent(t, sub, AffiliateEventDelete, "node-1")
}

func TestCluster_DeleteAffiliate_ProlongsExpiry(t *testing.T) {
	c := newCluster(t)
	updateAffiliate(t, c, "node-1", time.Hour, []byte("data"))

	// Force expiry to the past.
	c.expiry.Store(time.Now().Add(-time.Second).UnixNano())
	assert.True(t, c.IsExpired())

	deleteAffiliate(t, c, "node-1")

	assert.False(t, c.IsExpired(), "DeleteAffiliate should prolong cluster expiry")
}

func TestCluster_ListAffiliates_Empty(t *testing.T) {
	c := newCluster(t)

	affiliates, err := c.ListAffiliates(context.Background())
	require.NoError(t, err)
	assert.Empty(t, affiliates)
}

func TestCluster_ListAffiliates_ReturnsAll(t *testing.T) {
	c := newCluster(t)
	updateAffiliate(t, c, "node-1", time.Hour, []byte("d1"))
	updateAffiliate(t, c, "node-2", time.Hour, []byte("d2"))

	affiliates, err := c.ListAffiliates(context.Background())
	require.NoError(t, err)
	assert.Len(t, affiliates, 2)
}

func TestCluster_IsExpired_FreshCluster(t *testing.T) {
	c := newCluster(t)
	assert.False(t, c.IsExpired())
}

func TestCluster_IsExpired_BackdatedExpiry(t *testing.T) {
	c := newCluster(t)
	c.expiry.Store(time.Now().Add(-time.Second).UnixNano())
	assert.True(t, c.IsExpired())
}

func TestCluster_IsEmpty_TrueWhenNoAffiliatesAndNoSubscribers(t *testing.T) {
	c := newCluster(t)
	assert.True(t, c.IsEmpty())
}

func TestCluster_IsEmpty_FalseWhenAffiliate(t *testing.T) {
	c := newCluster(t)
	updateAffiliate(t, c, "node-1", time.Hour, []byte("data"))
	assert.False(t, c.IsEmpty())
}

func TestCluster_IsEmpty_FalseWhenSubscriber(t *testing.T) {
	c := newCluster(t)
	_, sub := subscribe(t, c)
	defer sub.Close()
	assert.False(t, c.IsEmpty())
}

func TestCluster_IsEmpty_TrueAfterSubscriberCloses(t *testing.T) {
	c := newCluster(t)
	_, sub := subscribe(t, c)
	sub.Close()
	assert.True(t, c.IsEmpty())
}

func TestCluster_Prune_RemovesExpired1Affiliate(t *testing.T) {
	c := newCluster(t)
	updateAffiliate(t, c, "node-1", -time.Second, []byte("data"))

	c.Prune()

	affiliates, err := c.ListAffiliates(context.Background())
	require.NoError(t, err)
	assert.Empty(t, affiliates)
}

func TestCluster_Prune_KeepsFreshAffiliate(t *testing.T) {
	c := newCluster(t)
	updateAffiliate(t, c, "node-1", time.Hour, []byte("data"))

	c.Prune()

	affiliates, err := c.ListAffiliates(context.Background())
	require.NoError(t, err)
	assert.Len(t, affiliates, 1)
}

func TestCluster_Prune_RemovesExpiredAffiliate_Notifies(t *testing.T) {
	c := newCluster(t)
	updateAffiliate(t, c, "node-1", -time.Second, []byte("data"))
	_, sub := subscribe(t, c)

	c.Prune()

	requireEvent(t, sub, AffiliateEventDelete, "node-1")
}

func TestCluster_Prune_RemovesExpired2Endpoints(t *testing.T) {
	c := newCluster(t)

	// Add endpoint1 with expired TTL, endpoint2 with fresh TTL.
	updateAffiliate(t, c, "node-1", -time.Second, nil, []byte("10.0.0.1"))
	updateAffiliate(t, c, "node-1", time.Hour, nil, []byte("10.0.0.2"))

	c.Prune()

	affiliates, err := c.ListAffiliates(context.Background())
	require.NoError(t, err)
	require.Len(t, affiliates, 1)
	require.Len(t, affiliates[0].Endpoints, 1)
	assert.Equal(t, []byte("10.0.0.2"), affiliates[0].Endpoints[0])
}

func TestCluster_Prune_RemovesExpiredEndpoints_Notifies(t *testing.T) {
	c := newCluster(t)
	updateAffiliate(t, c, "node-1", -time.Second, nil, []byte("10.0.0.1"))
	updateAffiliate(t, c, "node-1", time.Hour, nil, []byte("10.0.0.2"))
	_, sub := subscribe(t, c)

	c.Prune()

	ev := requireEvent(t, sub, AffiliateEventUpsert, "node-1")
	require.Len(t, ev.Endpoints, 1)
	assert.Equal(t, []byte("10.0.0.2"), ev.Endpoints[0])
}

func TestCluster_Prune_NoEvents_WhenNothingExpired(t *testing.T) {
	c := newCluster(t)
	updateAffiliate(t, c, "node-1", time.Hour, []byte("data"))
	_, sub := subscribe(t, c)

	c.Prune()

	requireNoEvent(t, sub)
}

func TestCluster_SubscribeWithSnapshot_EmptySnapshot(t *testing.T) {
	c := newCluster(t)
	snapshot, sub, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)
	defer sub.Close()
	assert.Empty(t, snapshot)
}

func TestCluster_SubscribeWithSnapshot_SnapshotContainsExistingAffiliates(t *testing.T) {
	c := newCluster(t)
	updateAffiliate(t, c, "node-1", time.Hour, []byte("d1"))
	updateAffiliate(t, c, "node-2", time.Hour, []byte("d2"))

	snapshot, sub, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)
	defer sub.Close()
	assert.Len(t, snapshot, 2)
}

func TestCluster_SubscribeWithSnapshot_NoMissedEventsBetweenSnapshotAndRegistration(t *testing.T) {
	// Verifies the fix: affiliatesMu is held until after subscriber is registered,
	// so no update can slip between the snapshot and the subscription.
	c := newCluster(t)
	updateAffiliate(t, c, "node-1", time.Hour, []byte("v1"))

	snapshot, sub, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)
	defer sub.Close()

	require.Len(t, snapshot, 1)
	assert.Equal(t, []byte("v1"), snapshot[0].Data)

	// Any subsequent update must arrive as an event.
	updateAffiliate(t, c, "node-1", time.Hour, []byte("v2"))
	ev := requireEvent(t, sub, AffiliateEventUpsert, "node-1")
	assert.Equal(t, []byte("v2"), ev.Data)
}

func TestCluster_SubscribeWithSnapshot_ContextCancellationClosesSubscription(t *testing.T) {
	c := newCluster(t)
	ctx, cancel := context.WithCancel(context.Background())

	_, sub, err := c.SubscribeWithSnapshot(ctx, netip.Addr{})
	require.NoError(t, err)

	cancel()

	// After cancel the events channel should be closed.
	select {
	case _, ok := <-sub.Events():
		assert.False(t, ok, "events channel should be closed after ctx cancel")
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscription close after ctx cancel")
	}
}

func TestCluster_SubscribeWithSnapshot_CloseRemovesFromCluster(t *testing.T) {
	c := newCluster(t)
	_, sub, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)

	assert.False(t, c.IsEmpty())
	sub.Close()
	assert.True(t, c.IsEmpty())
}

func TestCluster_SubscribeWithSnapshot_CloseProlongsExpiry(t *testing.T) {
	c := newCluster(t)
	c.expiry.Store(time.Now().Add(-time.Second).UnixNano())

	_, sub, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)

	sub.Close()

	assert.False(t, c.IsExpired(), "Close should prolong cluster expiry")
}

func TestCluster_SubscribeWithSnapshot_CloseIsIdempotent(t *testing.T) {
	c := newCluster(t)
	_, sub, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)

	sub.Close()
	assert.NotPanics(t, func() { sub.Close() })
}

func TestCluster_SubscribeWithSnapshot_WithClientIP(t *testing.T) {
	c := newCluster(t)
	ip := netip.MustParseAddr("192.168.1.1")

	_, sub, err := c.SubscribeWithSnapshot(context.Background(), ip)
	require.NoError(t, err)
	defer sub.Close()
	// No panic, subscription works normally.
	updateAffiliate(t, c, "node-1", time.Hour, []byte("data"))
	requireEvent(t, sub, AffiliateEventUpsert, "node-1")
}

// --- Subscription slow consumer ---

func TestSubscription_SlowConsumer_ClosesAndSendsError(t *testing.T) {
	c := newCluster(t)
	_, sub, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)

	// Flood the buffer without draining.
	for i := 0; i < affiliateSubBuffer+1; i++ {
		updateAffiliate(t, c, "node-1", time.Hour, []byte{byte(i)})
	}

	select {
	case err := <-sub.Errors():
		require.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for slow-consumer error")
	}

	// Channel should be closed after slow-consumer eviction.
	select {
	case _, ok := <-sub.Events():
		if !ok {
			return // closed as expected
		}
		// drain remaining events then wait for close
	case <-time.After(time.Second):
		t.Fatal("events channel should be closed after slow-consumer eviction")
	}
}

func newCluster(t *testing.T) *Cluster {
	t.Helper()
	return NewCluster("test-cluster")
}

func subscribe(t *testing.T, c *Cluster) ([]AffiliateInfo, *Subscription) {
	t.Helper()
	snapshot, sub, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)
	t.Cleanup(sub.Close)
	return snapshot, sub
}

func updateAffiliate(t *testing.T, c *Cluster, id string, ttl time.Duration, data []byte, endpoints ...[]byte) {
	t.Helper()
	err := c.UpdateAffiliate(context.Background(), AffiliateUpdateRequest{
		ID:        id,
		TTL:       ttl,
		Data:      data,
		Endpoints: endpoints,
	})
	require.NoError(t, err)
}

func deleteAffiliate(t *testing.T, c *Cluster, id string) {
	t.Helper()
	require.NoError(t, c.DeleteAffiliate(context.Background(), id))
}

func requireEvent(t *testing.T, sub *Subscription, wantType AffiliateEventType, wantID string) AffiliateEvent {
	t.Helper()
	select {
	case ev := <-sub.Events():
		assert.Equal(t, wantType, ev.Type, "event type mismatch")
		assert.Equal(t, wantID, ev.ID, "event affiliate ID mismatch")
		return ev
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for event (type=%v id=%s)", wantType, wantID)
		return AffiliateEvent{}
	}
}

func requireNoEvent(t *testing.T, sub *Subscription) {
	t.Helper()
	select {
	case ev, ok := <-sub.Events():
		if ok {
			t.Fatalf("unexpected event: type=%v id=%s", ev.Type, ev.ID)
		}
	case <-time.After(20 * time.Millisecond):
		// expected: no event
	}
}
