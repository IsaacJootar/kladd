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
	"github.com/IsaacJootar/kladd/backend/internal/orgauth"
)

func main() {
	organizationName := flag.String("organization", "", "organization name")
	organizationType := flag.String("type", "organization", "organization type")
	keyName := flag.String("name", "Local setup", "api key name")
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

	service := orgauth.NewService(orgauth.NewPostgresStore(db))
	issued, err := service.IssueAPIKey(ctx, orgauth.IssueInput{
		OrganizationName: *organizationName,
		OrganizationType: *organizationType,
		KeyName:          *keyName,
	})
	if err != nil {
		logger.Error("issue organization api key", "error", err)
		os.Exit(1)
	}

	if err := json.NewEncoder(os.Stdout).Encode(issued); err != nil {
		logger.Error("write organization api key", "error", err)
		os.Exit(1)
	}
}
