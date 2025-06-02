package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"integratorV2/internal/db"
	"integratorV2/internal/postman"
	"integratorV2/internal/queue"

	"github.com/labstack/echo/v4"
)

type APIKeyRequest struct {
	APIKey string `json:"api_key" validate:"required"`
}

type RotateAPIKeyRequest struct {
	NewAPIKey string `json:"new_api_key" validate:"required"`
}

type StoreCollectionRequest struct {
	CollectionID string `json:"collection_id" validate:"required"`
	Name         string `json:"name" validate:"required"`
}

func StoreAPIKey(c echo.Context) error {
	// Get user ID from JWT token
	userID := c.Get("user_id").(int64)

	var req APIKeyRequest
	if err := c.Bind(&req); err != nil {
		slog.Error("Invalid request", "error", err, "user_id", userID)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	if req.APIKey == "" {
		slog.Warn("Empty API key provided", "user_id", userID)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "API key is required"})
	}

	// Store API key
	if err := db.StorePostmanAPIKey(userID, req.APIKey); err != nil {
		slog.Error("Failed to store API key", "error", err, "user_id", userID)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to store API key"})
	}

	slog.Info("Successfully stored API key", "user_id", userID)
	return c.JSON(http.StatusOK, map[string]string{"message": "API key stored successfully"})
}

func RotateAPIKey(c echo.Context) error {
	// Get user ID from JWT token
	userID := c.Get("user_id").(int64)

	var req RotateAPIKeyRequest
	if err := c.Bind(&req); err != nil {
		slog.Error("Invalid request", "error", err, "user_id", userID)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	if req.NewAPIKey == "" {
		slog.Warn("Empty new API key provided", "user_id", userID)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "New API key is required"})
	}

	// Rotate API key
	if err := db.RotateAPIKey(userID, req.NewAPIKey); err != nil {
		slog.Error("Failed to rotate API key", "error", err, "user_id", userID)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to rotate API key"})
	}

	slog.Info("Successfully rotated API key", "user_id", userID)
	return c.JSON(http.StatusOK, map[string]string{"message": "API key rotated successfully"})
}

func GetCollections(c echo.Context) error {
	// Get user ID from JWT token
	userID := c.Get("user_id").(int64)

	// Get API key
	apiKey, err := db.GetPostmanAPIKey(userID)
	if err != nil {
		slog.Warn("No active API key found", "error", err, "user_id", userID)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "No active API key found. Please store your Postman API key first."})
	}

	// Update last used timestamp
	if err := db.UpdateLastUsedAPIKey(userID); err != nil {
		slog.Error("Failed to update API key usage", "error", err, "user_id", userID)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update API key usage"})
	}

	// Get collections from Postman
	collections, err := postman.GetCollections(apiKey)
	if err != nil {
		slog.Error("Failed to fetch collections from Postman", "error", err, "user_id", userID)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch collections from Postman"})
	}

	slog.Info("Successfully fetched collections", "user_id", userID, "count", len(collections))
	return c.JSON(http.StatusOK, collections)
}

func StoreCollection(c echo.Context) error {
	// Get user ID from JWT token
	userID := c.Get("user_id").(int64)

	// Get API key to verify it exists
	_, err := db.GetPostmanAPIKey(userID)
	if err != nil {
		slog.Error("No API key found", "error", err, "user_id", userID)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "No API key found. Please store your Postman API key first."})
	}

	var req StoreCollectionRequest
	if err := c.Bind(&req); err != nil {
		slog.Error("Invalid request", "error", err, "user_id", userID)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	if req.CollectionID == "" {
		slog.Warn("Empty collection ID provided", "user_id", userID)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Collection ID is required"})
	}

	if req.Name == "" {
		slog.Warn("Empty collection name provided", "user_id", userID)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Collection name is required"})
	}

	// Create task payload
	payload := queue.CollectionImportPayload{
		UserID:       userID,
		CollectionID: req.CollectionID,
		Name:         req.Name,
	}

	// Enqueue task
	taskID, err := queue.EnqueueCollectionImport(payload)
	if err != nil {
		slog.Error("Failed to enqueue collection import", "error", err, "user_id", userID)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to start collection import"})
	}

	slog.Info("Enqueued collection import",
		"user_id", userID,
		"collection_id", req.CollectionID,
		"name", req.Name,
		"task_id", taskID)

	return c.JSON(http.StatusAccepted, map[string]interface{}{
		"message": "Collection import started",
		"task_id": taskID,
	})
}

