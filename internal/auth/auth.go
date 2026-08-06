// Package auth guards the FaroOS panel itself with a single admin account.
// Deliberately simple for the MVP: one admin user, session cookie, no
// roles/RBAC yet — agents authenticate separately via their pairing token
// (see internal/registry), this package is only for the humans using the
// dashboard.
package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var ErrAlreadySetUp = errors.New("an admin account already exists")
var ErrInvalidCredentials = errors.New("invalid username or password")
var ErrSessionInvalid = errors.New("session is invalid or expired")

const sessionTTL = 7 * 24 * time.Hour

type Auth struct {
	db *sql.DB
}

func New(db *sql.DB) (*Auth, error) {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS admin (
			id            INTEGER PRIMARY KEY CHECK (id = 1),
			username      TEXT NOT NULL,
			password_hash TEXT NOT NULL
		)
	`); err != nil {
		return nil, err
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			token      TEXT PRIMARY KEY,
			expires_at TEXT NOT NULL
		)
	`); err != nil {
		return nil, err
	}
	return &Auth{db: db}, nil
}

// NeedsSetup reports whether no admin account exists yet.
func (a *Auth) NeedsSetup() bool {
	var count int
	a.db.QueryRow(`SELECT COUNT(*) FROM admin`).Scan(&count)
	return count == 0
}

func (a *Auth) CreateAdmin(username, password string) error {
	if !a.NeedsSetup() {
		return ErrAlreadySetUp
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = a.db.Exec(`INSERT INTO admin (id, username, password_hash) VALUES (1, ?, ?)`, username, string(hash))
	return err
}

func (a *Auth) VerifyLogin(username, password string) error {
	var hash string
	var storedUsername string
	err := a.db.QueryRow(`SELECT username, password_hash FROM admin WHERE id = 1`).Scan(&storedUsername, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidCredentials
	}
	if err != nil {
		return err
	}
	if storedUsername != username {
		return ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}

// CreateSession issues a new session token valid for sessionTTL.
func (a *Auth) CreateSession() (string, time.Time, error) {
	token, err := randomHex(32)
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().Add(sessionTTL)
	_, err = a.db.Exec(`INSERT INTO sessions (token, expires_at) VALUES (?, ?)`, token, expiresAt.Format(time.RFC3339Nano))
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

func (a *Auth) ValidateSession(token string) error {
	if token == "" {
		return ErrSessionInvalid
	}
	var expiresAtStr string
	err := a.db.QueryRow(`SELECT expires_at FROM sessions WHERE token = ?`, token).Scan(&expiresAtStr)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrSessionInvalid
	}
	if err != nil {
		return err
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, expiresAtStr)
	if err != nil || time.Now().After(expiresAt) {
		return ErrSessionInvalid
	}
	return nil
}

func (a *Auth) DeleteSession(token string) {
	a.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
