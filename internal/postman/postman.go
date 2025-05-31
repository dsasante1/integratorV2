package postman

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"integratorV2/internal/db"
)

const (
	PostmanAPIBaseURL = "https://api.getpostman.com"
)

type PostmanCollection struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type PostmanCollectionsResponse struct {
	Collections []PostmanCollection `json:"collections"`
}

type PostmanCollectionResponse struct {
	Collection struct {
		ID   string          `json:"id"`
		Name string          `json:"name"`
		Info json.RawMessage `json:"info"`
		Item json.RawMessage `json:"item"`
	} `json:"collection"`
}

func GetCollections(apiKey string) ([]PostmanCollection, error) {
	req, err := http.NewRequest("GET", PostmanAPIBaseURL+"/collections", nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("postman API returned status: %d", resp.StatusCode)
	}

	var result PostmanCollectionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	return result.Collections, nil
}

func GetCollection(apiKey, collectionID string) (*PostmanCollectionResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/collections/%s", PostmanAPIBaseURL, collectionID), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("postman API returned status: %d", resp.StatusCode)
	}

	var result PostmanCollectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	return &result, nil
}

func StoreCollectionSnapshot(collectionID string, content json.RawMessage) error {
	// Calculate hash of content
	hash := fmt.Sprintf("%x", content)

	// Store snapshot
	var snapshotID int64
	err := db.DB.QueryRow(`
		INSERT INTO snapshots (collection_id, content, hash)
		VALUES ($1, $2, $3)
		RETURNING id
	`, collectionID, content, hash).Scan(&snapshotID)
	if err != nil {
		return fmt.Errorf("error storing snapshot: %v", err)
	}

	// Get previous snapshot
	var oldSnapshotID *int64
	err = db.DB.QueryRow(`
		SELECT id FROM snapshots
		WHERE collection_id = $1 AND id != $2
		ORDER BY snapshot_time DESC
		LIMIT 1
	`, collectionID, snapshotID).Scan(&oldSnapshotID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error getting previous snapshot: %v", err)
	}

	// If there's a previous snapshot, compare and store changes
	if oldSnapshotID != nil {
		var oldContent json.RawMessage
		err = db.DB.QueryRow(`
			SELECT content FROM snapshots WHERE id = $1
		`, oldSnapshotID).Scan(&oldContent)
		if err != nil {
			return fmt.Errorf("error getting old snapshot content: %v", err)
		}

		// Compare and store changes
		changes := compareSnapshots(oldContent, content)
		for _, change := range changes {
			_, err = db.DB.Exec(`
				INSERT INTO changes (
					collection_id, old_snapshot_id, new_snapshot_id,
					change_type, path, old_value, new_value
				) VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, collectionID, oldSnapshotID, snapshotID,
				change.Type, change.Path, change.OldValue, change.NewValue)
			if err != nil {
				return fmt.Errorf("error storing change: %v", err)
			}
		}
	}

	// Update collection last_seen
	_, err = db.DB.Exec(`
		UPDATE collections
		SET last_seen = CURRENT_TIMESTAMP
		WHERE id = $1
	`, collectionID)
	if err != nil {
		return fmt.Errorf("error updating collection last_seen: %v", err)
	}

	return nil
}

type Change struct {
	Type     string
	Path     string
	OldValue *string
	NewValue *string
}

func compareSnapshots(old, new json.RawMessage) []Change {
	// This is a simplified comparison. In a real implementation,
	// you would want to use a proper JSON diff library or implement
	// a more sophisticated comparison algorithm.
	var changes []Change

	var oldMap, newMap map[string]interface{}
	if err := json.Unmarshal(old, &oldMap); err != nil {
		return changes
	}
	if err := json.Unmarshal(new, &newMap); err != nil {
		return changes
	}

	// Compare and collect changes
	// This is a basic implementation that only compares top-level fields
	for k, v := range newMap {
		if oldVal, exists := oldMap[k]; !exists {
			// New field added
			newVal, _ := json.Marshal(v)
			strVal := string(newVal)
			changes = append(changes, Change{
				Type:     "added",
				Path:     k,
				NewValue: &strVal,
			})
		} else if !jsonEqual(oldVal, v) {
			// Field modified
			oldValStr, _ := json.Marshal(oldVal)
			newValStr, _ := json.Marshal(v)
			oldStr := string(oldValStr)
			newStr := string(newValStr)
			changes = append(changes, Change{
				Type:     "modified",
				Path:     k,
				OldValue: &oldStr,
				NewValue: &newStr,
			})
		}
	}

	// Check for deleted fields
	for k := range oldMap {
		if _, exists := newMap[k]; !exists {
			// Field deleted
			oldVal, _ := json.Marshal(oldMap[k])
			strVal := string(oldVal)
			changes = append(changes, Change{
				Type:     "deleted",
				Path:     k,
				OldValue: &strVal,
			})
		}
	}

	return changes
}

func jsonEqual(a, b interface{}) bool {
	aBytes, _ := json.Marshal(a)
	bBytes, _ := json.Marshal(b)
	return bytes.Equal(aBytes, bBytes)
}
