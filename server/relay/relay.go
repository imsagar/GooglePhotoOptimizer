// Package relay fans out messages between a user's UI WebSocket connection(s)
// and their runner WebSocket connection. The in-memory implementation
// (Memory) is meant to be swappable for a Redis-backed one later without
// changing callers.
package relay

import "github.com/google/uuid"

const (
	ChanUI     = "ui"
	ChanRunner = "runner"
)

// Relay routes messages between a user's UI and runner connections.
type Relay interface {
	SendToRunner(userID uuid.UUID, msg []byte)
	SendToUI(userID uuid.UUID, msg []byte)
	Subscribe(userID uuid.UUID, channel string) <-chan []byte
	Unsubscribe(userID uuid.UUID, channel string)
}
