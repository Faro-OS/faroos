// Package registry holds the set of paired nodes, persisted to a local
// SQLite file so restarting the server doesn't forget them. Uses
// modernc.org/sqlite (pure Go, no CGO) to keep static binaries and
// cross-compilation simple.
package registry

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	_ "modernc.org/sqlite"

	"github.com/faroos/faroos/internal/model"
)

var ErrNotFound = errors.New("node not found")
var ErrBadToken = errors.New("invalid pairing token")

type Registry struct {
	db *sql.DB
}

// Open opens (creating if needed) the SQLite database at path and ensures
// the schema exists.
func Open(path string) (*Registry, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// SQLite handles one writer at a time; avoid "database is locked" churn
	// under our light concurrent access pattern.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS nodes (
			id         TEXT PRIMARY KEY,
			name       TEXT NOT NULL,
			token      TEXT NOT NULL,
			connected  INTEGER NOT NULL DEFAULT 0,
			paired_at  TEXT NOT NULL,
			last_seen  TEXT,
			stats_json TEXT
		)
	`); err != nil {
		db.Close()
		return nil, err
	}

	return &Registry{db: db}, nil
}

func (r *Registry) Close() error {
	return r.db.Close()
}

// DB exposes the underlying connection so other packages (e.g. auth) can
// share the same SQLite file instead of opening a second one.
func (r *Registry) DB() *sql.DB {
	return r.db
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
	_, err = r.db.Exec(
		`INSERT INTO nodes (id, name, token, connected, paired_at) VALUES (?, ?, ?, 0, ?)`,
		n.ID, n.Name, n.Token, n.PairedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, err
	}
	return n, nil
}

// Authenticate checks a node ID + token pair presented by a connecting agent.
func (r *Registry) Authenticate(id, token string) (*model.Node, error) {
	n, err := r.Get(id)
	if err != nil {
		return nil, err
	}
	if n.Token != token {
		return nil, ErrBadToken
	}
	return n, nil
}

// RotatePairingToken issues fresh credentials for a node that has not yet
// connected. The old installation command stops working immediately, so the
// UI can safely show a newly generated command without ever reading the
// existing long-lived token back through the API.
func (r *Registry) RotatePairingToken(id string) (*model.Node, error) {
	token, err := randomHex(32)
	if err != nil {
		return nil, err
	}
	result, err := r.db.Exec(`UPDATE nodes SET token = ? WHERE id = ? AND connected = 0`, token, id)
	if err != nil {
		return nil, err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if updated == 0 {
		if _, err := r.Get(id); err != nil {
			return nil, err
		}
		return nil, errors.New("connected nodes cannot rotate pairing credentials")
	}
	return r.Get(id)
}

func (r *Registry) SetConnected(id string, connected bool) {
	r.db.Exec(
		`UPDATE nodes SET connected = ?, last_seen = ? WHERE id = ?`,
		boolToInt(connected), time.Now().Format(time.RFC3339Nano), id,
	)
}

func (r *Registry) UpdateStats(id string, s model.Stats) {
	statsJSON, err := json.Marshal(s)
	if err != nil {
		return
	}
	r.db.Exec(
		`UPDATE nodes SET stats_json = ?, last_seen = ? WHERE id = ?`,
		string(statsJSON), time.Now().Format(time.RFC3339Nano), id,
	)
}

func (r *Registry) List() []*model.Node {
	rows, err := r.db.Query(`SELECT id, name, token, connected, paired_at, last_seen, stats_json FROM nodes ORDER BY paired_at ASC`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	out := []*model.Node{}
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			continue
		}
		out = append(out, n)
	}
	return out
}

func (r *Registry) Get(id string) (*model.Node, error) {
	row := r.db.QueryRow(`SELECT id, name, token, connected, paired_at, last_seen, stats_json FROM nodes WHERE id = ?`, id)
	n, err := scanNode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return n, nil
}

// rowScanner covers both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanNode(row rowScanner) (*model.Node, error) {
	var (
		n            model.Node
		connectedInt int
		pairedAt     string
		lastSeen     sql.NullString
		statsJSON    sql.NullString
	)
	if err := row.Scan(&n.ID, &n.Name, &n.Token, &connectedInt, &pairedAt, &lastSeen, &statsJSON); err != nil {
		return nil, err
	}
	n.Connected = connectedInt != 0
	if t, err := time.Parse(time.RFC3339Nano, pairedAt); err == nil {
		n.PairedAt = t
	}
	if lastSeen.Valid {
		if t, err := time.Parse(time.RFC3339Nano, lastSeen.String); err == nil {
			n.LastSeen = t
		}
	}
	if statsJSON.Valid {
		json.Unmarshal([]byte(statsJSON.String), &n.Stats)
	}
	return &n, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
