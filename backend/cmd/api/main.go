package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/IsaacJootar/kladd/backend/internal/config"
	"github.com/IsaacJootar/kladd/backend/internal/database"
	"github.com/IsaacJootar/kladd/backend/internal/server"
	"github.com/IsaacJootar/kladd/backend/internal/users"
)

func main() {
	cfg := config.FromEnv()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	db, err := database.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	userStore := users.NewPostgresStore(db)
	userService := users.NewService(userStore)

	apiServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.NewRouter(cfg, userService),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("starting kladd api", "addr", cfg.HTTPAddr)

	if err := apiServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("api server stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}
