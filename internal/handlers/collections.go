package handlers

import (
	"encoding/json"
	"net/http"

	"integratorV2/internal/db"
	"integratorV2/internal/postman"

	"github.com/labstack/echo/v4"
)

type APIKeyRequest struct {
	APIKey string `json:"api_key"`
}

type StoreCollectionRequest struct {
	CollectionID string `json:"collection_id"`
}

func StoreAPIKey(c echo.Context) error {
	// Get user ID from JWT token
	userID := c.Get("user_id").(int64)

	var req APIKeyRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	if req.APIKey == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "API key is required"})
	}

	// Store API key
	if err := db.StorePostmanAPIKey(userID, req.APIKey); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to store API key"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "API key stored successfully"})
}

func GetCollections(c echo.Context) error {
	// Get user ID from JWT token
	userID := c.Get("user_id").(int64)

	// Get API key
	apiKey, err := db.GetPostmanAPIKey(userID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "No API key found. Please store your Postman API key first."})
	}

	// Update last used timestamp
	if err := db.UpdateLastUsedAPIKey(userID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update API key usage"})
	}

	// Get collections from Postman
	collections, err := postman.GetCollections(apiKey)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch collections from Postman"})
	}

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
