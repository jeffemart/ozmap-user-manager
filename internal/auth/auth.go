package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"github.com/example/ozmap-user-manager/internal/security"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Service struct{ user, hash, secret string }

func NewService(u, h, s string) *Service   { return &Service{u, h, s} }
func (s *Service) Verify(u, p string) bool { return u == s.user && security.VerifyArgon2id(p, s.hash) }

type Session struct {
	CSRF    string
	Expires time.Time
}
type SessionStore struct {
	secret   string
	mu       sync.RWMutex
	sessions map[string]Session
}

func NewSessionStore(secret string) *SessionStore {
	return &SessionStore{secret: secret, sessions: map[string]Session{}}
}
func (s *SessionStore) Create() (string, Session, error) {
	id, e := security.Random(32)
	if e != nil {
		return "", Session{}, e
	}
	csrf, e := security.Random(32)
	if e != nil {
		return "", Session{}, e
	}
	x := Session{csrf, time.Now().Add(8 * time.Hour)}
	s.mu.Lock()
	s.sessions[id] = x
	s.mu.Unlock()
	return id, x, nil
}
func (s *SessionStore) Valid(r *http.Request) (Session, bool) {
	c, e := r.Cookie("oz_session")
	if e != nil {
		return Session{}, false
	}
	s.mu.RLock()
	x, ok := s.sessions[c.Value]
	s.mu.RUnlock()
	return x, ok && time.Now().Before(x.Expires)
}
func (s *SessionStore) Delete(r *http.Request) {
	if c, e := r.Cookie("oz_session"); e == nil {
		s.mu.Lock()
		delete(s.sessions, c.Value)
		s.mu.Unlock()
	}
}
func (s *SessionStore) SetCookie(w http.ResponseWriter, id string, secure bool) {
	http.SetCookie(w, &http.Cookie{Name: "oz_session", Value: id, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: 8 * 3600})
}
func (s *SessionStore) ClearCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{Name: "oz_session", Value: "", Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
}
func (s *SessionStore) SignCSRF(v string) string {
	h := hmac.New(sha256.New, []byte(s.secret))
	h.Write([]byte(v))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
func (s *SessionStore) CSRFToken(id string) string { return s.SignCSRF(id) }
func (s *SessionStore) CheckCSRF(r *http.Request) bool {
	c, e := r.Cookie("oz_session")
	if e != nil {
		return false
	}
	got := r.Header.Get("X-CSRF-Token")
	if got == "" {
		got = r.FormValue("csrf_token")
	}
	return hmac.Equal([]byte(got), []byte(s.SignCSRF(c.Value)))
}
func IsSecureRequest(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}
