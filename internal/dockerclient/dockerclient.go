// Package dockerclient talks to the local Docker daemon over its unix
// socket using plain HTTP — the Engine API is just JSON over HTTP, so this
// avoids pulling in the full docker/docker SDK (a large dependency) for the
// handful of calls the agent actually needs.
package dockerclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

const apiVersion = "v1.43"

type Client struct {
	http *http.Client
}

// New returns a client talking to the Docker daemon over the given unix
// socket path (typically /var/run/docker.sock).
func New(socketPath string) *Client {
	return &Client{
		http: &http.Client{
			// Stopping a container gracefully can itself take up to Docker's
			// default 10s SIGTERM grace period before it's force-killed.
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					d := net.Dialer{}
					return d.DialContext(ctx, "unix", socketPath)
				},
			},
		},
	}
}

// Ping checks whether the Docker socket is reachable at all — used to
// decide whether to advertise container management for this node.
func (c *Client) Ping(ctx context.Context) error {
	res, err := c.do(ctx, http.MethodGet, "/_ping", nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("docker ping: unexpected status %d", res.StatusCode)
	}
	return nil
}

type Container struct {
	ID      string            `json:"id"`
	Names   []string          `json:"names"`
	Image   string            `json:"image"`
	State   string            `json:"state"`
	Status  string            `json:"status"`
	Ports   []Port            `json:"ports"`
	Labels  map[string]string `json:"labels"`
	Created int64             `json:"created"`
}

type Port struct {
	PrivatePort uint16 `json:"privatePort"`
	PublicPort  uint16 `json:"publicPort,omitempty"`
	Type        string `json:"type"`
}

// rawContainer mirrors the subset of Docker's /containers/json response we
// care about.
type rawContainer struct {
	ID      string   `json:"Id"`
	Names   []string `json:"Names"`
	Image   string   `json:"Image"`
	State   string   `json:"State"`
	Status  string   `json:"Status"`
	Created int64    `json:"Created"`
	Ports   []struct {
		PrivatePort uint16 `json:"PrivatePort"`
		PublicPort  uint16 `json:"PublicPort"`
		Type        string `json:"Type"`
	} `json:"Ports"`
	Labels map[string]string `json:"Labels"`
}

func (c *Client) ListContainers(ctx context.Context) ([]Container, error) {
	res, err := c.do(ctx, http.MethodGet, "/containers/json?all=true", nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, statusErr(res)
	}

	var raw []rawContainer
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, err
	}

	out := make([]Container, 0, len(raw))
	for _, rc := range raw {
		ports := make([]Port, 0, len(rc.Ports))
		for _, p := range rc.Ports {
			ports = append(ports, Port{PrivatePort: p.PrivatePort, PublicPort: p.PublicPort, Type: p.Type})
		}
		out = append(out, Container{
			ID:      rc.ID,
			Names:   rc.Names,
			Image:   rc.Image,
			State:   rc.State,
			Status:  rc.Status,
			Ports:   ports,
			Labels:  rc.Labels,
			Created: rc.Created,
		})
	}
	return out, nil
}

func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.postAction(ctx, id, "start")
}

func (c *Client) StopContainer(ctx context.Context, id string) error {
	return c.postAction(ctx, id, "stop")
}

func (c *Client) RestartContainer(ctx context.Context, id string) error {
	return c.postAction(ctx, id, "restart")
}

func (c *Client) postAction(ctx context.Context, id, action string) error {
	res, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/containers/%s/%s", id, action), nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	// Docker returns 204 on success, 304 if already in that state — both fine.
	if res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusNotModified {
		return statusErr(res)
	}
	return nil
}

// Logs returns the last `tail` lines of a container's combined stdout/stderr.
func (c *Client) Logs(ctx context.Context, id string, tail int) (string, error) {
	if tail <= 0 {
		tail = 200
	}
	path := fmt.Sprintf("/containers/%s/logs?stdout=true&stderr=true&tail=%d", id, tail)
	res, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", statusErr(res)
	}
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return demultiplexLogs(raw), nil
}

// demultiplexLogs strips Docker's 8-byte stream-framing header from each
// log frame when the container wasn't started with a TTY, returning plain
// text. If the frames don't look like Docker's format (e.g. TTY-attached
// containers, which are already plain text), it returns the input as-is.
func demultiplexLogs(raw []byte) string {
	out := make([]byte, 0, len(raw))
	i := 0
	for i+8 <= len(raw) {
		streamType := raw[i]
		if streamType > 2 {
			// Doesn't look like a framed stream — treat the rest as plain text.
			out = append(out, raw[i:]...)
			return string(out)
		}
		size := int(raw[i+4])<<24 | int(raw[i+5])<<16 | int(raw[i+6])<<8 | int(raw[i+7])
		start := i + 8
		end := start + size
		if end > len(raw) {
			end = len(raw)
		}
		out = append(out, raw[start:end]...)
		i = end
	}
	if i < len(raw) {
		out = append(out, raw[i:]...)
	}
	return string(out)
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, "http://docker/"+apiVersion+path, body)
	if err != nil {
		return nil, err
	}
	return c.http.Do(req)
}

func statusErr(res *http.Response) error {
	msg, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
	return fmt.Errorf("docker API %s: %d %s", res.Request.URL.Path, res.StatusCode, string(msg))
}
