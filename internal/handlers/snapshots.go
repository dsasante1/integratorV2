package handlers

import (
	"integratorV2/internal/db"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
)

// GetSnapshotDetail retrieves a specific snapshot with optional field filtering
func GetSnapshotDetail(c echo.Context) error {
	snapshotID := c.Param("snapshotId")

	snapshotDetails, err := db.GetSnapshotDetail(snapshotID)

	if err != nil {
		return c.JSON(http.StatusNotFound,
			map[string]string{"error": "Snapshot not found"})
	}

	return c.JSON(http.StatusOK, snapshotDetails)
}

func GetSnapshotItems(c echo.Context) error {
	snapshotID := c.Param("snapshotId")
	collectionID := c.Param("id")
	page := getPage(c)
	pageSize := getPageSize(c)

	// Parse filtering options
	fields := c.QueryParam("fields")
	search := c.QueryParam("search")
	itemType := c.QueryParam("type") // "folder" or "request"
	depth := c.QueryParam("depth")   // "shallow" or "deep" (default)

	filterOptions := db.SnapshotFilterOptions{
		Fields:   fields,
		Search:   search,
		ItemType: itemType,
		Depth:    depth,
	}

	itemsInfo, err := db.GetSnapshotItemsFiltered(snapshotID, collectionID, page, pageSize, filterOptions)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to retrieve snapshot items"})
	}

	return c.JSON(http.StatusOK, itemsInfo)
}

func GetSnapshotItemTree(c echo.Context) error {
	snapshotID := c.Param("snapshotId")
	collectionID := c.Param("id")

	items, err := db.GetSnapshotItemsFlattened(snapshotID, collectionID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to retrieve item tree"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"snapshot_id":   snapshotID,
		"collection_id": collectionID,
		"items":         items,
		"total":         len(items),
	})
}

func DeleteSnapshot(c echo.Context) error {
	snapshotID := c.Param("id")
	id, err := strconv.ParseInt(snapshotID, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid snapshotID"})
	}

	if err := db.DeleteSnapshot(id); err != nil {
		slog.Error("Failed to delete snapshot", "error", err)
		
		// Check if it's a foreign key constraint error
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == "23503" { 
				if strings.Contains(pqErr.Message, "changes_old_snapshot_id_fkey") {
					return c.JSON(http.StatusConflict, map[string]string{
						"error": "This snapshot can’t be deleted because it’s linked to existing changes. Please update or remove those related changes before trying again.",
					})
				}
				return c.JSON(http.StatusConflict, map[string]string{
					"error": "Cannot delete snapshot as it is referenced by other records.",
				})
			}
		}
		
		if strings.Contains(err.Error(), "changes_old_snapshot_id_fkey") {
			return c.JSON(http.StatusConflict, map[string]string{
				"error": "Cannot delete snapshot as it is referenced in the changes table. Please remove or update the related changes first.",
			})
		}
		
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete snapshot"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "snapshot deleted successfully"})
}

func DeleteSnapshotChanges(c echo.Context) error {
		snapshotID := c.Param("id")
	id, err := strconv.ParseInt(snapshotID, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid snapshotID"})
	}

	if err := db.DeleteSnapshotChanges(id); err != nil {
		slog.Error("Failed to delete snapshot", "error", err)
	}
return c.JSON(http.StatusOK, map[string]string{"message": "snapshot deleted successfully"})
}
