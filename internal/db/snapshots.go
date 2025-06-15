package db

import (
	"fmt"
	"encoding/json"
	"log/slog"
	"strings"
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


type SnapshotFilterOptions struct {
	Fields   string // comma-separated list of fields to include
	Search   string // search term for item names
	ItemType string // filter by type: "folder" or "request"
	Depth    string // "shallow" for summary only, "deep" for full data
}

func GetSnapshotItemsFiltered(snapshotID string, collectionID string, page int, pageSize int, filters SnapshotFilterOptions) (map[string]interface{}, error) {

	var snapshotInfo struct {
		Exists         bool   `db:"exists"`
		CollectionName string `db:"collection_name"`
	}
	
	err := DB.Get(&snapshotInfo, `
		SELECT
			EXISTS(SELECT 1 FROM snapshots WHERE id = $1) as exists,
			COALESCE(
				content->'collection'->'collection'->>'name',
				''
			) as collection_name
		FROM snapshots
		WHERE id = $1 AND collection_id = $2
	`, snapshotID, collectionID)
	
	if err != nil || !snapshotInfo.Exists {
		slog.Error("failed to fetch snapshot info", "error", err)
		return nil, err
	}
	
	query, countQuery := buildFilteredQuery(snapshotID, filters)
	
	var totalItems int
	err = DB.Get(&totalItems, countQuery, snapshotID)
	if err != nil {
		slog.Error("failed to count items", "error", err)
		return nil, err
	}
	
	var result struct {
		Items json.RawMessage `json:"items"`
	}
	
	err = DB.Get(&result, query, snapshotID, pageSize, (page-1)*pageSize)
	if err != nil {
		slog.Error("failed to retrieve snapshot items", "error", err)
		return nil, err
	}
	
	return map[string]interface{}{
		"snapshot_id":      snapshotID,
		"collection_id":    collectionID,
		"collection_name":  snapshotInfo.CollectionName,
		"items":           result.Items,
		"filters_applied": filters,
		"pagination": map[string]interface{}{
			"page":        page,
			"page_size":   pageSize,
			"total_items": totalItems,
			"total_pages": (totalItems + pageSize - 1) / pageSize,
		},
	}, nil
}

func buildFilteredQuery(snapshotID string, filters SnapshotFilterOptions) (string, string) {
	var selectClause string
	var whereClause string
	
	if filters.Depth == "" {
		filters.Depth = "deep"
	}
	
	whereConditions := []string{}
	
	if filters.Search != "" {
		whereConditions = append(whereConditions, 
			`(item->>'name' ILIKE '%' || '` + filters.Search + `' || '%')`)
	}
	
	if filters.ItemType == "folder" {
		whereConditions = append(whereConditions, 
			`(item->'item' IS NOT NULL AND jsonb_array_length(item->'item') > 0)`)
	} else if filters.ItemType == "request" {
		whereConditions = append(whereConditions, 
			`(item->'request' IS NOT NULL)`)
	}
	
	if len(whereConditions) > 0 {
		whereClause = "WHERE " + strings.Join(whereConditions, " AND ")
	}
	
	// shallow - return metrics only
	if filters.Depth == "shallow" {

		selectClause = `
			jsonb_build_object(
				'id', item->>'id',
				'uid', item->>'uid',
				'name', item->>'name',
				'type', CASE 
					WHEN item->'request' IS NOT NULL THEN 'request'
					WHEN item->'item' IS NOT NULL THEN 'folder'
					ELSE 'unknown'
				END,
				'item_count', CASE 
					WHEN item->'item' IS NOT NULL 
					THEN jsonb_array_length(item->'item')
					ELSE 0
				END,
				'has_request', item->'request' IS NOT NULL,
				'method', item->'request'->'method',
				'url', item->'request'->'url'->>'raw'
			) as item`
	} else if filters.Fields != "" {
		// Custom field selection
		fields := strings.Split(filters.Fields, ",")
		fieldSelectors := []string{}
		
		for _, field := range fields {
			field = strings.TrimSpace(field)
			// Safe field selection - only allow specific fields
			switch field {
			case "id", "uid", "name":
				fieldSelectors = append(fieldSelectors, 
					fmt.Sprintf("'%s', item->>'%s'", field, field))
			case "request":
				fieldSelectors = append(fieldSelectors, 
					"'request', item->'request'")
			case "response":
				fieldSelectors = append(fieldSelectors, 
					"'response', item->'response'")
			case "event":
				fieldSelectors = append(fieldSelectors, 
					"'event', item->'event'")
			case "item":
				fieldSelectors = append(fieldSelectors, 
					"'item', item->'item'")
			}
		}
		
		if len(fieldSelectors) > 0 {
			selectClause = fmt.Sprintf("jsonb_build_object(%s) as item", 
				strings.Join(fieldSelectors, ", "))
		} else {
			selectClause = "item" // fallback to full item
		}
	} else {
		// Default: return full item
		selectClause = "item"
	}
	
	// Build main query
	query := fmt.Sprintf(`
		WITH items AS (
			SELECT %s
			FROM (
				SELECT jsonb_array_elements(
					content->'collection'->'collection'->'item'
				) as item
				FROM snapshots
				WHERE id = $1
			) t
			%s
		),
		paginated_items AS (
			SELECT item
			FROM items
			LIMIT $2 OFFSET $3
		)
		SELECT COALESCE(jsonb_agg(item), '[]'::jsonb) as items
		FROM paginated_items
	`, selectClause, whereClause)
	
	// Build count query
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)::int
		FROM (
			SELECT jsonb_array_elements(
				content->'collection'->'collection'->'item'
			) as item
			FROM snapshots
			WHERE id = $1
		) t
		%s
	`, whereClause)
	
	return query, countQuery
}

func GetSnapshotItemsFlattened(snapshotID string, collectionID string) ([]map[string]interface{}, error) {
	query := `
		WITH RECURSIVE item_tree AS (
			-- Base case: top-level items
			SELECT 
				jsonb_array_elements(content->'collection'->'collection'->'item') as item,
				'' as parent_id,
				0 as depth
			FROM snapshots
			WHERE id = $1 AND collection_id = $2
			
			UNION ALL
			
			-- Recursive case: nested items
			SELECT 
				jsonb_array_elements(it.item->'item') as item,
				it.item->>'id' as parent_id,
				it.depth + 1 as depth
			FROM item_tree it
			WHERE it.item->'item' IS NOT NULL 
				AND jsonb_array_length(it.item->'item') > 0
		)
		SELECT 
			jsonb_build_object(
				'id', item->>'id',
				'uid', item->>'uid', 
				'name', item->>'name',
				'parent_id', parent_id,
				'depth', depth,
				'type', CASE 
					WHEN item->'request' IS NOT NULL THEN 'request'
					WHEN item->'item' IS NOT NULL THEN 'folder'
					ELSE 'unknown'
				END,
				'method', item->'request'->'method',
				'url', item->'request'->'url'->>'raw'
			) as item_summary
		FROM item_tree
		ORDER BY depth, item->>'name'
	`
	
	rows, err := DB.Query(query, snapshotID, collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var items []map[string]interface{}
	for rows.Next() {
		var itemJSON json.RawMessage
		if err := rows.Scan(&itemJSON); err != nil {
			return nil, err
		}
		
		var item map[string]interface{}
		if err := json.Unmarshal(itemJSON, &item); err != nil {
			return nil, err
		}
		
		items = append(items, item)
	}
	
	return items, nil
}

func DeleteSnapshot(snapshotID int64)error {

	result, err := DB.Exec(
		`
		DELETE FROM snapshots
		WHERE id = $1
	`,
		snapshotID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete snapshot: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no data found for snapshot id: %d", snapshotID)
	}

	return nil
}

func DeleteSnapshotChanges(snapshotID int64) error {
	tx, err := DB.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	changesResult, err := tx.Exec(
		`DELETE FROM changes WHERE old_snapshot_id = $1`,
		snapshotID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete related changes: %v", err)
	}

	_, err = changesResult.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get changes rows affected: %v", err)
	}

	
	snapshotResult, err := tx.Exec(
		`DELETE FROM snapshots WHERE id = $1`,
		snapshotID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete snapshot: %v", err)
	}

	snapshotRowsAffected, err := snapshotResult.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get snapshot rows affected: %v", err)
	}

	if snapshotRowsAffected == 0 {
		return fmt.Errorf("no snapshot found with id: %d", snapshotID)
	}


	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	
	slog.Info("Successfully deleted snapshot and related changes\n", "snapshotID", "snapshotID")

	return nil
}
