//go:build mage

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"

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

// Helper functions

func loadEnv() error {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}
	return nil
}

func getDBURL() string {
	dbHost := getEnv("DB_HOST")
	dbPort := getEnv("DB_PORT")
	dbUser := getEnv("DB_USER")
	dbPassword := getEnv("DB_PASSWORD")
	dbName := getEnv("DB_NAME")

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
