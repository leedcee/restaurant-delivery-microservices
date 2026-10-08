package httpapi

import (
	"sync"

	"github.com/google/uuid"
)

// partnerOrderHub fans out order changes to connected restaurant dashboards.
// Notifications are edge-triggered: subscribers reload the authoritative
// order snapshot from PostgreSQL before an SSE event is sent.
type partnerOrderHub struct {
	mu          sync.Mutex
	subscribers map[uuid.UUID]map[chan struct{}]struct{}
}

func newPartnerOrderHub() *partnerOrderHub {
	return &partnerOrderHub{subscribers: make(map[uuid.UUID]map[chan struct{}]struct{})}
}

func (h *partnerOrderHub) subscribe(restaurantID uuid.UUID) (<-chan struct{}, func()) {
	updates := make(chan struct{}, 1)
	h.mu.Lock()
	if h.subscribers[restaurantID] == nil {
		h.subscribers[restaurantID] = make(map[chan struct{}]struct{})
	}
	h.subscribers[restaurantID][updates] = struct{}{}
	h.mu.Unlock()

	return updates, func() {
		h.mu.Lock()
		delete(h.subscribers[restaurantID], updates)
		if len(h.subscribers[restaurantID]) == 0 {
			delete(h.subscribers, restaurantID)
		}
		h.mu.Unlock()
	}
}

func (h *partnerOrderHub) publish(restaurantID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for updates := range h.subscribers[restaurantID] {
		select {
		case updates <- struct{}{}:
		default:
		}
	}
}
