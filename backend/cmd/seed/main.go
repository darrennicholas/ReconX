package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"

	"reconx/backend/internal/modules/identity"
	"reconx/backend/internal/platform/config"
	"reconx/backend/internal/platform/database"
)

func main() {
	_ = godotenv.Load("../.env", ".env")
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := identity.SeedBootstrapAdmin(ctx, db, os.Getenv("BOOTSTRAP_ADMIN_NAME"), os.Getenv("BOOTSTRAP_ADMIN_EMAIL"), os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")); err != nil {
		slog.Error("seed bootstrap admin", "error", err)
		os.Exit(1)
	}
	slog.Info("bootstrap admin created if user table was empty")
}
