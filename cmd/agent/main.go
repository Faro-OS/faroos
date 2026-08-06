// Command agent runs on each managed server. It connects outbound to the
// central panel over a websocket (so it works behind NAT with no inbound
// ports needed), reports system stats periodically, and executes commands
// the panel sends it (e.g. Docker container management).
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/faroos/faroos/internal/appcatalog"
	"github.com/faroos/faroos/internal/dockerclient"
	"github.com/faroos/faroos/internal/fileops"
	"github.com/faroos/faroos/internal/proto"
	"github.com/faroos/faroos/internal/sysstats"
	"github.com/faroos/faroos/internal/termsession"
)

const version = "0.0.1-dev"

// deps bundles the agent's local capabilities so they can be threaded
// through the command dispatcher without an ever-growing parameter list.
type deps struct {
	docker    *dockerclient.Client
	files     *fileops.Root
	terminals *termsession.Manager
	appsDir   string
}

func main() {
	serverURL := requireEnv("FAROOS_SERVER") // e.g. ws://panel.example.com/api/agent/connect
	nodeID := requireEnv("FAROOS_NODE_ID")
	token := requireEnv("FAROOS_TOKEN")

	dockerSocket := os.Getenv("FAROOS_DOCKER_SOCKET")
	if dockerSocket == "" {
		dockerSocket = "/var/run/docker.sock"
	}

	filesRoot := os.Getenv("FAROOS_FILES_ROOT")
	if filesRoot == "" {
		if home, err := os.UserHomeDir(); err == nil {
			filesRoot = home
		} else {
			filesRoot = "."
		}
	}
	files, err := fileops.NewRoot(filesRoot)
	if err != nil {
		log.Fatalf("failed to initialize files root %s: %v", filesRoot, err)
	}
	log.Printf("file manager root: %s", filesRoot)

	appsDir := os.Getenv("FAROOS_APPS_DATA_DIR")
	if appsDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			appsDir = home + "/faroos-apps"
		} else {
			appsDir = "./faroos-apps"
		}
	}
	if err := os.MkdirAll(appsDir, 0o755); err != nil {
		log.Fatalf("failed to create apps data dir %s: %v", appsDir, err)
	}
	log.Printf("app store data dir: %s", appsDir)

	d := &deps{
		docker:    dockerclient.New(dockerSocket),
		files:     files,
		terminals: termsession.NewManager(),
		appsDir:   appsDir,
	}

	for {
		if err := runOnce(serverURL, nodeID, token, d); err != nil {
			log.Printf("connection lost: %v — reconnecting in 5s", err)
		}
		time.Sleep(5 * time.Second)
	}
}

func runOnce(serverURL, nodeID, token string, d *deps) error {
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
			switch {
			case env.Type == proto.TypeCommand && env.Command != nil:
				// Run in its own goroutine so a slow command (e.g. fetching
				// logs) doesn't stall reading further messages/pings.
				go handleCommand(d, *env.Command, writeJSON)
			case env.Type == proto.TypeTerminalOpen && env.TerminalOpen != nil:
				handleTerminalOpen(d.terminals, *env.TerminalOpen, writeJSON)
			case env.Type == proto.TypeTerminalInput && env.TerminalData != nil:
				handleTerminalInput(d.terminals, *env.TerminalData)
			case env.Type == proto.TypeTerminalResize && env.TerminalResize != nil:
				d.terminals.Resize(env.TerminalResize.SessionID, env.TerminalResize.Cols, env.TerminalResize.Rows)
			case env.Type == proto.TypeTerminalClose && env.TerminalClose != nil:
				d.terminals.Close(env.TerminalClose.SessionID)
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

func handleCommand(d *deps, cmd proto.Command, writeJSON func(any) error) {
	timeout := 20 * time.Second
	if cmd.Action == "apps.deploy" {
		// Pulling a cold image can take minutes on a slow connection.
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	result, err := dispatch(ctx, d, cmd)

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

type pathParams struct {
	Path string `json:"path"`
}

type writeFileParams struct {
	Path       string `json:"path"`
	ContentB64 string `json:"contentB64"`
}

func dispatch(ctx context.Context, d *deps, cmd proto.Command) (any, error) {
	switch cmd.Action {
	case "containers.list":
		return d.docker.ListContainers(ctx)

	case "containers.start", "containers.stop", "containers.restart":
		var p containerIDParams
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return nil, err
		}
		var err error
		switch cmd.Action {
		case "containers.start":
			err = d.docker.StartContainer(ctx, p.ID)
		case "containers.stop":
			err = d.docker.StopContainer(ctx, p.ID)
		case "containers.restart":
			err = d.docker.RestartContainer(ctx, p.ID)
		}
		return nil, err

	case "containers.logs":
		var p logsParams
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return nil, err
		}
		logs, err := d.docker.Logs(ctx, p.ID, p.Tail)
		if err != nil {
			return nil, err
		}
		return map[string]string{"logs": logs}, nil

	case "files.list":
		var p pathParams
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return nil, err
		}
		return d.files.List(p.Path)

	case "files.download":
		var p pathParams
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return nil, err
		}
		data, err := d.files.ReadFile(p.Path)
		if err != nil {
			return nil, err
		}
		return map[string]string{"contentB64": base64.StdEncoding.EncodeToString(data)}, nil

	case "files.upload":
		var p writeFileParams
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return nil, err
		}
		data, err := base64.StdEncoding.DecodeString(p.ContentB64)
		if err != nil {
			return nil, err
		}
		return nil, d.files.WriteFile(p.Path, data)

	case "files.delete":
		var p pathParams
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return nil, err
		}
		return nil, d.files.Delete(p.Path)

	case "apps.deploy":
		var p appIDParams
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return nil, err
		}
		return nil, deployApp(ctx, d, p.AppID)

	case "apps.remove":
		var p appIDParams
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return nil, err
		}
		return nil, removeApp(ctx, d, p.AppID)

	default:
		return nil, unknownActionError(cmd.Action)
	}
}

