// Package stunserver runs the small public rendezvous helper used for NAT
// discovery. It never carries FaroOS management data; it only reports the
// source IP and UDP port that a node presents to the public internet.
package stunserver

import (
	"net"
	"sync"
	"time"

	"github.com/pion/turn/v5"
)

const (
	requestsPerSecond = 20.0
	requestBurst      = 40.0
)

type Server struct {
	listener net.PacketConn
	server   *turn.Server
	close    sync.Once
}

func New(address string) (*Server, error) {
	listener, err := net.ListenPacket("udp4", address)
	if err != nil {
		return nil, err
	}
	limited := &rateLimitedPacketConn{
		PacketConn: listener,
		clients:    make(map[string]*clientBucket),
	}
	server, err := turn.NewServer(turn.ServerConfig{
		PacketConnConfigs: []turn.PacketConnConfig{{PacketConn: limited}},
	})
	if err != nil {
		listener.Close()
		return nil, err
	}
	return &Server{listener: listener, server: server}, nil
}

func (s *Server) Addr() net.Addr { return s.listener.LocalAddr() }

func (s *Server) Close() error {
	var closeErr error
	s.close.Do(func() {
		closeErr = s.server.Close()
		if err := s.listener.Close(); closeErr == nil {
			closeErr = err
		}
	})
	return closeErr
}

type clientBucket struct {
	tokens float64
	last   time.Time
}

type rateLimitedPacketConn struct {
	net.PacketConn
	mu          sync.Mutex
	clients     map[string]*clientBucket
	lastCleanup time.Time
}

func (c *rateLimitedPacketConn) ReadFrom(buffer []byte) (int, net.Addr, error) {
	for {
		n, address, err := c.PacketConn.ReadFrom(buffer)
		if err != nil {
			return 0, nil, err
		}
		if c.allow(address, time.Now()) {
			return n, address, nil
		}
	}
}

func (c *rateLimitedPacketConn) allow(address net.Addr, now time.Time) bool {
	host, _, err := net.SplitHostPort(address.String())
	if err != nil {
		host = address.String()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastCleanup.IsZero() || now.Sub(c.lastCleanup) >= time.Minute {
		for key, bucket := range c.clients {
			if now.Sub(bucket.last) > 5*time.Minute {
				delete(c.clients, key)
			}
		}
		c.lastCleanup = now
	}
	bucket := c.clients[host]
	if bucket == nil {
		bucket = &clientBucket{tokens: requestBurst, last: now}
		c.clients[host] = bucket
	}
	bucket.tokens += now.Sub(bucket.last).Seconds() * requestsPerSecond
	if bucket.tokens > requestBurst {
		bucket.tokens = requestBurst
	}
	bucket.last = now
	if bucket.tokens < 1 {
		return false
	}
	bucket.tokens--
	return true
}
