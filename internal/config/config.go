package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
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
	// OCR (EPIC V2.1): binary Tesseract dipanggil sebagai subprocess.
	TesseractBin  string
	TesseractLang string
	OcrTimeout    time.Duration

	// Habit Engine (EPIC V2.2): event bus + aturan streak.
	EventBus        string // "memory" (default) | "redis"
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	HabitStreakDays int
	HabitReward     int64

	// Swagger UI dinonaktifkan di production (temuan pentest).
	SwaggerEnabled bool

	// Rate limiting per IP per menit; 0 = nonaktif (untuk load test).
	RateLimitLoginPerMin    int
	RateLimitRegisterPerMin int
	RateLimitOcrPerMin      int
}

// Load membaca file .env (best-effort) lalu mengekstrak variabel lingkungan.
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		GinMode:       getEnv("GIN_MODE", "debug"),
		TesseractBin:  getEnv("TESSERACT_BIN", "tesseract"),
		TesseractLang: getEnv("TESSERACT_LANG", "eng"),
	}

	ocrt, err := time.ParseDuration(getEnv("OCR_TIMEOUT", "30s"))
	if err != nil {
		return nil, fmt.Errorf("OCR_TIMEOUT tidak valid (contoh: 30s): %w", err)
	}
	cfg.OcrTimeout = ocrt

	cfg.EventBus = getEnv("EVENT_BUS", "memory")
	if cfg.EventBus != "memory" && cfg.EventBus != "redis" {
		return nil, fmt.Errorf("EVENT_BUS harus \"memory\" atau \"redis\"")
	}
	cfg.RedisAddr = getEnv("REDIS_ADDR", "localhost:6379")
	cfg.RedisPassword = os.Getenv("REDIS_PASSWORD")
	cfg.RedisDB, err = strconv.Atoi(getEnv("REDIS_DB", "0"))
	if err != nil {
		return nil, fmt.Errorf("REDIS_DB harus angka: %w", err)
	}
	cfg.HabitStreakDays, err = strconv.Atoi(getEnv("HABIT_STREAK_DAYS", "3"))
	if err != nil || cfg.HabitStreakDays < 1 {
		return nil, fmt.Errorf("HABIT_STREAK_DAYS harus angka >= 1")
	}
	cfg.HabitReward, err = strconv.ParseInt(getEnv("HABIT_REWARD_AMOUNT", "10000"), 10, 64)
	if err != nil || cfg.HabitReward <= 0 {
		return nil, fmt.Errorf("HABIT_REWARD_AMOUNT harus angka > 0")
	}
	cfg.SwaggerEnabled = getEnv("SWAGGER_ENABLED", "true") == "true"

	cfg.RateLimitLoginPerMin, err = strconv.Atoi(getEnv("RATE_LIMIT_LOGIN_PER_MIN", "12"))
	if err != nil || cfg.RateLimitLoginPerMin < 0 {
		return nil, fmt.Errorf("RATE_LIMIT_LOGIN_PER_MIN harus angka >= 0")
	}
	cfg.RateLimitRegisterPerMin, err = strconv.Atoi(getEnv("RATE_LIMIT_REGISTER_PER_MIN", "5"))
	if err != nil || cfg.RateLimitRegisterPerMin < 0 {
		return nil, fmt.Errorf("RATE_LIMIT_REGISTER_PER_MIN harus angka >= 0")
	}
	cfg.RateLimitOcrPerMin, err = strconv.Atoi(getEnv("RATE_LIMIT_OCR_PER_MIN", "10"))
	if err != nil || cfg.RateLimitOcrPerMin < 0 {
		return nil, fmt.Errorf("RATE_LIMIT_OCR_PER_MIN harus angka >= 0")
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
