package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
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

func InitCollectionTables() error {
	// Create users table first
	_, err := DB.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			password VARCHAR(255) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating users table: %v", err)
	}

	// Create collections table
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS collections (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			first_seen TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			last_seen TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating collections table: %v", err)
	}

	// Create snapshots table
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS snapshots (
			id SERIAL PRIMARY KEY,
			collection_id TEXT NOT NULL,
			snapshot_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			content JSONB NOT NULL,
			hash TEXT NOT NULL,
			FOREIGN KEY (collection_id) REFERENCES collections(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating snapshots table: %v", err)
	}

	// Create changes table
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS changes (
			id SERIAL PRIMARY KEY,
			collection_id TEXT NOT NULL,
			old_snapshot_id INTEGER,
			new_snapshot_id INTEGER NOT NULL,
			change_type TEXT NOT NULL,
			path TEXT NOT NULL,
			old_value TEXT,
			new_value TEXT,
			change_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (collection_id) REFERENCES collections(id),
			FOREIGN KEY (old_snapshot_id) REFERENCES snapshots(id),
			FOREIGN KEY (new_snapshot_id) REFERENCES snapshots(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating changes table: %v", err)
	}

	// Create postman_api_keys table
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS postman_api_keys (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL,
			api_key TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			last_used_at TIMESTAMP WITH TIME ZONE,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating postman_api_keys table: %v", err)
	}

	return nil
}

func StorePostmanAPIKey(userID int64, apiKey string) error {
	_, err := DB.Exec(`
		INSERT INTO postman_api_keys (user_id, api_key)
		VALUES ($1, $2)
	`, userID, apiKey)
	return err
}

func GetPostmanAPIKey(userID int64) (string, error) {
	var apiKey string
	err := DB.Get(&apiKey, `
		SELECT api_key FROM postman_api_keys
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, userID)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("no API key found for user")
	}
	return apiKey, err
}

func UpdateLastUsedAPIKey(userID int64) error {
	_, err := DB.Exec(`
		UPDATE postman_api_keys
		SET last_used_at = CURRENT_TIMESTAMP
		WHERE user_id = $1
	`, userID)
	return err
}
