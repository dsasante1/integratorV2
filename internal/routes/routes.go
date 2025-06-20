package routes

import (
	"integratorV2/internal/auth"
	"integratorV2/internal/handlers"
	"integratorV2/internal/security"

	"github.com/labstack/echo/v4"
)

func SetupRoutes(api *echo.Group) {

	api.GET("/health", handlers.HealthCheck)

	authGroup := api.Group("/auth")
	authGroup.Use(security.ValidateEmail)
	authGroup.POST("/signup", handlers.Signup)
	authGroup.POST("/login", handlers.Login)


	api.Use(auth.JWTMiddleware)

	keys := api.Group("/keys")
	keys.POST("/api-key", handlers.StoreAPIKey)
	keys.GET("/api-keys", handlers.GetAPIKeys)
	keys.DELETE("/api-key/:id", handlers.DeleteAPIKey)


	collections := api.Group("/collections")
	collections.POST("/api-key/rotate", handlers.RotateAPIKey)
	collections.GET("", handlers.GetCollections)
	collections.GET("/user", handlers.GetUserCollections)
	// save collection or create snapshot
	collections.POST("/save-collection", handlers.SaveCollection)

	collections.GET("/:id/snapshots", handlers.GetCollectionSnapshots)
	collections.GET("/:id/snapshots/:snapshotId", handlers.GetSnapshotDetail)
	collections.GET("/:id/snapshots/:snapshotId/items", handlers.GetSnapshotItems)
	collections.DELETE("/snapshot/:id", handlers.DeleteSnapshot)
	// delete snapshot and changes data
	collections.DELETE("snapshot/changes/:id", handlers.DeleteSnapshotChanges)

	collections.GET("/:id/changes", handlers.GetCollectionChanges)
	collections.GET("/snapshot/compare/:collectionId", handlers.CompareSnapShots)


	collections.GET("/:collectionId/snapshot-id", handlers.GetSnapshotID)
	collections.GET("/:collectionId/changes/summary/:oldSnapshot/:newSnapshot", handlers.GetChangeSummary)
	
	// List changes with filtering
	collections.GET("/:collectionId/changes", handlers.GetChanges)
	
	// Hierarchical view
	collections.GET("/:id/snapshots/:snapshotId/hierarchy", handlers.GetChangeHierarchy)
	
	// Changes by endpoint
	collections.GET("/:collectionId/snapshots/:snapshotId/by-endpoint", handlers.GetChangesByEndpoint)
	
	// Change details
	collections.GET("/changes/:changeId", handlers.GetChangeDetails)

	collections.GET("/:collectionId/change/summary", handlers.GetCollectionChangeSummary)
	
	// Export changes
	collections.GET("/:id/changes/export", handlers.ExportChanges)

	//show snapshot diff like github
	collections.GET("/:collectionId/changes/diff/:snapshotId", handlers.GetSnapshotDiff)
	collections.GET("/:collectionId/diff/:snapshotId", handlers.GetSnapshotDiffID)
	
	// Advanced analysis endpoints
	collections.GET("/:collectionId/snapshots/:snapshotId/impact-analysis", handlers.GetChangeImpactAnalysis)
	collections.GET("/:collectionId/changes/frequency-analysis", handlers.GetChangeFrequencyAnalysis)
	collections.GET("/:collectionId/snapshots/compare", handlers.CompareSnapshots)

	jobs := api.Group("/jobs")
	jobs.GET("", handlers.GetUserJobs)
	jobs.GET("/:id", handlers.GetJobStatus)
}
