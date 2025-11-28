package session

import (
	"net/http"

	"github.com/gorilla/sessions"
)

const (
	// SessionName is the name of the session cookie
	SessionName = "hivemind_session"

	// TokenKey is the session key for storing the JWT token
	TokenKey = "token"

	// TokenIDKey is the session key for storing the token ID for refresh
	TokenIDKey = "token_id"
)

// Manager wraps gorilla/sessions for our use case
type Manager struct {
	store *sessions.CookieStore
}

// NewManager creates a new session manager
// secretKey should be 32 bytes for AES-256
func NewManager(secretKey []byte) *Manager {
	store := sessions.NewCookieStore(secretKey)

	// Configure session options
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   90 * 24 * 60 * 60, // 90 days
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
	}

	return &Manager{
		store: store,
	}
}

// SetToken stores the JWT token and token ID in the session
func (m *Manager) SetToken(r *http.Request, w http.ResponseWriter, token, tokenID string) error {
	session, err := m.store.Get(r, SessionName)
	if err != nil {
		// Create new session if it doesn't exist
		session, _ = m.store.New(r, SessionName)
	}

	session.Values[TokenKey] = token
	session.Values[TokenIDKey] = tokenID
	return session.Save(r, w)
}

// GetToken retrieves the JWT token from the session
func (m *Manager) GetToken(r *http.Request) (string, error) {
	session, err := m.store.Get(r, SessionName)
	if err != nil {
		return "", err
	}

	token, ok := session.Values[TokenKey].(string)
	if !ok {
		return "", http.ErrNoCookie
	}

	return token, nil
}

// GetTokenID retrieves the token ID from the session
func (m *Manager) GetTokenID(r *http.Request) (string, error) {
	session, err := m.store.Get(r, SessionName)
	if err != nil {
		return "", err
	}

	tokenID, ok := session.Values[TokenIDKey].(string)
	if !ok {
		return "", http.ErrNoCookie
	}

	return tokenID, nil
}

// ClearToken removes the session (logout)
func (m *Manager) ClearToken(r *http.Request, w http.ResponseWriter) error {
	session, err := m.store.Get(r, SessionName)
	if err != nil {
		return nil // Session doesn't exist, nothing to clear
	}

	// Set MaxAge to -1 to delete the session
	session.Options.MaxAge = -1
	return session.Save(r, w)
}

// HasToken checks if a session token exists
func (m *Manager) HasToken(r *http.Request) bool {
	_, err := m.GetToken(r)
	return err == nil
}

// GetSession returns the session object for storing additional data
func (m *Manager) GetSession(r *http.Request) (*sessions.Session, error) {
	return m.store.Get(r, SessionName)
}
