package services

// contains checks if the haystack contains the needle (case-insensitive)
func contains(haystack, needle string) bool {
	h := []rune(haystack)
	n := []rune(needle)

	if len(n) == 0 {
		return true
	}
	if len(h) < len(n) {
		return false
	}

	// Simple case-insensitive search
	for i := 0; i <= len(h)-len(n); i++ {
		match := true
		for j := 0; j < len(n); j++ {
			if toLower(h[i+j]) != toLower(n[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// toLower converts a rune to lowercase (simplified ASCII version)
func toLower(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	return r
}
