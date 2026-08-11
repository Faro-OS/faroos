package api

import (
	"io"
	"sync/atomic"
	"testing"
	"time"
)

type fakeAgentConnection struct{ closed atomic.Bool }

func (*fakeAgentConnection) ReadJSON(any) error              { return io.EOF }
func (*fakeAgentConnection) WriteJSON(any) error             { return nil }
func (*fakeAgentConnection) SetReadDeadline(time.Time) error { return nil }
func (c *fakeAgentConnection) Close() error                  { c.closed.Store(true); return nil }

func TestHubReplacementKeepsNewestConnectionRegistered(t *testing.T) {
	hub := newHub()
	firstConn := &fakeAgentConnection{}
	first := hub.register("node", firstConn, "relay")
	secondConn := &fakeAgentConnection{}
	second := hub.register("node", secondConn, "direct-p2p")

	if !firstConn.closed.Load() {
		t.Fatal("replaced connection was not closed")
	}
	if hub.unregister("node", first) {
		t.Fatal("unregistering the replaced connection removed the newest connection")
	}
	current, ok := hub.get("node")
	if !ok || current != second || current.mode != "direct-p2p" {
		t.Fatalf("newest connection was not retained: %+v, %v", current, ok)
	}
	if !hub.unregister("node", second) {
		t.Fatal("unregistering the current connection did not remove it")
	}
}
