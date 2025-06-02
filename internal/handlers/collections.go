package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"integratorV2/internal/db"
	"integratorV2/internal/postman"

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

	// Get API key
	apiKey, err := db.GetPostmanAPIKey(userID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "No API key found. Please store your Postman API key first."})
	}

	var req StoreCollectionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	if req.CollectionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Collection ID is required"})
	}

	// Get collection from Postman
	collection, err := postman.GetCollection(apiKey, req.CollectionID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch collection from Postman"})
	}

	// Store collection in database
	if err := db.StoreCollection(collection.Collection.ID, collection.Collection.Name); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to store collection"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Collection stored successfully"})
}

func GetCollectionDetails(c echo.Context) error {
	// Get user ID from JWT token
	userID := c.Get("user_id").(int64)

	// Get collection ID from path
	collectionID := c.Param("id")
	if collectionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Collection ID is required"})
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

	// Get snapshots
	var snapshots []db.Snapshot
	err = db.DB.Select(&snapshots, `
		SELECT * FROM snapshots
		WHERE collection_id = $1
		ORDER BY snapshot_time DESC
	`, collectionID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch collection history"})
	}

	// Get changes
	var changes []db.Change
	err = db.DB.Select(&changes, `
		SELECT * FROM changes
		WHERE collection_id = $1
		ORDER BY change_time DESC
	`, collectionID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch collection changes"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"collection": collection,
		"snapshots":  snapshots,
		"changes":    changes,
	})
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
