// Package termsession manages PTY-backed shell sessions on the agent side,
// keyed by a session ID chosen by the server so several terminal tabs can
// be multiplexed over the single agent websocket connection.
package termsession

import (
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

type Manager struct {
	mu       sync.Mutex
	sessions map[string]*session
}

type session struct {
	cmd  *exec.Cmd
	ptmx *os.File
}

func NewManager() *Manager {
	return &Manager{sessions: make(map[string]*session)}
}

func shellPath() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "/bin/sh"
}

// Open spawns a new shell attached to a PTY of the given size and starts
// forwarding its output to onOutput until the session is closed (either by
// the shell exiting or a call to Close). onOutput may be called from a
// background goroutine.
func (m *Manager) Open(sessionID string, cols, rows int, onOutput func([]byte), onClose func(reason string)) error {
	cmd := exec.Command(shellPath())
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.sessions[sessionID] = &session{cmd: cmd, ptmx: ptmx}
	m.mu.Unlock()

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				onOutput(chunk)
			}
			if err != nil {
				m.mu.Lock()
				delete(m.sessions, sessionID)
				m.mu.Unlock()
				ptmx.Close()
				cmd.Wait()
				onClose("shell exited")
				return
			}
		}
	}()

	return nil
}

func (m *Manager) Write(sessionID string, data []byte) error {
	m.mu.Lock()
	s, ok := m.sessions[sessionID]
	m.mu.Unlock()
	if !ok {
		return errSessionNotFound
	}
	_, err := s.ptmx.Write(data)
	return err
}

func (m *Manager) Resize(sessionID string, cols, rows int) error {
	m.mu.Lock()
	s, ok := m.sessions[sessionID]
	m.mu.Unlock()
	if !ok {
		return errSessionNotFound
	}
	return pty.Setsize(s.ptmx, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

// Close terminates the shell and its PTY. Safe to call even if the session
// already ended on its own.
func (m *Manager) Close(sessionID string) {
	m.mu.Lock()
	s, ok := m.sessions[sessionID]
	delete(m.sessions, sessionID)
	m.mu.Unlock()
	if !ok {
		return
	}
	if s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}
	s.ptmx.Close()
}

type sessionNotFoundError string

func (e sessionNotFoundError) Error() string { return string(e) }

const errSessionNotFound = sessionNotFoundError("terminal session not found")
