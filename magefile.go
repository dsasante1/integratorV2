//go:build mage

package main

import (
	"fmt"
	"os"
	"os/exec"

	"log/slog"

	"github.com/joho/godotenv"
)

// MigrateUp runs all pending migrations
func MigrateUp() error {
	if err := loadEnv(); err != nil {
		return err
	}

	dbURL := getDBURL()
	cmd := exec.Command("migrate", "-path", "migrations", "-database", dbURL, "up")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// MigrateDown rolls back the last migration
func MigrateDown() error {
	if err := loadEnv(); err != nil {
		return err
	}

	dbURL := getDBURL()
	cmd := exec.Command("migrate", "-path", "migrations", "-database", dbURL, "down", "1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// MigrateCreate creates new migration files
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
	if err := loadEnv(); err != nil {
		return err
	}

	if version == "" {
		return fmt.Errorf("version is required")
	}

	dbURL := getDBURL()
	cmd := exec.Command("migrate", "-path", "migrations", "-database", dbURL, "force", version)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// MigrateVersion shows current migration version
func MigrateVersion() error {
	if err := loadEnv(); err != nil {
		return err
	}

	dbURL := getDBURL()
	cmd := exec.Command("migrate", "-path", "migrations", "-database", dbURL, "version")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Drop drops the entire database (use with extreme caution)
func MigrateDrop() error {
	if err := loadEnv(); err != nil {
		return err
	}

	dbURL := getDBURL()
	cmd := exec.Command("migrate", "-path", "migrations", "-database", dbURL, "drop", "-f")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Helper functions

func loadEnv() error {
	if err := godotenv.Load(); err != nil {
		slog.Warn("Warning: .env file not found", "error", err)
	}
	return nil
}

func getDBURL() string {
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "")
	dbName := getEnv("DB_NAME", "mydb")

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPassword, dbHost, dbPort, dbName)
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
