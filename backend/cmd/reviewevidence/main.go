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
	"github.com/IsaacJootar/kladd/backend/internal/evidencereview"
	"github.com/google/uuid"
)

func main() {
	userEmail := flag.String("user-email", "", "user email that owns the evidence")
	evidenceIDValue := flag.String("evidence-id", "", "evidence item id")
	status := flag.String("status", "", "review status: verified or rejected")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	cfg := config.FromEnv()

	evidenceID, err := uuid.Parse(*evidenceIDValue)
	if err != nil {
		logger.Error("parse evidence id", "error", evidencereview.ErrInvalidEvidenceID)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	service := evidencereview.NewService(evidencereview.NewPostgresStore(db))
	result, err := service.Review(ctx, evidencereview.ReviewInput{
		UserEmail:  *userEmail,
		EvidenceID: evidenceID,
		Status:     *status,
	})
	if err != nil {
		logger.Error("review evidence", "error", err)
		os.Exit(1)
	}

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		logger.Error("write evidence review result", "error", err)
		os.Exit(1)
	}
}
