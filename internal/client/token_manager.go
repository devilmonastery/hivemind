package client

// TokenManager is an interface for managing authentication tokens
// Different implementations can store tokens in files, sessions, databases, etc.
type TokenManager interface {
	// GetToken returns the current access token
	GetToken() (token string, err error)

	// GetTokenID returns the token ID used for refresh
	GetTokenID() (tokenID string, err error)

	// SaveToken stores both the access token and token ID
	SaveToken(token, tokenID string) error

	// ClearToken removes stored credentials
	ClearToken() error
}
