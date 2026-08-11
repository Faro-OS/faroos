// Package dockerclient talks to the local Docker daemon over its unix
// socket using plain HTTP — the Engine API is just JSON over HTTP, so this
// avoids pulling in the full docker/docker SDK (a large dependency) for the
// handful of calls the agent actually needs.
package dockerclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Client struct {
	http    *http.Client
	baseURL string

	versionMu  sync.Mutex
	apiVersion string
}

// New returns a client talking to the Docker daemon over the given unix
// socket path (typically /var/run/docker.sock).
func New(socketPath string) *Client {
	return &Client{
		http: &http.Client{
			// Generous ceiling — actual bounding comes from the context each
			// call is given. Needs to be long enough for a cold image pull
			// (can be minutes), not just quick container actions.
			Timeout: 10 * time.Minute,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					d := net.Dialer{}
					return d.DialContext(ctx, "unix", socketPath)
				},
			},
		},
		baseURL: "http://docker",
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

// PullImage pulls an image (with tag/digest included in the ref, e.g.
// "louislam/uptime-kuma:1") and blocks until the pull finishes, erroring
// out if the daemon reports a failure partway through the stream.
func (c *Client) PullImage(ctx context.Context, image string) error {
	res, err := c.do(ctx, http.MethodPost, "/images/create?fromImage="+url.QueryEscape(image), nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return statusErr(res)
	}

	dec := json.NewDecoder(res.Body)
	for {
		var line struct {
			Error string `json:"error"`
		}
		if err := dec.Decode(&line); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if line.Error != "" {
			return fmt.Errorf("pulling %s: %s", image, line.Error)
		}
	}
}

// ContainerSpec is the minimal set of options FaroOS's app catalog needs to
// deploy a single-container app.
type ContainerSpec struct {
	Name    string
	Image   string
	Command []string
	// Env entries are "KEY=VALUE".
	Env []string
	// PortBindings maps "containerPort/proto" (e.g. "80/tcp") to a host
	// port.
	PortBindings map[string]int
	// Binds are "hostPath:containerPath" volume mounts.
	Binds []string
}

// CreateContainer creates (but does not start) a container per spec,
// returning its ID.
func (c *Client) CreateContainer(ctx context.Context, spec ContainerSpec) (string, error) {
	exposedPorts := map[string]struct{}{}
	portBindings := map[string][]map[string]string{}
	for containerPort, hostPort := range spec.PortBindings {
		exposedPorts[containerPort] = struct{}{}
		portBindings[containerPort] = []map[string]string{{"HostPort": fmt.Sprintf("%d", hostPort)}}
	}

	body := map[string]any{
		"Image":        spec.Image,
		"Env":          spec.Env,
		"ExposedPorts": exposedPorts,
		"HostConfig": map[string]any{
			"PortBindings":  portBindings,
			"Binds":         spec.Binds,
			"RestartPolicy": map[string]string{"Name": "unless-stopped"},
		},
	}
	if len(spec.Command) > 0 {
		body["Cmd"] = spec.Command
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	res, err := c.do(ctx, http.MethodPost, "/containers/create?name="+url.QueryEscape(spec.Name), bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		return "", statusErr(res)
	}

	var created struct {
		ID string `json:"Id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		return "", err
	}
	return created.ID, nil
}

// FindByName returns the container whose first name matches "/"+name, if
// any — Docker always prefixes container names with a slash internally.
func (c *Client) FindByName(ctx context.Context, name string) (*Container, error) {
	containers, err := c.ListContainers(ctx)
	if err != nil {
		return nil, err
	}
	target := "/" + name
	for i := range containers {
		for _, n := range containers[i].Names {
			if n == target {
				return &containers[i], nil
			}
		}
	}
	return nil, nil
}

// RemoveContainer force-removes a container (stopping it first if
// running).
func (c *Client) RemoveContainer(ctx context.Context, id string) error {
	res, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/containers/%s?force=true", id), nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusNotFound {
		return statusErr(res)
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	apiVersion, err := c.negotiateAPIVersion(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+"/v"+apiVersion+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		// The Docker daemon rejects POST bodies (e.g. /containers/create)
		// without an explicit Content-Type: "malformed Content-Type
		// header (): mime: no media type".
		req.Header.Set("Content-Type", "application/json")
	}
	return c.http.Do(req)
}

// negotiateAPIVersion asks the daemon for its supported API version instead
// of pinning the agent to one Docker release. Docker exposes /version without
// an API prefix specifically so clients can negotiate before making requests.
func (c *Client) negotiateAPIVersion(ctx context.Context) (string, error) {
	c.versionMu.Lock()
	defer c.versionMu.Unlock()
	if c.apiVersion != "" {
		return c.apiVersion, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/version", nil)
	if err != nil {
		return "", err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("negotiate Docker API version: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", statusErr(res)
	}

	var info struct {
		APIVersion string `json:"ApiVersion"`
	}
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		return "", fmt.Errorf("decode Docker API version: %w", err)
	}
	version, err := normalizeAPIVersion(info.APIVersion)
	if err != nil {
		return "", err
	}
	c.apiVersion = version
	return version, nil
}

func normalizeAPIVersion(raw string) (string, error) {
	version := strings.TrimPrefix(strings.TrimSpace(raw), "v")
	parts := strings.Split(version, ".")
	if len(parts) != 2 {
		return "", fmt.Errorf("Docker reported an invalid API version %q", raw)
	}
	major, majorErr := strconv.Atoi(parts[0])
	minor, minorErr := strconv.Atoi(parts[1])
	if majorErr != nil || minorErr != nil || major < 1 || minor < 0 {
		return "", fmt.Errorf("Docker reported an invalid API version %q", raw)
	}
	return strconv.Itoa(major) + "." + strconv.Itoa(minor), nil
}

func statusErr(res *http.Response) error {
	msg, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
	return fmt.Errorf("docker API %s: %d %s", res.Request.URL.Path, res.StatusCode, string(msg))
}
