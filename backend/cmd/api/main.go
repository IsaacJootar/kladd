package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/IsaacJootar/kladd/backend/internal/audit"
	"github.com/IsaacJootar/kladd/backend/internal/auth"
	"github.com/IsaacJootar/kladd/backend/internal/claimrequests"
	"github.com/IsaacJootar/kladd/backend/internal/claims"
	"github.com/IsaacJootar/kladd/backend/internal/config"
	"github.com/IsaacJootar/kladd/backend/internal/database"
	"github.com/IsaacJootar/kladd/backend/internal/evidence"
	"github.com/IsaacJootar/kladd/backend/internal/securitypin"
	"github.com/IsaacJootar/kladd/backend/internal/server"
	"github.com/IsaacJootar/kladd/backend/internal/truths"
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
	pinStore := securitypin.NewPostgresStore(db)
	pinService := securitypin.NewSetupService(pinStore)
	pinResetService := securitypin.NewResetService(pinStore)
	pinValidationService := securitypin.NewValidationService(pinStore)
	authStore := auth.NewPostgresStore(db)
	authService := auth.NewService(authStore, auth.NewTokenManager(cfg.JWTSecret, auth.DefaultTokenTTL))
	evidenceStore := evidence.NewPostgresStore(db)
	evidenceStorage := evidence.NewLocalStorage(cfg.StorageDir)
	evidenceService := evidence.NewService(evidenceStore, evidenceStorage)
	auditStore := audit.NewPostgresStore(db)
	auditService := audit.NewService(auditStore)
	truthStore := truths.NewPostgresStore(db)
	truthService := truths.NewService(truthStore)
	claimRequestStore := claimrequests.NewPostgresStore(db)
	claimRequestService := claimrequests.NewService(claimRequestStore, pinValidationService)
	claimStore := claims.NewPostgresStore(db)
	claimService := claims.NewService(claimStore)

	apiServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.NewRouter(cfg, userService, userService, pinService, pinResetService, authService, evidenceService, auditService, truthService, claimRequestService, claimService),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("starting kladd api", "addr", cfg.HTTPAddr)

	if err := apiServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("api server stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}
