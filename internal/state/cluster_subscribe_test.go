package state

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscription_Close_ClosesEventChannel(t *testing.T) {
	c := newCluster(t)
	_, sub, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)

	sub.Close()

	select {
	case _, ok := <-sub.Events():
		assert.False(t, ok, "events channel must be closed after Close()")
	case <-time.After(time.Second):
		t.Fatal("timed out: events channel not closed")
	}
}

func TestSubscription_Close_ClosesErrorChannel(t *testing.T) {
	c := newCluster(t)
	_, sub, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)

	sub.Close()

	select {
	case _, ok := <-sub.Errors():
		assert.False(t, ok, "errors channel must be closed after Close()")
	case <-time.After(time.Second):
		t.Fatal("timed out: errors channel not closed")
	}
}

func TestSubscription_Close_IsIdempotent(t *testing.T) {
	c := newCluster(t)
	_, sub, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)

	assert.NotPanics(t, func() {
		sub.Close()
		sub.Close()
		sub.Close()
	}, "Close must be safe to call multiple times")
}

func TestSubscription_Close_RemovesSubscriberFromCluster(t *testing.T) {
	c := newCluster(t)
	_, sub, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)

	c.subsMu.Lock()
	countBefore := len(c.subscribers)
	c.subsMu.Unlock()
	require.Equal(t, 1, countBefore)

	sub.Close()

	c.subsMu.Lock()
	countAfter := len(c.subscribers)
	c.subsMu.Unlock()
	assert.Equal(t, 0, countAfter, "closed subscription must be removed from cluster")
}

func TestSubscription_Close_SecondCloseDoesNotDoubleFree(t *testing.T) {
	c := newCluster(t)
	_, sub, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)

	sub.Close()

	// A second subscriber registers itself after the first closes.
	_, sub2, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)
	defer sub2.Close()

	// Closing the first subscription again must not affect the cluster or panic.
	assert.NotPanics(t, func() { sub.Close() })
}

func TestSubscription_notify_DeliversSingleEvent(t *testing.T) {
	c := newCluster(t)
	_, sub, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)
	defer sub.Close()

	ev := AffiliateEvent{
		Type:          AffiliateEventUpsert,
		AffiliateInfo: AffiliateInfo{ID: "node-1", Data: []byte("data")},
	}
	sub.notify(ev)

	select {
	case got := <-sub.Events():
		assert.Equal(t, ev, got)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestSubscription_notify_BufferFull_SendsErrorAndCloses(t *testing.T) {
	c := newCluster(t)
	_, sub, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)

	ev := AffiliateEvent{Type: AffiliateEventUpsert, AffiliateInfo: AffiliateInfo{ID: "x"}}

	// Fill the buffer exactly.
	for i := 0; i < affiliateSubBuffer; i++ {
		sub.notify(ev)
	}

	// One more must trigger slow-consumer eviction.
	sub.notify(ev)

	select {
	case err := <-sub.Errors():
		require.EqualError(t, err, "lost update")
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for error after buffer overflow")
	}

	// Channel must be closed.
	select {
	case _, ok := <-sub.Events():
		if !ok {
			return
		}
		// drain any buffered events, then expect close
		for range sub.Events() {
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for events channel to close after eviction")
	}
}

func TestSubscription_notify_AfterClose_DoesNotPanic(t *testing.T) {
	c := newCluster(t)
	_, sub, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)

	sub.Close()

	ev := AffiliateEvent{Type: AffiliateEventUpsert, AffiliateInfo: AffiliateInfo{ID: "node-1"}}
	assert.NotPanics(t, func() { sub.notify(ev) })
}

func TestCluster_notify_FansOutToAllSubscribers(t *testing.T) {
	c := newCluster(t)

	_, sub1, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)
	defer sub1.Close()

	_, sub2, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)
	defer sub2.Close()

	_, sub3, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)
	defer sub3.Close()

	ev := AffiliateEvent{Type: AffiliateEventUpsert, AffiliateInfo: AffiliateInfo{ID: "node-1"}}
	c.notify(ev)

	for i, sub := range []*Subscription{sub1, sub2, sub3} {
		select {
		case got := <-sub.Events():
			assert.Equal(t, ev, got, "subscriber %d got wrong event", i+1)
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timed out", i+1)
		}
	}
}

func TestCluster_notify_SlowSubscriberDoesNotBlockOthers(t *testing.T) {
	c := newCluster(t)

	_, slow, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)
	defer slow.Close()

	_, fast, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)
	defer fast.Close()

	ev := AffiliateEvent{Type: AffiliateEventUpsert, AffiliateInfo: AffiliateInfo{ID: "x"}}

	for i := 0; i < affiliateSubBuffer+1; i++ {
		slow.notify(ev)
	}
	<-slow.Errors()

	// fast should still receive normally.
	c.notify(ev)
	select {
	case got := <-fast.Events():
		assert.Equal(t, ev, got)
	case <-time.After(time.Second):
		t.Fatal("fast subscriber should still receive events after slow subscriber was evicted")
	}
}

func TestSubscription_ContextCancel_ClosesSubscription(t *testing.T) {
	c := newCluster(t)
	ctx, cancel := context.WithCancel(context.Background())

	_, sub, err := c.SubscribeWithSnapshot(ctx, netip.Addr{})
	require.NoError(t, err)

	cancel()

	select {
	case _, ok := <-sub.Events():
		assert.False(t, ok, "events channel must close when context is cancelled")
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscription close on ctx cancel")
	}
}

func TestSubscription_ContextCancel_RemovesFromCluster(t *testing.T) {
	c := newCluster(t)
	ctx, cancel := context.WithCancel(context.Background())

	_, _, err := c.SubscribeWithSnapshot(ctx, netip.Addr{})
	require.NoError(t, err)

	cancel()
	time.Sleep(20 * time.Millisecond) // let the goroutine run

	c.subsMu.Lock()
	count := len(c.subscribers)
	c.subsMu.Unlock()
	assert.Equal(t, 0, count, "cancelled subscription must be removed from cluster")
}

func TestSubscription_ContextAlreadyCancelled_ClosesImmediately(t *testing.T) {
	c := newCluster(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before subscribing

	_, sub, err := c.SubscribeWithSnapshot(ctx, netip.Addr{})
	require.NoError(t, err)

	select {
	case _, ok := <-sub.Events():
		assert.False(t, ok)
	case <-time.After(time.Second):
		t.Fatal("timed out: already-cancelled context should close subscription quickly")
	}
}

func TestSubscription_IDs_AreUnique(t *testing.T) {
	c := newCluster(t)

	ids := make(map[uint64]bool)
	for i := 0; i < 10; i++ {
		_, sub, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
		require.NoError(t, err)
		assert.False(t, ids[sub.id], "subscription ID %d seen twice", sub.id)
		ids[sub.id] = true
		sub.Close()
	}
}

func TestSubscription_EventsAndErrors_ReturnCorrectChannels(t *testing.T) {
	c := newCluster(t)
	_, sub, err := c.SubscribeWithSnapshot(context.Background(), netip.Addr{})
	require.NoError(t, err)
	defer sub.Close()

	assert.NotNil(t, sub.Events())
	assert.NotNil(t, sub.Errors())
	assert.Equal(t, affiliateSubBuffer, cap(sub.ch))
	assert.Equal(t, 1, cap(sub.errCh))
}
