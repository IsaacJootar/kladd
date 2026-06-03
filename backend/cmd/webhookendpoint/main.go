package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/IsaacJootar/kladd/backend/internal/config"
	"github.com/IsaacJootar/kladd/backend/internal/database"
	"github.com/IsaacJootar/kladd/backend/internal/webhooks"
)

func main() {
	organizationName := flag.String("organization", "", "organization name")
	organizationType := flag.String("type", "organization", "organization type")
	endpointURL := flag.String("url", "", "webhook endpoint url")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	cfg := config.FromEnv()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	service := webhooks.NewEndpointService(webhooks.NewPostgresStore(db))
	endpoint, err := service.ConfigureEndpoint(ctx, webhooks.ConfigureEndpointInput{
		OrganizationName: *organizationName,
		OrganizationType: *organizationType,
		URL:              *endpointURL,
	})
	if err != nil {
		logger.Error("configure webhook endpoint", "error", err)
		os.Exit(1)
	}

	if err := json.NewEncoder(os.Stdout).Encode(endpoint); err != nil {
		logger.Error("write webhook endpoint", "error", err)
		os.Exit(1)
	}
}
