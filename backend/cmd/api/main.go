package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"baramail/backend/internal/config"
	"baramail/backend/internal/database"
	"baramail/backend/internal/handlers"
	"baramail/backend/internal/jobs"
	"baramail/backend/internal/repositories"
	"baramail/backend/internal/services"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.Load("")
	if err != nil {
		logger.Error("config_load_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	db, err := database.Connect(cfg.DSN())
	if err != nil {
		logger.Error("db_connect_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	ctx := context.Background()
	if err := database.RunMigrations(ctx, db, "migrations"); err != nil {
		logger.Error("db_migration_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if err := database.SeedDomains(ctx, db, cfg.AllowedEmailDomains); err != nil {
		logger.Error("domain_seed_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	adminRepo := repositories.NewAdminRepository(db)
	domainRepo := repositories.NewDomainRepository(db)
	mailboxRepo := repositories.NewMailboxRepository(db)
	msgRepo := repositories.NewMessageRepository(db)

	if cfg.AdminInitialUsername != "" && len(cfg.AdminInitialPassword) >= 8 {
		if _, err := adminRepo.GetByUsername(ctx, cfg.AdminInitialUsername); errors.Is(err, repositories.ErrNotFound) {
			if hash, hashErr := services.HashPassword(cfg.AdminInitialPassword); hashErr == nil {
				if _, createErr := adminRepo.CreateAdmin(ctx, cfg.AdminInitialUsername, nil, hash); createErr == nil {
					logger.Info("initial_admin_bootstrapped", slog.String("username", cfg.AdminInitialUsername))
				}
			}
		}
	}

	authSvc := services.NewAuthService(cfg, adminRepo)
	mailboxSvc := services.NewMailboxService(cfg, domainRepo, mailboxRepo, msgRepo, adminRepo)
	ingestionSvc := services.NewIngestionService(cfg, mailboxRepo, msgRepo)

	cleanupCtx, cancelCleanup := context.WithCancel(context.Background())
	defer cancelCleanup()
	cleanupRunner := jobs.NewCleanupRunner(cfg, msgRepo, adminRepo, logger)
	go cleanupRunner.Start(cleanupCtx)

	router := handlers.NewRouter(cfg, logger, authSvc, mailboxSvc, ingestionSvc, mailboxRepo, msgRepo, adminRepo)

	srv := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server_starting", slog.String("port", cfg.AppPort), slog.String("env", cfg.AppEnv))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		logger.Error("server_error", slog.String("error", err.Error()))
		os.Exit(1)
	case sig := <-sigCh:
		logger.Info("server_shutting_down", slog.String("signal", sig.String()))
		cancelCleanup()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}
}

