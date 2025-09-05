package db

import (
	"encoding/json"
	"fmt"
	"reflect"
	"log/slog"
)


type CompareResult struct {
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	Modified []struct {
		Path     string `json:"path"`
		OldValue string `json:"old_value"`
		NewValue string `json:"new_value"`
	} `json:"modified"`
}


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

	
	compareMaps("", oldContent, newContent, result)

	return result, nil
}


func compareMaps(path string, oldMap, newMap map[string]interface{}, result *CompareResult) {
	
	for key, newValue := range newMap {
		newPath := key
		if path != "" {
			newPath = path + "." + key
		}

		oldValue, exists := oldMap[key]
		if !exists {
			
			result.Added = append(result.Added, newPath)
			continue
		}

		
		if !reflect.DeepEqual(oldValue, newValue) {
			if oldMap, ok := oldValue.(map[string]interface{}); ok {
				if newMap, ok := newValue.(map[string]interface{}); ok {
					
					compareMaps(newPath, oldMap, newMap, result)
					continue
				}
			}

			
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