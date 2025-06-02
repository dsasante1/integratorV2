package postman

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"

	"integratorV2/internal/db"
)

// StoreCollectionSnapshot orchestrates the process of storing a collection snapshot
// and tracking any changes from the previous snapshot
func StoreCollectionSnapshot(collectionID string, content json.RawMessage) error {
	slog.Info("Starting collection snapshot process", "collection_id", collectionID)

	// Parse collection metadata
	collection, err := parseCollectionMetadata(content)
	if err != nil {
		slog.Error("Failed to parse collection metadata", "error", err, "collection_id", collectionID)
		return fmt.Errorf("error parsing collection metadata: %v", err)
	}

	// Store collection metadata
	if err := storeCollectionMetadata(collectionID, collection.Collection.Name); err != nil {
		slog.Error("Failed to store collection metadata", "error", err, "collection_id", collectionID)
		return err
	}

	// Create new snapshot
	snapshotID, err := createSnapshot(collectionID, content)
	if err != nil {
		slog.Error("Failed to create snapshot", "error", err, "collection_id", collectionID)
		return err
	}

	// Process changes if there's a previous snapshot
	if err := processSnapshotChanges(collectionID, snapshotID); err != nil {
		slog.Error("Failed to process snapshot changes", "error", err, "collection_id", collectionID)
		return err
	}

	slog.Info("Successfully completed collection snapshot process", "collection_id", collectionID)
	return nil
}

// StoreCollectionSnapshotWithName orchestrates the process of storing a collection snapshot
// with a custom name and tracking any changes from the previous snapshot
func StoreCollectionSnapshotWithName(collectionID, name string, content json.RawMessage) error {
	slog.Info("Starting collection snapshot process", "collection_id", collectionID, "name", name)

	// Store collection metadata with provided name
	if err := storeCollectionMetadata(collectionID, name); err != nil {
		slog.Error("Failed to store collection metadata", "error", err, "collection_id", collectionID)
		return err
	}

	// Create new snapshot
	snapshotID, err := createSnapshot(collectionID, content)
	if err != nil {
		slog.Error("Failed to create snapshot", "error", err, "collection_id", collectionID)
		return err
	}

	// Process changes if there's a previous snapshot
	if err := processSnapshotChanges(collectionID, snapshotID); err != nil {
		slog.Error("Failed to process snapshot changes", "error", err, "collection_id", collectionID)
		return err
	}

	slog.Info("Successfully completed collection snapshot process", "collection_id", collectionID, "name", name)
	return nil
}

// parseCollectionMetadata extracts the collection metadata from the raw content
func parseCollectionMetadata(content json.RawMessage) (*PostmanCollectionResponse, error) {
	var collection PostmanCollectionResponse
	if err := json.Unmarshal(content, &collection); err != nil {
		return nil, fmt.Errorf("error unmarshaling collection: %v", err)
	}
	return &collection, nil
}

// storeCollectionMetadata stores the basic collection information
func storeCollectionMetadata(collectionID, name string) error {
	if err := db.StoreCollection(collectionID, name); err != nil {
		return fmt.Errorf("error storing collection metadata: %v", err)
	}
	slog.Info("Stored collection metadata", "collection_id", collectionID, "name", name)
	return nil
}

// createSnapshot creates a new snapshot record with the collection content
func createSnapshot(collectionID string, content json.RawMessage) (int64, error) {
	hash := fmt.Sprintf("%x", content)

	var snapshotID int64
	err := db.DB.QueryRow(`
		INSERT INTO snapshots (collection_id, content, hash)
		VALUES ($1, $2, $3)
		RETURNING id
	`, collectionID, content, hash).Scan(&snapshotID)
	if err != nil {
		return 0, fmt.Errorf("error creating snapshot: %v", err)
	}

	slog.Info("Created new snapshot", "collection_id", collectionID, "snapshot_id", snapshotID)
	return snapshotID, nil
}

// processSnapshotChanges handles the comparison and storage of changes between snapshots
func processSnapshotChanges(collectionID string, newSnapshotID int64) error {
	// Get previous snapshot
	oldSnapshotID, oldContent, err := getPreviousSnapshot(collectionID, newSnapshotID)
	if err != nil {
		return err
	}

	// If no previous snapshot, we're done
	if oldSnapshotID == nil {
		slog.Info("No previous snapshot found, skipping change detection", "collection_id", collectionID)
		return nil
	}

	// Get new snapshot content
	var newContent json.RawMessage
	err = db.DB.QueryRow(`
		SELECT content FROM snapshots WHERE id = $1
	`, newSnapshotID).Scan(&newContent)
	if err != nil {
		return fmt.Errorf("error getting new snapshot content: %v", err)
	}

	// Compare and store changes
	changes := compareSnapshots(oldContent, newContent)
	if err := storeChanges(collectionID, oldSnapshotID, newSnapshotID, changes); err != nil {
		return err
	}

	slog.Info("Processed snapshot changes",
		"collection_id", collectionID,
		"old_snapshot_id", *oldSnapshotID,
		"new_snapshot_id", newSnapshotID,
		"change_count", len(changes))
	return nil
}

// getPreviousSnapshot retrieves the most recent snapshot before the given one
func getPreviousSnapshot(collectionID string, currentSnapshotID int64) (*int64, json.RawMessage, error) {
	var oldSnapshotID *int64
	var oldContent json.RawMessage

	err := db.DB.QueryRow(`
		SELECT id, content FROM snapshots
		WHERE collection_id = $1 AND id != $2
		ORDER BY snapshot_time DESC
		LIMIT 1
	`, collectionID, currentSnapshotID).Scan(&oldSnapshotID, &oldContent)
	if err != nil && err != sql.ErrNoRows {
		return nil, nil, fmt.Errorf("error getting previous snapshot: %v", err)
	}

	return oldSnapshotID, oldContent, nil
}

// storeChanges persists the detected changes to the database
func storeChanges(collectionID string, oldSnapshotID *int64, newSnapshotID int64, changes []Change) error {
	for _, change := range changes {
		_, err := db.DB.Exec(`
			INSERT INTO changes (
				collection_id, old_snapshot_id, new_snapshot_id,
				change_type, path, old_value, new_value
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, collectionID, oldSnapshotID, newSnapshotID,
			change.Type, change.Path, change.OldValue, change.NewValue)
		if err != nil {
			return fmt.Errorf("error storing change: %v", err)
		}
	}
	return nil
}