type appIDParams struct {
	AppID string `json:"appId"`
}

func deployApp(ctx context.Context, d *deps, appID string) error {
	app, ok := appcatalog.Find(appID)
	if !ok {
		return fmt.Errorf("unknown app: %s", appID)
	}

	if err := d.docker.PullImage(ctx, app.Image); err != nil {
		return err
	}

	portBindings := make(map[string]int, len(app.Ports))
	for _, p := range app.Ports {
		portProto := p.Protocol
		if portProto == "" {
			portProto = "tcp"
		}
		portBindings[fmt.Sprintf("%d/%s", p.Container, portProto)] = p.Host
	}

	binds := make([]string, 0, len(app.Volumes))
	for _, v := range app.Volumes {
		hostPath := filepath.Join(d.appsDir, app.ID, v.Name)
		if err := os.MkdirAll(hostPath, 0o755); err != nil {
			return err
		}
		binds = append(binds, hostPath+":"+v.Container)
	}

	env := make([]string, 0, len(app.Env))
	for k, v := range app.Env {
		env = append(env, k+"="+v)
	}

	containerName := appcatalog.ContainerName(app.ID)
	if existing, err := d.docker.FindByName(ctx, containerName); err == nil && existing != nil {
		return fmt.Errorf("%s is already deployed", app.Name)
	}

	id, err := d.docker.CreateContainer(ctx, dockerclient.ContainerSpec{
		Name:         containerName,
		Image:        app.Image,
		Env:          env,
		PortBindings: portBindings,
		Binds:        binds,
	})
	if err != nil {
		return err
	}
	return d.docker.StartContainer(ctx, id)
}

func removeApp(ctx context.Context, d *deps, appID string) error {
	containerName := appcatalog.ContainerName(appID)
	existing, err := d.docker.FindByName(ctx, containerName)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("%s is not deployed", appID)
	}
	return d.docker.RemoveContainer(ctx, existing.ID)
}

func handleTerminalOpen(terminals *termsession.Manager, open proto.TerminalOpen, writeJSON func(any) error) {
	cols, rows := open.Cols, open.Rows
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}

	err := terminals.Open(open.SessionID, cols, rows,
		func(chunk []byte) {
			writeJSON(proto.Envelope{
				Type: proto.TypeTerminalOutput,
				TerminalData: &proto.TerminalData{
					SessionID: open.SessionID,
					DataB64:   base64.StdEncoding.EncodeToString(chunk),
				},
			})
		},
		func(reason string) {
			writeJSON(proto.Envelope{
				Type:          proto.TypeTerminalClose,
				TerminalClose: &proto.TerminalClose{SessionID: open.SessionID, Reason: reason},
			})
		},
	)
	if err != nil {
		writeJSON(proto.Envelope{
			Type:          proto.TypeTerminalClose,
			TerminalClose: &proto.TerminalClose{SessionID: open.SessionID, Reason: err.Error()},
		})
	}
}

func handleTerminalInput(terminals *termsession.Manager, data proto.TerminalData) {
	raw, err := base64.StdEncoding.DecodeString(data.DataB64)
	if err != nil {
		return
	}
	terminals.Write(data.SessionID, raw)
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
