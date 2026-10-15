package config

import (
	"fmt"
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

	JWTSecret           string
	AccessTokenExpiry   time.Duration
	RefreshTokenExpiry  time.Duration
	GoogleOAuthClientID string

	AllowedOrigins []string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	// Cloud Run supplies PORT. Local development may override it in .env.
	port := getEnv("PORT", "10000")
	env := getEnv("APP_ENV", "development")
	dbURL := os.Getenv("DATABASE_URL")
	jwtSecret := os.Getenv("JWT_SECRET")

	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL must be configured")
	}
	if len(jwtSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}

	accessExpiryHours, _ := strconv.Atoi(getEnv("JWT_ACCESS_EXPIRY_HOURS", "24"))
	refreshExpiryDays, _ := strconv.Atoi(getEnv("JWT_REFRESH_EXPIRY_DAYS", "30"))

	defaultOrigins := []string{
		"https://whisperledger.in",
		"https://www.whisperledger.in",
		"https://admin.whisperledger.in",
	}
	if env != "production" {
		defaultOrigins = append(defaultOrigins,
			"http://localhost:3000",
			"http://localhost:3003",
			"http://localhost:5173",
			"http://localhost:5174",
		)
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
		Environment:         env,
		Port:                port,
		DatabaseURL:         dbURL,
		JWTSecret:           jwtSecret,
		AccessTokenExpiry:   time.Duration(accessExpiryHours) * time.Hour,
		RefreshTokenExpiry:  time.Duration(refreshExpiryDays) * 24 * time.Hour,
		GoogleOAuthClientID: getEnv("GOOGLE_OAUTH_CLIENT_ID", "1064792050841-amo41uudlpvmuoou362ijcktfe198hdr.apps.googleusercontent.com"),
		AllowedOrigins:      defaultOrigins,
	}, nil
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}
