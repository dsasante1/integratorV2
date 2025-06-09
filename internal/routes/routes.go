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
	collections.POST("/save-collection", handlers.SaveCollection)

	collections.GET("/:id", handlers.GetCollection)

	collections.GET("/:id/snapshots", handlers.GetCollectionSnapshots)
	collections.GET("/:id/snapshots/:snapshotId", handlers.GetSnapshotDetail)
	collections.GET("/:id/snapshots/:snapshotId/items", handlers.GetSnapshotItems)

	collections.GET("/:id/changes", handlers.GetCollectionChanges)
	collections.GET("/compare/:id", handlers.CompareCollections)

	jobs := api.Group("/jobs")
	jobs.GET("", handlers.GetUserJobs)
	jobs.GET("/:id", handlers.GetJobStatus)
}
