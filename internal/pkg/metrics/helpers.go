package metrics

import (
	"strings"
	"time"
)

// RecordDBOperation records database operation metrics consistently
// repo: repository name (e.g., "wiki_page", "note", "quote")
// operation: operation name (e.g., "create", "get", "update", "delete", "list", "search")
// duration: time taken for the operation
// rowsAffected: number of rows affected/returned (-1 if not applicable)
// err: error from the operation (nil if successful)
func RecordDBOperation(repo, operation string, duration time.Duration, rowsAffected int64, err error) {
	ms := float64(duration.Milliseconds())
	DBDuration.WithLabelValues(repo, operation).Observe(ms)

	if rowsAffected >= 0 {
		DBRowsAffected.WithLabelValues(repo, operation).Observe(float64(rowsAffected))
	}

	status := "success"
	if err != nil {
		status = "error"
		DBErrors.WithLabelValues(repo, operation, classifyDBError(err)).Inc()
	}
	DBOperations.WithLabelValues(repo, operation, status).Inc()
}

// classifyDBError categorizes database errors for metrics
func classifyDBError(err error) string {
	if err == nil {
		return "none"
	}

	errStr := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errStr, "duplicate") || strings.Contains(errStr, "unique constraint"):
		return "duplicate"
	case strings.Contains(errStr, "not found") || strings.Contains(errStr, "no rows"):
		return "not_found"
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline"):
		return "timeout"
	case strings.Contains(errStr, "connection") || strings.Contains(errStr, "connect"):
		return "connection"
	case strings.Contains(errStr, "foreign key") || strings.Contains(errStr, "fk_"):
		return "foreign_key"
	case strings.Contains(errStr, "constraint"):
		return "constraint"
	case strings.Contains(errStr, "deadlock"):
		return "deadlock"
	case strings.Contains(errStr, "syntax"):
		return "syntax"
	default:
		return "other"
	}
}
