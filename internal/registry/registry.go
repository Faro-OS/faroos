// Package registry holds the in-memory set of paired nodes.
//
// This is deliberately minimal for the MVP: no persistence yet. Restarting
// the server currently forgets paired nodes and they need to re-pair. That's
// a known gap, tracked for whoever picks up storage/persistence next.
package registry

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/faroos/faroos/internal/model"
)

var ErrNotFound = errors.New("node not found")
var ErrBadToken = errors.New("invalid pairing token")

type Registry struct {
	mu    sync.RWMutex
	nodes map[string]*model.Node
}

func New() *Registry {
	return &Registry{nodes: make(map[string]*model.Node)}
}

// CreatePairing issues a new node ID + long-lived token pair. The admin
// hands the token to whatever machine is about to run the agent.
func (r *Registry) CreatePairing(name string) (*model.Node, error) {
	id, err := randomHex(8)
	if err != nil {
		return nil, err
	}
	token, err := randomHex(32)
	if err != nil {
		return nil, err
	}
	n := &model.Node{
		ID:       id,
		Name:     name,
		Token:    token,
		PairedAt: time.Now(),
	}
	r.mu.Lock()
	r.nodes[id] = n
	r.mu.Unlock()
	return n, nil
}

// Authenticate checks a node ID + token pair presented by a connecting agent.
func (r *Registry) Authenticate(id, token string) (*model.Node, error) {
	r.mu.RLock()
	n, ok := r.nodes[id]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	if n.Token != token {
		return nil, ErrBadToken
	}
	return n, nil
}

func (r *Registry) SetConnected(id string, connected bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n, ok := r.nodes[id]; ok {
		n.Connected = connected
		n.LastSeen = time.Now()
	}
}

func (r *Registry) UpdateStats(id string, s model.Stats) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n, ok := r.nodes[id]; ok {
		n.Stats = s
		n.LastSeen = time.Now()
	}
}

func (r *Registry) List() []*model.Node {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*model.Node, 0, len(r.nodes))
	for _, n := range r.nodes {
		cp := *n
		out = append(out, &cp)
	}
	return out
}

func (r *Registry) Get(id string) (*model.Node, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n, ok := r.nodes[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *n
	return &cp, nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
