package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gobase/internal/api"
	"gobase/internal/auth"
	"gobase/internal/database"
	"gobase/internal/logger"
	"gobase/internal/realtime"
	"gobase/internal/storage"

	"github.com/joho/godotenv"
)

func main() {
	// Initialize structured logging
	logger.Setup()

	// Load environments
	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found, relying on system environment variables")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initialize Database Pool
	dbPool, err := database.Connect(ctx)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	// Ensure core tables exist
	if err := auth.SetupAuthTables(context.Background(), dbPool); err != nil {
		slog.Error("Failed to setup Auth schemas", "error", err)
		os.Exit(1)
	}
	if err := storage.SetupStorageTables(context.Background(), dbPool); err != nil {
		slog.Error("Failed to setup Storage schemas", "error", err)
		os.Exit(1)
	}
	if err := api.MigrateCollectionMetadata(context.Background(), dbPool); err != nil {
		slog.Error("Failed to setup Collection metadata", "error", err)
		os.Exit(1)
	}
	if err := auth.SetupRefreshTokensTable(context.Background(), dbPool); err != nil {
		slog.Error("Failed to setup Refresh Tokens table", "error", err)
		os.Exit(1)
	}

	slog.Info("Successfully connected and migrated primary schemas on PostgreSQL")

	// Initialize Realtime Hub
	hub := realtime.NewHub()
	go hub.Run()

	// Start Postgres event listener for realtime broadcast
	realtime.StartListener(dbPool, hub)

	// Setup basic HTTP router
	mux := api.NewRouterWithHub(dbPool, hub)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Run Server
	go func() {
		slog.Info("Starting GoBase server", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("Server stopped")
}
