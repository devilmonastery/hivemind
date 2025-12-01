package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"

	"github.com/devilmonastery/hivemind/internal/pkg/logger"
	"github.com/devilmonastery/hivemind/web/internal/config"
	"github.com/devilmonastery/hivemind/web/internal/handlers"
	"github.com/devilmonastery/hivemind/web/internal/middleware"
	"github.com/devilmonastery/hivemind/web/internal/render"
	"github.com/devilmonastery/hivemind/web/internal/session"
)

// setupWebLogging configures the global logger for the web service
func setupWebLogging(logLevel, logFormat string) error {
	cfg := logger.Config{
		Level:       logger.ParseLevel(logLevel),
		LogToStderr: true, // Web service always logs to stderr
		Format:      logFormat,
	}

	globalLogger, err := logger.SetupLogger(cfg)
	if err != nil {
		return err
	}

	// Set as default logger so all slog.Info/Warn/Error calls use our configured logger
	slog.SetDefault(globalLogger)

	return nil
}

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "path to config file")
	flag.Parse()

	// Load web configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Set up structured logging (must be done before any logging calls)
	if err = setupWebLogging(cfg.Logging.Level, cfg.Logging.Format); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to setup logging: %v\n", err)
		os.Exit(1)
	}

	log := slog.Default().With("component", "web")
	log.Info("starting hivemind web service")

	// Load templates from configured path (defaults to "web/templates")
	templates, err := render.LoadTemplates(cfg.Templates.Path)
	if err != nil {
		log.Error("failed to load templates", slog.Any("error", err))
		os.Exit(1)
	}

	// Log loaded template names for debugging
	render.LogTemplateNames(templates, slog.Default())

	// Get session secret - priority: env var > config file > random
	var sessionSecret []byte
	secretSource := ""

	// 1. Try environment variable first (best for production)
	if envSecret := os.Getenv("SESSION_SECRET"); envSecret != "" {
		sessionSecret, err = base64.StdEncoding.DecodeString(envSecret)
		if err != nil {
			log.Warn("failed to decode SESSION_SECRET env var, trying config", slog.Any("error", err))
		} else {
			secretSource = "environment variable"
		}
	}

	// 2. Try config file if env var not set or failed
	if sessionSecret == nil && cfg.Session.Secret != "" {
		sessionSecret, err = base64.StdEncoding.DecodeString(cfg.Session.Secret)
		if err != nil {
			log.Warn("failed to decode session secret from config", slog.Any("error", err))
		} else {
			secretSource = "config file"
		}
	}

	// 3. Fall back to random generation (dev mode only)
	if sessionSecret == nil {
		log.Warn("no session secret configured, generating random one (sessions won't persist)")
		sessionSecret = make([]byte, 32)
		if _, err := rand.Read(sessionSecret); err != nil {
			log.Error("failed to generate session secret", slog.Any("error", err))
			os.Exit(1)
		}
		secretSource = "random (temporary)"
	}

	if secretSource != "random (temporary)" {
		log.Info("using session secret (sessions will persist across restarts)", slog.String("source", secretSource))
	}

	// Initialize session manager
	sessionMgr := session.NewManager(sessionSecret)

	// Initialize auth middleware
	authMw := middleware.NewAuthMiddleware(sessionMgr, log)

	// Initialize handlers with server address and redirect URI from config
	log.Info("initializing handlers and waiting for backend...")
	h := handlers.New(cfg.GRPC.Address, sessionMgr, templates, cfg.OAuth.RedirectURI, log)

	// Create HTTP router
	router := createRouter(h, authMw)

	// Start HTTP server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Info("starting hivemind web service", slog.String("address", addr))

	if err := http.ListenAndServe(addr, router); err != nil {
		log.Error("failed to start server", slog.Any("error", err))
		os.Exit(1)
	}
}

// createRouter sets up the HTTP router with all routes and middleware
func createRouter(h *handlers.Handler, authMw *middleware.AuthMiddleware) http.Handler {
	router := mux.NewRouter()

	// Static files with version path: /static/{version}/...
	// Strip /static/{version}/ prefix and serve from web/static/
	staticDir := http.Dir("web/static")
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Remove version from path (format: {version}/file.ext)
		// Split path and skip the first segment (version)
		parts := strings.SplitN(r.URL.Path, "/", 2)
		if len(parts) == 2 {
			// Rewrite path without version
			r.URL.Path = "/" + parts[1]
		}
		// Set aggressive cache headers for versioned assets
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.FileServer(staticDir).ServeHTTP(w, r)
	})))

	// Health check endpoint (no auth required)
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}).Methods("GET")

	// Version info endpoint
	router.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"version":"%s"}`, render.Version)
	}).Methods("GET")

	// Public routes (no auth required)
	router.HandleFunc("/", h.Home).Methods("GET")
	router.HandleFunc("/login", h.Login).Methods("GET")
	router.HandleFunc("/auth/callback", h.AuthCallback).Methods("GET")
	router.HandleFunc("/logout", h.Logout).Methods("GET", "POST")
	router.HandleFunc("/admin/login", h.AdminLogin).Methods("POST")
	router.HandleFunc("/api/set-timezone", h.SetTimezone).Methods("POST")

	// Wiki routes (auth required)
	router.Handle("/wikis", authMw.RequireAuth(http.HandlerFunc(h.WikiListPage))).Methods("GET")
	router.Handle("/wiki", authMw.RequireAuth(http.HandlerFunc(h.WikiPage))).Methods("GET")
	router.Handle("/wiki/edit", authMw.RequireAuth(http.HandlerFunc(h.WikiEdit))).Methods("GET")
	router.Handle("/wiki/preview", authMw.RequireAuth(http.HandlerFunc(h.WikiPreview))).Methods("POST")
	router.Handle("/wiki/save", authMw.RequireAuth(http.HandlerFunc(h.WikiSave))).Methods("POST")

	// Notes routes (auth required)
	router.Handle("/notes", authMw.RequireAuth(http.HandlerFunc(h.NotesListPage))).Methods("GET")
	router.Handle("/note", authMw.RequireAuth(http.HandlerFunc(h.NotePage))).Methods("GET")
	router.Handle("/note/edit", authMw.RequireAuth(http.HandlerFunc(h.NoteEdit))).Methods("GET")
	router.Handle("/note/preview", authMw.RequireAuth(http.HandlerFunc(h.NotePreview))).Methods("POST")
	router.Handle("/note/save", authMw.RequireAuth(http.HandlerFunc(h.NoteSave))).Methods("POST")

	// Quotes routes (auth required)
	router.Handle("/quotes", authMw.RequireAuth(http.HandlerFunc(h.QuotesListPage))).Methods("GET")
	router.Handle("/quote", authMw.RequireAuth(http.HandlerFunc(h.QuotePage))).Methods("GET")
	router.Handle("/quote/edit", authMw.RequireAuth(http.HandlerFunc(h.QuoteEdit))).Methods("GET")
	router.Handle("/quote/preview", authMw.RequireAuth(http.HandlerFunc(h.QuotePreview))).Methods("POST")
	router.Handle("/quote/save", authMw.RequireAuth(http.HandlerFunc(h.QuoteSave))).Methods("POST")

	// Wrap router with logging middleware
	return middleware.LogRequest(router)
}
