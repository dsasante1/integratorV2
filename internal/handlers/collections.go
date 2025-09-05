package handlers

import (
	// "encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"integratorV2/internal/config"
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

type APIKeyResponse struct {
	ID            int64     `json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	LastUsedAt    time.Time `json:"last_used_at"`
	LastRotatedAt time.Time `json:"last_rotated_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	IsActive      bool      `json:"is_active"`
	MaskedKey     string    `json:"masked_key"`
}


func getPage(c echo.Context) int {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		return 1
	}
	return page
}

func getPageSize(c echo.Context) int {
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
	if pageSize < 1 {
		return 10
	}
	if pageSize > 100 {
		return 100
	}
	return pageSize
}

func StoreAPIKey(c echo.Context) error {
	
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

	
	if err := db.StorePostmanAPIKey(userID, req.APIKey); err != nil {
		slog.Error("Failed to store API key", "error", err, "user_id", userID)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to store API key"})
	}

	slog.Info("Successfully stored API key", "user_id", userID)
	return c.JSON(http.StatusOK, map[string]string{"message": "API key stored successfully"})
}

func RotateAPIKey(c echo.Context) error {
	
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

	if err := db.RotateAPIKey(userID, req.NewAPIKey); err != nil {
		slog.Error("Failed to rotate API key", "error", err, "user_id", userID)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to rotate API key"})
	}

	slog.Info("Successfully rotated API key", "user_id", userID)
	return c.JSON(http.StatusOK, map[string]string{"message": "API key rotated successfully"})
}

func GetCollections(c echo.Context) error {
	
	userID := c.Get("user_id").(int64)

	apiKey, err := db.GetPostmanAPIKey(userID)
	if err != nil {
		slog.Warn("No active API key found", "error", err, "user_id", userID)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "No active API key found. Please store your Postman API key first."})
	}

	if err := db.UpdateLastUsedAPIKey(userID); err != nil {
		slog.Error("Failed to update API key usage", "error", err, "user_id", userID)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update API key usage"})
	}

	collections, err := postman.GetCollections(apiKey)
	if err != nil {
		slog.Error("Failed to fetch collections from Postman", "error", err, "user_id", userID)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch collections from Postman"})
	}

	slog.Info("Successfully fetched collections", "user_id", userID, "count", len(collections))
	return c.JSON(http.StatusOK, collections)
}

func SaveCollection(c echo.Context) error {

	userID := c.Get("user_id").(int64)

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

	payload := queue.CollectionImportPayload{
		UserID:       userID,
		CollectionID: req.CollectionID,
		Name:         req.Name,
	}

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

func GetCollectionSnapshots(c echo.Context) error {
	collectionID := c.Param("id")
	page := getPage(c)
	if page < 1 {
		page = 1
	}
	pageSize := getPageSize(c)
	if pageSize < 1 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	snapshots, err := db.GetCollectionSnapshots(collectionID, offset, pageSize, page)
	slog.Info("Error", "error", err)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch snapshots"})
	}

	return c.JSON(http.StatusOK, snapshots)
}

func GetCollectionChanges(c echo.Context) error {
	
	collectionID := c.Param("id")
	if collectionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Collection ID is required"})
	}

	
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

	change, err := db.GetCollectionChanges(collectionID, page, pageSize)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "fetch collection changes failed"})
	}

	return c.JSON(http.StatusOK, change)
}

func GetJobStatus(c echo.Context) error {
	
	userID := c.Get("user_id").(int64)

	
	jobID := c.Param("id")
	if jobID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Job ID is required"})
	}

	id, err := strconv.ParseInt(jobID, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid job ID"})
	}

	job, err := db.GetCollectionJob(id)
	if err != nil {
		slog.Error("Failed to get job status", "error", err, "user_id", userID, "job_id", id)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get job status"})
	}

	
	if job.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Access denied"})
	}

	return c.JSON(http.StatusOK, job)
}

func GetUserJobs(c echo.Context) error {
	
	userID := c.Get("user_id").(int64)

	
	jobs, err := db.GetUserCollectionJobs(userID)
	if err != nil {
		slog.Error("Failed to get user jobs", "error", err, "user_id", userID)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get user jobs"})
	}

	return c.JSON(http.StatusOK, jobs)
}

func CompareSnapShots(c echo.Context) error {
	
	userID := c.Get("user_id").(int64)

	
	collectionID := c.Param("collectionId")
	if collectionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Collection ID is required"})
	}

	
	latestSnapshot, previousSnapshot, err := db.GetLatestSnapshots(collectionID)
	if err != nil {
		slog.Error("Failed to get snapshots for comparison", "error", err, "user_id", userID, "collection_id", collectionID)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get snapshots for comparison"})
	}

	
	result, err := db.CompareSnapShots(previousSnapshot, latestSnapshot)
	if err != nil {
		slog.Error("Failed to compare collections", "error", err, "user_id", userID, "collection_id", collectionID)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to compare collections"})
	}

	return c.JSON(http.StatusOK, result)
}

//TODO move to utilities
func maskAPIKey(key string) string {
	if len(key) < 8 {
		return "PMAK-XXXXXXXXXXXX"
	}

	prefix := key[:6]
	suffix := key[len(key)-4:]
	return prefix + "XXXXXXXXXXXXX" + suffix
}

func GetAPIKeys(c echo.Context) error {
	
	userID := c.Get("user_id").(int64)

	
	keys, err := db.GetAPIKeyInfo(userID)
	if err != nil {
		slog.Error("Failed to get API keys", "error", err, "user_id", userID)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get API keys"})
	}

	
	var response []APIKeyResponse
	for _, key := range keys {
		
		decryptedKey, err := config.DecryptAPIKey(key.EncryptedKey)
		if err != nil {
			slog.Error("Failed to decrypt API key", "error", err, "user_id", userID, "key_id", key.ID)
			continue
		}

		
		response = append(response, APIKeyResponse{
			ID:            key.ID,
			CreatedAt:     key.CreatedAt,
			LastUsedAt:    key.LastUsedAt,
			LastRotatedAt: key.LastRotatedAt,
			ExpiresAt:     key.ExpiresAt,
			IsActive:      key.IsActive,
			MaskedKey:     maskAPIKey(decryptedKey),
		})
	}

	return c.JSON(http.StatusOK, response)
}

func DeleteAPIKey(c echo.Context) error {
	
	userID := c.Get("user_id").(int64)

	
	keyID := c.Param("id")
	if keyID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Key ID is required"})
	}

	id, err := strconv.ParseInt(keyID, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid key ID"})
	}

	if err := db.DeleteAPIKey(id, userID); err != nil {
		slog.Error("Failed to delete API key", "error", err, "user_id", userID, "key_id", id)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete API key"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "API key deleted successfully"})
}

func GetUserCollections(c echo.Context) error {
	
	userID := c.Get("user_id").(int64)

	collections, err := db.GetUserCollections(userID)
	if err != nil {
		slog.Error("Failed to get user collections", "error", err, "user_id", userID)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get collections"})
	}

	return c.JSON(http.StatusOK, collections)
}
