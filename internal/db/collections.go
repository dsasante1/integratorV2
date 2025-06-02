package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"integratorV2/internal/security"
)

type Collection struct {
	ID        string    `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	FirstSeen time.Time `db:"first_seen" json:"first_seen"`
	LastSeen  time.Time `db:"last_seen" json:"last_seen"`
}

type Snapshot struct {
	ID           int64           `db:"id" json:"id"`
	CollectionID string          `db:"collection_id" json:"collection_id"`
	SnapshotTime time.Time       `db:"snapshot_time" json:"snapshot_time"`
	Content      json.RawMessage `db:"content" json:"content"`
	Hash         string          `db:"hash" json:"hash"`
}

type Change struct {
	ID            int64     `db:"id" json:"id"`
	CollectionID  string    `db:"collection_id" json:"collection_id"`
	OldSnapshotID *int64    `db:"old_snapshot_id" json:"old_snapshot_id"`
	NewSnapshotID int64     `db:"new_snapshot_id" json:"new_snapshot_id"`
	ChangeType    string    `db:"change_type" json:"change_type"`
	Path          string    `db:"path" json:"path"`
	OldValue      *string   `db:"old_value" json:"old_value"`
	NewValue      *string   `db:"new_value" json:"new_value"`
	ChangeTime    time.Time `db:"change_time" json:"change_time"`
}

func StoreCollection(id, name string) error {
	_, err := DB.Exec(`
		INSERT INTO collections (id, name)
		VALUES ($1, $2)
		ON CONFLICT (id) DO UPDATE
		SET name = $2,
		    last_seen = CURRENT_TIMESTAMP
	`, id, name)
	return err
}

// func InitCollectionTables() error {
// 	// Create users table first
// 	_, err := DB.Exec(`
// 		CREATE TABLE IF NOT EXISTS users (
// 			id SERIAL PRIMARY KEY,
// 			email VARCHAR(255) UNIQUE NOT NULL,
// 			password VARCHAR(255) NOT NULL,
// 			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
// 		)
// 	`)
// 	if err != nil {
// 		return fmt.Errorf("error creating users table: %v", err)
// 	}

// 	// Create collections table
// 	_, err = DB.Exec(`
// 		CREATE TABLE IF NOT EXISTS collections (
// 			id TEXT PRIMARY KEY,
// 			name TEXT NOT NULL,
// 			first_seen TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
// 			last_seen TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
// 		)
// 	`)
// 	if err != nil {
// 		return fmt.Errorf("error creating collections table: %v", err)
// 	}

// 	// Create snapshots table
// 	_, err = DB.Exec(`
// 		CREATE TABLE IF NOT EXISTS snapshots (
// 			id SERIAL PRIMARY KEY,
// 			collection_id TEXT NOT NULL,
// 			snapshot_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
// 			content JSONB NOT NULL,
// 			hash TEXT NOT NULL,
// 			FOREIGN KEY (collection_id) REFERENCES collections(id)
// 		)
// 	`)
// 	if err != nil {
// 		return fmt.Errorf("error creating snapshots table: %v", err)
// 	}

// 	// Create changes table
// 	_, err = DB.Exec(`
// 		CREATE TABLE IF NOT EXISTS changes (
// 			id SERIAL PRIMARY KEY,
// 			collection_id TEXT NOT NULL,
// 			old_snapshot_id INTEGER,
// 			new_snapshot_id INTEGER NOT NULL,
// 			change_type TEXT NOT NULL,
// 			path TEXT NOT NULL,
// 			old_value TEXT,
// 			new_value TEXT,
// 			change_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
// 			FOREIGN KEY (collection_id) REFERENCES collections(id),
// 			FOREIGN KEY (old_snapshot_id) REFERENCES snapshots(id),
// 			FOREIGN KEY (new_snapshot_id) REFERENCES snapshots(id)
// 		)
// 	`)
// 	if err != nil {
// 		return fmt.Errorf("error creating changes table: %v", err)
// 	}

// 	// Create postman_api_keys table
// 	_, err = DB.Exec(`
// 		CREATE TABLE IF NOT EXISTS postman_api_keys (
// 			id SERIAL PRIMARY KEY,
// 			user_id INTEGER NOT NULL,
// 			api_key TEXT NOT NULL,
// 			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
// 			last_used_at TIMESTAMP WITH TIME ZONE,
// 			FOREIGN KEY (user_id) REFERENCES users(id)
// 		)
// 	`)
// 	if err != nil {
// 		return fmt.Errorf("error creating postman_api_keys table: %v", err)
// 	}

// 	return nil
// }

func StorePostmanAPIKey(userID int64, apiKey string) error {
	// Encrypt the API key
	encryptedKey, err := security.EncryptAPIKey(apiKey)
	if err != nil {
		slog.Error("Failed to encrypt API key", "error", err, "user_id", userID)
		return fmt.Errorf("failed to encrypt API key: %v", err)
	}

	// Calculate expiration date (90 days from now)
	expiresAt := time.Now().Add(90 * 24 * time.Hour)

	// Store encrypted key
	_, err = DB.Exec(`
		INSERT INTO postman_api_keys (
			user_id, encrypted_key, key_version,
			expires_at, last_rotated_at, is_active
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, userID, encryptedKey, 1, expiresAt, time.Now(), true)
	if err != nil {
		slog.Error("Failed to store API key", "error", err, "user_id", userID)
		return fmt.Errorf("failed to store API key: %v", err)
	}

	slog.Info("Successfully stored API key", "user_id", userID)
	return nil
}

func GetPostmanAPIKey(userID int64) (string, error) {
	var encryptedKey string
	err := DB.Get(&encryptedKey, `
		SELECT encrypted_key FROM postman_api_keys
		WHERE user_id = $1
		AND is_active = true
		AND expires_at > NOW()
		ORDER BY created_at DESC
		LIMIT 1
	`, userID)
	if err == sql.ErrNoRows {
		slog.Warn("No active API key found", "user_id", userID)
		return "", fmt.Errorf("no active API key found for user")
	}
	if err != nil {
		slog.Error("Failed to get API key", "error", err, "user_id", userID)
		return "", fmt.Errorf("failed to get API key: %v", err)
	}

	// Decrypt the API key
	apiKey, err := security.DecryptAPIKey(encryptedKey)
	if err != nil {
		slog.Error("Failed to decrypt API key", "error", err, "user_id", userID)
		return "", fmt.Errorf("failed to decrypt API key: %v", err)
	}

	slog.Info("Successfully retrieved API key", "user_id", userID)
	return apiKey, nil
}

func RotateAPIKey(userID int64, newAPIKey string) error {
	// Encrypt the new API key
	encryptedKey, err := security.EncryptAPIKey(newAPIKey)
	if err != nil {
		slog.Error("Failed to encrypt new API key", "error", err, "user_id", userID)
		return fmt.Errorf("failed to encrypt API key: %v", err)
	}

	// Calculate new expiration date
	expiresAt := time.Now().Add(90 * 24 * time.Hour)

	// Start transaction
	tx, err := DB.Begin()
	if err != nil {
		slog.Error("Failed to begin transaction", "error", err, "user_id", userID)
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Deactivate old keys
	_, err = tx.Exec(`
		UPDATE postman_api_keys
		SET is_active = false
		WHERE user_id = $1 AND is_active = true
	`, userID)
	if err != nil {
		slog.Error("Failed to deactivate old keys", "error", err, "user_id", userID)
		return fmt.Errorf("failed to deactivate old keys: %v", err)
	}

	// Insert new key
	_, err = tx.Exec(`
		INSERT INTO postman_api_keys (
			user_id, encrypted_key, key_version,
			expires_at, last_rotated_at, is_active
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, userID, encryptedKey, 1, expiresAt, time.Now(), true)
	if err != nil {
		slog.Error("Failed to insert new key", "error", err, "user_id", userID)
		return fmt.Errorf("failed to insert new key: %v", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		slog.Error("Failed to commit transaction", "error", err, "user_id", userID)
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	slog.Info("Successfully rotated API key", "user_id", userID)
	return nil
}

func UpdateLastUsedAPIKey(userID int64) error {
	_, err := DB.Exec(`
		UPDATE postman_api_keys
		SET last_used_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND is_active = true
	`, userID)
	if err != nil {
		slog.Error("Failed to update last used timestamp", "error", err, "user_id", userID)
		return fmt.Errorf("failed to update last used timestamp: %v", err)
	}

	slog.Info("Successfully updated last used timestamp", "user_id", userID)
	return nil
}
