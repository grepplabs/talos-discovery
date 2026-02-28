package web

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestHubBroadcastDeliversEvent(t *testing.T) {
	m, err := newWatchManagerMetrics(prometheus.NewRegistry())
	require.NoError(t, err)

	hub := NewHub(m)
	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	hub.Broadcast(Event{Data: "hello"})

	select {
	case evt := <-ch:
		require.Equal(t, "hello", evt.Data)
	default:
		t.Fatal("expected broadcast event")
	}

	require.Equal(t, float64(0), testutil.ToFloat64(m.broadcastDroppedEvents))
}

func TestHubBroadcastDroppedEventsMetric(t *testing.T) {
	m, err := newWatchManagerMetrics(prometheus.NewRegistry())
	require.NoError(t, err)

	hub := NewHub(m)
	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	// Fill subscriber buffer to force drops.
	for i := 0; i < subscriberChannelSize; i++ {
		hub.Broadcast(Event{Data: "x"})
	}
	hub.Broadcast(Event{Data: "dropped"})

	require.Equal(t, float64(1), testutil.ToFloat64(m.broadcastDroppedEvents))
}
