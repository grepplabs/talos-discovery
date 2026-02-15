package web

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"sync"
	"time"

	"github.com/grepplabs/loggo/zlog"
	"github.com/grepplabs/talos-discovery/api/v1alpha1/server/pb"
	"github.com/prometheus/client_golang/prometheus"
)

type watchEntry struct {
	hub    *Hub
	cancel context.CancelFunc
	refs   int
}

type DiscoveryWatchManager struct {
	mu      sync.Mutex
	entries map[string]*watchEntry
	client  *DiscoveryClient
	//nolint:containedctx // root context is intentionally stored for manager lifetime control
	rootCtx context.Context
	metrics *watchManagerMetrics
}

func NewDiscoveryWatchManager(ctx context.Context, client *DiscoveryClient, registerer prometheus.Registerer) (*DiscoveryWatchManager, error) {
	metrics, err := newWatchManagerMetrics(registerer)
	if err != nil {
		return nil, err
	}

	return &DiscoveryWatchManager{
		entries: make(map[string]*watchEntry),
		client:  client,
		rootCtx: ctx,
		metrics: metrics,
	}, nil
}

func (m *DiscoveryWatchManager) Subscribe(clusterID string) chan Event {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.entries[clusterID]
	if !ok {
		ctx, cancel := context.WithCancel(m.rootCtx)
		hub := NewHub(m.metrics)
		entry = &watchEntry{hub: hub, cancel: cancel}
		m.entries[clusterID] = entry
		go watchDiscovery(ctx, m.client.client, clusterID, hub, m.metrics)
		m.metrics.activeDiscoveryWatchers.Inc()
		zlog.Infof("[manager] started watcher for cluster %q", clusterID)
	}
	entry.refs++
	m.metrics.activeSubscribers.Inc()
	return entry.hub.Subscribe()
}

func (m *DiscoveryWatchManager) Unsubscribe(clusterID string, ch chan Event) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.entries[clusterID]
	if !ok {
		return
	}
	entry.hub.Unsubscribe(ch)
	entry.refs--
	m.metrics.activeSubscribers.Dec()
	if entry.refs <= 0 {
		entry.cancel()
		delete(m.entries, clusterID)
		m.metrics.activeDiscoveryWatchers.Dec()
		zlog.Infof("[manager] stopped watcher for cluster %q", clusterID)
	}
}

func waitWithContext(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func nextBackoff(attempt int) time.Duration {
	const (
		base     = 500 * time.Millisecond
		maxDelay = 15 * time.Second
	)
	if attempt < 0 {
		attempt = 0
	}
	d := base * time.Duration(1<<attempt)
	if d > maxDelay || d <= 0 {
		d = maxDelay
	}
	// #nosec G404 -- jitter does not require cryptographic randomness
	jitter := time.Duration(rand.Int63n(int64(d / 2)))
	return d + jitter
}

//nolint:cyclop
func watchDiscovery(ctx context.Context, client pb.ClusterClient, clusterID string, hub *Hub, metrics *watchManagerMetrics) {
	retryAttempt := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		stream, err := client.Watch(ctx, &pb.WatchRequest{ClusterId: clusterID})
		if err != nil {
			metrics.watchStreamErrors.Inc()
			zlog.Infof("watch error for %q: %v", clusterID, err)
			metrics.watchReconnects.Inc()
			if !waitWithContext(ctx, nextBackoff(retryAttempt)) {
				return
			}
			retryAttempt++
			continue
		}
		retryAttempt = 0

		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				metrics.watchStreamErrors.Inc()
				zlog.Infof("stream recv error for %q: %v", clusterID, err)
				break
			}
			metrics.watchEvents.Inc()
			data, err := affiliatesToJSON(resp.GetAffiliates(), resp.GetDeleted())
			if err != nil {
				zlog.Infof("failed to marshal affiliates for %q: %v", clusterID, err)
				break
			}
			hub.Broadcast(Event{Data: data})
		}

		metrics.watchReconnects.Inc()
		if !waitWithContext(ctx, nextBackoff(retryAttempt)) {
			return
		}
		retryAttempt++
	}
}
