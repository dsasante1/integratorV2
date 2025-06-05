package routes

import (
	"integratorV2/internal/auth"
	"integratorV2/internal/handlers"
	"integratorV2/internal/security"

	"github.com/labstack/echo/v4"
)

func SetupRoutes(api *echo.Group) {
	// Public routes
	api.GET("/health", handlers.HealthCheck)

	// Auth routes with email validation
	authGroup := api.Group("/auth")
	authGroup.Use(security.ValidateEmail)
	authGroup.POST("/signup", handlers.Signup)
	authGroup.POST("/login", handlers.Login)

	// Protected routes
	api.Use(auth.JWTMiddleware)

	keys := api.Group("/keys")
	keys.POST("/api-key", handlers.StoreAPIKey)
	keys.GET("/api-keys", handlers.GetAPIKeys)
	keys.DELETE("/api-key/:id", handlers.DeleteAPIKey)

	// Collection routes
	collections := api.Group("/collections")
	collections.POST("/api-key/rotate", handlers.RotateAPIKey)
	collections.GET("", handlers.GetCollections)
	collections.GET("/user", handlers.GetUserCollections)
	collections.POST("/store", handlers.StoreCollection)
	// Collection details endpoint
	collections.GET("/:id", handlers.GetCollection)
	// Snapshots endpoints
	collections.GET("/:id/snapshots", handlers.GetCollectionSnapshots)
	// Changes endpoints
	collections.GET("/:id/changes", handlers.GetCollectionChanges)
	collections.GET("/compare/:id", handlers.CompareCollections)
	// Job routes
	jobs := api.Group("/jobs")
	jobs.GET("", handlers.GetUserJobs)
	jobs.GET("/:id", handlers.GetJobStatus)
}
