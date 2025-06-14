package handlers

import (
	"github.com/labstack/echo/v4"
	"integratorV2/internal/db"
	"net/http"
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