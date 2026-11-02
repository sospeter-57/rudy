package events

import "sync"

// Event is the SSE payload. Data is already shaped for JSON.
type Event struct {
	Type   string `json:"type"`
	RoomID int64  `json:"roomId,omitempty"`
	Data   any    `json:"data,omitempty"`
}

// Hub fans messages out to every live connection for a user. It is in-process,
// so the deployment runs one API instance for now.
type Hub struct {
	mu          sync.RWMutex
	subscribers map[int64]map[chan Event]struct{}
}

func NewHub() *Hub {
	return &Hub{subscribers: map[int64]map[chan Event]struct{}{}}
}

// Subscribe returns a channel plus an unsubscribe function. Slow consumers drop
// events instead of blocking publishers; clients refetch after reconnecting.
func (h *Hub) Subscribe(userID int64) (<-chan Event, func()) {
	channel := make(chan Event, 16)

	h.mu.Lock()
	if h.subscribers[userID] == nil {
		h.subscribers[userID] = map[chan Event]struct{}{}
	}
	h.subscribers[userID][channel] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.mu.Lock()
			if channels, ok := h.subscribers[userID]; ok {
				delete(channels, channel)
				if len(channels) == 0 {
					delete(h.subscribers, userID)
				}
			}
			h.mu.Unlock()
			close(channel)
		})
	}
	return channel, cancel
}

func (h *Hub) Publish(userIDs []int64, event Event) {
	if len(userIDs) == 0 {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, userID := range userIDs {
		for channel := range h.subscribers[userID] {
			select {
			case channel <- event:
			default:
			}
		}
	}
}
