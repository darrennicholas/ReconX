package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"

	"reconx/backend/internal/modules/identity"
	"reconx/backend/internal/platform/config"
	"reconx/backend/internal/platform/database"
)

func main() {
	reset := flag.Bool("reset-bootstrap-password", false, "reset the configured bootstrap account password (development only)")
	flag.Parse()
	_ = godotenv.Load("../.env", ".env")
	cfg := config.Load()
	if cfg.Environment != "development" && (os.Getenv("BOOTSTRAP_ADMIN_PASSWORD") == "developer" || *reset) {
		slog.Error("development seed password and reset are restricted to development")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if *reset {
		email := os.Getenv("BOOTSTRAP_ADMIN_EMAIL")
		hash, err := bcrypt.GenerateFromPassword([]byte("developer"), bcrypt.DefaultCost)
		if err != nil {
			slog.Error("hash password", "error", err)
			os.Exit(1)
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			slog.Error("begin reset", "error", err)
			os.Exit(1)
		}
		defer tx.Rollback()
		result, err := tx.ExecContext(ctx, `UPDATE users SET password_hash=$1 WHERE lower(email)=lower($2)`, string(hash), email)
		if err != nil {
			slog.Error("reset password", "error", err)
			os.Exit(1)
		}
		count, err := result.RowsAffected()
		if err != nil || count != 1 {
			slog.Error("bootstrap account not found")
			os.Exit(1)
		}
		if _, err = tx.ExecContext(ctx, `UPDATE sessions SET revoked_at=now() WHERE user_id=(SELECT id FROM users WHERE lower(email)=lower($1)) AND revoked_at IS NULL`, email); err != nil {
			slog.Error("revoke sessions", "error", err)
			os.Exit(1)
		}
		if err = tx.Commit(); err != nil {
			slog.Error("commit reset", "error", err)
			os.Exit(1)
		}
		slog.Info("bootstrap account password reset")
		return
	}
	if err := identity.SeedBootstrapAdmin(ctx, db, os.Getenv("BOOTSTRAP_ADMIN_NAME"), os.Getenv("BOOTSTRAP_ADMIN_EMAIL"), os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")); err != nil {
		slog.Error("seed bootstrap admin", "error", err)
		os.Exit(1)
	}
	if cfg.Environment == "development" {
		if err := seedDevelopment(ctx, db, os.Getenv("BOOTSTRAP_ADMIN_EMAIL"), os.Getenv("IMPORT_STORAGE_DIR")); err != nil {
			slog.Error("seed development data", "error", err)
			os.Exit(1)
		}
	}
	slog.Info("bootstrap admin created if user table was empty")
}
