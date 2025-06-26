package migrations

import (
	"embed"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
)

// Up runs all available migrations
func Up(migrationFiles embed.FS) error {
	m, err := NewMigrator(migrationFiles)
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

// Down rolls back one migration
func Down(migrationFiles embed.FS) error {
	m, err := NewMigrator(migrationFiles)
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

// Force forces the database to a specific migration version
func Force(migrationFiles embed.FS, version string) error {
	m, err := NewMigrator(migrationFiles)
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

// Version shows the current migration version
func Version(migrationFiles embed.FS) error {
	m, err := NewMigrator(migrationFiles)
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

// Drop drops the entire database (DANGEROUS)
func Drop(migrationFiles embed.FS) error {
	m, err := NewMigrator(migrationFiles)
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

// Reset drops all tables and re-runs all migrations
func Reset(migrationFiles embed.FS) error {
	if err := Drop(migrationFiles); err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	if err := Up(migrationFiles); err != nil {
		return fmt.Errorf("failed to run migrations after reset: %w", err)
	}

	return nil
}