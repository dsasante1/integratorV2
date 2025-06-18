package postman

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"integratorV2/internal/db"
	"crypto/sha256"
	"sort"
	"strings"
	"time"
	"errors"
	"integratorV2/utils"

)

var ErrIdenticalSnapshotFound = errors.New("snapshot with identical content already exists")

type NormalizedSnapshot struct {
	Collection json.RawMessage `json:"collection"`
	
}

type SnapshotInfo struct {
	ID          int64     `json:"id"`
	ContentHash string    `json:"hash"`
	CreatedAt   time.Time `json:"created_at"`
}

func StoreCollectionSnapshot(collectionID string, content json.RawMessage, userID int64,) error {
	slog.Info("Starting collection snapshot process", "collection_id", collectionID)

	
	collection, err := parseCollectionMetadata(content)
	if err != nil {
		slog.Error("Failed to parse collection metadata", "error", err, "collection_id", collectionID)
		return fmt.Errorf("error parsing collection metadata: %v", err)
	}

	if err := storeCollectionMetadata(collectionID, collection.Collection.Name, userID); err != nil {
		slog.Error("Failed to store collection metadata", "error", err, "collection_id", collectionID)
		return err
	}

	snapshotID, err := createSnapshot(collectionID, content)
	if err != nil {
		slog.Error("Failed to create snapshot", "error", err, "collection_id", collectionID)
		return err
	}


	if err := processSnapshotChanges(collectionID, snapshotID); err != nil {
		slog.Error("Failed to process snapshot changes", "error", err, "collection_id", collectionID)
		return err
	}

	slog.Info("Successfully completed collection snapshot process", "collection_id", collectionID)
	return nil
}

func StoreCollectionSnapshotWithName(collectionID, name string, content json.RawMessage, userID int64) error {
	slog.Info("Starting collection snapshot process", "collection_id", collectionID, "name", name)

	if err := storeCollectionMetadata(collectionID, name, userID); err != nil {
		slog.Error("Failed to store collection metadata", "error", err, "collection_id", collectionID)
		return err
	}

	snapshotID, err := createSnapshot(collectionID, content)
	if err != nil {
		if errors.Is(err, ErrIdenticalSnapshotFound) {
			return nil
		}
		slog.Error("Failed to create snapshot", "error", err, "collection_id", collectionID)
		return err
	}

	if err := processSnapshotChanges(collectionID, snapshotID); err != nil {
		slog.Error("Failed to process snapshot changes", "error", err, "collection_id", collectionID)
		return err
	}

	slog.Info("Successfully completed collection snapshot process", "collection_id", collectionID, "name", name)
	return nil
}


func parseCollectionMetadata(content json.RawMessage) (*PostmanCollectionResponse, error) {
	var collection PostmanCollectionResponse
	if err := json.Unmarshal(content, &collection); err != nil {
		return nil, fmt.Errorf("error unmarshaling collection: %v", err)
	}
	return &collection, nil
}


func storeCollectionMetadata(collectionID, name string, userID int64) error {
	if err := db.StoreCollection(collectionID, name, userID); err != nil {
		return fmt.Errorf("error storing collection metadata: %v", err)
	}
	slog.Info("Stored collection metadata", "collection_id", collectionID, "name", name)
	return nil
}




func generateContentHash(content json.RawMessage) (string, error) {
	var snapshot map[string]interface{}
	if err := json.Unmarshal(content, &snapshot); err != nil {
		return "", fmt.Errorf("failed to parse snapshot: %w", err)
	}

	
	normalized := NormalizedSnapshot{}
	if collection, ok := snapshot["collection"]; ok {
		collectionBytes, err := json.Marshal(collection)
		if err != nil {
			return "", fmt.Errorf("failed to marshal collection: %w", err)
		}
		normalized.Collection = collectionBytes
	}

	
	canonicalBytes, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("failed to create canonical JSON: %w", err)
	}

	
	hash := sha256.Sum256(canonicalBytes)
	return fmt.Sprintf("%x", hash), nil
}


func generateSemanticHash(content json.RawMessage) (string, error) {
	var snapshot map[string]interface{}
	if err := json.Unmarshal(content, &snapshot); err != nil {
		return "", fmt.Errorf("failed to parse snapshot: %w", err)
	}

	
	collection, ok := snapshot["collection"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("no collection found in snapshot")
	}

	
	normalized := normalizeForHashing(collection)
	
	
	canonical := createCanonicalJSON(normalized)
	
	
	hash := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("%x", hash), nil
}


