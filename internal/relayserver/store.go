package relayserver

import (
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"errors"

	_ "modernc.org/sqlite"
)

var ErrBadCredentials = errors.New("invalid relay credentials")

type credentialStore struct {
	db *sql.DB
}

func openCredentialStore(path string) (*credentialStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// Registrations are tiny and rare; one connection avoids SQLite lock
	// contention during a burst of first-time panel connections.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS relay_panels (
			id TEXT PRIMARY KEY,
			secret_hash BLOB NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		db.Close()
		return nil, err
	}
	return &credentialStore{db: db}, nil
}

// authorize registers a cryptographically random panel ID on first use and
// requires the same secret on every reconnect. The public URL contains only
// the ID; the secret never passes through browsers or managed agents.
func (s *credentialStore) authorize(id, secret string) error {
	hash := sha256.Sum256([]byte(secret))
	if _, err := s.db.Exec(
		`INSERT INTO relay_panels(id, secret_hash) VALUES(?, ?) ON CONFLICT(id) DO NOTHING`,
		id, hash[:],
	); err != nil {
		return err
	}
	var stored []byte
	if err := s.db.QueryRow(`SELECT secret_hash FROM relay_panels WHERE id = ?`, id).Scan(&stored); err != nil {
		return err
	}
	if len(stored) != len(hash) || subtle.ConstantTimeCompare(stored, hash[:]) != 1 {
		return ErrBadCredentials
	}
	return nil
}

func (s *credentialStore) close() error {
	return s.db.Close()
}
