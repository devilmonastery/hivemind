package grpc

// StaticTokenManager implements a simple token manager for service account tokens
// It returns the same static token without any refresh logic
type StaticTokenManager struct {
	token string
}

// NewStaticTokenManager creates a token manager with a static service account token
func NewStaticTokenManager(token string) *StaticTokenManager {
	return &StaticTokenManager{
		token: token,
	}
}

// GetToken returns the static service account token
func (s *StaticTokenManager) GetToken() (string, error) {
	return s.token, nil
}

// GetTokenID returns empty string for static tokens (no refresh needed)
func (s *StaticTokenManager) GetTokenID() (string, error) {
	return "", nil
}

// SaveToken is a no-op for static tokens (they don't change)
func (s *StaticTokenManager) SaveToken(token, tokenID string) error {
	return nil
}

// ClearToken is a no-op for static tokens (they're not persisted)
func (s *StaticTokenManager) ClearToken() error {
	return nil
}