func normalizeForHashing(obj interface{}) interface{} {
	switch v := obj.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			
			if isVolatileField(key) {
				continue
			}
			result[key] = normalizeForHashing(value)
		}
		return result
		
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = normalizeForHashing(item)
		}
		return result
		
	default:
		return v
	}
}

// isVolatileField checks if a field should be ignored for content hashing
func isVolatileField(key string) bool {
	volatileFields := []string{
		"_postman_id",     // Auto-generated IDs
		"id",              // Response IDs that might change
		"processing_time", // Masking processing time
		"masked_at",       // Masking timestamp
		"masking_id",      // Masking session ID
		"Date",            // HTTP response dates
		"ETag",            // HTTP ETags
		"currentHelper",   // UI state
		"helperAttributes", // UI state
	}
	
	for _, volatile := range volatileFields {
		if key == volatile {
			return true
		}
	}
	return false
}


func createCanonicalJSON(obj interface{}) string {
	switch v := obj.(type) {
	case map[string]interface{}:
		var keys []string
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		
		var parts []string
		for _, k := range keys {
			value := createCanonicalJSON(v[k])
			parts = append(parts, fmt.Sprintf(`"%s":%s`, k, value))
		}
		return "{" + strings.Join(parts, ",") + "}"
		
	case []interface{}:
		var parts []string
		for _, item := range v {
			parts = append(parts, createCanonicalJSON(item))
		}
		return "[" + strings.Join(parts, ",") + "]"
		
	case string:
		bytes, _ := json.Marshal(v)
		return string(bytes)
		
	case nil:
		return "null"
		
	default:
		bytes, _ := json.Marshal(v)
		return string(bytes)
	}
}


