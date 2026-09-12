// Package ws is the runner's WebSocket client: it connects to the server,
// authenticates with the runner's token, and exchanges AES-256-GCM encrypted
// commands/status with it (see internal/protocol and internal/crypto).
package ws

import (
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/user/gpoptimizer/internal/crypto"
	"github.com/user/gpoptimizer/internal/protocol"
	"github.com/user/gpoptimizer/runner/config"
)

// Client holds one runner's connection to the server. The zero value is not
// usable; construct with NewClient.
type Client struct {
	cfg    *config.Config
	aesKey []byte

	mu   sync.Mutex // guards conn writes (gorilla allows only one writer at a time)
	conn *websocket.Conn

	cmdFn func(protocol.Command)
}

// NewClient builds a Client for cfg. Call SetCommandHandler before Run's
// connectLoop starts dispatching, or pass the handler to Run directly.
func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg:    cfg,
		aesKey: crypto.DeriveKey([]byte(cfg.Token)),
	}
}

// SetCommandHandler wires the function invoked for each command the server
// sends. Task 10's transcode pipeline hangs off this.
func (c *Client) SetCommandHandler(fn func(protocol.Command)) {
	c.cmdFn = fn
}

// Run connects to the server and reconnects forever with exponential
// backoff. cmdFn (may be nil) handles incoming commands.
func Run(cfg *config.Config, cmdFn func(protocol.Command)) {
	c := NewClient(cfg)
	c.SetCommandHandler(cmdFn)
	c.connectLoop()
}

func (c *Client) connectLoop() {
	attempt := 0
	for {
		err := c.connect()
		log.Printf("runner ws: connection lost: %v", err)

		delay := time.Duration(1<<attempt) * time.Second // 1s, 2s, 4s, ... capped at 60s
		if delay > 60*time.Second {
			delay = 60 * time.Second
		}
		time.Sleep(delay)
		if attempt < 6 { // 1<<6 == 64s already clamps to the cap above
			attempt++
		}
	}
}

func (c *Client) connect() error {
	wsURL := c.cfg.ServerURL + "/ws/runner"
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	defer conn.Close()

	// Auth is the one unencrypted message: the runner has no key until the
	// server confirms this token (see HandleRunnerWebSocket).
	authMsg, _ := json.Marshal(map[string]string{
		"type":  "auth",
		"token": c.cfg.Token,
	})
	if err := conn.WriteMessage(websocket.TextMessage, authMsg); err != nil {
		return err
	}

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		decrypted, err := crypto.Decrypt(c.aesKey, string(msg))
		if err != nil {
			log.Printf("runner ws: decrypt error: %v", err)
			continue
		}
		var cmd protocol.Command
		if err := json.Unmarshal(decrypted, &cmd); err != nil {
			log.Printf("runner ws: unmarshal command error: %v", err)
			continue
		}
		go c.handleCommand(cmd)
	}
}

// SendStatus encrypts and sends a status update to the server.
func (c *Client) SendStatus(s protocol.Status) {
	data, err := json.Marshal(s)
	if err != nil {
		log.Printf("runner ws: marshal status: %v", err)
		return
	}
	encrypted, err := crypto.Encrypt(c.aesKey, data)
	if err != nil {
		log.Printf("runner ws: encrypt status: %v", err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return
	}
	if err := c.conn.WriteMessage(websocket.TextMessage, []byte(encrypted)); err != nil {
		log.Printf("runner ws: send status: %v", err)
	}
}

func (c *Client) handleCommand(cmd protocol.Command) {
	if c.cmdFn == nil {
		log.Printf("runner ws: received command %q but no handler is registered", cmd.Type)
		return
	}
	c.cmdFn(cmd)
}
