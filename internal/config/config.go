package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Config menampung seluruh konfigurasi runtime aplikasi.
type Config struct {
	Port         string
	DatabaseURL  string
	JWTSecret    string
	JWTExpiresIn time.Duration
	GinMode      string
}

// Load membaca file .env (best-effort) lalu mengekstrak variabel lingkungan.
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		GinMode:     getEnv("GIN_MODE", "debug"),
	}

	ttl, err := time.ParseDuration(getEnv("JWT_EXPIRES_IN", "72h"))
	if err != nil {
		return nil, fmt.Errorf("JWT_EXPIRES_IN tidak valid (contoh: 72h): %w", err)
	}
	cfg.JWTExpiresIn = ttl

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL wajib diisi (contoh: postgres://user:pass@localhost:5432/caupocket?sslmode=disable)")
	}
	if len(cfg.JWTSecret) < 16 {
		return nil, fmt.Errorf("JWT_SECRET wajib diisi dengan minimal 16 karakter")
	}

	return cfg, nil
}

// MustLoad sama dengan Load tetapi menghentikan aplikasi saat konfigurasi tidak valid.
func MustLoad() *Config {
	cfg, err := Load()
	if err != nil {
		log.Fatalf("konfigurasi tidak valid: %v", err)
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
