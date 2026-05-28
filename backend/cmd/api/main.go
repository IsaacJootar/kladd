package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/IsaacJootar/kladd/backend/internal/config"
	"github.com/IsaacJootar/kladd/backend/internal/server"
)

func main() {
	cfg := config.FromEnv()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	apiServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.NewRouter(cfg),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("starting kladd api", "addr", cfg.HTTPAddr)

	if err := apiServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("api server stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}
