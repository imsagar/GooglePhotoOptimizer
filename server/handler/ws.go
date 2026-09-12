// Package handler wires HTTP/WebSocket endpoints to the store and relay.
package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/user/gpoptimizer/internal/crypto"
	"github.com/user/gpoptimizer/internal/protocol"
	"github.com/user/gpoptimizer/server/relay"
	"github.com/user/gpoptimizer/server/store"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// HandleUIWebSocket streams relay events for the authenticated user to a UI
// client. Read-only from the UI's perspective: the UI never sends commands
// over this socket (that's plain REST, added in Task 6).
func HandleUIWebSocket(r relay.Relay) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.MustGet("userID").(uuid.UUID)
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		ch := r.Subscribe(userID, relay.ChanUI)
		defer r.Unsubscribe(userID, relay.ChanUI)

		for msg := range ch {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}
}

// HandleRunnerWebSocket authenticates a local runner via its first message
// (a bearer token, bcrypt-matched against the runners table) and then
// relays encrypted commands/status between the runner and the relay. Every
// message after auth is AES-256-GCM sealed under a key derived from the
// runner's token via HKDF.
func HandleRunnerWebSocket(db *store.DB, r relay.Relay) gin.HandlerFunc {
	return func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// First message is {"type":"auth","token":"..."}, sent unencrypted
		// since the runner doesn't have the derived key yet.
		_, authMsg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var authPayload struct {
			Type  string `json:"type"`
			Token string `json:"token"`
		}
		if err := json.Unmarshal(authMsg, &authPayload); err != nil {
			return
		}

		runner, err := db.AuthenticateRunner(c.Request.Context(), authPayload.Token)
		if err != nil {
			conn.WriteJSON(map[string]string{"error": "invalid token"})
			return
		}

		aesKey := crypto.DeriveKey([]byte(authPayload.Token))
		userID := runner.UserID

		db.UpdateRunnerStatus(c.Request.Context(), runner.ID, "online", time.Now())
		defer db.UpdateRunnerStatus(context.Background(), runner.ID, "offline", time.Now())

		onlineMsg, _ := json.Marshal(map[string]interface{}{"type": "connected", "platform": runner.Platform})
		r.SendToUI(userID, onlineMsg)
		defer func() {
			offMsg, _ := json.Marshal(map[string]string{"type": "runner_offline"})
			r.SendToUI(userID, offMsg)
		}()

		cmdCh := r.Subscribe(userID, relay.ChanRunner)
		defer r.Unsubscribe(userID, relay.ChanRunner)

		// Reader runs in its own goroutine so we can also write commands to
		// the runner as they arrive on cmdCh; `done` lets the write loop
		// below notice the connection died and stop blocking on cmdCh.
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				_, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				decrypted, err := crypto.Decrypt(aesKey, string(msg))
				if err != nil {
					log.Printf("runner ws: decrypt error: %v", err)
					continue
				}
				var status protocol.Status
				if err := json.Unmarshal(decrypted, &status); err != nil {
					log.Printf("runner ws: unmarshal status error: %v", err)
					continue
				}
				handleRunnerStatus(c.Request.Context(), db, r, userID, runner.ID, status)
			}
		}()

		for {
			select {
			case cmd, ok := <-cmdCh:
				if !ok {
					return
				}
				encrypted, err := crypto.Encrypt(aesKey, cmd)
				if err != nil {
					log.Printf("runner ws: encrypt error: %v", err)
					continue
				}
				if err := conn.WriteMessage(websocket.TextMessage, []byte(encrypted)); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}
}

// handleRunnerStatus updates the DB based on a status message from the
// runner, then forwards the (decrypted, plaintext) status to the user's UI
// connections as JSON.
func handleRunnerStatus(ctx context.Context, db *store.DB, r relay.Relay, userID, runnerID uuid.UUID, s protocol.Status) {
	switch s.Type {
	case "progress":
		if err := db.UpdateJobStatus(ctx, s.JobID, userID, s.Stage, s.Percent); err != nil {
			log.Printf("runner ws: update job status: %v", err)
		}
	case "job_complete":
		var savingsPct float32
		if s.OriginalSize > 0 {
			savingsPct = (1 - float32(s.OptimizedSize)/float32(s.OriginalSize)) * 100
		}
		if err := db.UpdateJobResult(ctx, s.JobID, userID, s.OptimizedSize, savingsPct); err != nil {
			log.Printf("runner ws: update job result: %v", err)
		}
		if err := db.UpdateJobStatus(ctx, s.JobID, userID, "ready", 100); err != nil {
			log.Printf("runner ws: update job status: %v", err)
		}
	case "upload_complete":
		if err := db.UpdateJobStatus(ctx, s.JobID, userID, "uploaded", 100); err != nil {
			log.Printf("runner ws: update job status: %v", err)
		}
	case "error":
		if err := db.UpdateJobStatus(ctx, s.JobID, userID, "failed", 0); err != nil {
			log.Printf("runner ws: update job status: %v", err)
		}
	case "videos_synced":
		// Video upsert (parsing s.Videos into store.Video rows) lands in
		// Task 6 alongside the REST handlers that need the same shape.
	case "google_auth_status":
		if err := db.SetGoogleConnected(ctx, runnerID, s.Connected); err != nil {
			log.Printf("runner ws: set google connected: %v", err)
		}
	}

	msg, err := json.Marshal(s)
	if err != nil {
		log.Printf("runner ws: marshal status: %v", err)
		return
	}
	r.SendToUI(userID, msg)
}
