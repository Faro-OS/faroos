// Command agent runs on each managed server. It connects outbound to the
// central panel over a websocket (so it works behind NAT with no inbound
// ports needed), reports system stats periodically, and executes commands
// the panel sends it (e.g. Docker container management).
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/faroos/faroos/internal/dockerclient"
	"github.com/faroos/faroos/internal/proto"
	"github.com/faroos/faroos/internal/sysstats"
)

const version = "0.0.1-dev"

func main() {
	serverURL := requireEnv("FAROOS_SERVER") // e.g. ws://panel.example.com/api/agent/connect
	nodeID := requireEnv("FAROOS_NODE_ID")
	token := requireEnv("FAROOS_TOKEN")

	dockerSocket := os.Getenv("FAROOS_DOCKER_SOCKET")
	if dockerSocket == "" {
		dockerSocket = "/var/run/docker.sock"
	}
	docker := dockerclient.New(dockerSocket)

	for {
		if err := runOnce(serverURL, nodeID, token, docker); err != nil {
			log.Printf("connection lost: %v — reconnecting in 5s", err)
		}
		time.Sleep(5 * time.Second)
	}
}

func runOnce(serverURL, nodeID, token string, docker *dockerclient.Client) error {
	conn, _, err := websocket.DefaultDialer.Dial(serverURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	var writeMu sync.Mutex
	writeJSON := func(v any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteJSON(v)
	}

	if err := writeJSON(proto.Envelope{
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

	errCh := make(chan error, 1)
	go func() {
		for {
			var env proto.Envelope
			if err := conn.ReadJSON(&env); err != nil {
				errCh <- err
				return
			}
			if env.Type == proto.TypeCommand && env.Command != nil {
				// Run in its own goroutine so a slow command (e.g. fetching
				// logs) doesn't stall reading further messages/pings.
				go handleCommand(docker, *env.Command, writeJSON)
			}
		}
	}()

	for {
		select {
		case err := <-errCh:
			return err
		case <-ticker.C:
			stats := sysstats.Collect()
			if err := writeJSON(proto.Envelope{Type: proto.TypeStats, Stats: &stats}); err != nil {
				return err
			}
		}
	}
}

func handleCommand(docker *dockerclient.Client, cmd proto.Command, writeJSON func(any) error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	result, err := dispatch(ctx, docker, cmd)

	reply := proto.Envelope{
		Type:          proto.TypeCommandResult,
		CommandResult: &proto.CommandResult{ID: cmd.ID, OK: err == nil},
	}
	if err != nil {
		reply.CommandResult.Error = err.Error()
	} else if result != nil {
		raw, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			reply.CommandResult.OK = false
			reply.CommandResult.Error = marshalErr.Error()
		} else {
			reply.CommandResult.Result = raw
		}
	}

	if err := writeJSON(reply); err != nil {
		log.Printf("failed to send result for command %s: %v", cmd.ID, err)
	}
}

type containerIDParams struct {
	ID string `json:"id"`
}

type logsParams struct {
	ID   string `json:"id"`
	Tail int    `json:"tail"`
}

func dispatch(ctx context.Context, docker *dockerclient.Client, cmd proto.Command) (any, error) {
	switch cmd.Action {
	case "containers.list":
		return docker.ListContainers(ctx)

	case "containers.start", "containers.stop", "containers.restart":
		var p containerIDParams
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return nil, err
		}
		var err error
		switch cmd.Action {
		case "containers.start":
			err = docker.StartContainer(ctx, p.ID)
		case "containers.stop":
			err = docker.StopContainer(ctx, p.ID)
		case "containers.restart":
			err = docker.RestartContainer(ctx, p.ID)
		}
		return nil, err

	case "containers.logs":
		var p logsParams
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return nil, err
		}
		logs, err := docker.Logs(ctx, p.ID, p.Tail)
		if err != nil {
			return nil, err
		}
		return map[string]string{"logs": logs}, nil

	default:
		return nil, unknownActionError(cmd.Action)
	}
}

type unknownActionError string

func (e unknownActionError) Error() string {
	return "unknown command action: " + string(e)
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required environment variable %s", key)
	}
	return v
}
