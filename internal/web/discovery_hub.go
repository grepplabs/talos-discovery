package web

import "sync"

const subscriberChannelSize = 64

type Event struct {
	Data string
}

type Hub struct {
	mu      sync.Mutex
	clients map[chan Event]struct{}
	metrics *watchManagerMetrics
}

func NewHub(metrics *watchManagerMetrics) *Hub {
	return &Hub{
		clients: make(map[chan Event]struct{}),
		metrics: metrics,
	}
}

func (h *Hub) Subscribe() chan Event {
	ch := make(chan Event, subscriberChannelSize)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) Unsubscribe(ch chan Event) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
}

func (h *Hub) Broadcast(e Event) {
	h.mu.Lock()
	subs := make([]chan Event, 0, len(h.clients))
	for ch := range h.clients {
		subs = append(subs, ch)
	}
	h.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- e:
		default:
			h.metrics.broadcastDroppedEvents.Inc()
		}
	}
}
