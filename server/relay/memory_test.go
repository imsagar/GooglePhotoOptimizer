package relay

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMemoryRelay(t *testing.T) {
	m := NewMemory()
	userID := uuid.New()

	ui := m.Subscribe(userID, ChanUI)
	runner := m.Subscribe(userID, ChanRunner)

	m.SendToUI(userID, []byte("hello-ui"))
	m.SendToRunner(userID, []byte("hello-runner"))

	select {
	case msg := <-ui:
		if string(msg) != "hello-ui" {
			t.Errorf("ui channel got %q, want %q", msg, "hello-ui")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ui message")
	}

	select {
	case msg := <-runner:
		if string(msg) != "hello-runner" {
			t.Errorf("runner channel got %q, want %q", msg, "hello-runner")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for runner message")
	}

	m.Unsubscribe(userID, ChanUI, ui)
	if _, open := <-ui; open {
		t.Error("ui channel should be closed after Unsubscribe")
	}

	// SendToUI after Unsubscribe must not panic (no subscriber).
	m.SendToUI(userID, []byte("dropped"))

	// A different user's channels are unaffected.
	other := m.Subscribe(uuid.New(), ChanUI)
	select {
	case <-other:
		t.Fatal("unexpected message on unrelated user's channel")
	default:
	}
}
