package timeutil

import (
	"time"
)

// GetUserLocalDate returns today's date in the user's timezone
func GetUserLocalDate(timezone string) string {
	if timezone == "" {
		// Fallback to UTC if no timezone specified
		return time.Now().UTC().Format("2006-01-02")
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// If timezone is invalid, fallback to UTC
		return time.Now().UTC().Format("2006-01-02")
	}

	return time.Now().In(loc).Format("2006-01-02")
}

// GetUserLocalTime returns the current time in the user's timezone
func GetUserLocalTime(timezone string) time.Time {
	if timezone == "" {
		// Fallback to UTC if no timezone specified
		return time.Now().UTC()
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// If timezone is invalid, fallback to UTC
		return time.Now().UTC()
	}

	return time.Now().In(loc)
}

// GetUserLocalDateTime returns the current date and time in the user's timezone
// If timezone is empty or invalid, falls back to UTC
func GetUserLocalDateTime(timezone string) time.Time {
	if timezone == "" {
		return time.Now().UTC()
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// Invalid timezone, fallback to UTC
		return time.Now().UTC()
	}

	return time.Now().In(loc)
}

// ParseDateInUserTimezone parses a YYYY-MM-DD date string as if it were in the user's timezone
// Returns the time at the start of that day in the user's timezone
func ParseDateInUserTimezone(dateStr, timezone string) (time.Time, error) {
	if timezone == "" {
		return time.Parse("2006-01-02", dateStr)
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// Invalid timezone, fallback to UTC
		return time.Parse("2006-01-02", dateStr)
	}

	return time.ParseInLocation("2006-01-02", dateStr, loc)
}

// ConvertToUserTimezone converts a UTC time to the user's timezone
func ConvertToUserTimezone(utcTime time.Time, timezone string) time.Time {
	if timezone == "" {
		return utcTime
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// Invalid timezone, return as-is
		return utcTime
	}

	return utcTime.In(loc)
}

// IsValidTimezone checks if a timezone string is valid
func IsValidTimezone(timezone string) bool {
	if timezone == "" {
		return false
	}
	_, err := time.LoadLocation(timezone)
	return err == nil
}
