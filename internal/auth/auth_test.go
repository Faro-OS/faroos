package auth

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSetupAndLoginFlow(t *testing.T) {
	a, err := New(newTestDB(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if !a.NeedsSetup() {
		t.Fatal("expected NeedsSetup to be true before any admin is created")
	}

	if err := a.CreateAdmin("gonzalo", "correcthorsebattery"); err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}

	if a.NeedsSetup() {
		t.Fatal("expected NeedsSetup to be false after creating an admin")
	}

	if err := a.CreateAdmin("someoneelse", "irrelevant"); err != ErrAlreadySetUp {
		t.Fatalf("expected ErrAlreadySetUp on a second CreateAdmin, got %v", err)
	}

	if err := a.VerifyLogin("gonzalo", "correcthorsebattery"); err != nil {
		t.Fatalf("VerifyLogin with correct credentials failed: %v", err)
	}
	if err := a.VerifyLogin("gonzalo", "wrongpassword"); err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials for wrong password, got %v", err)
	}
	if err := a.VerifyLogin("nobody", "correcthorsebattery"); err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials for wrong username, got %v", err)
	}
}

func TestVerifyLoginBeforeSetup(t *testing.T) {
	a, err := New(newTestDB(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := a.VerifyLogin("anyone", "anything"); err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials before setup, got %v", err)
	}
}

func TestSessionLifecycle(t *testing.T) {
	a, err := New(newTestDB(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := a.ValidateSession(""); err != ErrSessionInvalid {
		t.Fatalf("expected ErrSessionInvalid for empty token, got %v", err)
	}
	if err := a.ValidateSession("does-not-exist"); err != ErrSessionInvalid {
		t.Fatalf("expected ErrSessionInvalid for unknown token, got %v", err)
	}

	token, expiresAt, err := a.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if token == "" {
		t.Fatal("expected a non-empty session token")
	}
	if !expiresAt.After(time.Now()) {
		t.Fatal("expected expiresAt to be in the future")
	}

	if err := a.ValidateSession(token); err != nil {
		t.Fatalf("expected freshly created session to validate, got %v", err)
	}

	a.DeleteSession(token)
	if err := a.ValidateSession(token); err != ErrSessionInvalid {
		t.Fatalf("expected ErrSessionInvalid after DeleteSession, got %v", err)
	}
}

func TestExpiredSessionIsRejected(t *testing.T) {
	db := newTestDB(t)
	a, err := New(db)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Insert an already-expired session directly, bypassing CreateSession's
	// TTL, to test the expiry check itself rather than waiting real time.
	token := "expired-token"
	past := time.Now().Add(-time.Hour).Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO sessions (token, expires_at) VALUES (?, ?)`, token, past); err != nil {
		t.Fatalf("insert expired session: %v", err)
	}

	if err := a.ValidateSession(token); err != ErrSessionInvalid {
		t.Fatalf("expected ErrSessionInvalid for expired session, got %v", err)
	}
}

func TestDifferentAdminsGetDifferentPasswordHashes(t *testing.T) {
	// Two Auth instances (simulating two separate FaroOS installs) creating
	// admins with the same password should not produce identical hashes —
	// bcrypt salts per call. Sanity check that we're not accidentally
	// storing anything predictable.
	a1, _ := New(newTestDB(t))
	a2, _ := New(newTestDB(t))
	a1.CreateAdmin("gonzalo", "samepassword123")
	a2.CreateAdmin("gonzalo", "samepassword123")

	var hash1, hash2 string
	a1.db.QueryRow(`SELECT password_hash FROM admin WHERE id = 1`).Scan(&hash1)
	a2.db.QueryRow(`SELECT password_hash FROM admin WHERE id = 1`).Scan(&hash2)
	if hash1 == "" || hash2 == "" {
		t.Fatal("expected both hashes to be populated")
	}
	if hash1 == hash2 {
		t.Fatal("expected different bcrypt salts to produce different hashes for the same password")
	}
}
