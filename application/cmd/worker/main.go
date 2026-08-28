package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sPreetham42/timetable-platform/application/internal/curra"
	"github.com/sPreetham42/timetable-platform/application/internal/database"
	"github.com/sPreetham42/timetable-platform/application/internal/database/repositories"
	"github.com/sPreetham42/timetable-platform/application/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	dbCfg := database.LoadConfig()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.New(ctx, dbCfg, logger)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.RunMigrations(ctx); err != nil {
		logger.Error("migrations failed", "error", err)
		os.Exit(1)
	}

	repos := repositories.NewRepos(db.Pool)
	adapter := curra.New(logger)

	w := worker.New("", repos, adapter, logger)

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		logger.Info("shutting down worker")
		workerCancel()
	}()

	logger.Info("curra standalone solver worker starting", "worker_id", w.ID())
	w.Start(workerCtx)
}
