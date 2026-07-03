package web

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/yashikota/enbu/app"
)

var ClientSecret string

type Server struct {
	app       *app.App
	csrfToken string
	mux       *http.ServeMux
	clientID  string
}

func NewServer(a *app.App, clientID string, frontend fs.FS) *Server {
	s := &Server{
		app:       a,
		csrfToken: generateToken(),
		clientID:  clientID,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/auth/status", s.csrfMiddleware(s.handleAuthStatus))
	mux.HandleFunc("GET /api/auth/login", s.handleAuthLogin)
	mux.HandleFunc("GET /api/auth/callback", s.handleAuthCallback)
	mux.HandleFunc("POST /api/auth/logout", s.csrfMiddleware(s.handleAuthLogout))

	mux.HandleFunc("GET /api/repo", s.csrfMiddleware(s.handleRepoStatus))
	mux.HandleFunc("POST /api/init", s.csrfMiddleware(s.handleInit))

	mux.HandleFunc("GET /api/environments", s.csrfMiddleware(s.handleListEnvironments))
	mux.HandleFunc("POST /api/environments", s.csrfMiddleware(s.handleCreateEnvironment))
	mux.HandleFunc("PUT /api/environments/{name}/switch", s.csrfMiddleware(s.handleSwitchEnvironment))
	mux.HandleFunc("PUT /api/environments/{name}", s.csrfMiddleware(s.handleRenameEnvironment))
	mux.HandleFunc("DELETE /api/environments/{name}", s.csrfMiddleware(s.handleDeleteEnvironment))

	mux.HandleFunc("GET /api/secrets", s.csrfMiddleware(s.handleListSecrets))
	mux.HandleFunc("POST /api/secrets", s.csrfMiddleware(s.handleAddSecret))
	mux.HandleFunc("PUT /api/secrets/{key}", s.csrfMiddleware(s.handleEditSecret))
	mux.HandleFunc("DELETE /api/secrets/{key}", s.csrfMiddleware(s.handleDeleteSecret))
	mux.HandleFunc("POST /api/secrets/pull", s.csrfMiddleware(s.handlePullSecrets))
	mux.HandleFunc("POST /api/secrets/sync", s.csrfMiddleware(s.handleSyncSecrets))

	mux.HandleFunc("GET /api/history", s.csrfMiddleware(s.handleListHistory))
	mux.HandleFunc("GET /api/history/diff", s.csrfMiddleware(s.handleDiffHistory))
	mux.HandleFunc("POST /api/history/{index}/restore", s.csrfMiddleware(s.handleRestoreHistory))

	if frontend != nil {
		fileServer := http.FileServerFS(frontend)
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Set CSRF token cookie on every page load
			http.SetCookie(w, &http.Cookie{
				Name:     "enbu_csrf",
				Value:    s.csrfToken,
				Path:     "/",
				SameSite: http.SameSiteStrictMode,
			})

			path := strings.TrimPrefix(r.URL.Path, "/")
			if path == "" {
				fileServer.ServeHTTP(w, r)
				return
			}
			// Serve static file if it exists
			if _, err := fs.Stat(frontend, path); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
			// SPA fallback: serve index.html for client-side routes
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
		})
	}

	s.mux = mux
	return s
}

func (s *Server) CSRFToken() string {
	return s.csrfToken
}

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) csrfMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-ENBU-Token") != s.csrfToken {
			writeError(w, http.StatusForbidden, "CSRF_INVALID", "invalid or missing X-ENBU-Token header")
			return
		}
		next(w, r)
	}
}

type apiResponse struct {
	Data  any       `json:"data,omitempty"`
	Error *apiError `json:"error,omitempty"`
}

type apiError struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiResponse{Data: data})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiResponse{Error: &apiError{Message: message, Code: code}})
}

func generateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("generating random token: %v", err))
	}
	return hex.EncodeToString(b)
}
