package web

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

type watchManagerMetrics struct {
	activeDiscoveryWatchers prometheus.Gauge
	activeSubscribers       prometheus.Gauge
	broadcastDroppedEvents  prometheus.Counter
	watchEvents             prometheus.Counter
	watchStreamErrors       prometheus.Counter
	watchReconnects         prometheus.Counter
}

func newWatchManagerMetrics(registerer prometheus.Registerer) (*watchManagerMetrics, error) {
	m := &watchManagerMetrics{
		activeDiscoveryWatchers: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "discovery_web_active_discovery_watchers",
			Help: "Number of active discovery watcher goroutines.",
		}),
		activeSubscribers: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "discovery_web_active_subscribers",
			Help: "Number of active SSE subscribers across all clusters.",
		}),
		broadcastDroppedEvents: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "discovery_web_broadcast_dropped_events_total",
			Help: "Number of events dropped because subscriber channels were full.",
		}),
		watchEvents: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "discovery_web_watch_events_total",
			Help: "Number of watch events received from gRPC stream.",
		}),
		watchStreamErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "discovery_web_watch_stream_errors_total",
			Help: "Number of gRPC watch stream receive/open errors.",
		}),
		watchReconnects: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "discovery_web_watch_reconnects_total",
			Help: "Number of watch reconnect attempts after stream interruption.",
		}),
	}

	collectors := []prometheus.Collector{
		m.activeDiscoveryWatchers,
		m.activeSubscribers,
		m.broadcastDroppedEvents,
		m.watchEvents,
		m.watchStreamErrors,
		m.watchReconnects,
	}

	for _, c := range collectors {
		if err := registerer.Register(c); err != nil {
			return nil, fmt.Errorf("register web discovery metric: %w", err)
		}
	}

	return m, nil
}
