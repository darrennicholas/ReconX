package httpserver

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"reconx/backend/internal/modules/identity"
)

type response struct {
	Status string `json:"status"`
	Time   string `json:"time,omitempty"`
}

func New(db *sql.DB, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, response{Status: "ok", Time: time.Now().UTC().Format(time.RFC3339)})
	})
	mux.HandleFunc("GET /api/v1/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := db.PingContext(r.Context()); err != nil {
			logger.Warn("readiness check failed", "error", err)
			writeJSON(w, http.StatusServiceUnavailable, response{Status: "not_ready"})
			return
		}
		writeJSON(w, http.StatusOK, response{Status: "ready"})
	})
	mux.HandleFunc("GET /api/v1/meta/rbac", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"roles":           identity.RoleDefinitions(),
			"permission_keys": identity.PermissionKeys(),
			"segregation_of_duties": map[string]any{
				"checker_approves_maker": false,
				"admin_approval_bypass":  false,
			},
		})
	})
	registerFeatureRoutes(mux, db, logger)
	mux.HandleFunc("GET /api/v1/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
	})
	return requestLogger(logger, requestContext(mux))
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func requestLogger(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("HTTP request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started))
	})
}
