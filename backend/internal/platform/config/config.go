package config

import "os"

type Config struct {
	Environment string
	APIAddr     string
	DatabaseURL string
}

func Load() Config {
	return Config{
		Environment: valueOrDefault("APP_ENV", "development"),
		APIAddr:     valueOrDefault("API_ADDR", ":8080"),
		DatabaseURL: valueOrDefault("DATABASE_URL", "postgres://reconx:reconx@localhost:5432/reconx?sslmode=disable"),
	}
}

func valueOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
