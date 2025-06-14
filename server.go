package main

import (
	"context"
	"integratorV2/internal/config"
	"integratorV2/internal/db"
	"integratorV2/internal/notification"
	"integratorV2/internal/queue"
	"integratorV2/internal/routes"
	"integratorV2/internal/security"
	"integratorV2/internal/worker"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {

	if err := db.InitDB(); err != nil {
		slog.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := queue.InitQueue(); err != nil {
		slog.Error("Failed to initialize task queue", "error", err)
		os.Exit(1)
	}
	defer queue.Close()

	if err := security.InitSecurity(); err != nil {
		slog.Error("Failed to initialize security features", "error", err)
		os.Exit(1)
	}

	if err := config.InitFireStore(); err != nil {
		slog.Error("Failed to initialize Firebase:", slog.Any("err", err))
	}
	defer config.CloseFirebaseConnection()

	if err := notification.InitNotificationService(); err != nil {

		slog.Error("Failed to initialize notification:", slog.Any("err", err))
	}
	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.Gzip())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))
	e.Use(security.RateLimiter)
	e.Use(security.ValidateEmail)

	v1 := e.Group("/integrator/api/v1")

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

		// Create a timeout context for shutdown
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := e.Shutdown(shutdownCtx); err != nil {
			slog.Error("Error shutting down server", "error", err)
		}
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if err := e.Start(":" + port); err != nil {
		slog.Error("Error starting server", "error", err)
	}
}
