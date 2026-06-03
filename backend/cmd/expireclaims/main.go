package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"time"

	"github.com/IsaacJootar/kladd/backend/internal/claims"
	"github.com/IsaacJootar/kladd/backend/internal/config"
	"github.com/IsaacJootar/kladd/backend/internal/database"
)

type result struct {
	ExpiredCount int      `json:"expired_count"`
	ClaimIDs     []string `json:"claim_ids"`
}

func main() {
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

	service := claims.NewService(claims.NewPostgresStore(db, cfg.WebhookSigningSecret))
	expiredClaims, err := service.ExpireDue(ctx)
	if err != nil {
		logger.Error("expire due claims", "error", err)
		os.Exit(1)
	}

	output := result{
		ExpiredCount: len(expiredClaims),
		ClaimIDs:     make([]string, 0, len(expiredClaims)),
	}
	for _, claim := range expiredClaims {
		output.ClaimIDs = append(output.ClaimIDs, claim.ID.String())
	}

	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		logger.Error("write expiry result", "error", err)
		os.Exit(1)
	}
}
