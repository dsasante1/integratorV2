package handlers

import (
	"integratorV2/internal/db"
	"net/http"
	"github.com/labstack/echo/v4"
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

// this retrieves the actual contents of the snapshot ie the postman collection
func GetSnapshotItems(c echo.Context) error {
	snapshotID := c.Param("snapshotId")
	collectionID := c.Param("id")
	page := getPage(c)
	pageSize := getPageSize(c)

	itemsInfo, err := db.GetSnapshotItems(snapshotID, collectionID, page, pageSize)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to retrieve snapshot items"})
	}

	return c.JSON(http.StatusOK, itemsInfo)
}
