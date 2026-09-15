package relay

import (
	"sync"

	"github.com/google/uuid"
)

type Memory struct {
	mu   sync.RWMutex
	subs map[string][]chan []byte // key: "userID:channel"
}

func NewMemory() *Memory {
	return &Memory{subs: make(map[string][]chan []byte)}
}

func key(userID uuid.UUID, ch string) string {
	return userID.String() + ":" + ch
}

func (m *Memory) Subscribe(userID uuid.UUID, channel string) <-chan []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := key(userID, channel)
	ch := make(chan []byte, 64)
	m.subs[k] = append(m.subs[k], ch)
	return ch
}

func (m *Memory) Unsubscribe(userID uuid.UUID, channel string, ch <-chan []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := key(userID, channel)
	list := m.subs[k]
	for i, c := range list {
		if c == ch {
			close(c)
			m.subs[k] = append(list[:i], list[i+1:]...)
			if len(m.subs[k]) == 0 {
				delete(m.subs, k)
			}
			return
		}
	}
}

func (m *Memory) send(userID uuid.UUID, channel string, msg []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	k := key(userID, channel)
	for _, ch := range m.subs[k] {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (m *Memory) SendToRunner(userID uuid.UUID, msg []byte) {
	m.send(userID, ChanRunner, msg)
}

func (m *Memory) SendToUI(userID uuid.UUID, msg []byte) {
	m.send(userID, ChanUI, msg)
}
