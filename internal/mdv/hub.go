package mdv

import "sync"

// Hub manages a set of SSE client channels and broadcasts reload signals.
type Hub struct {
	mu      sync.Mutex
	clients map[chan struct{}]struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[chan struct{}]struct{}),
	}
}

// Register adds a new client channel to the hub.
func (h *Hub) Register(ch chan struct{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[ch] = struct{}{}
}

// Unregister removes a client channel from the hub.
func (h *Hub) Unregister(ch chan struct{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, ch)
}

// Broadcast sends a non-blocking signal to all registered clients.
func (h *Hub) Broadcast() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
