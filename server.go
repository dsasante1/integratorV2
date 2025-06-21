package main

import (
	"context"
	"embed"
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
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Embed migration files into the binary
//go:embed migrations/*.sql
var migrationFiles embed.FS

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

// Create a migrate instance using embedded files
func createMigrator() (*migrate.Migrate, error) {
	if err := LoadEnv(); err != nil {
		return nil, err
	}

	// Create source from embedded files
	sourceDriver, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to create migration source: %w", err)
	}

	dbURL := GetDBURL()
	
	// Create migrator
	m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrator: %w", err)
	}

	return m, nil
}

func MigrateUp() error {
	m, err := createMigrator()
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	if err == migrate.ErrNoChange {
		slog.Info("No new migrations to apply")
	} else {
		slog.Info("Migrations applied successfully")
	}

	return nil
}

func MigrateDown() error {
	m, err := createMigrator()
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Steps(-1); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to rollback migration: %w", err)
	}

	slog.Info("Migration rolled back successfully")
	return nil
}

func MigrateForce(version string) error {
	m, err := createMigrator()
	if err != nil {
		return err
	}
	defer m.Close()

	versionInt := 0
	if _, err := fmt.Sscanf(version, "%d", &versionInt); err != nil {
		return fmt.Errorf("invalid version format: %w", err)
	}

	if err := m.Force(versionInt); err != nil {
		return fmt.Errorf("failed to force migration to version %s: %w", version, err)
	}

	slog.Info("Migration forced successfully", "version", version)
	return nil
}

func MigrateVersion() error {
	m, err := createMigrator()
	if err != nil {
		return err
	}
	defer m.Close()

	version, dirty, err := m.Version()
	if err != nil {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	status := "clean"
	if dirty {
		status = "dirty"
	}

	slog.Info("Current migration version", "version", version, "status", status)
	fmt.Printf("Current version: %d (%s)\n", version, status)
	return nil
}

func MigrateDrop() error {
	m, err := createMigrator()
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Drop(); err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	slog.Info("Database dropped successfully")
	return nil
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
	migrateUp      = flag.Bool("migrate", false, "Run database migrations and exit")
	migrateDown    = flag.Bool("migrate-down", false, "Roll back one migration and exit")
	migrateReset   = flag.Bool("migrate-reset", false, "Reset all migrations and exit")
	migrateForce   = flag.String("migrate-force", "", "Force database to specific version and exit")
	migrateVersion = flag.Bool("migrate-version", false, "Show current migration version and exit")
	migrateDrop    = flag.Bool("migrate-drop", false, "Drop entire database and exit (DANGEROUS)")
	autoMigrate    = flag.Bool("auto-migrate", false, "Run migrations automatically on startup")
)

func main() {
	flag.Parse()

	if *migrateUp {
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