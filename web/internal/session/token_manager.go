package session

import (
	"net/http"

	"github.com/devilmonastery/hivemind/internal/client"
)

// SessionTokenManager implements TokenManager using HTTP session storage
type SessionTokenManager struct {
	manager *Manager
	request *http.Request
	writer  http.ResponseWriter
}

// NewSessionTokenManager creates a new session-based token manager
// Note: This must be created per-request since it needs access to the request/response
func NewSessionTokenManager(manager *Manager, r *http.Request, w http.ResponseWriter) client.TokenManager {
	return &SessionTokenManager{
		manager: manager,
		request: r,
		writer:  w,
	}
}

// GetToken returns the current access token from session
func (s *SessionTokenManager) GetToken() (string, error) {
	return s.manager.GetToken(s.request)
}

// GetTokenID returns the token ID from session
func (s *SessionTokenManager) GetTokenID() (string, error) {
	return s.manager.GetTokenID(s.request)
}

// SaveToken saves the token and token ID to session
func (s *SessionTokenManager) SaveToken(token, tokenID string) error {
	return s.manager.SetToken(s.request, s.writer, token, tokenID)
}

// ClearToken removes the credentials from session
func (s *SessionTokenManager) ClearToken() error {
	s.manager.ClearToken(s.request, s.writer)
	return nil
}
