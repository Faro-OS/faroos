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
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/faroos/faroos/internal/appcatalog"
	"github.com/faroos/faroos/internal/dockerclient"
	"github.com/faroos/faroos/internal/fileops"
	"github.com/faroos/faroos/internal/netspeed"
	"github.com/faroos/faroos/internal/p2p"
	"github.com/faroos/faroos/internal/proto"
	"github.com/faroos/faroos/internal/sysstats"
	"github.com/faroos/faroos/internal/termsession"
)

// version is overridden at release build time via
// -ldflags "-X main.version=vX.Y.Z" (see .github/workflows/release.yml) —
// has to be a var, not a const, for -X to be able to touch it.
var version = "0.0.1-dev"

const (
	heartbeatInterval = 10 * time.Second
	heartbeatTimeout  = 30 * time.Second
)

// deps bundles the agent's local capabilities so they can be threaded
// through the command dispatcher without an ever-growing parameter list.
type deps struct {
	docker    *dockerclient.Client
	files     *fileops.Root
	terminals *termsession.Manager
	network   *netspeed.Tester
	appsDir   string
}

type jsonConnection interface {
	ReadJSON(any) error
	WriteJSON(any) error
	SetReadDeadline(time.Time) error
	Close() error
}

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Println(version)
		return
	}

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
		network:   netspeed.New(),
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
	conn, transport, err := dialAgentConnection(serverURL, nodeID, token)
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

	log.Printf("connected to %s as node %s via %s", serverURL, nodeID, transport)

	// A one-second cadence keeps the dashboard genuinely live while remaining
	// cheap: every snapshot is read from local kernel counters.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	statsCollector := sysstats.NewCollector()

	stats := statsCollector.Collect()
	if err := writeJSON(proto.Envelope{Type: proto.TypeStats, Stats: &stats}); err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		for {
			if err := conn.SetReadDeadline(time.Now().Add(heartbeatTimeout)); err != nil {
				errCh <- err
				return
			}
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

	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()
	for {
		select {
		case err := <-errCh:
			return err
		case <-heartbeat.C:
			if err := writeJSON(proto.Envelope{Type: proto.TypePing}); err != nil {
				return err
			}
		case <-ticker.C:
			stats := statsCollector.Collect()
			if err := writeJSON(proto.Envelope{Type: proto.TypeStats, Stats: &stats}); err != nil {
				return err
			}
		}
	}
}

func dialAgentConnection(serverURL, nodeID, token string) (jsonConnection, string, error) {
	p2pDisabled := envTruthy("FAROOS_P2P_DISABLED")
	dialer := *websocket.DefaultDialer
	if !p2pDisabled {
		dialer.Subprotocols = []string{p2p.Subprotocol}
	}
	websocketConn, _, err := dialer.Dial(serverURL, nil)
	if err != nil {
		return nil, "", err
	}

	var (
		peer  *p2p.Peer
		offer string
	)
	if !p2pDisabled && websocketConn.Subprotocol() == p2p.Subprotocol {
		stunURL := strings.TrimSpace(os.Getenv("FAROOS_STUN_URL"))
		if stunURL == "" {
			stunURL = p2p.DefaultSTUNURL
		}
		offerCtx, cancelOffer := context.WithTimeout(context.Background(), 8*time.Second)
		peer, offer, err = p2p.NewOffer(offerCtx, stunURL)
		cancelOffer()
		if err != nil {
			log.Printf("direct P2P offer unavailable, using relay: %v", err)
			peer = nil
			offer = ""
		}
	}

	if err := websocketConn.WriteJSON(proto.Envelope{
		Type: proto.TypeHello,
		Hello: &proto.Hello{
			NodeID:   nodeID,
			Token:    token,
			Version:  version,
			P2POffer: offer,
		},
	}); err != nil {
		if peer != nil {
			peer.Close()
		}
		websocketConn.Close()
		return nil, "", err
	}
	if peer == nil {
		return websocketConn, "relay", nil
	}

	websocketConn.SetReadDeadline(time.Now().Add(12 * time.Second))
	var response proto.Envelope
	if err := websocketConn.ReadJSON(&response); err != nil {
		peer.Close()
		websocketConn.Close()
		return nil, "", fmt.Errorf("read P2P answer: %w", err)
	}
	websocketConn.SetReadDeadline(time.Time{})
	if response.Type != proto.TypeP2PAnswer || response.P2PAnswer == nil || response.P2PAnswer.SDP == "" {
		peer.Close()
		if response.P2PAnswer != nil && response.P2PAnswer.Error != "" {
			log.Printf("direct P2P unavailable, using relay: %s", response.P2PAnswer.Error)
		}
		return websocketConn, "relay", nil
	}
	if err := peer.SetAnswer(response.P2PAnswer.SDP); err != nil {
		peer.Close()
		log.Printf("direct P2P answer rejected, using relay: %v", err)
		return websocketConn, "relay", nil
	}
	directCtx, cancelDirect := context.WithTimeout(context.Background(), 12*time.Second)
	direct, err := peer.Connect(directCtx)
	cancelDirect()
	if err != nil {
		peer.Close()
		log.Printf("direct path unavailable, using relay: %v", err)
		return websocketConn, "relay", nil
	}
	websocketConn.Close()
	return direct, "direct-p2p", nil
}

