package db

import (
	"encoding/json"
	"log/slog"
	"time"
)


func GetCollectionSnapshots(collectionID string, offset int, pageSize int, page int) (map[string]interface{}, error){
	var snapshots []struct {
		ID             int             `db:"id" json:"id"`
		CollectionID   string          `db:"collection_id" json:"collection_id"`
		SnapshotTime   time.Time       `db:"snapshot_time" json:"snapshot_time"`
		CollectionName string          `db:"collection_name" json:"collection_name"`
		ItemCount      int             `db:"item_count" json:"item_count"`
		SizeKB         int             `db:"size_kb" json:"size_kb"`
	}
	
	
	err := DB.Select(&snapshots, `
		SELECT 
			s.id AS id,
			s.collection_id AS collection_id,
			s.snapshot_time AS snapshot_time,
			c.name as collection_name,
			jsonb_array_length(
				content->'collection'->'collection'->'item'
			) as item_count,
			pg_column_size(content::text)/1024 as size_kb
		FROM snapshots s
		LEFT JOIN collections c ON s.collection_id = c.id
		WHERE s.collection_id = $1
		ORDER BY s.snapshot_time DESC
		LIMIT $2 OFFSET $3
	`, collectionID, pageSize, offset)

	if err != nil {
		slog.Error("failed to fetch snapshots", "error", err)
		return nil, err
	}

	var total int
	err = DB.Get(&total,
		"SELECT COUNT(*) FROM snapshots WHERE collection_id = $1",
		collectionID)

	if err != nil {
		slog.Error("failed to fetch total count", "error", err)
		return nil, err
	}

	response := map[string]interface{}{
		"data": snapshots,
		"pagination": map[string]interface{}{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": (total + pageSize - 1) / pageSize,
		},
	}
	return response, nil
}

func GetSnapshotDetail(snapshotID string) (map[string]interface{}, error) {

	var snapshot Snapshot
	err := DB.Get(&snapshot, `
		SELECT * FROM snapshots
		WHERE id = $1
	`, snapshotID)

	if err != nil {
		slog.Error("failed to fetch snapshot detail", "error", err)
		return nil, err
	}

	return map[string]interface{}{
		"snapshot": snapshot,
	}, nil
}

func GetSnapshotItems(snapshotID string, collectionID string, page int, pageSize int) (map[string]interface{}, error) {

	var snapshotInfo struct {
		TotalItems     int    `db:"total_items" json:"total_items"`
		Exists         bool   `db:"exists" json:"exists"`
		CollectionName string `db:"collection_name" json:"collection_name"`
	}
	
	err := DB.Get(&snapshotInfo, `
		SELECT
			EXISTS(SELECT 1 FROM snapshots WHERE id = $1) as exists,
			COALESCE(
				jsonb_array_length(
					content->'collection'->'collection'->'item'
				), 0
			) as total_items,
			COALESCE(
				content->'collection'->'collection'->>'name',
				''
			) as collection_name
		FROM snapshots
		WHERE id = $1 AND collection_id = $2
	`, snapshotID, collectionID)
	
	if err != nil {
		slog.Error("failed to fetch snapshot items", "error", err)
		return nil, err
	}
	
	var result struct {
		Items json.RawMessage `json:"items"`
	}
	
	err = DB.Get(&result, `
		SELECT
			COALESCE(
				jsonb_agg(item)::jsonb,
				'[]'::jsonb
			) as items
		FROM (
			SELECT jsonb_array_elements(
				content->'collection'->'collection'->'item'
			) as item
			FROM snapshots
			WHERE id = $1
			LIMIT $2 OFFSET $3
		) t
	`, snapshotID, pageSize, (page-1)*pageSize)
	
	if err != nil {
		slog.Error("failed to retrieve snapshot items", "error", err)
		return nil, err
	}
	
	return map[string]interface{}{
		"snapshot_id":     snapshotID,
		"collection_id":   collectionID,
		"destination_name": snapshotInfo.CollectionName,
		"items":          result.Items,
		"pagination": map[string]interface{}{
			"page":        page,
			"page_size":   pageSize,
			"total_items": snapshotInfo.TotalItems,
			"total_pages": (snapshotInfo.TotalItems + pageSize - 1) / pageSize,
		},
	}, nil
}