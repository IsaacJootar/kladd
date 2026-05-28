package config

import "os"

type Config struct {
	HTTPAddr    string
	DatabaseURL string
}

func FromEnv() Config {
	return Config{
		HTTPAddr:    envOrDefault("KLADD_HTTP_ADDR", ":8080"),
		DatabaseURL: envOrDefault("KLADD_DATABASE_URL", "postgres://kladd:kladd_local_password@localhost:5432/kladd?sslmode=disable"),
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