func envTruthy(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func handleCommand(d *deps, cmd proto.Command, writeJSON func(any) error) {
	timeout := 20 * time.Second
	if cmd.Action == "apps.deploy" {
		// Pulling a cold image can take minutes on a slow connection.
		timeout = 10 * time.Minute
	} else if cmd.Action == "network.speedtest" {
		timeout = 65 * time.Second
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

type renameFileParams struct {
	From string `json:"from"`
	To   string `json:"to"`
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

	case "files.mkdir":
		var p pathParams
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return nil, err
		}
		return nil, d.files.Mkdir(p.Path)

	case "files.rename":
		var p renameFileParams
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return nil, err
		}
		return nil, d.files.Rename(p.From, p.To)

	case "apps.deploy":
		var spec appcatalog.DeploySpec
		if err := json.Unmarshal(cmd.Params, &spec); err != nil {
			return nil, err
		}
		return nil, deployApp(ctx, d, spec)

	case "apps.remove":
		var p appIDParams
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return nil, err
		}
		return nil, removeApp(ctx, d, p.AppID)

	case "ports.inspect":
		var p portParams
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return nil, err
		}
		return inspectPort(ctx, d, p.Port)

	case "network.speedtest":
		return d.network.Run(ctx)

	default:
		return nil, unknownActionError(cmd.Action)
	}
}

type appIDParams struct {
	AppID string `json:"appId"`
}

type portParams struct {
	Port int `json:"port"`
}

// portStatus reports whether a host port is free, and if not, whether it's
// a FaroOS-deployed container (which the caller can safely offer to stop)
// or something else entirely (host process, unrelated container — we
// deliberately don't try to identify or offer to kill those: this agent
// runs on a real production machine and guessing wrong about what's safe
// to kill is exactly the kind of "confidently automated" mistake that
// takes down an unrelated service).
type portStatus struct {
	Port          int    `json:"port"`
	InUse         bool   `json:"inUse"`
	OwnApp        bool   `json:"ownApp"`
	ContainerID   string `json:"containerId,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
}

func inspectPort(ctx context.Context, d *deps, port int) (*portStatus, error) {
	containers, err := d.docker.ListContainers(ctx)
	if err == nil {
		for _, c := range containers {
			for _, p := range c.Ports {
				if int(p.PublicPort) == port {
					name := ""
					if len(c.Names) > 0 {
						name = strings.TrimPrefix(c.Names[0], "/")
					}
					_, isOurs := strings.CutPrefix(name, "faroos-app-")
					return &portStatus{
						Port: port, InUse: true, OwnApp: isOurs,
						ContainerID: c.ID, ContainerName: name,
					}, nil
				}
			}
		}
	}

	// Not one of our containers by that published port — fall back to a
	// direct bind test, which works regardless of what's using it (docker
	// container we don't recognize, or a plain host process) without
	// needing root or shelling out to `ss`.
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return &portStatus{Port: port, InUse: true}, nil
	}
	ln.Close()
	return &portStatus{Port: port, InUse: false}, nil
}

func deployApp(ctx context.Context, d *deps, spec appcatalog.DeploySpec) error {
	if spec.Image == "" {
		return fmt.Errorf("deploy spec for %s is missing an image", spec.AppID)
	}

	if err := d.docker.PullImage(ctx, spec.Image); err != nil {
		return err
	}

	portBindings := make(map[string]int, len(spec.Ports))
	for _, p := range spec.Ports {
		portProto := p.Protocol
		if portProto == "" {
			portProto = "tcp"
		}
		portBindings[fmt.Sprintf("%d/%s", p.Container, portProto)] = p.Host
	}

	binds := make([]string, 0, len(spec.Volumes))
	for _, v := range spec.Volumes {
		hostPath := filepath.Join(d.appsDir, spec.AppID, v.Name)
		if err := os.MkdirAll(hostPath, 0o755); err != nil {
			return err
		}
		binds = append(binds, hostPath+":"+v.Container)
	}

	env := make([]string, 0, len(spec.Env))
	for _, e := range spec.Env {
		env = append(env, e.Key+"="+e.Default)
	}

	containerName := appcatalog.ContainerName(spec.AppID)
	if existing, err := d.docker.FindByName(ctx, containerName); err == nil && existing != nil {
		return fmt.Errorf("%s is already deployed", spec.AppID)
	}

	id, err := d.docker.CreateContainer(ctx, dockerclient.ContainerSpec{
		Name:         containerName,
		Image:        spec.Image,
		Env:          env,
		Command:      spec.Command,
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
