// Package relayclient connects a FaroOS panel outbound to the public relay and
// forwards multiplexed relay streams to the panel's local HTTP listener.
package relayclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"

	"github.com/faroos/faroos/internal/relaytransport"
)

type Config struct {
	RelayURL     string
	PublicBase   string
	LocalAddress string
	Credentials  Credentials
}

type Client struct {
	config    Config
	publicURL string
	connected atomic.Bool
}

func New(config Config) (*Client, error) {
	relayURL, err := url.Parse(config.RelayURL)
	if err != nil || (relayURL.Scheme != "ws" && relayURL.Scheme != "wss") || relayURL.Host == "" {
		return nil, errors.New("relay URL must be ws:// or wss://")
	}
	publicBase, err := url.Parse(config.PublicBase)
	if err != nil || (publicBase.Scheme != "http" && publicBase.Scheme != "https") || publicBase.Host == "" {
		return nil, errors.New("relay public base must be http:// or https://")
	}
	if config.LocalAddress == "" || config.Credentials.PanelID == "" || config.Credentials.Secret == "" {
		return nil, errors.New("incomplete relay client configuration")
	}
	return &Client{
		config:    config,
		publicURL: strings.TrimRight(config.PublicBase, "/") + "/" + config.Credentials.PanelID,
	}, nil
}

func (c *Client) PublicURL() string { return c.publicURL }
func (c *Client) Connected() bool   { return c.connected.Load() }

func (c *Client) Run(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		connected, err := c.runOnce(ctx)
		if connected {
			backoff = time.Second
		}
		if err != nil && ctx.Err() == nil {
			log.Printf("relay: %v; reconnecting in %s", err, backoff)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (c *Client) runOnce(ctx context.Context) (bool, error) {
	endpoint, _ := url.Parse(c.config.RelayURL)
	query := endpoint.Query()
	query.Set("id", c.config.Credentials.PanelID)
	endpoint.RawQuery = query.Encode()
	headers := http.Header{"Authorization": []string{"Bearer " + c.config.Credentials.Secret}}
	ws, response, err := websocket.DefaultDialer.DialContext(ctx, endpoint.String(), headers)
	if err != nil {
		if response != nil {
			return false, fmt.Errorf("connect rejected with HTTP %d", response.StatusCode)
		}
		return false, fmt.Errorf("connect: %w", err)
	}
	ws.SetReadLimit(1 << 20)
	transport := relaytransport.NewWebSocketConn(ws)
	config := yamux.DefaultConfig()
	config.EnableKeepAlive = true
	config.KeepAliveInterval = 20 * time.Second
	config.StreamOpenTimeout = 15 * time.Second
	config.LogOutput = io.Discard
	session, err := yamux.Server(transport, config)
	if err != nil {
		transport.Close()
		return false, err
	}
	defer session.Close()
	c.connected.Store(true)
	defer c.connected.Store(false)
	log.Printf("relay: connected; public agent endpoint %s", c.publicURL)

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			session.Close()
		case <-stop:
		}
	}()
	var streams sync.WaitGroup
	for {
		stream, err := session.Accept()
		if err != nil {
			session.Close()
			streams.Wait()
			return true, err
		}
		streams.Add(1)
		go func() {
			defer streams.Done()
			c.forward(stream)
		}()
	}
}

func (c *Client) forward(stream net.Conn) {
	local, err := net.DialTimeout("tcp", c.config.LocalAddress, 10*time.Second)
	if err != nil {
		stream.Close()
		return
	}
	defer local.Close()
	defer stream.Close()

	done := make(chan struct{}, 2)
	go func() {
		io.Copy(local, stream)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(stream, local)
		done <- struct{}{}
	}()
	<-done
}
