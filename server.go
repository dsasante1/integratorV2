package main

import (
	"context"
	"integratorV2/internal/db"
	"integratorV2/internal/queue"
	"integratorV2/internal/routes"
	"integratorV2/internal/security"
	"integratorV2/internal/worker"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	// Initialize database
	if err := db.InitDB(); err != nil {
		slog.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Initialize security features
	if err := security.InitSecurity(); err != nil {
		slog.Error("Failed to initialize security features", "error", err)
		os.Exit(1)
	}

	// Initialize AWS KMS
	if err := security.InitKMS(); err != nil {
		slog.Error("Failed to initialize AWS KMS", "error", err)
		os.Exit(1)
	}

	// Initialize task queue
	if err := queue.InitQueue(); err != nil {
		slog.Error("Failed to initialize task queue", "error", err)
		os.Exit(1)
	}
	defer queue.Close()

	// Create Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(security.RateLimiter)
	e.Use(security.ValidateEmail)

	v1 := e.Group("/integrator/api/v1")

	// Setup v1 routes
	routes.SetupRoutes(v1)

	// Create and start worker
	w := worker.NewWorker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start worker in a goroutine
	go func() {
		if err := w.Start(ctx); err != nil {
			slog.Error("Worker error", "error", err)
		}
	}()

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		slog.Info("Shutting down...")
		cancel() // Stop the worker
		if err := e.Shutdown(context.Background()); err != nil {
			slog.Error("Error shutting down server", "error", err)
		}
	}()

	// Get port from environment variable with default fallback
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Start server
	if err := e.Start(":" + port); err != nil {
		slog.Error("Error starting server", "error", err)
	}
}
