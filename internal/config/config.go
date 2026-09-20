package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment string
	Port        string
	DatabaseURL string

	JWTSecret          string
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration

	AllowedOrigins []string
}

func Load() *Config {
	_ = godotenv.Load()

	// Default to 10000 for Render, or 8080 for Cloud Run/local
	port := getEnv("PORT", "10000")
	env := getEnv("APP_ENV", "development")
	dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/whisperledger?sslmode=disable")
	jwtSecret := getEnv("JWT_SECRET", "super-secret-production-whisperledger-key-2026")

	accessExpiryHours, _ := strconv.Atoi(getEnv("JWT_ACCESS_EXPIRY_HOURS", "24"))
	refreshExpiryDays, _ := strconv.Atoi(getEnv("JWT_REFRESH_EXPIRY_DAYS", "30"))

	defaultOrigins := []string{
		"http://localhost:3000",
		"http://localhost:5173",
		"https://whisperledger.com",
		"https://admin.whisperledger.com",
	}

	if corsEnv := os.Getenv("CORS_ORIGINS"); corsEnv != "" {
		for _, o := range strings.Split(corsEnv, ",") {
			trimmed := strings.TrimSpace(o)
			if trimmed != "" {
				defaultOrigins = append(defaultOrigins, trimmed)
			}
		}
	}

	if corsEnv := os.Getenv("CORS_ALLOWED_ORIGINS"); corsEnv != "" {
		for _, o := range strings.Split(corsEnv, ",") {
			trimmed := strings.TrimSpace(o)
			if trimmed != "" {
				defaultOrigins = append(defaultOrigins, trimmed)
			}
		}
	}

	return &Config{
		Environment:        env,
		Port:               port,
		DatabaseURL:        dbURL,
		JWTSecret:          jwtSecret,
		AccessTokenExpiry:  time.Duration(accessExpiryHours) * time.Hour,
		RefreshTokenExpiry: time.Duration(refreshExpiryDays) * 24 * time.Hour,
		AllowedOrigins:     defaultOrigins,
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}
