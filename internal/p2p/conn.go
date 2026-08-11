package p2p

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/datachannel"
)

const (
	frameMagic        = "F2P1"
	frameHeaderSize   = 24
	maxFramePayload   = 48 * 1024
	maxEnvelopeSize   = 64 * 1024 * 1024
	maxDataChannelMsg = frameHeaderSize + maxFramePayload
)

// Conn exposes the same JSON-oriented operations used by the existing agent
// WebSocket, while its bytes travel over an encrypted WebRTC data channel.
// Messages are split into bounded frames so large log and file responses do
// not depend on a single SCTP message being unusually large.
type Conn struct {
	raw  datachannel.ReadWriteCloser
	peer *Peer

	writeMu sync.Mutex
	close   sync.Once
	nextID  atomic.Uint64
}

func newConn(raw datachannel.ReadWriteCloser, peer *Peer) *Conn {
	return &Conn{raw: raw, peer: peer}
}

func (c *Conn) WriteJSON(value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(payload) > maxEnvelopeSize {
		return fmt.Errorf("P2P envelope exceeds %d bytes", maxEnvelopeSize)
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	messageID := c.nextID.Add(1)
	total := (len(payload) + maxFramePayload - 1) / maxFramePayload
	if total == 0 {
		total = 1
	}
	for index := 0; index < total; index++ {
		start := index * maxFramePayload
		end := start + maxFramePayload
		if end > len(payload) {
			end = len(payload)
		}
		frame := make([]byte, frameHeaderSize+end-start)
		copy(frame[:4], frameMagic)
		binary.BigEndian.PutUint64(frame[4:12], messageID)
		binary.BigEndian.PutUint32(frame[12:16], uint32(index))
		binary.BigEndian.PutUint32(frame[16:20], uint32(total))
		binary.BigEndian.PutUint32(frame[20:24], uint32(len(payload)))
		copy(frame[frameHeaderSize:], payload[start:end])
		n, writeErr := c.raw.Write(frame)
		if writeErr != nil {
			return writeErr
		}
		if n != len(frame) {
			return io.ErrShortWrite
		}
	}
	return nil
}

func (c *Conn) ReadJSON(value any) error {
	frame := make([]byte, maxDataChannelMsg)
	var (
		messageID uint64
		total     uint32
		next      uint32
		expected  uint32
		payload   []byte
	)
	for {
		n, err := c.raw.Read(frame)
		if err != nil {
			return err
		}
		if n < frameHeaderSize || string(frame[:4]) != frameMagic {
			return errors.New("invalid P2P frame")
		}
		frameID := binary.BigEndian.Uint64(frame[4:12])
		index := binary.BigEndian.Uint32(frame[12:16])
		frameTotal := binary.BigEndian.Uint32(frame[16:20])
		frameLength := binary.BigEndian.Uint32(frame[20:24])
		if frameTotal == 0 || frameLength > maxEnvelopeSize {
			return errors.New("invalid P2P frame size")
		}
		if index == 0 {
			messageID = frameID
			total = frameTotal
			next = 0
			expected = frameLength
			payload = make([]byte, 0, int(frameLength))
		}
		if payload == nil || frameID != messageID || index != next || frameTotal != total || frameLength != expected {
			return errors.New("out-of-order P2P frame")
		}
		payload = append(payload, frame[frameHeaderSize:n]...)
		if len(payload) > int(expected) {
			return errors.New("oversized P2P payload")
		}
		next++
		if next != total {
			continue
		}
		if len(payload) != int(expected) {
			return errors.New("incomplete P2P payload")
		}
		return json.Unmarshal(payload, value)
	}
}

func (c *Conn) SetReadDeadline(deadline time.Time) error {
	deadliner, ok := c.raw.(datachannel.ReadWriteCloserDeadliner)
	if !ok {
		return errors.New("P2P data channel does not support deadlines")
	}
	return deadliner.SetReadDeadline(deadline)
}

func (c *Conn) Close() error {
	var closeErr error
	c.close.Do(func() {
		closeErr = c.raw.Close()
		if err := c.peer.Close(); closeErr == nil {
			closeErr = err
		}
	})
	return closeErr
}
