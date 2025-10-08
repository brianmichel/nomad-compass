package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brianmichel/nomad-compass/internal/auth"
	"github.com/brianmichel/nomad-compass/internal/config"
	"github.com/brianmichel/nomad-compass/internal/nomadclient"
	"github.com/brianmichel/nomad-compass/internal/reconcile"
	"github.com/brianmichel/nomad-compass/internal/repo"
	"github.com/brianmichel/nomad-compass/internal/server"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	db, err := storage.Open(cfg.Database.Path)
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := storage.Migrate(ctx, db); err != nil {
		logger.Error("migrate database", "error", err)
		os.Exit(1)
	}

	encryptor, err := auth.NewEncryptor(cfg.Crypto.CredentialKey)
	if err != nil {
		logger.Error("init encryptor", "error", err)
		os.Exit(1)
	}

	credStore := storage.NewCredentialStore(db, encryptor)
	repoStore := storage.NewRepoStore(db)
	fileStore := storage.NewRepoFileStore(db)

	gitManager := repo.NewManager(cfg.Repo.BaseDir)

	nomad, err := nomadclient.New(cfg.Nomad)
	if err != nil {
		logger.Error("init nomad client", "error", err)
		os.Exit(1)
	}

	reconciler := reconcile.New(repoStore, fileStore, credStore, gitManager, nomad, cfg.Repo.PollInterval, logger)

	srv := server.New(repoStore, credStore, reconciler, logger)
	httpServer := &http.Server{Addr: cfg.Server.Address, Handler: srv.Handler()}

	go func() {
		if err := reconciler.Run(ctx); err != nil && err != context.Canceled {
			logger.Error("reconciler stopped", "error", err)
		}
	}()

	go func() {
		logger.Info("http server listening", "addr", cfg.Server.Address)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown", "error", err)
	}
}
