package config

import "os"

type Config struct {
	HTTPAddr             string
	DatabaseURL          string
	JWTSecret            string
	WebhookSigningSecret string
	StorageDir           string
}

func FromEnv() Config {
	jwtSecret := envOrDefault("KLADD_JWT_SECRET", "local-dev-change-me")

	return Config{
		HTTPAddr:             envOrDefault("KLADD_HTTP_ADDR", ":8080"),
		DatabaseURL:          envOrDefault("KLADD_DATABASE_URL", "postgres://kladd:kladd_local_password@localhost:5432/kladd?sslmode=disable"),
		JWTSecret:            jwtSecret,
		WebhookSigningSecret: envOrDefault("KLADD_WEBHOOK_SIGNING_SECRET", jwtSecret),
		StorageDir:           envOrDefault("KLADD_STORAGE_DIR", "storage"),
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
