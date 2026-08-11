// Package p2p negotiates direct, encrypted server-to-server connections. The
// existing FaroOS relay carries only the authenticated WebRTC offer/answer;
// once ICE finds a viable path, management traffic bypasses the relay.
package p2p

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/pion/datachannel"
	"github.com/pion/webrtc/v4"
)

const (
	Subprotocol     = "faroos-p2p-v1"
	DefaultSTUNURL  = "stun:relay.faroos.dev:3478"
	dataChannelName = "faroos-control-v1"
)

type peerResult struct {
	conn *Conn
	err  error
}

// Peer is one side of an in-progress WebRTC negotiation.
type Peer struct {
	connection *webrtc.PeerConnection
	ready      chan peerResult
	closeOnce  sync.Once
}

func NewOffer(ctx context.Context, stunURL string) (*Peer, string, error) {
	peer, err := newPeer(stunURL)
	if err != nil {
		return nil, "", err
	}
	channel, err := peer.connection.CreateDataChannel(dataChannelName, nil)
	if err != nil {
		peer.Close()
		return nil, "", err
	}
	peer.attach(channel)

	offer, err := peer.connection.CreateOffer(nil)
	if err != nil {
		peer.Close()
		return nil, "", err
	}
	description, err := peer.setLocalAndGather(ctx, offer)
	if err != nil {
		peer.Close()
		return nil, "", err
	}
	return peer, description, nil
}

func Answer(ctx context.Context, stunURL, encodedOffer string) (*Peer, string, error) {
	peer, err := newPeer(stunURL)
	if err != nil {
		return nil, "", err
	}
	peer.connection.OnDataChannel(func(channel *webrtc.DataChannel) {
		if channel.Label() != dataChannelName {
			_ = channel.Close()
			return
		}
		peer.attach(channel)
	})

	var offer webrtc.SessionDescription
	if err := json.Unmarshal([]byte(encodedOffer), &offer); err != nil {
		peer.Close()
		return nil, "", fmt.Errorf("decode P2P offer: %w", err)
	}
	if err := peer.connection.SetRemoteDescription(offer); err != nil {
		peer.Close()
		return nil, "", err
	}
	answer, err := peer.connection.CreateAnswer(nil)
	if err != nil {
		peer.Close()
		return nil, "", err
	}
	description, err := peer.setLocalAndGather(ctx, answer)
	if err != nil {
		peer.Close()
		return nil, "", err
	}
	return peer, description, nil
}

func newPeer(stunURL string) (*Peer, error) {
	settings := webrtc.SettingEngine{}
	settings.DetachDataChannels()
	settings.SetSCTPMaxMessageSize(maxDataChannelMsg)
	settings.SetSCTPMaxReceiveBufferSize(4 * 1024 * 1024)
	api := webrtc.NewAPI(webrtc.WithSettingEngine(settings))
	config := webrtc.Configuration{}
	if stunURL != "" {
		config.ICEServers = []webrtc.ICEServer{{URLs: []string{stunURL}}}
	}
	connection, err := api.NewPeerConnection(config)
	if err != nil {
		return nil, err
	}
	peer := &Peer{connection: connection, ready: make(chan peerResult, 1)}
	connection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateFailed {
			peer.deliver(nil, errors.New("P2P connection failed"))
		}
	})
	return peer, nil
}

func (p *Peer) attach(channel *webrtc.DataChannel) {
	channel.OnOpen(func() {
		raw, err := channel.Detach()
		if err != nil {
			p.deliver(nil, err)
			return
		}
		p.deliver(raw, nil)
	})
}

func (p *Peer) deliver(raw datachannel.ReadWriteCloser, err error) {
	result := peerResult{err: err}
	if raw != nil {
		result.conn = newConn(raw, p)
	}
	select {
	case p.ready <- result:
	default:
		if raw != nil {
			_ = raw.Close()
		}
	}
}

func (p *Peer) setLocalAndGather(ctx context.Context, description webrtc.SessionDescription) (string, error) {
	gathered := webrtc.GatheringCompletePromise(p.connection)
	if err := p.connection.SetLocalDescription(description); err != nil {
		return "", err
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-gathered:
	}
	local := p.connection.LocalDescription()
	if local == nil {
		return "", errors.New("P2P local description unavailable")
	}
	payload, err := json.Marshal(local)
	return string(payload), err
}

func (p *Peer) SetAnswer(encodedAnswer string) error {
	var answer webrtc.SessionDescription
	if err := json.Unmarshal([]byte(encodedAnswer), &answer); err != nil {
		return fmt.Errorf("decode P2P answer: %w", err)
	}
	return p.connection.SetRemoteDescription(answer)
}

func (p *Peer) Connect(ctx context.Context) (*Conn, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-p.ready:
		return result.conn, result.err
	}
}

func (p *Peer) Close() error {
	var closeErr error
	p.closeOnce.Do(func() { closeErr = p.connection.Close() })
	return closeErr
}
