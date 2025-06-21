package main

import (
	"context"
	"flag"
	"fmt"
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
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)
//TODO handle config properly
func LoadEnv() error {
	if err := godotenv.Load(); err != nil {
		slog.Warn("Warning: .env file not found", "error", err)
	}
	return nil
}

func GetDBURL() string {
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "")
	dbName := getEnv("DB_NAME", "mydb")

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPassword, dbHost, dbPort, dbName)
}

func MigrateUp() error {
	if err := LoadEnv(); err != nil {
		return err
	}

	dbURL := GetDBURL()
	cmd := exec.Command("migrate", "-path", "migrations", "-database", dbURL, "up")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func MigrateDown() error {
	if err := LoadEnv(); err != nil {
		return err
	}

	dbURL := GetDBURL()
	cmd := exec.Command("migrate", "-path", "migrations", "-database", dbURL, "down", "1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func MigrateCreate(name string) error {
	if name == "" {
		return fmt.Errorf("migration name is required")
	}

	cmd := exec.Command("migrate", "create", "-ext", "sql", "-dir", "migrations", "-seq", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// MigrateForce forces the database to a specific version (use carefully)
func MigrateForce(version string) error {
	if err := LoadEnv(); err != nil {
		return err
	}

	if version == "" {
		return fmt.Errorf("version is required")
	}

	dbURL := GetDBURL()
	cmd := exec.Command("migrate", "-path", "migrations", "-database", dbURL, "force", version)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func MigrateVersion() error {
	if err := LoadEnv(); err != nil {
		return err
	}

	dbURL := GetDBURL()
	cmd := exec.Command("migrate", "-path", "migrations", "-database", dbURL, "version")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// MigrateDrop drops the entire database (use with extreme caution)
func MigrateDrop() error {
	if err := LoadEnv(); err != nil {
		return err
	}

	dbURL := GetDBURL()
	cmd := exec.Command("migrate", "-path", "migrations", "-database", dbURL, "drop", "-f")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func MigrateReset() error {
	if err := MigrateDrop(); err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	if err := MigrateUp(); err != nil {
		return fmt.Errorf("failed to run migrations after reset: %w", err)
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

var (
	migrate        = flag.Bool("migrate", false, "Run database migrations and exit")
	migrateDown    = flag.Bool("migrate-down", false, "Roll back one migration and exit")
	migrateReset   = flag.Bool("migrate-reset", false, "Reset all migrations and exit")
	migrateCreate  = flag.String("migrate-create", "", "Create new migration with given name and exit")
	migrateForce   = flag.String("migrate-force", "", "Force database to specific version and exit")
	migrateVersion = flag.Bool("migrate-version", false, "Show current migration version and exit")
	migrateDrop    = flag.Bool("migrate-drop", false, "Drop entire database and exit (DANGEROUS)")
	autoMigrate    = flag.Bool("auto-migrate", false, "Run migrations automatically on startup")
)

func main() {
	flag.Parse()

	
	if *migrate {
		if err := MigrateUp(); err != nil {
			slog.Error("Migration failed", "error", err)
			os.Exit(1)
		}
		slog.Info("Migrations completed successfully")
		return
	}

	if *migrateDown {
		if err := MigrateDown(); err != nil {
			slog.Error("Migration down failed", "error", err)
			os.Exit(1)
		}
		slog.Info("Migration down completed successfully")
		return
	}

	if *migrateReset {
		if err := MigrateReset(); err != nil {
			slog.Error("Migration reset failed", "error", err)
			os.Exit(1)
		}
		slog.Info("Migration reset completed successfully")
		return
	}

	if *migrateCreate != "" {
		if err := MigrateCreate(*migrateCreate); err != nil {
			slog.Error("Migration create failed", "error", err)
			os.Exit(1)
		}
		slog.Info("Migration files created successfully", "name", *migrateCreate)
		return
	}

	if *migrateForce != "" {
		if err := MigrateForce(*migrateForce); err != nil {
			slog.Error("Migration force failed", "error", err)
			os.Exit(1)
		}
		slog.Info("Migration force completed successfully", "version", *migrateForce)
		return
	}

	if *migrateVersion {
		if err := MigrateVersion(); err != nil {
			slog.Error("Migration version failed", "error", err)
			os.Exit(1)
		}
		return
	}

	if *migrateDrop {
		fmt.Print("Are you sure you want to drop the entire database? This cannot be undone. Type 'yes' to confirm: ")
		var confirmation string
		fmt.Scanln(&confirmation)
		if confirmation != "yes" {
			slog.Info("Database drop cancelled")
			return
		}

		if err := MigrateDrop(); err != nil {
			slog.Error("Migration drop failed", "error", err)
			os.Exit(1)
		}
		slog.Info("Database dropped successfully")
		return
	}

	
	if err := LoadEnv(); err != nil {
		slog.Error("Failed to load environment", "error", err)
		os.Exit(1)
	}

	
	if *autoMigrate {
		slog.Info("Running auto-migration...")
		if err := MigrateUp(); err != nil {
			slog.Error("Auto-migration failed", "error", err)
			os.Exit(1)
		}
		slog.Info("Auto-migration completed successfully")
	}

	
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

	// Setup Echo server
	e := echo.New()

	// Middleware
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

	// Setup routes
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

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	slog.Info("Starting server", "port", port)
	if err := e.Start(":" + port); err != nil {
		slog.Error("Error starting server", "error", err)
	}
}