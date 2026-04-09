package auth

import (
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
)

const (
	sessionName = "gtra_session"
	ctxKeyUser  = "auth_user"
)

type SessionManager struct {
	store *sessions.CookieStore
}

func NewSessionManager(cookieKeyB64 string, dev bool) (*SessionManager, error) {
	key, err := base64.StdEncoding.DecodeString(cookieKeyB64)
	if err != nil {
		return nil, err
	}
	if len(key) < 32 {
		return nil, errors.New("cookie key must be >= 32 bytes after base64 decode")
	}

	cs := sessions.NewCookieStore(key)
	cs.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   int((14 * 24 * time.Hour).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   !dev,
	}
	return &SessionManager{store: cs}, nil
}

func (s *SessionManager) Close() error { return nil }

func (s *SessionManager) Get(r *http.Request) (*sessions.Session, error) {
	return s.store.Get(r, sessionName)
}

func (s *SessionManager) Save(r *http.Request, w http.ResponseWriter, sess *sessions.Session) error {
	return sess.Save(r, w)
}

