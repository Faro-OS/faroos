// Command agent runs on each managed server. It connects outbound to the
// central panel over a websocket (so it works behind NAT with no inbound
// ports needed) and reports system stats periodically.
package main

import (
	"log"
	"os"
	"time"

	"github.com/faroos/faroos/internal/proto"
	"github.com/faroos/faroos/internal/sysstats"
	"github.com/gorilla/websocket"
)

const version = "0.0.1-dev"

func main() {
	serverURL := requireEnv("FAROOS_SERVER") // e.g. ws://panel.example.com/api/agent/connect
	nodeID := requireEnv("FAROOS_NODE_ID")
	token := requireEnv("FAROOS_TOKEN")

	for {
		if err := runOnce(serverURL, nodeID, token); err != nil {
			log.Printf("connection lost: %v — reconnecting in 5s", err)
		}
		time.Sleep(5 * time.Second)
	}
}

func runOnce(serverURL, nodeID, token string) error {
	conn, _, err := websocket.DefaultDialer.Dial(serverURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.WriteJSON(proto.Envelope{
		Type: proto.TypeHello,
		Hello: &proto.Hello{
			NodeID:  nodeID,
			Token:   token,
			Version: version,
		},
	}); err != nil {
		return err
	}

	log.Printf("connected to %s as node %s", serverURL, nodeID)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Reader goroutine keeps the connection alive and surfaces errors.
	errCh := make(chan error, 1)
	go func() {
		for {
			var env proto.Envelope
			if err := conn.ReadJSON(&env); err != nil {
				errCh <- err
				return
			}
		}
	}()

	for {
		select {
		case err := <-errCh:
			return err
		case <-ticker.C:
			stats := sysstats.Collect()
			if err := conn.WriteJSON(proto.Envelope{Type: proto.TypeStats, Stats: &stats}); err != nil {
				return err
			}
		}
	}
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required environment variable %s", key)
	}
	return v
}