func createSnapshot(collectionID string, content json.RawMessage) (int64, error) {
	
	contentHash, err := generateSemanticHash(content)

		if err != nil {
		slog.Warn("Failed to generate semantic hash, falling back to simple hash", "error", err)
		contentHash, err = generateContentHash(content)

		if err != nil {
			return 0, fmt.Errorf("failed to generate content hash: %w", err)
		}
	}

	generatedSnapshotID, err := utils.GenerateRandomAlphaNumeric(6)

	if err != nil {
		slog.Warn("Failed to generate snapshot ID", "error", err)
		return 0, fmt.Errorf("failed to generate snapshot ID: %w", err)
	}

	formatGeneratedSnapshotID := "s-" + generatedSnapshotID
	
	var existingID int64
	err = db.DB.QueryRow(`
		SELECT id FROM snapshots 
		WHERE collection_id = $1 AND hash = $2
		ORDER BY created_at DESC LIMIT 1
	`, collectionID, contentHash).Scan(&existingID)
	
	if err == nil {
		slog.Info("Snapshot with identical content already exists", 
			"collection_id", collectionID, 
			"existing_snapshot_id", existingID,
			"hash", contentHash)
		return 0, ErrIdenticalSnapshotFound
	}

	
	var snapshotID int64
	err = db.DB.QueryRow(`
		INSERT INTO snapshots (collection_id, content, hash, snapshot_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, collectionID, content, contentHash, formatGeneratedSnapshotID).Scan(&snapshotID)
	
	if err != nil {
		slog.Warn("creating snapshot failed", "error", err)
		return 0, fmt.Errorf("error creating snapshot: %v", err)
	}

	slog.Info("Created new snapshot", 
		"collection_id", collectionID, 
		"snapshot_id", snapshotID,
		"hash", contentHash)
	return snapshotID, nil
}

func hasContentChanged(collectionID string, newContent json.RawMessage) (bool, error) {
	newHash, err := generateSemanticHash(newContent)
	if err != nil {
		return true, fmt.Errorf("failed to generate hash for new content: %w", err)
	}

	var existingHash string
	err = db.DB.QueryRow(`
		SELECT hash FROM snapshots 
		WHERE collection_id = $1 
		ORDER BY created_at DESC LIMIT 1
	`, collectionID).Scan(&existingHash)
	
	if err != nil {
		
		return true, nil
	}

	return newHash != existingHash, nil
}



func processSnapshotChanges(collectionID string, newSnapshotID int64) error {
	
	hasChanges, err := quickChangeCheck(collectionID, newSnapshotID)
	if err != nil {
		return fmt.Errorf("quick change check failed: %w", err)
	}

	if !hasChanges {
		slog.Info("No content changes detected via hash comparison, skipping detailed analysis",
			"collection_id", collectionID,
			"snapshot_id", newSnapshotID)
		return nil
	}

	
	oldSnapshot, err := getPreviousSnapshot(collectionID, newSnapshotID)
	if err != nil {
		return fmt.Errorf("failed to get previous snapshot: %w", err)
	}

	if oldSnapshot == nil {
		slog.Info("No previous snapshot found, this is the first snapshot",
			"collection_id", collectionID,
			"snapshot_id", newSnapshotID)
		return nil
	}

	
	newContent, err := getSnapshotContent(newSnapshotID)
	if err != nil {
		return fmt.Errorf("failed to get new snapshot content: %w", err)
	}

	
	oldContent, err := getSnapshotContent(oldSnapshot.ID)
	if err != nil {
		return fmt.Errorf("failed to get old snapshot content: %w", err)
	}

	slog.Info("Starting detailed change analysis",
		"collection_id", collectionID,
		"old_snapshot_id", oldSnapshot.ID,
		"new_snapshot_id", newSnapshotID,
		"old_hash", oldSnapshot.ContentHash)

	
	changes := compareSnapshots(oldContent, newContent)
	
	if len(changes) == 0 {
		slog.Warn("Hash indicated changes but detailed comparison found none - possible hash collision",
			"collection_id", collectionID,
			"old_snapshot_id", oldSnapshot.ID,
			"new_snapshot_id", newSnapshotID)
		return nil
	}

	
	if err := storeChanges(collectionID, &oldSnapshot.ID, newSnapshotID, changes); err != nil {
		return fmt.Errorf("failed to store changes: %w", err)
	}

	slog.Info("Successfully processed snapshot changes",
		"collection_id", collectionID,
		"old_snapshot_id", oldSnapshot.ID,
		"new_snapshot_id", newSnapshotID,
		"change_count", len(changes))

	return nil
}


func quickChangeCheck(collectionID string, currentSnapshotID int64) (bool, error) {
	var currentHash, previousHash string

	
	err := db.DB.QueryRow(`
		SELECT hash FROM snapshots WHERE id = $1
	`, currentSnapshotID).Scan(&currentHash)
	if err != nil {
		return true, fmt.Errorf("failed to get current snapshot hash: %w", err)
	}

	
	err = db.DB.QueryRow(`
		SELECT hash FROM snapshots
		WHERE collection_id = $1 AND id != $2
		ORDER BY created_at DESC
		LIMIT 1
	`, collectionID, currentSnapshotID).Scan(&previousHash)
	if err == sql.ErrNoRows {
		
		return false, nil
	}
	if err != nil {
		return true, fmt.Errorf("failed to get previous snapshot hash: %w", err)
	}

	hasChanges := currentHash != previousHash
	
	slog.Debug("Hash comparison result",
		"collection_id", collectionID,
		"current_hash", currentHash,
		"previous_hash", previousHash,
		"has_changes", hasChanges)

	return hasChanges, nil
}


func getPreviousSnapshot(collectionID string, currentSnapshotID int64) (*SnapshotInfo, error) {
	var snapshot SnapshotInfo

	err := db.DB.QueryRow(`
		SELECT id, hash, created_at 
		FROM snapshots
		WHERE collection_id = $1 AND id != $2
		ORDER BY created_at DESC
		LIMIT 1
	`, collectionID, currentSnapshotID).Scan(&snapshot.ID, &snapshot.ContentHash, &snapshot.CreatedAt)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error querying previous snapshot: %w", err)
	}

	return &snapshot, nil
}


func getSnapshotContent(snapshotID int64) (json.RawMessage, error) {
	var content json.RawMessage
	err := db.DB.QueryRow(`
		SELECT content FROM snapshots WHERE id = $1
	`, snapshotID).Scan(&content)
	if err != nil {
		return nil, fmt.Errorf("error getting snapshot content for ID %d: %w", snapshotID, err)
	}
	return content, nil
}


func storeChanges(collectionID string, oldSnapshotID *int64, newSnapshotID int64, changes []Change) error {
	if len(changes) == 0 {
		slog.Debug("No changes to store", "collection_id", collectionID)
		return nil
	}

	
	tx, err := db.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	
	stmt, err := tx.Prepare(`
		INSERT INTO changes (
			collection_id, old_snapshot_id, new_snapshot_id,
			change_type, path, modification, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	
	for i, change := range changes {
		_, err := stmt.Exec(
			collectionID, oldSnapshotID, newSnapshotID,
			change.Type, change.Path, change.Modification,
		)
		if err != nil {
			return fmt.Errorf("failed to insert change %d: %w", i, err)
		}
	}

	
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit changes transaction: %w", err)
	}

	slog.Info("Successfully stored changes",
		"collection_id", collectionID,
		"old_snapshot_id", oldSnapshotID,
		"new_snapshot_id", newSnapshotID,
		"change_count", len(changes))

	return nil
}