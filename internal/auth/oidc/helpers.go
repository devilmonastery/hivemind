package oidc

// isUserAllowed checks if a user is allowed based on domain and individual user allowlists
func isUserAllowed(email, hostedDomain string, allowedDomains, allowedUsers []string) bool {
	// If no restrictions are configured, allow all users
	if len(allowedDomains) == 0 && len(allowedUsers) == 0 {
		return true
	}

	// Check individual user allowlist first (most specific)
	if len(allowedUsers) > 0 {
		for _, allowedUser := range allowedUsers {
			if email == allowedUser {
				return true
			}
		}
	}

	// Check domain allowlist
	if len(allowedDomains) > 0 {
		// Check email domain
		if isEmailInAllowedDomains(email, allowedDomains) {
			return true
		}

		// For G Suite accounts, also check hosted domain if specified
		if hostedDomain != "" && contains(allowedDomains, hostedDomain) {
			return true
		}
	}

	return false
}

func isEmailInAllowedDomains(email string, allowedDomains []string) bool {
	for _, domain := range allowedDomains {
		if len(email) > len(domain)+1 && email[len(email)-len(domain)-1:] == "@"+domain {
			return true
		}
	}
	return false
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
