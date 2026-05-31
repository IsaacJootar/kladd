package config

import "os"

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	JWTSecret   string
	StorageDir  string
}

func FromEnv() Config {
	return Config{
		HTTPAddr:    envOrDefault("KLADD_HTTP_ADDR", ":8080"),
		DatabaseURL: envOrDefault("KLADD_DATABASE_URL", "postgres://kladd:kladd_local_password@localhost:5432/kladd?sslmode=disable"),
		JWTSecret:   envOrDefault("KLADD_JWT_SECRET", "local-dev-change-me"),
		StorageDir:  envOrDefault("KLADD_STORAGE_DIR", "storage"),
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
