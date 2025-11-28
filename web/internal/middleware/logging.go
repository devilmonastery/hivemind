package middleware

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/devilmonastery/hivemind/internal/auth"
)

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// LogRequest logs HTTP requests in structured JSON format for Kubernetes
func LogRequest(next http.Handler) http.Handler {
	logger := json.NewEncoder(os.Stdout)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip logging health checks and static files to reduce noise
		if r.URL.Path == "/health" || isStaticFile(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     200, // default if WriteHeader not called
		}

		// Process request
		next.ServeHTTP(wrapped, r)

		// Calculate duration
		duration := time.Since(start)

		// Extract user info if authenticated
		userID := ""
		username := ""
		if userCtx, err := auth.GetUserFromContext(r.Context()); err == nil {
			userID = userCtx.UserID
			username = userCtx.Username
		}

		// Get real IP (consider X-Forwarded-For if behind proxy)
		clientIP := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			clientIP = forwarded
		} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
			clientIP = realIP
		}

		// Build log entry
		logEntry := map[string]interface{}{
			"timestamp":   start.UTC().Format(time.RFC3339Nano),
			"method":      r.Method,
			"path":        r.URL.Path,
			"query":       r.URL.RawQuery,
			"status":      wrapped.statusCode,
			"duration_ms": duration.Milliseconds(),
			"bytes":       wrapped.written,
			"client_ip":   clientIP,
			"user_agent":  r.UserAgent(),
			"proto":       r.Proto,
		}

		// Add user info if authenticated
		if userID != "" {
			logEntry["user_id"] = userID
			logEntry["username"] = username
		}

		// Add error flag for failed requests
		if wrapped.statusCode >= 400 {
			logEntry["error"] = true
		}

		// Write JSON log entry to stdout
		logger.Encode(logEntry)
	})
}

// isStaticFile checks if the path is a static file request
func isStaticFile(path string) bool {
	return len(path) > 8 && path[:8] == "/static/"
}
