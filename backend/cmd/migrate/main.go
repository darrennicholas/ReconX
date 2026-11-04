package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"

	"reconx/backend/internal/platform/config"
	"reconx/backend/internal/platform/database"
	"reconx/backend/internal/platform/migrate"
)

func main() {
	_ = godotenv.Load("../.env", ".env")
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := migrate.Apply(ctx, db); err != nil {
		slog.Error("apply migrations", "error", err)
		os.Exit(1)
	}
	slog.Info("database migrations applied")
}