func GetCollection(c echo.Context) error {
	// Get user ID from JWT token
	userID := c.Get("user_id").(int64)

	// Get collection ID from path
	collectionID := c.Param("id")
	if collectionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Collection ID is required"})
	}

	// Get pagination parameters with defaults
	page := 1
	pageSize := 10

	if pageStr := c.QueryParam("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := c.QueryParam("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	// Get API key
	apiKey, err := db.GetPostmanAPIKey(userID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "No API key found. Please store your Postman API key first."})
	}

	// Get collection from Postman
	collection, err := postman.GetCollection(apiKey, collectionID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch collection from Postman"})
	}

	// Store collection content as snapshot
	content, _ := json.Marshal(collection)
	if err := postman.StoreCollectionSnapshot(collectionID, content); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to store collection snapshot"})
	}

	// Parse the items from the collection
	var items []interface{}
	if err := json.Unmarshal(collection.Collection.Item, &items); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to parse collection items"})
	}

	// Calculate pagination for items
	totalItems := len(items)
	start := (page - 1) * pageSize
	end := start + pageSize
	if end > totalItems {
		end = totalItems
	}

	// Get paginated items
	var paginatedItems []interface{}
	if start < totalItems {
		paginatedItems = items[start:end]
	}

	// Create paginated response
	response := map[string]interface{}{
		"id":   collection.Collection.ID,
		"name": collection.Collection.Name,
		"info": collection.Collection.Info,
		"item": paginatedItems,
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": response,
		"pagination": map[string]interface{}{
			"page":        page,
			"page_size":   pageSize,
			"total":       totalItems,
			"total_pages": (totalItems + pageSize - 1) / pageSize,
		},
	})
}

func GetCollectionSnapshots(c echo.Context) error {
	// Get collection ID from path
	collectionID := c.Param("id")
	if collectionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Collection ID is required"})
	}

	// Get pagination parameters with defaults
	page := 1
	pageSize := 10

	if pageStr := c.QueryParam("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := c.QueryParam("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	offset := (page - 1) * pageSize

	// Get total count of snapshots
	var totalSnapshots int
	err := db.DB.Get(&totalSnapshots, `
		SELECT COUNT(*) FROM snapshots
		WHERE collection_id = $1
	`, collectionID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch collection history"})
	}

	// Get paginated snapshots
	var snapshots []db.Snapshot
	err = db.DB.Select(&snapshots, `
		SELECT * FROM snapshots
		WHERE collection_id = $1
		ORDER BY snapshot_time DESC
		LIMIT $2 OFFSET $3
	`, collectionID, pageSize, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch collection history"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": snapshots,
		"pagination": map[string]interface{}{
			"page":        page,
			"page_size":   pageSize,
			"total":       totalSnapshots,
			"total_pages": (totalSnapshots + pageSize - 1) / pageSize,
		},
	})
}

func GetCollectionChanges(c echo.Context) error {
	// Get collection ID from path
	collectionID := c.Param("id")
	if collectionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Collection ID is required"})
	}

	// Get pagination parameters with defaults
	page := 1
	pageSize := 10

	if pageStr := c.QueryParam("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := c.QueryParam("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	offset := (page - 1) * pageSize

	// Get total count of changes
	var totalChanges int
	err := db.DB.Get(&totalChanges, `
		SELECT COUNT(*) FROM changes
		WHERE collection_id = $1
	`, collectionID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch collection changes"})
	}

	// Get paginated changes
	var changes []db.Change
	err = db.DB.Select(&changes, `
		SELECT * FROM changes
		WHERE collection_id = $1
		ORDER BY change_time DESC
		LIMIT $2 OFFSET $3
	`, collectionID, pageSize, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch collection changes"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": changes,
		"pagination": map[string]interface{}{
			"page":        page,
			"page_size":   pageSize,
			"total":       totalChanges,
			"total_pages": (totalChanges + pageSize - 1) / pageSize,
		},
	})
}

func GetJobStatus(c echo.Context) error {
	// Get user ID from JWT token
	userID := c.Get("user_id").(int64)

	// Get job ID from path
	jobID := c.Param("id")
	if jobID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Job ID is required"})
	}

	// Convert job ID to int64
	id, err := strconv.ParseInt(jobID, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid job ID"})
	}

	// Get job status
	job, err := db.GetCollectionJob(id)
	if err != nil {
		slog.Error("Failed to get job status", "error", err, "user_id", userID, "job_id", id)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get job status"})
	}

	// Verify the job belongs to the user
	if job.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Access denied"})
	}

	return c.JSON(http.StatusOK, job)
}

func GetUserJobs(c echo.Context) error {
	// Get user ID from JWT token
	userID := c.Get("user_id").(int64)

	// Get user's jobs
	jobs, err := db.GetUserCollectionJobs(userID)
	if err != nil {
		slog.Error("Failed to get user jobs", "error", err, "user_id", userID)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get user jobs"})
	}

	return c.JSON(http.StatusOK, jobs)
}

func CompareCollections(c echo.Context) error {
	// Get user ID from JWT token
	userID := c.Get("user_id").(int64)

	// Get collection ID from path
	collectionID := c.Param("id")
	if collectionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Collection ID is required"})
	}

	// Get latest snapshots
	latestSnapshot, previousSnapshot, err := db.GetLatestSnapshots(collectionID)
	if err != nil {
		slog.Error("Failed to get snapshots for comparison", "error", err, "user_id", userID, "collection_id", collectionID)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get snapshots for comparison"})
	}

	// Compare snapshots
	result, err := db.CompareCollections(previousSnapshot, latestSnapshot)
	if err != nil {
		slog.Error("Failed to compare collections", "error", err, "user_id", userID, "collection_id", collectionID)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to compare collections"})
	}

	return c.JSON(http.StatusOK, result)
}
