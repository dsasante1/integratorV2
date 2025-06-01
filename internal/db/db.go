package db

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var DB *sqlx.DB

// InitDB initializes the database connection and terminates the application if connection fails
func InitDB() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		slog.Warn("Failed to load .env file", "error", err)
	}

	// Get database connection details from environment variables
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	// Validate required environment variables
	if dbHost == "" || dbPort == "" || dbUser == "" || dbPassword == "" || dbName == "" {
		slog.Error("Missing required database environment variables",
			"required_vars", []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME"})
		os.Exit(1)
	}

	// Create connection string
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	// Connect to database
	var err error
	DB, err = sqlx.Connect("postgres", connStr)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}

	// Test connection
	if err := DB.Ping(); err != nil {
		slog.Error("Failed to ping database", "error", err)
		os.Exit(1)
	}

	slog.Info("Successfully connected to database",
		"host", dbHost,
		"port", dbPort,
		"database", dbName)
}
