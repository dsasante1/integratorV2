package handlers

import (
	// "database/sql"
	// "encoding/json"
	"integratorV2/internal/db"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

// GetChangeSummaryHandler returns a high-level summary of changes
func GetChangeSummaryHandler(c echo.Context) error{
	collectionID := c.Param("id")
	
	// Parse query parameters
	oldSnapshotStr := c.QueryParam("oldSnapshot")
	newSnapshotStr := c.QueryParam("newSnapshot")
	
	var oldSnapshotID *int64
	if oldSnapshotStr != "" {
		if id, err := strconv.ParseInt(oldSnapshotStr, 10, 64); err == nil {
			oldSnapshotID = &id
		}
	}
	
	newSnapshotID, err := strconv.ParseInt(newSnapshotStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "snapshot id required"})
	}
	
	summary, err := db.GetChangeSummary(collectionID, oldSnapshotID, newSnapshotID)
	if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch change summary"})
	}
	
		return c.JSON(http.StatusOK, summary)
}

// GetChangesHandler returns paginated list of changes with filtering
func GetChangesHandler(c echo.Context) error {
	
	collectionID := c.Param("id")
	
	// Parse query parameters
	filter := db.ChangeFilter{
		CollectionID: collectionID,
		Limit:        50,  // Default limit
		Offset:       0,
	}
	
	// Parse pagination
	if limit := c.QueryParam("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 && l <= 100 {
			filter.Limit = l
		}
	}
	
	if offset := c.QueryParam("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			filter.Offset = o
		}
	}
	
	// // Parse filters
	// if changeTypes := c.QueryParam("type"); len(changeTypes) > 0 {
	// 	filter.ChangeTypes = changeTypes
	// }
	
	if pattern := c.QueryParam("path"); pattern != "" {
		filter.PathPattern = pattern
	}
	
	if snapshot := c.QueryParam("snapshot"); snapshot != "" {
		if id, err := strconv.ParseInt(snapshot, 10, 64); err == nil {
			filter.SnapshotID = &id
		}
	}
	
	// Parse time filters
	if start := c.QueryParam("startTime"); start != "" {
		if t, err := time.Parse(time.RFC3339, start); err == nil {
			filter.StartTime = &t
		}
	}
	
	if end := c.QueryParam("endTime"); end != "" {
		if t, err := time.Parse(time.RFC3339, end); err == nil {
			filter.EndTime = &t
		}
	}
	
	changes, total, err := db.GetChanges(filter)
	if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch changes"})
	}
	
	response := map[string]interface{}{
		"changes": changes,
		"total":   total,
		"limit":   filter.Limit,
		"offset":  filter.Offset,
	}
	
		return c.JSON(http.StatusOK, response)
}

// GetChangeHierarchyHandler returns changes in a tree structure
func GetChangeHierarchyHandler(c echo.Context) error {
	
	collectionID := c.Param("id")
	snapshotID := c.Param("snapshotId")
	
	snapshot, err := strconv.ParseInt(snapshotID, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Failed to fetch snapshots"})
	}
	
	hierarchy, err := db.GetChangeHierarchy(collectionID, snapshot)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Failed to fetch hierarchy"})
	}
	
	return c.JSON(http.StatusOK, hierarchy)
}

// GetChangesByEndpointHandler returns changes grouped by endpoint
func GetChangesByEndpointHandler(c echo.Context) error{
	
	collectionID := c.Param("id")
	snapshotID := c.Param("snapshotId")
	
	snapshot, err := strconv.ParseInt(snapshotID, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Failed to fetch snapshots"})
	}
	
	endpointChanges, err := db.GetChangesByEndpoint(collectionID, snapshot)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Failed to fetch endpoint changes"})
	}
	
	return c.JSON(http.StatusOK, endpointChanges)
}

// GetChangeDetailsHandler returns details for a specific change
// func GetChangeDetailsHandler(c echo.Context) error {
	
// 	changeID := c.Param("changeId")
	
// 	id, err := strconv.ParseInt(changeID, 10, 64)
// 	if err != nil {
// 		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid change id"})
// 	}
	
// 	// Query for specific change
// 	query := `
// 		SELECT 
// 			id, collection_id, old_snapshot_id, new_snapshot_id,
// 			change_type, path, modification, created_at
// 		FROM changes
// 		WHERE id = $1
// 	`
	
// 	change := &db.ChangeDetail{}
// 	err = db.DB.QueryRow(query, id).Scan(
// 		&change.ID,
// 		&change.CollectionID,
// 		&change.OldSnapshotID,
// 		&change.NewSnapshotID,
// 		&change.ChangeType,
// 		&change.Path,
// 		&change.Modification,
// 		&change.CreatedAt,
// 	)
	
// 	if err != nil {
// 		if err == sql.ErrNoRows {
// 			return c.JSON(http.StatusNotFound, map[string]string{"error": "change not found"})
// 		} 
// 			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "fetch change failed"})
// 	}
	
	
// 	// Parse modification for better display
// 	if change.Modification != nil {
// 		var parsed interface{}
// 		if err := json.Unmarshal([]byte(*change.Modification), &parsed); err == nil {
// 			response := map[string]interface{}{
// 				"change": change,
// 				"parsedModification": parsed,
// 			}
// 				return c.JSON(http.StatusOK, response)
// 		}
// 	}
	
// 	return nil
// }
