package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/IsaacJootar/kladd/backend/internal/config"
	"github.com/IsaacJootar/kladd/backend/internal/database"
	"github.com/IsaacJootar/kladd/backend/internal/migrations"
)

func main() {
	migrationsDir := flag.String("dir", "migrations", "directory containing SQL migration files")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.FromEnv()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	applied, err := migrations.NewRunner(db, *migrationsDir).Apply(ctx)
	if err != nil {
		logger.Error("apply migrations", "error", err)
		os.Exit(1)
	}

	if len(applied) == 0 {
		logger.Info("database already up to date")
		return
	}

	for _, migration := range applied {
		logger.Info("applied migration", "id", migration.ID, "applied_at", migration.AppliedAt)
	}
}
