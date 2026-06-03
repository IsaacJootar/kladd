package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/IsaacJootar/kladd/backend/internal/config"
	"github.com/IsaacJootar/kladd/backend/internal/database"
	"github.com/IsaacJootar/kladd/backend/internal/webhooks"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	cfg := config.FromEnv()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	store := webhooks.NewPostgresStore(db)
	sender := webhooks.NewHTTPSender(&http.Client{Timeout: 10 * time.Second})
	service := webhooks.NewDeliveryService(store, sender)

	summary, err := service.DeliverPending(ctx)
	if err != nil {
		logger.Error("deliver pending webhooks", "error", err)
		os.Exit(1)
	}

	if err := json.NewEncoder(os.Stdout).Encode(summary); err != nil {
		logger.Error("write webhook delivery summary", "error", err)
		os.Exit(1)
	}
}
