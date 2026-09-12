package relay

import (
	"sync"

	"github.com/google/uuid"
)

// Memory is an in-memory Relay implementation, suitable for a single server
// instance. Swap for a Redis-backed Relay if the server scales horizontally.
type Memory struct {
	mu   sync.RWMutex
	subs map[string]chan []byte // key: "userID:channel"
}

func NewMemory() *Memory {
	return &Memory{subs: make(map[string]chan []byte)}
}

func key(userID uuid.UUID, ch string) string {
	return userID.String() + ":" + ch
}

func (m *Memory) Subscribe(userID uuid.UUID, channel string) <-chan []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan []byte, 64)
	m.subs[key(userID, channel)] = ch
	return ch
}

func (m *Memory) Unsubscribe(userID uuid.UUID, channel string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := key(userID, channel)
	if ch, ok := m.subs[k]; ok {
		close(ch)
		delete(m.subs, k)
	}
}

func (m *Memory) send(userID uuid.UUID, channel string, msg []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ch, ok := m.subs[key(userID, channel)]
	if ok {
		select {
		case ch <- msg:
		default: // drop if buffer full
		}
	}
}

func (m *Memory) SendToRunner(userID uuid.UUID, msg []byte) {
	m.send(userID, ChanRunner, msg)
}

func (m *Memory) SendToUI(userID uuid.UUID, msg []byte) {
	m.send(userID, ChanUI, msg)
}
