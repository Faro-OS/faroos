// Package relaytransport adapts the FaroOS relay WebSocket into the ordered
// byte stream expected by the yamux multiplexer.
package relaytransport

import (
	"io"
	"sync"

	"github.com/gorilla/websocket"
)

type WebSocketConn struct {
	conn *websocket.Conn

	readMu  sync.Mutex
	reader  io.Reader
	writeMu sync.Mutex
}

func NewWebSocketConn(conn *websocket.Conn) *WebSocketConn {
	return &WebSocketConn{conn: conn}
}

func (c *WebSocketConn) Read(p []byte) (int, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	for {
		if c.reader != nil {
			n, err := c.reader.Read(p)
			if err == io.EOF {
				c.reader = nil
				if n > 0 {
					return n, nil
				}
				continue
			}
			return n, err
		}
		messageType, reader, err := c.conn.NextReader()
		if err != nil {
			return 0, err
		}
		if messageType != websocket.BinaryMessage {
			continue
		}
		c.reader = reader
	}
}

func (c *WebSocketConn) Write(p []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.conn.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *WebSocketConn) Close() error {
	return c.conn.Close()
}
