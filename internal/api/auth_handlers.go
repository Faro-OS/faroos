package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/faroos/faroos/internal/auth"
)

const sessionCookieName = "faroos_session"

type credentialsReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	authenticated := false
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		authenticated = s.authSvc.ValidateSession(cookie.Value) == nil
	}
	writeJSON(w, map[string]bool{
		"needsSetup":    s.authSvc.NeedsSetup(),
		"authenticated": authenticated,
	})
}

func (s *Server) handleAuthSetup(w http.ResponseWriter, r *http.Request) {
	var req credentialsReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" || len(req.Password) < 8 {
		http.Error(w, "username and a password of at least 8 characters are required", http.StatusBadRequest)
		return
	}
	token, expiresAt, err := s.authSvc.CreateAdminSession(req.Username, req.Password)
	if err != nil {
		if err == auth.ErrAlreadySetUp {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, "failed to create admin account", http.StatusInternalServerError)
		return
	}
	s.setSession(w, token, expiresAt)
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	var req credentialsReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := s.authSvc.VerifyLogin(req.Username, req.Password); err != nil {
		http.Error(w, "invalid username or password", http.StatusUnauthorized)
		return
	}
	s.startSession(w, req.Username, req.Password)
}

// startSession issues a session cookie after login/setup has already
// verified the credentials (setup trivially "verifies" by having just set
// them).
func (s *Server) startSession(w http.ResponseWriter, _, _ string) {
	token, expiresAt, err := s.authSvc.CreateSession()
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}
	s.setSession(w, token, expiresAt)
}

func (s *Server) setSession(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		s.authSvc.DeleteSession(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, map[string]bool{"ok": true})
}

// requireAuth guards handlers that manage nodes — only an authenticated
// admin should be able to pair/list/inspect servers.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || s.authSvc.ValidateSession(cookie.Value) != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
