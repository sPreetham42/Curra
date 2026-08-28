package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sPreetham42/timetable-platform/application/internal/api"
	"github.com/sPreetham42/timetable-platform/application/internal/api/handlers"
	"github.com/sPreetham42/timetable-platform/application/internal/api/middleware"
	"github.com/sPreetham42/timetable-platform/application/internal/curra"
	"github.com/sPreetham42/timetable-platform/application/internal/database"
	"github.com/sPreetham42/timetable-platform/application/internal/database/repositories"
	"github.com/sPreetham42/timetable-platform/application/internal/services"
	"github.com/sPreetham42/timetable-platform/application/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Load config
	dbCfg := database.LoadConfig()

	// Connect to database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.New(ctx, dbCfg, logger)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Run migrations
	if err := db.RunMigrations(ctx); err != nil {
		logger.Error("migrations failed", "error", err)
		os.Exit(1)
	}

	// Create repositories
	repos := repositories.NewRepos(db.Pool)

	// Create CURRA adapter
	adapter := curra.New(logger)

	// Create services
	timetableSvc := services.NewTimetableService(repos)
	snapshotSvc := services.NewSnapshotService(repos)
	runSvc := services.NewRunService(repos, adapter)
	versionSvc := services.NewVersionService(repos)
	publishingSvc := services.NewPublishingService(repos, adapter)
	moveSwapSvc := services.NewMoveSwapService(repos, adapter)
	verificationSvc := services.NewVerificationService(repos, adapter)
	catalogSvc := services.NewCatalogService(repos)

	// Create handlers & router
	h := handlers.New(
		timetableSvc,
		snapshotSvc,
		runSvc,
		versionSvc,
		publishingSvc,
		moveSwapSvc,
		verificationSvc,
		catalogSvc,
	)
	authMiddleware := middleware.NewAuthMiddleware()
	router := api.NewRouter(h, authMiddleware)

	// Start embedded background worker
	w := worker.New("", repos, adapter, logger)
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	go w.Start(workerCtx)

	// Start HTTP server
	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info("shutting down server and worker")
		workerCancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	logger.Info("server starting", "addr", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}
