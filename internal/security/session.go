package security

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"
	"time"
)

const SessionCookieName = "diswatch_session"

type SessionManager struct {
	mu       sync.Mutex
	ttl      time.Duration
	sessions map[string]time.Time
}

func NewSessionManager(ttl time.Duration) *SessionManager {
	return &SessionManager{
		ttl:      ttl,
		sessions: make(map[string]time.Time),
	}
}

func (m *SessionManager) Create(w http.ResponseWriter, r *http.Request) error {
	id, err := randomSessionID()
	if err != nil {
		return err
	}
	expires := time.Now().Add(m.ttl)
	m.mu.Lock()
	m.cleanupLocked(time.Now())
	m.sessions[id] = expires
	m.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    id,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(m.ttl.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   isHTTPS(r),
	})
	return nil
}

func (m *SessionManager) Valid(r *http.Request) bool {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupLocked(now)
	expires, ok := m.sessions[cookie.Value]
	if !ok || now.After(expires) {
		delete(m.sessions, cookie.Value)
		return false
	}
	m.sessions[cookie.Value] = now.Add(m.ttl)
	return true
}

func (m *SessionManager) Destroy(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(SessionCookieName); err == nil {
		m.mu.Lock()
		delete(m.sessions, cookie.Value)
		m.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   isHTTPS(r),
	})
}

func (m *SessionManager) cleanupLocked(now time.Time) {
	for id, expires := range m.sessions {
		if now.After(expires) {
			delete(m.sessions, id)
		}
	}
}

func randomSessionID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}
