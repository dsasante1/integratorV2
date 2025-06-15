package db

import (
	"encoding/json"
	"fmt"
	"reflect"
	"log/slog"
)

// CompareResult represents the differences between two collections
type CompareResult struct {
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	Modified []struct {
		Path     string `json:"path"`
		OldValue string `json:"old_value"`
		NewValue string `json:"new_value"`
	} `json:"modified"`
}

// CompareSnapShots compares two collection snapshots and returns their differences
func CompareSnapShots(oldSnapshot, newSnapshot *Snapshot) (*CompareResult, error) {
	if oldSnapshot == nil || newSnapshot == nil {
		return nil, fmt.Errorf("both snapshots must be provided")
	}

	var oldContent, newContent map[string]interface{}
	if err := json.Unmarshal(oldSnapshot.Content, &oldContent); err != nil {
		return nil, fmt.Errorf("error unmarshaling old snapshot: %v", err)
	}
	if err := json.Unmarshal(newSnapshot.Content, &newContent); err != nil {
		return nil, fmt.Errorf("error unmarshaling new snapshot: %v", err)
	}

	result := &CompareResult{
		Added:   make([]string, 0),
		Removed: make([]string, 0),
		Modified: make([]struct {
			Path     string `json:"path"`
			OldValue string `json:"old_value"`
			NewValue string `json:"new_value"`
		}, 0),
	}

	// Compare the collections recursively
	compareMaps("", oldContent, newContent, result)

	return result, nil
}

// compareMaps recursively compares two maps and updates the CompareResult
func compareMaps(path string, oldMap, newMap map[string]interface{}, result *CompareResult) {
	// Check for added and modified items
	for key, newValue := range newMap {
		newPath := key
		if path != "" {
			newPath = path + "." + key
		}

		oldValue, exists := oldMap[key]
		if !exists {
			// Item was added
			result.Added = append(result.Added, newPath)
			continue
		}

		// Compare values
		if !reflect.DeepEqual(oldValue, newValue) {
			if oldMap, ok := oldValue.(map[string]interface{}); ok {
				if newMap, ok := newValue.(map[string]interface{}); ok {
					// Recursively compare nested maps
					compareMaps(newPath, oldMap, newMap, result)
					continue
				}
			}

			// Values are different
			result.Modified = append(result.Modified, struct {
				Path     string `json:"path"`
				OldValue string `json:"old_value"`
				NewValue string `json:"new_value"`
			}{
				Path:     newPath,
				OldValue: fmt.Sprintf("%v", oldValue),
				NewValue: fmt.Sprintf("%v", newValue),
			})
		}
	}

	// Check for removed items
	for key := range oldMap {
		if _, exists := newMap[key]; !exists {
			removedPath := key
			if path != "" {
				removedPath = path + "." + key
			}
			result.Removed = append(result.Removed, removedPath)
		}
	}
}

// GetLatestSnapshots returns the two most recent snapshots for a collection
func GetLatestSnapshots(collectionID string) (*Snapshot, *Snapshot, error) {
	var snapshots []Snapshot
	err := DB.Select(&snapshots, `
		SELECT * FROM snapshots
		WHERE collection_id = $1
		ORDER BY snapshot_time DESC
		LIMIT 2
	`, collectionID)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting snapshots: %v", err)
	}

	if len(snapshots) < 2 {
		return nil, nil, fmt.Errorf("not enough snapshots to compare")
	}

	return &snapshots[0], &snapshots[1], nil
}



func GetCollectionChanges(collectionID string, page int, pageSize int) (ChangesResponse, error) {
	offset := (page - 1) * pageSize

	var totalChanges int
	err := DB.Get(&totalChanges, `
		SELECT COUNT(*) FROM changes
		WHERE collection_id = $1
	`, collectionID)
	if err != nil {
		slog.Error("an error occurred fetching collection changes", "error", err)
		return ChangesResponse{}, fmt.Errorf("get total changes failed")
	}


	var changes []Change
	err = DB.Select(&changes, `
		SELECT * FROM changes
		WHERE collection_id = $1
		ORDER BY change_time DESC
		LIMIT $2 OFFSET $3
	`, collectionID, pageSize, offset)
	if err != nil {
		slog.Error("an error occurred fetching collection changes", "error", err)
		return ChangesResponse{}, fmt.Errorf("get paginated changes failed")
	}

	return ChangesResponse{
		Data: changes,
		Pagination: map[string]interface{}{
			"page":        page,
			"page_size":   pageSize,
			"total":       totalChanges,
			"total_pages": (totalChanges + pageSize - 1) / pageSize,
		},
	}, nil
}